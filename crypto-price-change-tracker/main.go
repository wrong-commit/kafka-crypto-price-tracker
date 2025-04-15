package main

import (
	"context"
	"os"
	"fmt"
	"strconv"
	"time"
	"strings"
	"errors"
	"log"
	
	"crypto-price-change-tracker/metrics" 

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)
// Unused type for message
type Message struct {
    Name    string 
    Price   float32
}
type CryptoPriceDB struct {
    ID    string `bson:"_id,omitempty"`
    Name  string `bson:"name"`
    Price float32	 `bson:"price"`
}
type CryptoPriceChangeDB struct {
    ID    		string `bson:"_id,omitempty"`
    Name  		string `bson:"name"`
    Price  	float32	    `bson:"lastPrice"`
    PriceChange  	float32	    `bson:"priceChange"`
    Time int64     `bson:"time"`
}
func parseKafkaMessage(message string) (*Message, error) { 
	parts := strings.Split(message, ":")
	if len(parts) != 2 {
		return nil, errors.New("Invalid message format")
	}
	price, err := strconv.ParseFloat(parts[1], 32)
	if err != nil {
		return nil, fmt.Errorf("Error converting string to int: %s", parts[1])
	}
	// log.Printf("Prefix: %s, Number: %f\n", parts[0], float32(price))
	return &Message{
		Name: parts[0],
		Price: float32(price),
	}, nil
} 

// Update the database when a message on the crypto price topic is received
// This method 1. updates the `prices` collection with the latest price,
// and 2. updates the `price_changes_over_time` collection with a new 
// entry for the received message. 
func updateDatabase(cryptoId string, price float32, checkedAt int64, client *mongo.Client) error { 
	// 1. Update Price of existing `prices` document
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    collection := client.Database("crypto").Collection("prices")
    // Define a filter to look up a specific crypto
    filter := bson.M{"name": cryptoId}
	// Find previous price of crypto
	var previousPrice CryptoPriceDB 
	err := collection.FindOne(ctx, filter).Decode(&previousPrice)
	if err != nil { 
        log.Println("Initial crypto " + cryptoId + " price not found:", err)
		return err
	}
	// Define the update query to update the price field
	update := bson.M{
		"$set": bson.M{"price": price}, 
	}
	// Update main `price` collection object only if the time since the message is received is less than 1 minute
	updateResult := collection.FindOneAndUpdate(ctx, filter, update)
	if updateResult.Err() != nil {
		log.Printf("Could not find and update: %v\n", updateResult.Err())
		panic(updateResult.Err())
	}
	// 2. Create a new document in the `price_changes_over_time` 
	priceChangeEntry := CryptoPriceChangeDB{
		Name: cryptoId,
		Price: price, 
		// Time is Kafka event time
		Time: checkedAt,
		// 120,000 - 100,000 = 20,000
		PriceChange: price - previousPrice.Price,
	}
	// Insert record into database
	collection = client.Database("crypto").Collection("price_changes_over_time")
	_, err = collection.InsertOne(ctx, &priceChangeEntry)
	if err != nil {
		log.Printf("Insert price change record failed: %v\n", err)
		return err
	}
	return nil
}

// Receive Kafka messages with new Crypto prices and update 2 tables in the MongoDB database.
func main() { 
	// Initialize context for killing application
	_, cancel := context.WithCancel(context.Background())
	// Initialize prometheus metrics and expose on separate port
	metrics.Init(cancel)

	topic := "crypto.price.updated"
	// Lookup necessary environment variables
	// Example: "localhost:9091"
	kafkaServer, didFind := os.LookupEnv("KAFKA_SERVER")
	if !didFind { 
		panic("No Kafka server provided")
	}
	// Example: "mongodb://localhost:27017"
	mongoURL, didFind := os.LookupEnv("MONGO_URL")
	if !didFind { 
		panic("No MongoDB URL provided")
	}
	// Example: 1 or 2
	kafkaPartition, didFind := os.LookupEnv("KAFKA_PARTITION")
	if !didFind { 
		panic("No Kafka partition provided")
	}
	kafkaPartition_i, err := strconv.Atoi(kafkaPartition)
    if err != nil {
        panic("Invalid Kafka partition provided")
    }
	// Connect to MongoDB 
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
    if err != nil {
        panic(err)
    }
    defer client.Disconnect(ctx)
	// Define Kafka Consumer client
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaServer,
		"group.id":          "go-price-change-consumer-group",
		"auto.offset.reset": "earliest",
		"enable.auto.commit": true,
	})
	if err != nil {
		panic(err)
	}
	// Subscribe to topic
	err = c.Subscribe(topic, nil)
	if err != nil {
		panic("Did not subscribe to topic")
	}
	// Assign consumer to topic partition
	topicPartition := kafka.TopicPartition{
		Topic:             	&topic,
		Partition: 			int32(kafkaPartition_i),
		Offset:				-1,
	}
	err = c.Assign([]kafka.TopicPartition{topicPartition})
	if err != nil {
		panic("Did not assign topic partition to consumer")
	}
	// For Each Message, 
	run := true
	timeoutMs := 2000
	for run {
		currentDate := time.Now().Format("2006-01-02 15:04:05") // YYYY-MM-DD HH:mm:ss
		// log.Printf("Waiting %dms for new Kafka message@%s\n", timeoutMs, currentDate)
		ev := c.Poll(timeoutMs)
		switch e := ev.(type) {
		// Process Message
		case *kafka.Message:
			messageProcessingStart := time.Now()
			log.Printf("Received PART:[%d]OFF[%d]: %s @ %s\n", e.TopicPartition.Partition, e.TopicPartition.Offset, string(e.Value), currentDate)
			// Parse message into Message type
			cryptoMessage, err := parseKafkaMessage(string(e.Value))
			if err != nil { 
				metrics.FailedKafkaMessagesCounter.WithLabelValues().Inc()
				panic(err)
			}else{
				// Update current price and insert price at time
				err = updateDatabase(cryptoMessage.Name, cryptoMessage.Price, e.Timestamp.Unix(), client)
				if err != nil { 
					metrics.FailedKafkaMessagesCounter.WithLabelValues().Inc()
					log.Printf("Error updating crypto prices, %v", err)
				} else { 
					log.Printf("Updated price of crypto \"%s\" to \"%f\"\n", cryptoMessage.Name, cryptoMessage.Price)
					metrics.MessagesConsumedCounter.WithLabelValues(cryptoMessage.Name).Inc()
				}
			}
			metrics.PriceChangeMessageDuration.Observe(time.Since(messageProcessingStart).Seconds())
			run = true // continue processing messages
		// Handle Error
		case kafka.Error:
			log.Printf("Kafka error: %v\n", e)
			run = false
		// No message received, loop
		default:
			// log.Printf("No message received in %dms\n", timeoutMs)
			run = true
		}
	}
}
