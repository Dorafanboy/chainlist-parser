package http

import (
	"encoding/json"
	"strconv"
	"strings"

	"chainlist-parser/internal/entity"
	"chainlist-parser/internal/usecase"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
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

// ChainHandler holds dependencies for HTTP handlers related to chains.
type ChainHandler struct {
	useCase usecase.ChainUseCase
	logger  *zap.Logger
}

// NewChainHandler creates a new instance of ChainHandler.
func NewChainHandler(uc usecase.ChainUseCase, logger *zap.Logger) *ChainHandler {
	return &ChainHandler{
		useCase: uc,
		logger:  logger.Named("ChainHandler"),
	}
}

// GetAllChains handles requests for all chains, returning data possibly from cache or fresh.
func (h *ChainHandler) GetAllChains(ctx *fasthttp.RequestCtx) {
	chains, err := h.useCase.GetAllChainsChecked(ctx)
	if err != nil {
		h.logger.Error("Failed to get all chains data from use case", zap.Error(err))
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	responseChains := make([]ChainApiResponse, len(chains))
	for i, chain := range chains {
		httpEndpoints := make([]string, 0)
		wssEndpoints := make([]string, 0)

		if len(chain.CheckedRPCs) > 0 {
			h.logger.Debug("Processing checked RPCs for chain", zap.Int64("chainId", chain.ChainID))
			for _, rpc := range chain.CheckedRPCs {
				if rpc.Protocol == entity.ProtocolWSS || rpc.Protocol == entity.ProtocolWS {
					wssEndpoints = append(wssEndpoints, rpc.URL)
				} else if rpc.IsWorking != nil && *rpc.IsWorking {
					httpEndpoints = append(httpEndpoints, rpc.URL)
				}
			}
		} else {
			h.logger.Debug("Processing raw RPCs for chain", zap.Int64("chainId", chain.ChainID))
			for _, rawRpcURL := range chain.RPC {
				if strings.HasPrefix(rawRpcURL, "wss://") || strings.HasPrefix(rawRpcURL, "ws://") {
					wssEndpoints = append(wssEndpoints, rawRpcURL)
				} else if strings.HasPrefix(rawRpcURL, "https://") || strings.HasPrefix(rawRpcURL, "http://") {
					httpEndpoints = append(httpEndpoints, rawRpcURL)
				}
			}
		}

		responseChains[i] = ChainApiResponse{
			ChainID:      chain.ChainID,
			Name:         chain.Name,
			NativeSymbol: chain.Currency.Symbol,
			RPCEndpoints: RPCResponse{
				HTTP: httpEndpoints,
				WSS:  wssEndpoints,
			},
		}
	}

	ctx.SetContentType("application/json")
	err = json.NewEncoder(ctx).Encode(responseChains)
	if err != nil {
		// Log error, but headers might already be sent
		h.logger.Error("Failed to encode chains response", zap.Error(err))
		// Avoid writing ctx.Error here as response likely started
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

	checkedRPCs, err := h.useCase.GetCheckedRPCsForChain(ctx, chainID)
	if err != nil {
		h.logger.Error("Failed to get checked RPCs for chain", zap.Int64("chainId", chainID), zap.Error(err))
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return
	}

	if checkedRPCs == nil {
		h.logger.Warn("Chain data not found by use case (returned nil)", zap.Int64("chainId", chainID))
		ctx.Error("Not Found", fasthttp.StatusNotFound)
		return
	}

	httpEndpoints := make([]string, 0)
	wssEndpoints := make([]string, 0)
	for _, rpc := range checkedRPCs {
		if strings.HasPrefix(rpc.URL, "wss://") || strings.HasPrefix(rpc.URL, "ws://") {
			wssEndpoints = append(wssEndpoints, rpc.URL)
		} else if rpc.IsWorking != nil && *rpc.IsWorking {
			httpEndpoints = append(httpEndpoints, rpc.URL)
		}
	}

	if len(httpEndpoints) == 0 && len(wssEndpoints) == 0 {
		h.logger.Warn(
			"No working HTTP/S or any WSS endpoints found for chain after filtering",
			zap.Int64("chainId", chainID),
		)
		ctx.Error("Not Found", fasthttp.StatusNotFound)
		return
	}

	response := RPCResponse{
		HTTP: httpEndpoints,
		WSS:  wssEndpoints,
	}

	ctx.SetContentType("application/json")
	err = json.NewEncoder(ctx).Encode(response)
	if err != nil {
		h.logger.Error("Failed to encode filtered RPCs response", zap.Error(err))
	}
}
