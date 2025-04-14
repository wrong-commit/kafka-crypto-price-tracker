# MongoDB 

A MongoDB database exists to track all crypto prices and asset purchases and sales over time. 
Database migrations are performed on startup by the `golang-migrate/migrate` package. 
# List of Mongo collections
All collections in the `crypto` database. All prices are in AUD, and all times are Unix Epochs
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
## assets
Collection of crypto assets held by an individual
### Format
```
type AssetDB struct {
    ID    string        `bson:"_id,omitempty"`
    // Crypto Name
    Name  string        `bson:"name"`
    // Crypto Amount
    Amount float32        `bson:"amount"`
    // Price at time crypto was checked
    PurchasePrice float32	    `bson:"purchasePrice"`
    // Time that the crypto was purchased
    PurchaseTime float32     `bson:"purchaseTime"`
    // Whether the crypto is active. Valid values are "held" or "sold"
    Status      string `bson:"status"`
    SalePrice float32 `bson:"salePrice"`
    SaleTime float32 `bson:"saleTime"`
}
```
