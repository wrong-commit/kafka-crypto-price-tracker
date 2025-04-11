# List of Mongo collections
All collections in the `crypto` database
## prices
Collection of current Crypto prices
### Format
```
type CryptoPriceDB struct {
    ID    string        `bson:"_id,omitempty"`
    Name  string        `bson:"name"`
    Price float32	        `bson:"price"`
}
```
## price_changes_over_time
Collection of changes in Crypto prices. 
### Format
```
type PriceChangeDB struct {
    ID    string        `bson:"_id,omitempty"`
    // Crypto Name
    Name  string        `bson:"name"`
    // Price at time crypto was checked
    Price float32	    `bson:"lastPrice"`
    // Time that the crypto check occurred
    Time float32     `bson:"time"`
    // Increase/decrease since the last check
    PriceChange float32 `bson:"priceChange"`
}
```