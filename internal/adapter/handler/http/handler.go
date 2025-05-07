package http

import (
	"chainlist-parser/internal/entity"
	"encoding/json"
	"errors"
	"strconv"

	"chainlist-parser/internal/usecase"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// APIError represents a machine-readable error code and a human-readable message.
type APIError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"` // Optional details for the error
}

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

// APIErrorResponse is the standard structure for API error responses.
type APIErrorResponse struct {
	Error APIError `json:"error"`
}

// newAPIErrorResponse is a helper to create an APIErrorResponse.
func newAPIErrorResponse(code, message, details string) APIErrorResponse {
	return APIErrorResponse{
		Error: APIError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// ChainHandler holds dependencies for HTTP handlers related to chains.
type ChainHandler struct {
	chainService usecase.ChainService
	logger       *zap.Logger
}

// NewChainHandler creates a new instance of ChainHandler.
func NewChainHandler(cs usecase.ChainService, logger *zap.Logger) *ChainHandler {
	return &ChainHandler{
		chainService: cs,
		logger:       logger.Named("ChainHandler"),
	}
}

// GetAllChains handles requests for all chains, returning data possibly from cache or fresh.
func (h *ChainHandler) GetAllChains(ctx *fasthttp.RequestCtx) {
	chains, err := h.chainService.GetAllChainsChecked(ctx)
	if err != nil {
		var apiErrResp APIErrorResponse
		var httpStatus int

		switch {
		case errors.Is(err, usecase.ErrUpstreamSourceFailure):
			httpStatus = fasthttp.StatusServiceUnavailable
			apiErrResp = newAPIErrorResponse("upstream_failure", "Failed to fetch data from the upstream source.", err.Error())
		case errors.Is(err, usecase.ErrCacheFailure):
			h.logger.Error("Internal cache failure detected in handler", zap.Error(err))
			httpStatus = fasthttp.StatusInternalServerError
			apiErrResp = newAPIErrorResponse("cache_error", "An internal error occurred with the caching system.", "")
		default:
			h.logger.Error("Unhandled internal error in GetAllChains", zap.Error(err))
			httpStatus = fasthttp.StatusInternalServerError
			apiErrResp = newAPIErrorResponse("internal_error", "An unexpected internal server error occurred.", "")
		}
		h.respondWithError(ctx, httpStatus, apiErrResp)
		return
	}

	responseChains := make([]ChainApiResponse, len(chains))
	for i, chain := range chains {
		httpEndpoints, wssEndpoints := filterAndCategorizeRPCs(h.logger, chain.ChainID, chain.CheckedRPCs)

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
		h.logger.Error("Failed to encode chains response", zap.Error(err))
	}
}

// GetChainRPCs handles requests for checked RPC details of a specific chain.
func (h *ChainHandler) GetChainRPCs(ctx *fasthttp.RequestCtx) {
	chainIDStr, ok := ctx.UserValue("chainId").(string)
	if !ok {
		h.logger.Error("Failed to get chainId from context - not a string")
		h.respondWithError(
			ctx,
			fasthttp.StatusBadRequest,
			newAPIErrorResponse(
				"bad_request",
				"Invalid chainId format in path.",
				"chainId must be a string representing a number."),
		)
		return
	}

	chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
	if err != nil {
		h.logger.Error("Failed to parse chainId from string", zap.String("chainIdStr", chainIDStr), zap.Error(err))
		h.respondWithError(ctx, fasthttp.StatusBadRequest, newAPIErrorResponse("bad_request", "Invalid chainId in path.", "chainId must be a valid integer."))
		return
	}

	checkedRPCs, err := h.chainService.GetCheckedRPCsForChain(ctx, chainID)
	if err != nil {
		var apiErrResp APIErrorResponse
		var httpStatus int

		switch {
		case errors.Is(err, usecase.ErrChainNotFound):
			httpStatus = fasthttp.StatusNotFound
			apiErrResp = newAPIErrorResponse("chain_not_found", "The requested chain ID was not found.", "")
		case errors.Is(err, usecase.ErrNoRPCsAvailable):
			httpStatus = fasthttp.StatusNotFound
			apiErrResp = newAPIErrorResponse("no_rpcs_available", "No working RPCs found for the specified chain.", "")
		case errors.Is(err, usecase.ErrUpstreamSourceFailure):
			httpStatus = fasthttp.StatusServiceUnavailable
			apiErrResp = newAPIErrorResponse("upstream_failure", "Failed to fetch data from the upstream source to find the chain.", err.Error())
		case errors.Is(err, usecase.ErrCacheFailure):
			h.logger.Error("Internal cache failure detected in handler for GetChainRPCs", zap.Int64("chainId", chainID), zap.Error(err))
			httpStatus = fasthttp.StatusInternalServerError
			apiErrResp = newAPIErrorResponse("cache_error", "An internal error occurred with the caching system.", "")
		default:
			h.logger.Error("Unhandled internal error in GetChainRPCs", zap.Int64("chainId", chainID), zap.Error(err))
			httpStatus = fasthttp.StatusInternalServerError
			apiErrResp = newAPIErrorResponse("internal_error", "An unexpected internal server error occurred.", "")
		}
		h.respondWithError(ctx, httpStatus, apiErrResp)
		return
	}

	httpEndpoints, wssEndpoints := filterAndCategorizeRPCs(h.logger, chainID, checkedRPCs)

	if len(httpEndpoints) == 0 && len(wssEndpoints) == 0 {
		h.logger.Info(
			"No displayable HTTP/S or WSS endpoints found for chain after local filtering in handler",
			zap.Int64("chainId", chainID),
		)
		h.respondWithError(
			ctx,
			fasthttp.StatusNotFound,
			newAPIErrorResponse(
				"no_rpcs_displayable",
				"No displayable RPCs found for the specified chain after filtering.",
				""),
		)
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

// Вспомогательная функция для отправки JSON-ошибки
func (h *ChainHandler) respondWithError(ctx *fasthttp.RequestCtx, httpStatus int, apiErrResp APIErrorResponse) {
	ctx.SetStatusCode(httpStatus)
	ctx.SetContentType("application/json")
	if err := json.NewEncoder(ctx).Encode(apiErrResp); err != nil {
		h.logger.Error("Failed to encode error response", zap.Error(err), zap.Any("apiError", apiErrResp))
	}
}

// filterAndCategorizeRPCs разделяет RPC на HTTP и WSS, логируя процесс.
func filterAndCategorizeRPCs(
	logger *zap.Logger,
	chainID int64,
	rpcs []entity.RPCDetail,
) (httpEps []string, wssEps []string) {
	httpEps = make([]string, 0)
	wssEps = make([]string, 0)
	if len(rpcs) == 0 {
		logger.Debug(
			"filterAndCategorizeRPCs received empty rpcs slice, "+
				"implies no working RPCs from usecase or initial list was empty",
			zap.Int64("chainId", chainID),
		)
		return
	}

	for _, rpc := range rpcs {
		switch rpc.Protocol {
		case entity.ProtocolWSS, entity.ProtocolWS:
			wssEps = append(wssEps, rpc.URL)
		case entity.ProtocolHTTPS, entity.ProtocolHTTP:
			if rpc.IsWorking != nil && *rpc.IsWorking {
				httpEps = append(httpEps, rpc.URL)
			} else {
				logger.Debug(
					"Skipping non-working or non-HTTP/S RPC in filter",
					zap.Int64("chainId", chainID),
					zap.String("url", rpc.URL),
					zap.Any("protocol", rpc.Protocol),
					zap.Boolp("is_working", rpc.IsWorking),
				)
			}
		default:
			logger.Debug(
				"Skipping RPC with unknown/unhandled protocol in filter",
				zap.Int64("chainId", chainID),
				zap.String("url", rpc.URL),
				zap.Any("protocol", rpc.Protocol),
			)
		}
	}
	return
}
