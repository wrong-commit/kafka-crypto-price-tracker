# Crypto Price Tracker

Crypto Coin price tracker and Profit and Loss Tracker. Built with GoLang on Kafka and MongoDB.  

## Features 
- golang microservice for updating the database schema
- golang microservice for publishing Crypto price messages
- golang microservices for consuming crypto price changes messages and updating the database
- golang API for serving front end requests
- simple HTML front end for tracking profit and loss
- custom Prometheus metrics for all microservices

## Screenshot
<!-- ![Alt text](/screenshots/alpha.png "AlpBha Version") -->
![Alt text](/screenshots/beta-7.png "Beta Version")

## How TOs

### Startup
Run `docker compose build && docker compose up` to start MongoDB, Kafka, Golang consumer and producers, and Kafka monitoring tools

### Access the main Web UI
Access at `http://localhost:8083/index.html` after starting Docker containers

### API
Access at `http://localhost:8082` after starting Docker containers

### Grafana for monitoring 
Access at `http://localhost:3000/` after starting Docker containers. Log in with the credentials in /volumes/config.ini.  
Custom metrics are published and available in the "Micro Service Metrics" dashboard

![Alt text](/screenshots/grafana/microservice-metrics-1.png "Custom MS metrics")

### Kafka UI   
Access at `http://localhost:8080` after starting Docker containers 

### Mongo Express UI 
Access at `http://localhost:8081` after starting Docker containers 

<!-- ### Kubernetes  -->
<!-- An example k8s application exists in the `k8s-examples` folder which starts several load balanced echo servers.  -->
