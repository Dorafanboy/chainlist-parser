package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	appDeliveryHttp "chainlist-parser/internal/adapter/delivery/http"
	appRpc "chainlist-parser/internal/adapter/rpc"
	appStorageChainlist "chainlist-parser/internal/adapter/storage/chainlist"
	appStorageMemory "chainlist-parser/internal/adapter/storage/memory"
	appService "chainlist-parser/internal/application"
	"chainlist-parser/internal/config"
	parserLogger "chainlist-parser/internal/logger"

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
		log.Fatalf("Failed to load configuration: %v", err)
	}

	appLogger, err := parserLogger.NewLogger(cfg.Logger)
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	defer func(l *zap.Logger) {
		if syncErr := l.Sync(); syncErr != nil {
			log.Printf("Error syncing logger: %v\\n", syncErr)
		}
	}(appLogger)

	appLogger.Info("Logger initialized", zap.Any("loggerConfig", cfg.Logger))

	rootCtx, rootCancel := context.WithCancel(context.Background())

	appLogger.Info("Initializing dependencies...")
	chainlistStorage := appStorageChainlist.NewRepository(cfg.Chainlist, appLogger)
	memoryStorage := appStorageMemory.NewCacheRepository(*cfg, appLogger)
	rpcAdapter := appRpc.NewChecker(appLogger)
	chainApplicationService := appService.NewChainService(rootCtx, chainlistStorage, memoryStorage, rpcAdapter, appLogger, *cfg)
	chainHttpHandler := appDeliveryHttp.NewChainHandler(chainApplicationService, appLogger)

	appLogger.Info("Setting up HTTP router...")
	r := router.New()
	appDeliveryHttp.RegisterRoutes(r, chainHttpHandler, appLogger)

	loggingMiddlewareFactory := func(logger *zap.Logger) func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return func(ctx *fasthttp.RequestCtx) {
				logger.Info("Request received",
					zap.ByteString("method", ctx.Method()),
					zap.ByteString("uri", ctx.RequestURI()))
				next(ctx)
			}
		}
	}

	serverAddr := ":" + cfg.Server.Port
	server := &fasthttp.Server{
		Handler: loggingMiddlewareFactory(appLogger)(r.Handler),
	}

	go func(l *zap.Logger) {
		l.Info("Starting HTTP server", zap.String("address", serverAddr))
		if serveErr := server.ListenAndServe(serverAddr); checkServerStatus(serveErr) {
			l.Fatal("HTTP server ListenAndServe error", zap.Error(serveErr))
		}
	}(appLogger)

	gracefulShutdown(rootCtx, server, rootCancel, appLogger)

	appLogger.Info("Application shutdown process finished.")
}

func gracefulShutdown(ctx context.Context, server *fasthttp.Server, cancelAppTasks context.CancelFunc, logger *zap.Logger) {
	quitSignal := make(chan os.Signal, 1)
	signal.Notify(quitSignal, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quitSignal:
		logger.Info("Received OS shutdown signal", zap.String("signal", sig.String()))
	case <-ctx.Done():
		logger.Info("Shutdown initiated by application context cancellation.")
	}

	logger.Info("Initiating graceful shutdown sequence...")

	logger.Info("Signaling dependent background tasks to stop (cancelling app context)...")
	cancelAppTasks()

	shutdownTimeout := 15 * time.Second
	serverShutdownCtx, serverShutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer serverShutdownCancel()

	logger.Info("Attempting to shut down HTTP server gracefully...", zap.Duration("timeout", shutdownTimeout))

	shutdownComplete := make(chan struct{})
	go func() {
		if err := server.Shutdown(); err != nil {
			logger.Error("HTTP server Shutdown() method failed", zap.Error(err))
		} else {
			logger.Info("HTTP server Shutdown() method completed.")
		}
		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		logger.Info("HTTP server has been shut down gracefully.")
	case <-serverShutdownCtx.Done():
		if errors.Is(serverShutdownCtx.Err(), context.DeadlineExceeded) {
			logger.Warn("HTTP server graceful shutdown timed out. It might have been forced to close or is still hanging.")
		}
	}

	logger.Info("Graceful shutdown sequence finished.")
}

func checkServerStatus(serveErr error) bool {
	return serveErr != nil && !errors.Is(serveErr, fasthttp.ErrConnectionClosed) && !errors.Is(serveErr, fasthttp.ErrConnectionClosed)
}
