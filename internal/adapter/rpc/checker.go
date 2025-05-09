package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chainlist-parser/internal/domain/entity"
	domainService "chainlist-parser/internal/domain/service"
	"chainlist-parser/internal/pkg/apperrors"

	"github.com/gorilla/websocket"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// Compile-time check
var _ domainService.RPCChecker = (*Checker)(nil)

// Checker implements the domainService.RPCChecker interface.
type Checker struct {
	client *fasthttp.Client
	logger *zap.Logger
}

// NewChecker creates a new RPC checker instance.
func NewChecker(logger *zap.Logger) domainService.RPCChecker {
	return &Checker{
		client: &fasthttp.Client{
			ReadTimeout: 10 * time.Second,
		},
		logger: logger.Named("RPCCheckerAdapter"),
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
func (c *Checker) CheckRPC(
	ctx context.Context,
	rpcURL entity.RPCURL,
) (isWorking bool, latency time.Duration, err error) {
	startTime := time.Now()
	rawURL := rpcURL.String()

	if strings.HasPrefix(rawURL, "wss://") || strings.HasPrefix(rawURL, "ws://") {
		isWorking, latency, err = c.checkWSS(ctx, rawURL, startTime)
		return
	}

	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		isWorking, latency, err = c.checkHTTP(ctx, rawURL, startTime)
		return
	}

	c.logger.Warn("Skipping check for unsupported protocol in validated RPCURL", zap.String("url", rawURL))
	return false, 0, fmt.Errorf("%w: unsupported protocol in URL %s", apperrors.ErrInvalidInput, rawURL)
}

// checkHTTP performs the JSON-RPC check over HTTP/HTTPS.
func (c *Checker) checkHTTP(ctx context.Context, rpcURL string, startTime time.Time) (bool, time.Duration, error) {
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
		c.logger.Warn("No effective timeout specified for fasthttp request, using default Do",
			zap.String("url", rpcURL),
		)
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
			return false, latency, fmt.Errorf("%w: http request to %s timed out after %v: %v",
				apperrors.ErrTimeout, rpcURL, timeout, requestErr,
			)
		}
		c.logger.Debug("HTTP RPC check request failed", zap.String("url", rpcURL), zap.Error(requestErr))
		return false, latency, fmt.Errorf("%w: http request to %s failed: %v",
			apperrors.ErrExternalServiceFailure, rpcURL, requestErr,
		)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		c.logger.Debug(
			"HTTP RPC check returned non-OK status",
			zap.String("url", rpcURL),
			zap.Int("statusCode", resp.StatusCode()),
		)
		return false, latency, fmt.Errorf("%w: rpc %s returned non-OK http status: %d",
			apperrors.ErrExternalServiceFailure, rpcURL, resp.StatusCode(),
		)
	}

	isValid, jsonErr := c.validateJSONRPCResponse(rpcURL, resp.Body())
	return isValid, latency, jsonErr
}

