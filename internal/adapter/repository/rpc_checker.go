package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chainlist-parser/internal/usecase"

	"github.com/gorilla/websocket"
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

// CheckRPC determines the protocol and calls the appropriate check function.
func (c *rpcChecker) CheckRPC(ctx context.Context, rpcURL string) (isWorking bool, latency time.Duration, err error) {
	startTime := time.Now()

	if strings.HasPrefix(rpcURL, "wss://") || strings.HasPrefix(rpcURL, "ws://") {
		isWorking, latency, err = c.checkWSS(ctx, rpcURL, startTime)
		return
	}

	if strings.HasPrefix(rpcURL, "http://") || strings.HasPrefix(rpcURL, "https://") {
		isWorking, latency, err = c.checkHTTP(ctx, rpcURL, startTime)
		return
	}

	c.logger.Warn("Skipping check for unsupported protocol", zap.String("url", rpcURL))
	return false, 0, fmt.Errorf("unsupported protocol in URL: %s", rpcURL)
}

// checkHTTP performs the JSON-RPC check over HTTP/HTTPS.
func (c *rpcChecker) checkHTTP(ctx context.Context, rpcURL string, startTime time.Time) (bool, time.Duration, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(rpcURL)
	req.Header.SetMethod(fasthttp.MethodPost)
	req.Header.SetContentType("application/json")
	req.SetBody(checkPayload)

	deadline, hasDeadline := ctx.Deadline()
	timeout := c.client.ReadTimeout
	if hasDeadline {
		requestTimeout := time.Until(deadline)
		if requestTimeout > 0 && (timeout <= 0 || requestTimeout < timeout) {
			timeout = requestTimeout
		}
	}

	var requestErr error
	if timeout <= 0 {
		c.logger.Warn("No timeout specified for fasthttp request, using default Do", zap.String("url", rpcURL))
		requestErr = c.client.Do(req, resp)
	} else {
		requestErr = c.client.DoTimeout(req, resp, timeout)
	}

	latency := time.Since(startTime)

	if requestErr != nil {
		if errors.Is(requestErr, fasthttp.ErrTimeout) {
			c.logger.Debug(
				"HTTP RPC check timed out",
				zap.String("url", rpcURL),
				zap.Duration("timeout", timeout),
				zap.Error(requestErr),
			)
			return false, latency, fmt.Errorf("http request timed out after %v: %w", timeout, requestErr)
		}
		c.logger.Debug("HTTP RPC check request failed", zap.String("url", rpcURL), zap.Error(requestErr))
		return false, latency, fmt.Errorf("http request failed: %w", requestErr)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		c.logger.Debug("HTTP RPC check returned non-OK status",
			zap.String("url", rpcURL),
			zap.Int("statusCode", resp.StatusCode()))
		return false, latency, fmt.Errorf("non-OK http status: %d", resp.StatusCode())
	}

	// Reuse the validation logic for the response body
	isValid, jsonErr := c.validateJSONRPCResponse(rpcURL, resp.Body())
	return isValid, latency, jsonErr
}

// checkWSS performs the JSON-RPC check over WSS/WS.
func (c *rpcChecker) checkWSS(ctx context.Context, rpcURL string, startTime time.Time) (bool, time.Duration, error) {
	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 10 * time.Second,
	}

	c.logger.Debug("Attempting WSS connection", zap.String("url", rpcURL))

	conn, _, err := dialer.DialContext(ctx, rpcURL, nil)
	latency := time.Since(startTime)

	if err != nil {
		c.logger.Debug("WSS dial failed", zap.String("url", rpcURL), zap.Error(err))
		if ctxErr := context.Cause(ctx); ctxErr != nil {
			return false, latency, fmt.Errorf("wss dial context error: %w", ctxErr)
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
			return false, latency, fmt.Errorf("wss dial cancelled/timed out: %w", ctx.Err())
		}
		return false, latency, fmt.Errorf("wss dial failed: %w", err)
	}
	defer conn.Close()

	c.logger.Debug("WSS connection successful, sending check payload", zap.String("url", rpcURL))

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetWriteDeadline(deadline)
		_ = conn.SetReadDeadline(deadline)
	} else {
		defaultOpTimeout := 15 * time.Second
		_ = conn.SetWriteDeadline(time.Now().Add(defaultOpTimeout))
		_ = conn.SetReadDeadline(time.Now().Add(defaultOpTimeout))
	}

	if wErr := conn.WriteMessage(websocket.TextMessage, checkPayload); wErr != nil {
		c.logger.Debug("WSS write message failed", zap.String("url", rpcURL), zap.Error(wErr))
		return false, latency, fmt.Errorf("wss write failed: %w", wErr)
	}

	_, message, rErr := conn.ReadMessage()
	if rErr != nil {
		c.logger.Debug("WSS read message failed", zap.String("url", rpcURL), zap.Error(rErr))
		if ctxErr := context.Cause(ctx); ctxErr != nil {
			return false, latency, fmt.Errorf("wss read context error: %w", ctxErr)
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
			return false, latency, fmt.Errorf("wss read cancelled/timed out: %w", ctx.Err())
		}
		return false, latency, fmt.Errorf("wss read failed: %w", rErr)
	}

	c.logger.Debug("WSS received response", zap.String("url", rpcURL), zap.ByteString("body", message))

	isValid, jsonErr := c.validateJSONRPCResponse(rpcURL, message)
	if isValid {
		latency = time.Since(startTime)
	}
	return isValid, latency, jsonErr
}

// validateJSONRPCResponse checks if the response body is a valid, successful JSON-RPC response.
func (c *rpcChecker) validateJSONRPCResponse(rpcURL string, body []byte) (bool, error) {
	var rpcResp JSONRPCResponse
	err := json.Unmarshal(body, &rpcResp)
	if err != nil {
		c.logger.Debug("RPC check failed to unmarshal JSON response",
			zap.String("url", rpcURL),
			zap.ByteString("body", body),
			zap.Error(err))
		return false, fmt.Errorf("invalid JSON response: %w", err)
	}

	if rpcResp.Error != nil {
		c.logger.Debug("RPC check returned JSON-RPC error",
			zap.String("url", rpcURL),
			zap.Int("errorCode", rpcResp.Error.Code),
			zap.String("errorMessage", rpcResp.Error.Message),
		)
		return false, fmt.Errorf("json-rpc error: %d %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if rpcResp.Jsonrpc != "2.0" || rpcResp.Result == nil {
		c.logger.Debug("RPC check returned invalid JSON-RPC structure",
			zap.String("url", rpcURL),
			zap.ByteString("body", body),
		)
		return false, fmt.Errorf("invalid JSON-RPC structure")
	}

	// Check passed
	return true, nil
}
