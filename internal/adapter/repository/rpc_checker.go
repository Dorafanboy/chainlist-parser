package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"chainlist-parser/internal/usecase"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// Compile-time check
var _ usecase.RPCChecker = (*rpcChecker)(nil)

type rpcChecker struct {
	client *fasthttp.Client
	logger *zap.Logger
}

func NewRPCChecker(logger *zap.Logger) usecase.RPCChecker {
	return &rpcChecker{
		client: &fasthttp.Client{
			ReadTimeout: 10 * time.Second,
		},
		logger: logger.Named("RPCChecker"),
	}
}

// checkPayload is the standard JSON-RPC request to check node health.
var checkPayload = []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`)

// JSONRPCResponse defines the basic structure for a JSON-RPC response.
type JSONRPCResponse struct {
	ID      interface{}     `json:"id"`
	Jsonrpc string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError defines the structure for a JSON-RPC error.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *rpcChecker) CheckRPC(ctx context.Context, rpcURL string) (isWorking bool, latency time.Duration, err error) {
	if strings.HasPrefix(rpcURL, "wss://") {
		c.logger.Debug("Skipping WSS check", zap.String("url", rpcURL))
		return false, 0, nil
	}

	if !strings.HasPrefix(rpcURL, "http://") && !strings.HasPrefix(rpcURL, "https://") {
		c.logger.Warn("Skipping check for unsupported protocol", zap.String("url", rpcURL))
		return false, 0, fmt.Errorf("unsupported protocol in URL: %s", rpcURL)
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(rpcURL)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody(checkPayload)

	startTime := time.Now()

	deadline, hasDeadline := ctx.Deadline()
	timeout := c.client.ReadTimeout
	if hasDeadline {
		requestTimeout := time.Until(deadline)
		if requestTimeout > 0 && (timeout <= 0 || requestTimeout < timeout) {
			timeout = requestTimeout
		}
	}

	if timeout <= 0 {
		err = c.client.Do(req, resp)
	} else {
		err = c.client.DoTimeout(req, resp, timeout)
	}

	latency = time.Since(startTime)

	if err != nil {
		c.logger.Debug("RPC check HTTP request failed", zap.String("url", rpcURL), zap.Error(err))
		return false, latency, fmt.Errorf("http request failed: %w", err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		c.logger.Debug("RPC check returned non-OK status",
			zap.String("url", rpcURL),
			zap.Int("statusCode", resp.StatusCode()))
		return false, latency, fmt.Errorf("non-OK status: %d", resp.StatusCode())
	}

	// Basic check for valid JSON-RPC structure
	var rpcResp JSONRPCResponse
	err = json.Unmarshal(resp.Body(), &rpcResp)
	if err != nil {
		c.logger.Debug("RPC check failed to unmarshal JSON response",
			zap.String("url", rpcURL),
			zap.ByteString("body", resp.Body()),
			zap.Error(err))
		return false, latency, fmt.Errorf("invalid JSON response: %w", err)
	}

	if rpcResp.Error != nil {
		c.logger.Debug("RPC check returned JSON-RPC error",
			zap.String("url", rpcURL),
			zap.Int("errorCode", rpcResp.Error.Code),
			zap.String("errorMessage", rpcResp.Error.Message))
		return false, latency, fmt.Errorf("json-rpc error: %d %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if rpcResp.Jsonrpc != "2.0" || rpcResp.Result == nil {
		c.logger.Debug("RPC check returned invalid JSON-RPC structure",
			zap.String("url", rpcURL),
			zap.ByteString("body", resp.Body()))
		return false, latency, fmt.Errorf("invalid JSON-RPC structure")
	}

	return true, latency, nil
}
