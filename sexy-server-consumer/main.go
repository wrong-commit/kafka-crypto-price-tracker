package main

import (
	"os"
	"fmt"
	"strconv"
	"time"
	"sync"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

var latestMessage string
var latestMessageLock sync.Mutex

func main() {
	
	topic := "test-topic"
	fmt.Printf("Starting Kafka consumer on topic " + topic)
	
	// "localhost:9092"
	kafkaServer, didFind := os.LookupEnv("KAFKA_SERVER")
	if !didFind { 
		panic("No Kafka server provided")
	}
	kafkaPartition, didFind := os.LookupEnv("KAFKA_PARTITION")
	if !didFind { 
		panic("No Kafka partition provided")
	}
	kafkaPartition_i, err := strconv.Atoi(kafkaPartition)
    if err != nil {
        panic("Invalid Kafka partition provided")
    }
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaServer,
		"group.id":          "go-consumer-group",
		"auto.offset.reset": "earliest",
		"enable.auto.commit": true,
	})
	if err != nil {
		panic(err)
	}

	err = c.Subscribe(topic, nil)
	if err != nil {
		panic("Did not subscribe to topic")
	}
	
	topicPartition := kafka.TopicPartition{
		Topic:             	&topic,
		Partition: 			int32(kafkaPartition_i),
		Offset:				-1,
	}

	err = c.Assign([]kafka.TopicPartition{topicPartition})

	if err != nil {
		panic("Did not assign topic partition to consumer")
	}

	run := true
	timeoutMs := 2000
	messageCount := 0
	for run {
		// fmt.Printf("Waiting %dms for new Kafka message@%s\n", timeoutMs, currentDate)
		ev := c.Poll(timeoutMs)
		// fmt.Printf("Event: %#v\n", ev)
		currentDate := time.Now().Format("2006-01-02 15:04:05") // YYYY-MM-DD HH:mm:ss
		switch e := ev.(type) {
		case *kafka.Message:
			fmt.Printf("Received PART:[%d]OFF[%d]: %s @ %s\n", e.TopicPartition.Partition, e.TopicPartition.Offset, string(e.Value), currentDate)
			messageCount++
			run = true // continue processing all messages
		case kafka.Error:
			fmt.Printf("Kafka error: %v\n", e)
			run = false
		default:
			// fmt.Printf("No message received in %dms\n", timeoutMs)
			run = true
		}
	}
	fmt.Printf("Consumed %d messages", messageCount)
}
