# Chainlist Parser Microservice

API microservice to fetch and check EVM chain RPC endpoints from Chainlist data.

Built with Go using Clean Architecture.

## Features

*   Fetch all chains with their **working** RPCs.
*   Fetch **working** RPCs for a specific chain ID.
*   Uses `fasthttp`, `viper`, `zap`, `go-cache`.
*   Background checking and caching of RPCs.

## API Endpoints

*   `GET /chains`: Get all chains with working RPCs.
*   `GET /chains/{chainId:[0-9]+}/rpcs`: Get working RPCs for a specific chain.

## Setup & Run

**Prerequisites:**
*   Go (version 1.18 or later recommended)
*   Docker (optional, for running via docker-compose)
*   Make (optional, for using Makefile commands)
*   [golangci-lint](https://golangci-lint.run/usage/install/) (optional, for running `make lint`)

**Using Makefile (Recommended):**

The `Makefile` provides convenient targets:
*   `make help`: Show available commands.
*   `make lint`: Run linter.
*   `make generate`: Generate mocks.
*   `make test`: Run tests (includes mock generation).
*   `make build`: Build the binary.
*   `make run`: Run the service locally (requires mocks generated).
*   `make clean`: Remove build artifacts.
*   `make docker-build`: Build the Docker image.
*   `make docker-up`: Start the service in Docker (detached mode).
*   `make docker-down`: Stop the Docker service.

**Running Locally (Manual Steps):**

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/Dorafanboy/chainlist-parser
    cd chainlist-parser
    ```
2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```
3.  **Configure:**
    *   Copy `configs/config.yaml` if needed or set environment variables (Viper will automatically pick them up, e.g., `SERVER_PORT=8081`).
4.  **Run:**
    ```bash
    go run cmd/api/main.go
    ```
    The server will start, typically on port 8080 (or as configured).

**Building the Binary:**
```bash
go build -o build/chainlist-parser cmd/api/main.go
./build/chainlist-parser
```

**Running Tests:**

1.  **Install mocking tool:**
    ```bash
    go install github.com/golang/mock/mockgen@v1.6.0
    ```
2.  **Generate mocks:**
    ```bash
    go generate ./...
    ```
3.  **Run tests:**
    ```bash
    go test ./...
    ```

**Running with Docker Compose:**
```bash
make docker-up
# To stop: make docker-down
```

(Instructions to be added later)