// checkWSS performs the JSON-RPC check over WSS/WS.
func (c *Checker) checkWSS(ctx context.Context, rpcURL string, startTime time.Time) (bool, time.Duration, error) {
	handshakeTimeout := c.client.ReadTimeout
	if handshakeTimeout <= 0 {
		handshakeTimeout = 10 * time.Second
	}

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: handshakeTimeout,
	}

	c.logger.Debug("Attempting WSS connection",
		zap.String("url", rpcURL), zap.Duration("handshakeTimeout", handshakeTimeout),
	)

	conn, _, err := dialer.DialContext(ctx, rpcURL, nil)
	latencyAfterDial := time.Since(startTime)

	if err != nil {
		c.logger.Debug("WSS dial failed", zap.String("url", rpcURL), zap.Error(err))
		if ctxErr := context.Cause(ctx); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return false, latencyAfterDial, fmt.Errorf("%w: wss dial to %s context timed out: %v",
					apperrors.ErrTimeout, rpcURL, ctxErr,
				)
			}
			return false, latencyAfterDial, fmt.Errorf("%w: wss dial to %s context error: %v",
				apperrors.ErrExternalServiceFailure, rpcURL, ctxErr,
			)
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return false, latencyAfterDial, fmt.Errorf("%w: wss dial to %s timed out (ctx.Err): %v",
				apperrors.ErrTimeout, rpcURL, ctx.Err(),
			)
		}

		return false, latencyAfterDial, fmt.Errorf("%w: wss dial to %s failed: %v",
			apperrors.ErrExternalServiceFailure, rpcURL, err,
		)
	}
	defer conn.Close()

	c.logger.Debug("WSS connection successful, sending check payload", zap.String("url", rpcURL))

	operationTimeout := c.client.ReadTimeout
	if operationTimeout <= 0 {
		operationTimeout = 15 * time.Second
	}

	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < operationTimeout {
			operationTimeout = time.Until(deadline)
		}
	}

	if operationTimeout <= 0 {
		operationTimeout = 2 * time.Second
	}

	_ = conn.SetWriteDeadline(time.Now().Add(operationTimeout))
	_ = conn.SetReadDeadline(time.Now().Add(operationTimeout))

	if wErr := conn.WriteMessage(websocket.TextMessage, checkPayload); wErr != nil {
		c.logger.Debug("WSS write message failed", zap.String("url", rpcURL), zap.Error(wErr))
		return false, time.Since(startTime), fmt.Errorf("%w: wss write to %s failed: %v",
			apperrors.ErrExternalServiceFailure, rpcURL, wErr,
		)
	}

	_, message, rErr := conn.ReadMessage()
	finalLatency := time.Since(startTime)
	if rErr != nil {
		c.logger.Debug("WSS read message failed", zap.String("url", rpcURL), zap.Error(rErr))
		if ctxErr := context.Cause(ctx); ctxErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return false, finalLatency, fmt.Errorf("%w: wss read from %s context timed out: %v",
					apperrors.ErrTimeout, rpcURL, ctxErr,
				)
			}
			return false, finalLatency, fmt.Errorf("%w: wss read from %s context error: %v",
				apperrors.ErrExternalServiceFailure, rpcURL, ctxErr,
			)
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return false, finalLatency, fmt.Errorf("%w: wss read from %s timed out (ctx.Err): %v",
				apperrors.ErrTimeout, rpcURL, ctx.Err(),
			)
		}
		return false, finalLatency, fmt.Errorf("%w: wss read from %s failed: %v",
			apperrors.ErrExternalServiceFailure, rpcURL, rErr,
		)
	}

	c.logger.Debug("WSS received response", zap.String("url", rpcURL), zap.ByteString("body", message))

	isValid, jsonErr := c.validateJSONRPCResponse(rpcURL, message)
	if isValid {
		return isValid, finalLatency, jsonErr
	}
	return isValid, time.Since(startTime), jsonErr
}

// validateJSONRPCResponse checks if the response body is a valid, successful JSON-RPC response.
func (c *Checker) validateJSONRPCResponse(rpcURL string, body []byte) (bool, error) {
	var rpcResp JSONRPCResponse
	err := json.Unmarshal(body, &rpcResp)
	if err != nil {
		c.logger.Debug(
			"RPC check failed to unmarshal JSON response",
			zap.String("url", rpcURL),
			zap.ByteString("body", body),
			zap.Error(err),
		)
		return false, fmt.Errorf("%w: rpc %s returned invalid JSON response: %v",
			apperrors.ErrExternalServiceFailure, rpcURL, err,
		)
	}

	if rpcResp.Error != nil {
		c.logger.Debug(
			"RPC check returned JSON-RPC error",
			zap.String("url", rpcURL),
			zap.Int("errorCode", rpcResp.Error.Code),
			zap.String("errorMessage", rpcResp.Error.Message),
		)
		return false, fmt.Errorf("%w: rpc %s returned json-rpc error: %d %s",
			apperrors.ErrExternalServiceFailure, rpcURL, rpcResp.Error.Code, rpcResp.Error.Message,
		)
	}

	if rpcResp.Jsonrpc != "2.0" || rpcResp.Result == nil {
		c.logger.Debug(
			"RPC check returned invalid JSON-RPC structure",
			zap.String("url", rpcURL),
			zap.ByteString("body", body),
		)
		return false, fmt.Errorf("%w: rpc %s returned invalid JSON-RPC structure",
			apperrors.ErrExternalServiceFailure, rpcURL,
		)
	}

	return true, nil
}
