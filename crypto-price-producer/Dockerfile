# Start from the official Go image
FROM golang:1.21 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum and download dependencies
COPY go.mod go.sum ./
RUN go mod download
RUN go mod tidy 

# Copy source code
COPY . .

# Build the Go app (output binary named 'app')
RUN go build -o app .

# Create a minimal runtime image
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y libc6 ca-certificates
# Set working directory
WORKDIR /app

# Copy the binary from the builder
COPY --from=builder /app/app .

# Expose port if needed (e.g., for HTTP server)
EXPOSE 8080

# Run the binary
CMD ["./app"]