package main

import (
	"log"

	"chainlist-parser/internal/adapter/handler/http"
	"chainlist-parser/internal/adapter/repository"
	"chainlist-parser/internal/config"
	parserLogger "chainlist-parser/internal/logger"
	"chainlist-parser/internal/usecase"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func main() {
	cfgPath := "config"
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load configuration from %s: %v", cfgPath, err)
	}

	logger, err := parserLogger.NewLogger(cfg.Logger)
	if err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
	defer logger.Sync()
	logger.Info("Logger initialized", zap.Any("config", cfg.Logger))

	logger.Info("Initializing dependencies...")

	chainlistRepo := repository.NewChainlistRepo(cfg.Chainlist, logger)
	cacheRepo := repository.NewGoCacheRepo(*cfg, logger)
	rpcChecker := repository.NewRPCChecker(logger)

	chainUseCase := usecase.NewChainUseCase(chainlistRepo, cacheRepo, rpcChecker, logger, *cfg)
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
			logger.Info("Request received",
				zap.ByteString("method", ctx.Method()),
				zap.ByteString("uri", ctx.RequestURI()))
			next(ctx)
		}
	}

	serverAddr := ":" + cfg.Server.Port
	logger.Info("Starting HTTP server", zap.String("address", serverAddr))

	if err := fasthttp.ListenAndServe(serverAddr, loggingMiddleware(r.Handler)); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
