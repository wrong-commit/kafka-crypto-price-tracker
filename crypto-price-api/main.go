package main

import (
	"context"
	"os"
	"fmt"
	"strconv"
	"net/http"
	"time"
	"encoding/json"
	"log"

	"crypto-price-api/metrics"

    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
    "go.mongodb.org/mongo-driver/bson/primitive"
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
// `assets`
type AssetDB struct {
    ID    string        `bson:"_id,omitempty"`
    // Crypto Name
    Name  string        `bson:"name"`
    // Crypto Amount
    Amount  float32        `bson:"amount"`
    // Price at time crypto was checked
    PurchasePrice float32	    `bson:"purchasePrice"`
    // Time that the crypto was purchased
    PurchaseTime float32     `bson:"purchaseTime"`
    // Whether the crypto is active. Valid values are "held" or "sold"
    Status      string `bson:"status"`
    SalePrice float32 `bson:"salePrice"`
    SaleTime float32 `bson:"saleTime"`
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
// VM for assets 
type AssetVM struct {
    AssetId  string        `json:"_id"`
    Name  string        `json:"name"`
    Amount  float32        `json:"amount"`
    PurchasePrice float32	    `json:"purchasePrice"`
    PurchaseTime float32     `json:"purchaseTime"`
    // "held" or "sold"
    Status      string `json:"status"`
    SalePrice float32 `json:"salePrice"`
    SaleTime float32 `json:"saleTime"`
}
// Return the current price of a crypto. 
// Returns CurrentPriceResponseVM
func currentPriceHandler(w http.ResponseWriter, r *http.Request) {
	requestStartTime := time.Now()
	metrics.HTTPRequestCounter.WithLabelValues("/assets").Inc()
	// Connect to MongoDB 
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(MongoUrl))
    if err != nil {
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
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
        log.Println("No crypto prices found:", err)
		fmt.Fprintf(w, "[]")
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
		return
    }
	// Convert DB response to json view model
    for cursor.Next(findCtx) {
        var cryptoPrice CryptoPriceDB
        if err := cursor.Decode(&cryptoPrice); err != nil {
            fmt.Printf("%v", err)
        }else { 
			// fmt.Printf("Found crypto document in `prices`: %s\n", cryptoPrice.Name)
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
	metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
}
// Handle to look up price changes within a time period and return a list of all prices or the earliest price
// Returns: []CryptoPriceChangeVM
func priceChangeHandler(w http.ResponseWriter, r *http.Request) {
	requestStartTime := time.Now()
	metrics.HTTPRequestCounter.WithLabelValues("/assets").Inc()
	// Get coin from query params
	cryptoId := r.URL.Query().Get("cryptoId")
	if cryptoId == "" {
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
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
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
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
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
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
        log.Println("No crypto prices found:", err)
		json.NewEncoder(w).Encode(results)	
		return
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
	metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
}
// Handle to look up assets sorted by purchase time
// GET /assets
// Returns: []AssetVM
func findAssetsHandler(w http.ResponseWriter, r *http.Request) { 
	requestStartTime := time.Now()
	metrics.HTTPRequestCounter.WithLabelValues("/assets").Inc()
	// Get coin from query params
	// cryptoId := r.URL.Query().Get("cryptoId")
	// if cryptoId == "" {
	// 	panic("No crypto ID provided")
	// }

	// Connect to MongoDB 
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(MongoUrl))
    if err != nil {
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
        panic(err)
    }
    defer client.Disconnect(connectCtx)

	// Timeout for lookup
	findCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
	// Set content type to application/json
    w.Header().Set("Content-Type", "application/json")
	// Lookup all prices from `prices` MongoDB
    collection := client.Database("crypto").Collection("assets")

	// Filter to load times greater than the duration
    filter := bson.M{
		// "name": cryptoId,
	}
	// Sort in ascending order of purchase time
	opts := options.Find().SetSort(bson.D{{"purchaseTime", -1}})
	var results []AssetVM
	// Find all results
    cursor, err := collection.Find(findCtx, filter, opts)
	// Return [] on error
    if err != nil {
        log.Println("Error searching for assets:", err)
		json.NewEncoder(w).Encode(results)	
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
		return
	}
	// Return many elements.
	for cursor.Next(findCtx) {
		var cryptoAsset AssetDB
		if err := cursor.Decode(&cryptoAsset); err != nil {
			fmt.Printf("Could not decode AssetDB %v", err)
		}else { 
			// fmt.Printf("Found asset document in `assets`: %s\n", cryptoAsset.Name)
			results = append(results, AssetVM{ 
				AssetId: cryptoAsset.ID,
				Name: cryptoAsset.Name,
				Amount: cryptoAsset.Amount,
				PurchasePrice: cryptoAsset.PurchasePrice,
				PurchaseTime: cryptoAsset.PurchaseTime,
				Status: cryptoAsset.Status,
				SalePrice: cryptoAsset.SalePrice,
				SaleTime: cryptoAsset.SaleTime,
			})
		}
	}
	// Return asssets
	json.NewEncoder(w).Encode(results)	
	metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
}
// Create a new asset based on provided form data. 
// Required fields are:
//		cryptoId string
//		amount float32
//		purchaseTime float32
//		purchasePrice float32
// 		
// POST /assets
func createAssetHandler(w http.ResponseWriter, r *http.Request) { 
	requestStartTime := time.Now()
	metrics.HTTPRequestCounter.WithLabelValues("/assets").Inc()

	// Limit request size to 10MB (more than enough!)
	r.ParseMultipartForm(10 << 20) // 10MB
	
	// Access regular form fields
	cryptoId := r.FormValue("cryptoId")
	amount := r.FormValue("amount")
	purchaseTime := r.FormValue("purchaseTime")
	purchasePrice := r.FormValue("purchasePrice")
	// Convert strings to float32s or ints
	amountF32, err := strconv.ParseFloat(amount, 32)
	if err != nil {
        fmt.Printf("Error converting string to int:", err)
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
		return
	}
	purchaseTimeF32, err := strconv.ParseFloat(purchaseTime, 32)
	if err != nil {
        fmt.Printf("Error converting string to int:", err)
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
		return
	}
	// Specially handle purchaseTimeF32, incase its been incorrectly sent by the FE JS 
	if purchaseTimeF32 >= 1712893600000 {
		purchaseTimeF32 = purchaseTimeF32 / 1000
	}
	purchasePriceF32, err := strconv.ParseFloat(purchasePrice, 32)
	if err != nil {
        fmt.Printf("Error converting string to int:", err)
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
		return
	}
	// Connect to MongoDB 
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(MongoUrl))
    if err != nil {
		metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
		panic(err)
    }
    defer client.Disconnect(connectCtx)
	// Create new asset
	// log.Println("Creating new `assets` document for " + cryptoId)
	newAsset := AssetDB{
		Name: cryptoId,
		Amount: float32(amountF32),
		PurchasePrice: float32(purchasePriceF32),
		PurchaseTime: float32(purchaseTimeF32),
		Status: 	"held",
		SalePrice: -1,
		SaleTime: -1,
	}
	// Insert asset
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	collection := client.Database("crypto").Collection("assets")
	_, err = collection.InsertOne(ctx, &newAsset)
	if err != nil {
		fmt.Printf("Insert new `assets` document failed: %v\n", err)
	}else { 
		fmt.Printf("Created new asset [%s] at $%f AUD\n", newAsset.Name, newAsset.PurchasePrice)
	}
	metrics.HTTPRequestDuration.Observe(time.Since(requestStartTime).Seconds())
}
// Handler to mark asset as sold
func sellAssetHandler(w http.ResponseWriter, r *http.Request) {
	// Mark existing asset as sold
	assetIdStr := r.URL.Query().Get("assetId")
	if assetIdStr == "" {
		panic("No asset ID provided")
	}
	// Connect to MongoDB 
	connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(MongoUrl))
    if err != nil {
        panic(err)
    }
    defer client.Disconnect(connectCtx)
	// Lookup Asset
	assetCollection := client.Database("crypto").Collection("assets")
	// Define a filter to look up a specific crypto
	assetId, err := primitive.ObjectIDFromHex(assetIdStr) 
	if err != nil {
		panic(err)
	}
	assetFilter := bson.M{"_id": assetId}
	// Find the asset to sell
	findAssetCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var assetResult AssetDB 
	err = assetCollection.FindOne(findAssetCtx, assetFilter).Decode(&assetResult)
	// Look up current prices of crypto from database
	priceCollection := client.Database("crypto").Collection("prices")
	priceFilter := bson.M{"name": assetResult.Name}
	findPriceCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var currentPrice CryptoPriceDB
	err = priceCollection.FindOne(findPriceCtx, priceFilter).Decode(&currentPrice)
	if err != nil {
		panic(err)
	}
	// Define the update query to mark the asset as sold
	saleTime := time.Now().Unix()
	updateAssetFilter := bson.M{
		"$set": bson.M{
			"status": "sold",
			"salePrice": currentPrice.Price,
			"saleTime": float32(saleTime),
		}, 
	}
	findAndUpdateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Update main `assets` collection 
	_, err = assetCollection.UpdateOne(findAndUpdateCtx, assetFilter, updateAssetFilter)
	if err != nil {
		fmt.Printf("Could not find and update: %v\n", err)
		panic(err)
	}
	fmt.Printf("Sold [%f] of [%s] at %f\n", assetResult.Amount, assetResult.Name, saleTime)
	return
}
// Handler for /assets route
func assetHandler(w http.ResponseWriter, r *http.Request) { 
	switch r.Method {
	case http.MethodGet:
		// fmt.Fprintln(w, "This is a GET request")
		findAssetsHandler(w, r)
	case http.MethodPost:
		// fmt.Fprintln(w, "This is a POST request")
		createAssetHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
	// Initialize context for killing application
	_, cancel := context.WithCancel(context.Background())
	// Initialize prometheus metrics and expose on separate port
	metrics.Init(cancel)
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
	mux.Handle("/assets", withCORS(http.HandlerFunc(assetHandler)))
	mux.Handle("/assets/sell", withCORS(http.HandlerFunc(sellAssetHandler)))
	// Start server 
	log.Println("Starting server at :8082")
	err := http.ListenAndServe(":8082", mux)
	if err != nil {
		log.Println("Error starting server:", err)
	}
}
