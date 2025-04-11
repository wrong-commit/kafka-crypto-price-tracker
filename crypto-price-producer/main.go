package main

import (
	"context"
	"os"
	"fmt"
	"time"

	"net/http"
	"encoding/json"
	"strconv"
	
	"math/rand"
	"math"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)
const (
    coinbaseCryptoAPI   = "https://api.coinbase.com/v2/prices/%s-%s/spot"
	currency            = "AUD"
	retryDelay          = 30 // Delay between checking a tokens price
	MB_OK               = 0x00000000
	MB_ICONINFORMATION  = 0x00000040
	MB_SYSTEMMODAL      = 0x00001000
	WM_SETFOCUS         = 0x0007
	WM_ACTIVATEAPP      = 0x001C
)
// Coinbase API response
type coinbasePriceResponse struct {
    Data struct {
        Amount string `json:"amount"`
    } `json:"data"`
}
// `prices` collection document structure
type CryptoPriceDB struct {
    ID    string `bson:"_id,omitempty"`
    Name  string `bson:"name"`
    Price float32	 `bson:"price"`
}

func lookupOldCryptoPrice(cryptoId string, client *mongo.Client) (float32, error) { 
	// 5s timeout for MongoDB lookup
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

	// Choose database and collection
    collection := client.Database("crypto").Collection("prices")

    // Define a filter to look up a specific user
    filter := bson.M{"name": cryptoId}

	var result CryptoPriceDB
    err := collection.FindOne(ctx, filter).Decode(&result)
    if err != nil {
        fmt.Println("Crypto " + cryptoId + " not found:", err)
		return -1, err
    }
	fmt.Printf("Found crypto: %+v\n", result)
	return result.Price, nil 
}

func lookupNewCryptoPriceGarbage(cryptoId string) float32 { 
	rand.Seed(time.Now().UnixNano())
	// Generate a random integer between 0 and 99
	randomNumber := rand.Intn(1000)
	increaseOrDecrease := rand.Intn(2)
	modifier := -1
	if increaseOrDecrease > 1 { 
		modifier = 1
	} 
	basePrice := float32(100420.69)
	return float32(modifier) * float32(randomNumber) + basePrice
}

func lookupNewCryptoPrice(cryptoId string) float32 { 
	price, err := getCoinbasePrice(cryptoId)
	if err != nil {
		fmt.Printf("Error retrieving %s price: %v\n", cryptoId, err)
		return -1
	}
	// Round to lowest two decimal places
	roundedPrice := math.Floor(price*100)/100
	return float32(roundedPrice)
}

func getCoinbasePrice(symbol string,) (float64, error) {
    url := fmt.Sprintf(coinbaseCryptoAPI, symbol, currency)
    resp, err := http.Get(url)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()

    var priceResp coinbasePriceResponse
    err = json.NewDecoder(resp.Body).Decode(&priceResp)
    if err != nil {
        return 0, err
    }

    price, err := strconv.ParseFloat(priceResp.Data.Amount, 64)
    if err != nil {
        return 0, err
    }

    return price, nil
}

func main() {
	topic := "crypto.price.updated"
	// Lookup necessary environment variables
	// Example: "localhost:9091"
	kafkaServer, didFind := os.LookupEnv("KAFKA_SERVER")
	if !didFind { 
		panic("No Kafka server provided")
	}
	// Example: "BTC"
	cryptoId, didFind := os.LookupEnv("CRYPTO_ID")
	if !didFind { 
		panic("No crypto ID provided")
	}
	// Example: "mongodb://localhost:27017"
	mongoURL, didFind := os.LookupEnv("MONGO_URL")
	if !didFind { 
		panic("No MongoDB URL provided")
	}
	fmt.Printf("STARTUP: Tracking [%s] prices every 5s", cryptoId)
	// Connect to MongoDB 
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
    if err != nil {
        panic(err)
    }
    defer client.Disconnect(ctx)
	// Create Kafka Producer client
	p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": kafkaServer})
	if err != nil {
		panic(err)
	}
	defer p.Close()
	// Every 1 minute get crypto price
	messageCount := 0
	for { 
		// fmt.Println("Checking price for " + cryptoId)
		// Check crypto price from Coinbase
		cryptoPrice := lookupNewCryptoPrice(cryptoId)
		fmt.Printf("Fetched new crypto price: %f\n", cryptoPrice)
		if cryptoPrice == -1 { 
			fmt.Printf(fmt.Sprintf("Not updating crypto price for %f because of failed API lookup", cryptoPrice))
			// Sleep then restart loop
			time.Sleep(5*time.Second)
			continue
		}

		// Lookup previous price from DB
		_, err := lookupOldCryptoPrice(cryptoId, client)
		// err != nil means we are tracking the crypto for the first time!
		if err != nil { 
			fmt.Println("Creating new `prices` document for " + cryptoId)
			createNewPrice := CryptoPriceDB{
				Name: cryptoId,
				Price: cryptoPrice,
			}
			// Insert record with current price
			collection := client.Database("crypto").Collection("prices")
			_, err = collection.InsertOne(ctx, &createNewPrice)
			if err != nil {
				fmt.Printf("Insert new `prices` document failed: %v\n", err)
			}else { 
				fmt.Printf("Created new crypto success for %s\n", cryptoId)
			}
		}
		// Publish Message
		err = p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          []byte(fmt.Sprintf("%s:%f", cryptoId, cryptoPrice)),
		}, nil)
		if err != nil {
			fmt.Printf("Could not produce Kafka message: %v\n", err)
		} else { 
			messageCount++
		}
		// Wait for all messages to be delivered
		p.Flush(2000)
		// Sleep
		time.Sleep(5*time.Second)
	}
	fmt.Printf("Produced %d messages in microservice lifetime", messageCount)
}
