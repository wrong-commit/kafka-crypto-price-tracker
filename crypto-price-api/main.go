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
    Time int64     `bson:"time"`
}
// VM for current price response
type CurrentPriceResponseVM struct {
    Name    string 	`json:"name"`
    Price   float32 `json:"price"`
}
// VM for historical prices
type CryptoPriceChangeVM struct {
    Name    string 	`json:"name"`
    Price   float32 `json:"price"`
    Time    int64  	`json:"time"`
}
// Return the current price of a crypto. 
// Returns CurrentPriceResponseVM
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
// Handle to look up price changes within a time period and return a list of all prices or the earliest price
// Returns: []CryptoPriceChangeVM
func priceChangeHandler(w http.ResponseWriter, r *http.Request) {
	// Get coin from query params
	cryptoId := r.URL.Query().Get("cryptoId")
	if cryptoId == "" {
		panic("No crypto ID provided")
	}
	// Get duration from query params
	durationStr := r.URL.Query().Get("duration")
	if durationStr == "" {
		durationStr = "1w"
	}
	// Get if all should be returned from all price, or only the earliest record
	returnAllPrices := false
	allStr := r.URL.Query().Get("all")
	if allStr == "true" {
		returnAllPrices = true
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
	// Filter to load times greater than the duration
    filter := bson.M{
		"name": cryptoId,
		"time": bson.M{
			"$gt": sevenDaysAgo.Unix(),
		},
	}
	// Sort in ascending order to find earliest within provided time period
	opts := options.Find().SetSort(bson.D{{"time", 1}})
	var results []CryptoPriceChangeVM
	// Find all results
    cursor, err := collection.Find(findCtx, filter, opts)
	// Return [] on error
    if err != nil {
        fmt.Println("No crypto prices found:", err)
		json.NewEncoder(w).Encode(results)	
	}
	// Convert DB response to json view model. Either return array with single earliest element, or all array of all elements
	if returnAllPrices { 
		// Return many elements.
		for cursor.Next(findCtx) {
			var cryptoPrice CryptoPriceChangeDB
			if err := cursor.Decode(&cryptoPrice); err != nil {
				fmt.Printf("Could not decode CryptoPriceChangeDB %v", err)
			}else { 
				// fmt.Printf("Found crypto document in `price_changes_over_time`: %s\n", cryptoPrice.Name)
				results = append(results, CryptoPriceChangeVM{ 
					Name: cryptoPrice.Name,
					Price: cryptoPrice.Price,
					Time: cryptoPrice.Time,
				})
			}
		}
	} else { 
		// Return single element
		cursor.Next(findCtx)
		var cryptoPrice CryptoPriceChangeDB
		if err := cursor.Decode(&cryptoPrice); err != nil {
			fmt.Printf("Could not decode CryptoPriceChangeDB %v", err)
		}else { 
			// fmt.Printf("Found crypto document in `price_changes_over_time`: %s\n", cryptoPrice.Name)
			results = append(results, CryptoPriceChangeVM{ 
				Name: cryptoPrice.Name,
				Price: cryptoPrice.Price,
				Time: cryptoPrice.Time,
			})
		}
	}
	// Handle cursor errors
    if err := cursor.Err(); err != nil {
        fmt.Printf("%v\n", err)
	}
	// Return JSON array to user
	json.NewEncoder(w).Encode(results)	
}

// Create a http HandlerFunc to add permissive CORS headers 
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow all origins for simplicity. 
		// FIXME: Customize in production, 
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
	// Create HTTP Server Mux to support CORS middleware
	mux := http.NewServeMux()
	// Assign HTTP routes
	mux.Handle("/prices", withCORS(http.HandlerFunc(currentPriceHandler)))
	mux.Handle("/changes", withCORS(http.HandlerFunc(priceChangeHandler)))
	// Start server 
	fmt.Println("Starting server at :8082")
	err := http.ListenAndServe(":8082", mux)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
