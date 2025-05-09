package http

import (
	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// RegisterRoutes sets up the routes for the chain handler and common health checks.
func RegisterRoutes(r *router.Router, h *ChainHandler, logger *zap.Logger) {
	logger.Info("Setting up application-specific routes...")

	r.GET("/chains", h.GetAllChains)
	r.GET("/chains/{chainId:[0-9]+}/rpcs", h.GetChainRPCs)

	logger.Info("Setting up health check route...")
	r.GET("/health", func(ctx *fasthttp.RequestCtx) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.SetBodyString("OK")
	})

	logger.Info("All routes registered.")
}
