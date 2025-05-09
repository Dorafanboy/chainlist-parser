# Use the official Golang image as a builder image.
# Using alpine variant for smaller size.
# Specify the Go version matching your project.
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container.
WORKDIR /app

# Copy go module files and download dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code.
COPY . .

# Build the Go application.
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /api ./cmd/api/main.go

# Use a minimal base image like alpine for the final stage.
FROM alpine:latest

# Set the working directory.
WORKDIR /app

# Add a non-root user and switch to it.
# Using numeric IDs is generally safer.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Copy the compiled binary from the builder stage.
COPY --from=builder /parserapi /app/parserapi

# Copy the example configuration file into a config directory.
# The actual config might be mounted via volume to /app/config/config.yaml.
COPY config/config.yaml /app/config/config.yaml

# Expose the port the application listens on (matches default in config).
EXPOSE 8080

# Define the command to run the application.
# The config file path is now within the config directory.
CMD ["/app/parserapi", "-config=/app/config/config.yaml"] 