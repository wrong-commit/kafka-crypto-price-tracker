package main

import (
    "os"
	"log"

    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/mongodb"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	// Lookup necessary environment variables
    // Example: mongodb://localhost:27017/crypto
	mongoDbUrl, didFind := os.LookupEnv("MONGO_DB_URL")
	if !didFind { 
		panic("No MongoDB DB URL provided")
	}
    // Connect to MongoDB
    m, err := migrate.New(
        "file:///migrations",
        mongoDbUrl,
    )
    if err != nil {
        log.Printf("Could not create migration connection to mongo DB %v\n", err)
        panic(err)
    }

    if err := m.Up(); err != nil && err != migrate.ErrNoChange {
        log.Printf("Could not apply migrations to mongo DB %v\n", err)
        panic(err)
    }

    log.Println("Migrations applied successfully.")
}
