# Stage 1: Build the application
FROM golang:1.21-alpine AS builder

# Set necessary environment variables
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# Using -ldflags "-s -w" to make the binary smaller
RUN go build -ldflags="-s -w" -o /bin/chainlist-parser ./cmd/api/main.go

# Stage 2: Create the final lightweight image
FROM alpine:latest

# Install root certificates
RUN apk update && apk add --no-cache ca-certificates tzdata

# Copy the configuration file
COPY --from=builder /app/configs/ /app/configs/

# Copy the built binary from the builder stage
COPY --from=builder /bin/chainlist-parser /bin/chainlist-parser

# Set the working directory
WORKDIR /app

# Expose the port the app runs on (from config.yaml, default 8080)
# This should match the server.port in config.yaml
EXPOSE 8080

# Command to run the application
# The config path is relative to the WORKDIR
ENTRYPOINT ["/bin/chainlist-parser"]

# Optionally, specify default command arguments if needed
# CMD ["--config", "configs/config.yaml"] # Example if flags were used 