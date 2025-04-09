# Kafka and K8S First Project

Simple Crypto Coin price tracker. Built with GoLang on Kafka and MongoDB. 
Multiple consumers read from two partitions on a single topic. 

## Example Screenshot
![Alt text](/screenshots/alpha.png "Alpha Version")

# Startup
Run `docker compose build && docker compose up` to start MongoDB, Kafka, Golang consumer and producers, and Kafka monitoring tools

# Crypto Tracker Page
Access at `http://localhost:8083/index.html` after starting Docker containers

# Crypto Tracker API
Access at `http://localhost:8082` after starting Docker containers

# Grafana
Access at `http://localhost:3000/` after starting Docker containers

# Kafka UI 
Access at `http://localhost:8080` after starting Docker containers 

# Mongo UI
Access at `http://localhost:8081` after starting Docker containers 

# Kubernetes
An example k8s application exists in the `autoscaled-project` folder which runs an echo server with multiple echo servers. 

