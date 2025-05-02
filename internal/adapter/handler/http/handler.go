package http

import (
	"encoding/json"
	"strconv"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"chainlist-parser/internal/usecase"
)

type ChainHandler struct {
	useCase usecase.ChainUseCase
	logger  *zap.Logger
}

func NewChainHandler(uc usecase.ChainUseCase, logger *zap.Logger) *ChainHandler {
	return &ChainHandler{
		useCase: uc,
		logger:  logger.Named("ChainHandler"),
	}
}

// GetAllChains handles requests for all chains with working RPCs
func (h *ChainHandler) GetAllChains(ctx *fasthttp.RequestCtx) {
	chains, err := h.useCase.GetAllChainsWithWorkingRPCs(ctx)
	if err != nil {
		h.logger.Error("Failed to get all chains", zap.Error(err))
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// Simple response for now, might need pagination or filtering later
	ctx.SetContentType("application/json")
	err = json.NewEncoder(ctx).Encode(chains)
	if err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		// Response already started, can't set error code
	}
}

// GetChainRPCs handles requests for working RPCs of a specific chain
func (h *ChainHandler) GetChainRPCs(ctx *fasthttp.RequestCtx) {
	chainIDStr, ok := ctx.UserValue("chainId").(string)
	if !ok {
		h.logger.Error("Failed to get chainId from context")
		ctx.Error("Bad Request: Invalid chainId format", fasthttp.StatusBadRequest)
		return
	}

	chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
	if err != nil {
		h.logger.Error("Failed to parse chainId", zap.String("chainIdStr", chainIDStr), zap.Error(err))
		ctx.Error("Bad Request: Invalid chainId", fasthttp.StatusBadRequest)
		return
	}

	rpcs, err := h.useCase.GetWorkingRPCsForChain(ctx, chainID)
	if err != nil {
		// Handle specific errors like 'not found' if implemented in usecase
		h.logger.Error("Failed to get RPCs for chain", zap.Int64("chainId", chainID), zap.Error(err))
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	if rpcs == nil {
		// Handle case where chain exists but has no working RPCs or chain not found
		h.logger.Warn("No working RPCs found or chain does not exist", zap.Int64("chainId", chainID))
		ctx.Error("Not Found", fasthttp.StatusNotFound)
		return
	}

	ctx.SetContentType("application/json")
	err = json.NewEncoder(ctx).Encode(rpcs)
	if err != nil {
		h.logger.Error("Failed to encode RPC response", zap.Error(err))
	}
}
