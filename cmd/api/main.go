package main

import (
	"log"
	"os"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"chainlist-parser/internal/adapter/handler/http"
	"chainlist-parser/internal/adapter/repository"
	"chainlist-parser/internal/config"
	"chainlist-parser/internal/usecase"
)

func main() {
	// --- Configuration ---
	cfgPath := "configs"
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load configuration from %s: %v", cfgPath, err)
	}

	// --- Logger ---
	logger, err := setupLogger(cfg.Logger)
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	defer logger.Sync() // Ensure logs are flushed before exiting
	logger.Info("Logger initialized", zap.Any("config", cfg.Logger))

	// --- Dependency Injection (Manual) ---
	logger.Info("Initializing dependencies...")

	// Repositories & Checker
	chainlistRepo := repository.NewChainlistRepo(cfg.Chainlist, logger)
	cacheRepo := repository.NewGoCacheRepo(*cfg, logger)
	rpcChecker := repository.NewRPCChecker(logger)

	// Use Cases
	chainUseCase := usecase.NewChainUseCase(chainlistRepo, cacheRepo, rpcChecker, logger, *cfg)

	// Handlers
	chainHandler := http.NewChainHandler(chainUseCase, logger)

	// --- HTTP Router & Server ---
	logger.Info("Setting up HTTP router...")
	r := router.New()

	// API Routes
	r.GET("/chains", chainHandler.GetAllChains)
	r.GET("/chains/{chainId:[0-9]+}/rpcs", chainHandler.GetChainRPCs)

	// Health Check (optional but recommended)
	r.GET("/health", func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBodyString("OK")
	})

	// Middleware (example: logging)
	loggingMiddleware := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			logger.Info("Request received",
				zap.ByteString("method", ctx.Method()),
				zap.ByteString("uri", ctx.RequestURI()))
			next(ctx)
		}
	}

	// Correctly format the server address string
	serverAddr := ":" + cfg.Server.Port
	logger.Info("Starting HTTP server", zap.String("address", serverAddr))

	// Start server with logging middleware
	if err := fasthttp.ListenAndServe(serverAddr, loggingMiddleware(r.Handler)); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}

// setupLogger initializes the zap logger based on config.
func setupLogger(cfg config.LoggerConfig) (*zap.Logger, error) {
	logLevel := zap.NewAtomicLevel()
	if err := logLevel.UnmarshalText([]byte(cfg.Level)); err != nil {
		logLevel.SetLevel(zap.InfoLevel) // Default to info if parsing fails
		log.Printf("Warning: Failed to parse log level '%s', defaulting to 'info'. Error: %v\n", cfg.Level, err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if cfg.Encoding == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg) // Default to JSON
	}

	logger := zap.New(zapcore.NewCore(
		encoder,
		zapcore.Lock(os.Stdout),
		logLevel,
	), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}
