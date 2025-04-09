package main

import (
	"context"
	"os"
	"fmt"
	"net/http"
	"time"
	"encoding/json"

    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

var MongoUrl string

// `prices` collection document structure
type CryptoPriceDB struct {
    ID    string `bson:"_id,omitempty"`
    Name  string `bson:"name"`
    Price float32	 `bson:"price"`
}
// `prices_change_over_time`
type CryptoPriceChangeDB struct {
    ID    		string `bson:"_id,omitempty"`
    Name  		string `bson:"name"`
    Price  	float32	    `bson:"lastPrice"`
    PriceChange  	float32	    `bson:"priceChange"`
    LastChecked int64     `bson:"lastChecked"`
}
// Type for current price response
type CurrentPriceResponseVM struct {
    Name    string 	`json:"name"`
    Price   float32 `json:"price"`
}
func currentPriceHandler(w http.ResponseWriter, r *http.Request) {
	// Connect to MongoDB 
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(MongoUrl))
    if err != nil {
        panic(err)
    }
    defer client.Disconnect(connectCtx)

	// Timeout for lookup
	findCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
	// Set content type to application/json
    w.Header().Set("Content-Type", "application/json")
	// Lookup all prices from `prices` MongoDB
    collection := client.Database("crypto").Collection("prices")
	var results []CurrentPriceResponseVM
    cursor, err := collection.Find(findCtx, bson.M{})
    if err != nil {
        fmt.Println("No crypto prices found:", err)
		fmt.Fprintf(w, "[]")
    }
	// Convert DB response to json view model
    for cursor.Next(findCtx) {
        var cryptoPrice CryptoPriceDB
        if err := cursor.Decode(&cryptoPrice); err != nil {
            fmt.Printf("%v", err)
        }else { 
			fmt.Printf("Found crypto document in `prices`: %s\n", cryptoPrice.Name)
			results = append(results, CurrentPriceResponseVM{ 
				Name: cryptoPrice.Name,
				Price: cryptoPrice.Price,
			})
		}
	}
    if err := cursor.Err(); err != nil {
        fmt.Printf("%v\n", err)
    } else { 
		// Encode the struct to JSON and write to response
		json.NewEncoder(w).Encode(results)	
	}
}
type PriceChangeResponseVM struct {
    Name    string 
    Change  float32
	Period  string
}
func priceChangeHandler(w http.ResponseWriter, r *http.Request) {
	// Get coin
	cryptoId := r.URL.Query().Get("cryptoId")
	if cryptoId == "" {
		panic("No crypto ID provided")
	}
	durationStr := r.URL.Query().Get("duration")
	if durationStr == "" {
		durationStr = "1w"
	}

	// Connect to MongoDB 
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(MongoUrl))
    if err != nil {
        panic(err)
    }
    defer client.Disconnect(connectCtx)

	// Timeout for lookup
	findCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
	// Set content type to application/json
    w.Header().Set("Content-Type", "application/json")
	// Lookup all prices from `prices` MongoDB
    collection := client.Database("crypto").Collection("price_changes_over_time")

	// Calculate change in price from queried period
	duration, err := time.ParseDuration(durationStr)
    if err != nil {
		panic(err)
    }
	sevenDaysAgo := time.Now().Add(-duration)
    filter := bson.M{
		"name": cryptoId,
		"lastChecked": bson.M{
			"$gt": sevenDaysAgo.Unix(),
		},
	}
	// Find the most recent
	opts := options.FindOne().SetSort(bson.D{{"lastChecked", 1}})
	var result CryptoPriceChangeDB
    err = collection.FindOne(findCtx, filter, opts).Decode(&result)
    if err != nil {
        fmt.Println("No crypto prices found:", err)
		fmt.Fprintf(w, "[]")
    }else{
		json.NewEncoder(w).Encode(result)	
	}
	// FIXME: do not loop over results twice, too slow!
	// Convert DB response to json view model
    // for cursor.Next(findCtx) {
    //     var cryptoPrice CryptoPriceChangeDB
    //     if err := cursor.Decode(&cryptoPrice); err != nil {
    //         fmt.Printf("%v", err)
    //     }else { 
	// 		fmt.Printf("Found crypto document in `price_changes_over_time`: %s\n", cryptoPrice.Name)
	// 		results = append(results, cryptoPrice)
	// 		// if earliestCryptoPrice == nil {
	// 		// 	earliestCryptoPrice = cryptoPrice
	// 		// }
	// 	}
	// }
    // if err := cursor.Err(); err != nil {
    //     fmt.Printf("%v\n", err)
    // } else { 
		// Encode the struct to JSON and write to response
	// }
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow all origins for simplicity. Customize in production.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Lookup necessary environment variables
	// Example: "mongodb://localhost:27017"
	didFind := false
	MongoUrl, didFind = os.LookupEnv("MONGO_URL")
	if !didFind { 
		panic("No MongoDB URL provided")
	}

	mux := http.NewServeMux()

	mux.Handle("/prices", withCORS(http.HandlerFunc(currentPriceHandler)))
	mux.Handle("/changes", withCORS(http.HandlerFunc(priceChangeHandler)))

	fmt.Println("Starting server at :8082")
	err := http.ListenAndServe(":8082", mux)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
