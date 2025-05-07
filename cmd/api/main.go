package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chainlist-parser/internal/adapter/handler/http"
	"chainlist-parser/internal/adapter/repository"
	"chainlist-parser/internal/config"
	parserLogger "chainlist-parser/internal/logger"
	"chainlist-parser/internal/usecase"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

const (
	configFilePath = "config/config.yaml"
)

// main is the entry point of the application.
func main() {
	cfg, err := config.LoadFromYAML(configFilePath)
	if err != nil {
		log.Fatalf("Failed to load configuration from %s: %v", configFilePath, err)
	}

	logger, err := parserLogger.NewLogger(cfg.Logger)
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {
			log.Fatalf("Failed to sync logger: %v", err)
		}
	}(logger)
	logger.Info("Logger initialized", zap.Any("config", cfg.Logger))

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info("Initializing dependencies...")

	chainlistRepo := repository.NewChainlistRepo(cfg.Chainlist, logger)
	cacheRepo := repository.NewGoCacheRepo(*cfg, logger)
	rpcChecker := repository.NewRPCChecker(logger)

	chainUseCase := usecase.NewChainUseCase(rootCtx, chainlistRepo, cacheRepo, rpcChecker, logger, *cfg)
	chainHandler := http.NewChainHandler(chainUseCase, logger)

	logger.Info("Setting up HTTP router...")
	r := router.New()

	r.GET("/chains", chainHandler.GetAllChains)
	r.GET("/chains/{chainId:[0-9]+}/rpcs", chainHandler.GetChainRPCs)

	r.GET("/health", func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBodyString("OK")
	})

	loggingMiddleware := func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			logger.Info(
				"Request received",
				zap.ByteString("method", ctx.Method()),
				zap.ByteString("uri", ctx.RequestURI()))
			next(ctx)
		}
	}

	serverAddr := ":" + cfg.Server.Port
	server := &fasthttp.Server{
		Handler: loggingMiddleware(r.Handler),
	}

	go func() {
		logger.Info("Starting HTTP server", zap.String("address", serverAddr))
		if err := server.ListenAndServe(serverAddr); err != nil && !errors.Is(err, fasthttp.ErrConnectionClosed) {
			logger.Fatal("HTTP server ListenAndServe error", zap.Error(err))
		}
	}()

	gracefulShutdown(server, cancel, logger)

	time.Sleep(1 * time.Second)
	logger.Info("Application shut down finished.")
}

// gracefulShutdown waits for termination signals and performs graceful shutdown.
func gracefulShutdown(server *fasthttp.Server, cancel context.CancelFunc, logger *zap.Logger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	cancel()
	logger.Info("Signaling background tasks to stop (context cancelled)...")

	logger.Info("Shutting down HTTP server...")
	if err := server.Shutdown(); err != nil {
		logger.Error("HTTP server shutdown failed", zap.Error(err))
	}

	logger.Info("HTTP server shut down complete.")
}
