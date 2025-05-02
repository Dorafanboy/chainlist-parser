package http

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"chainlist-parser/internal/usecase"
)

// RPCResponse defines the structure for returning filtered RPC endpoints.
type RPCResponse struct {
	HTTP []string `json:"http"`
	WSS  []string `json:"wss"`
}

// ChainApiResponse defines the structure for the response of the /chains endpoint.
type ChainApiResponse struct {
	ChainID      int64       `json:"chainId"`
	Name         string      `json:"name"`
	NativeSymbol string      `json:"nativeSymbol"`
	RPCEndpoints RPCResponse `json:"rpcEndpoints"`
}

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

// GetAllChains handles requests for all chains with their checked RPC details.
func (h *ChainHandler) GetAllChains(ctx *fasthttp.RequestCtx) {
	// Call the renamed use case method
	chains, err := h.useCase.GetAllChainsChecked(ctx)
	if err != nil {
		h.logger.Error("Failed to get all chains checked data", zap.Error(err))
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	responseChains := make([]ChainApiResponse, 0, len(chains))
	for _, chain := range chains {
		httpEndpoints := make([]string, 0)
		wssEndpoints := make([]string, 0)
		for _, rpc := range chain.CheckedRPCs { // Iterate over the checked RPCs
			if strings.HasPrefix(rpc.URL, "wss://") || strings.HasPrefix(rpc.URL, "ws://") {
				wssEndpoints = append(wssEndpoints, rpc.URL)
			} else if rpc.IsWorking != nil && *rpc.IsWorking { // Check for non-nil pointer *and* true value
				httpEndpoints = append(httpEndpoints, rpc.URL)
			}
		}

		// Only include the chain if it has at least one working HTTP/S or any WSS endpoint
		if len(httpEndpoints) > 0 || len(wssEndpoints) > 0 {
			responseChains = append(responseChains, ChainApiResponse{
				ChainID:      chain.ChainID,
				Name:         chain.Name,
				NativeSymbol: chain.Currency.Symbol,
				RPCEndpoints: RPCResponse{
					HTTP: httpEndpoints,
					WSS:  wssEndpoints,
				},
			})
		}
	}

	ctx.SetContentType("application/json")
	err = json.NewEncoder(ctx).Encode(responseChains) // Encode the new response structure
	if err != nil {
		h.logger.Error("Failed to encode chains response", zap.Error(err))
	}
}

// GetChainRPCs handles requests for checked RPC details of a specific chain.
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

	// Call the renamed use case method which returns []entity.RPCDetail
	checkedRPCs, err := h.useCase.GetCheckedRPCsForChain(ctx, chainID)
	if err != nil {
		// Handle specific errors like 'not found' if implemented in usecase
		// Assume use case returns specific error type for not found, e.g., usecase.ErrChainNotFound
		// if errors.Is(err, usecase.ErrChainNotFound) {
		//     h.logger.Warn("Chain not found by use case", zap.Int64("chainId", chainID))
		//     ctx.Error("Not Found", fasthttp.StatusNotFound)
		//     return
		// }
		h.logger.Error("Failed to get checked RPCs for chain", zap.Int64("chainId", chainID), zap.Error(err))
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	// Original check: If the use case returns nil list (e.g., chain not found), return 404.
	// Now we check based on the content *after* filtering.
	if checkedRPCs == nil {
		h.logger.Warn("Chain data not found by use case (returned nil)", zap.Int64("chainId", chainID))
		ctx.Error("Not Found", fasthttp.StatusNotFound)
		return
	}

	// Filter the RPCs
	httpEndpoints := make([]string, 0)
	wssEndpoints := make([]string, 0)
	for _, rpc := range checkedRPCs {
		if strings.HasPrefix(rpc.URL, "wss://") || strings.HasPrefix(rpc.URL, "ws://") {
			wssEndpoints = append(wssEndpoints, rpc.URL)
		} else if rpc.IsWorking != nil && *rpc.IsWorking { // Check for non-nil pointer *and* true value
			httpEndpoints = append(httpEndpoints, rpc.URL)
		}
	}

	// Return 404 if *no* working HTTP/S endpoints AND *no* WSS endpoints were found for the chain.
	// This means the chain exists but either has no RPCs or none are suitable for the response.
	if len(httpEndpoints) == 0 && len(wssEndpoints) == 0 {
		h.logger.Warn("No working HTTP/S or any WSS endpoints found for chain after filtering", zap.Int64("chainId", chainID))
		ctx.Error("Not Found", fasthttp.StatusNotFound) // Or potentially 200 OK with empty lists? Decided 404 for now.
		return
	}

	// Return the filtered list with a 200 OK status.
	response := RPCResponse{
		HTTP: httpEndpoints,
		WSS:  wssEndpoints,
	}

	ctx.SetContentType("application/json")
	err = json.NewEncoder(ctx).Encode(response) // Encode the new response structure
	if err != nil {
		h.logger.Error("Failed to encode filtered RPCs response", zap.Error(err))
	}
}
