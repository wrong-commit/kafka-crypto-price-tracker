package main

import (
	"context"
	"os"
	"fmt"
	"strconv"
	"time"
	"strings"
	"errors"
	
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
    LastChecked int64     `bson:"lastChecked"`
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
	// fmt.Printf("Prefix: %s, Number: %f\n", parts[0], float32(price))
	return &Message{
		Name: parts[0],
		Price: float32(price),
	}, nil
} 

func updateDatabase(cryptoId string, price float32, lastChecked int64, client *mongo.Client) error { 
	// 1. Update Price of existing object
	// 5s timeout for MongoDB lookup
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
	// Choose database and collection
    collection := client.Database("crypto").Collection("prices")
    // Define a filter to look up a specific crypto
    filter := bson.M{"name": cryptoId}
	// Find previous price of crypto
	var previousPrice CryptoPriceDB 
	err := collection.FindOne(ctx, filter).Decode(&previousPrice)
	if err != nil { 
        fmt.Println("Initial crypto " + cryptoId + " price not found:", err)
		return err
	}
	// Define the update query
	update := bson.M{
		"$set": bson.M{"price": price}, // this will update the age field
	}
	// Update main `price` collection object
	updateResult := collection.FindOneAndUpdate(ctx, filter, update)
	if updateResult.Err() != nil {
		fmt.Printf("Could not find and update: %v\n", updateResult.Err())
		panic(updateResult.Err())
	}
	// 2. Create PriceChange entry 
	priceChangeEntry := CryptoPriceChangeDB{
		Name: cryptoId,
		Price: price, 
		// Time is Kafka event time
		LastChecked: lastChecked,
		// Increase since the last check
		PriceChange: previousPrice.Price - price,
	}
	// Insert record
	collection = client.Database("crypto").Collection("price_changes_over_time")
	_, err = collection.InsertOne(ctx, &priceChangeEntry)
	if err != nil {
		fmt.Printf("Insert price change record failed: %v\n", err)
		return err
	}
	return nil
}

func main() { 
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
	messageCount := 0
	for run {
		currentDate := time.Now().Format("2006-01-02 15:04:05") // YYYY-MM-DD HH:mm:ss
		// fmt.Printf("Waiting %dms for new Kafka message@%s\n", timeoutMs, currentDate)
		ev := c.Poll(timeoutMs)
		switch e := ev.(type) {
		// Process Message
		case *kafka.Message:
			fmt.Printf("Received PART:[%d]OFF[%d]: %s @ %s\n", e.TopicPartition.Partition, e.TopicPartition.Offset, string(e.Value), currentDate)
			messageCount++
			// Parse message
			cryptoMessage, err := parseKafkaMessage(string(e.Value))
			if err != nil { 
				panic(err)
			}
			// Update database
			err = updateDatabase(cryptoMessage.Name, cryptoMessage.Price, e.Timestamp.Unix(), client)
			if err != nil { 
				fmt.Printf("Error updating crypto prices, %v", err)
			} else { 
				fmt.Printf("Updated price of crypto \"%s\" to \"%f\"\n", cryptoMessage.Name, cryptoMessage.Price)
			}
			run = true // continue processing all messages
		// Handle Error
		case kafka.Error:
			fmt.Printf("Kafka error: %v\n", e)
			run = false
		// No message received, loop
		default:
			// fmt.Printf("No message received in %dms\n", timeoutMs)
			run = true
		}
	}
	fmt.Printf("Consumed %d messages", messageCount)
}
