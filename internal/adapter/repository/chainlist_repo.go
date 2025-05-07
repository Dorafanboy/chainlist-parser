package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chainlist-parser/internal/config"
	"chainlist-parser/internal/entity"
	"chainlist-parser/internal/pkg/apperrors"
	"chainlist-parser/internal/usecase"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// Compile-time check
var _ usecase.ChainRepository = (*chainlistRepo)(nil)

// chainlistRepo implements ChainRepository for fetching data from the Chainlist source.
type chainlistRepo struct {
	client *fasthttp.Client
	url    string
	logger *zap.Logger
}

// NewChainlistRepo creates a new Chainlist repository instance.
func NewChainlistRepo(cfg config.ChainlistConfig, logger *zap.Logger) usecase.ChainRepository {
	return &chainlistRepo{
		client: &fasthttp.Client{},
		url:    cfg.URL,
		logger: logger.Named("ChainlistRepo"),
	}
}

// GetAllChains fetches the full list of chains from the configured Chainlist URL.
func (r *chainlistRepo) GetAllChains(ctx context.Context) ([]entity.Chain, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(r.url)
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.Set(fasthttp.HeaderAcceptEncoding, "gzip")

	deadline, hasDeadline := ctx.Deadline()
	timeout := 15 * time.Second
	if hasDeadline {
		requestTimeout := time.Until(deadline)
		if requestTimeout > 0 && requestTimeout < timeout {
			timeout = requestTimeout
		}
	}

	r.logger.Debug(
		"Fetching chains from Chainlist",
		zap.String("url", r.url),
		zap.Duration("timeout", timeout),
	)

	err := r.client.DoTimeout(req, resp, timeout)
	if err != nil {
		r.logger.Error("Failed to execute request to Chainlist", zap.Error(err))
		return nil, fmt.Errorf("%w: failed to execute request to Chainlist: %v", apperrors.ErrExternalServiceFailure, err)
	}

	if resp.StatusCode() == fasthttp.StatusNotFound {
		r.logger.Warn(
			"Chainlist source reported not found",
			zap.Int("statusCode", resp.StatusCode()),
			zap.ByteString("body", resp.Body()),
		)
		return nil, fmt.Errorf("%w: chainlist source reported not found (%s)", apperrors.ErrNotFound, r.url)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		r.logger.Error(
			"Chainlist returned non-OK status",
			zap.Int("statusCode", resp.StatusCode()),
			zap.ByteString("body", resp.Body()),
		)
		return nil, fmt.Errorf("%w: chainlist returned status %d", apperrors.ErrExternalServiceFailure, resp.StatusCode())
	}

	var body []byte
	contentEncoding := resp.Header.Peek(fasthttp.HeaderContentEncoding)
	if bytes.EqualFold(contentEncoding, []byte("gzip")) {
		r.logger.Debug("Received gzipped response from Chainlist")
		body, err = resp.BodyGunzip()
		if err != nil {
			r.logger.Error("Failed to gunzip Chainlist response body", zap.Error(err))
			return nil, fmt.Errorf("%w: failed to decompress chainlist response: %v", apperrors.ErrExternalServiceFailure, err)
		}
	} else {
		body = resp.Body()
	}

	var chains []entity.Chain
	err = json.Unmarshal(body, &chains)
	if err != nil {
		r.logger.Error("Failed to unmarshal Chainlist response", zap.Error(err), zap.ByteString("bodySample", body[:min(1024, len(body))]))
		return nil, fmt.Errorf("%w: failed to parse chainlist response: %v", apperrors.ErrExternalServiceFailure, err)
	}

	r.logger.Info("Successfully fetched chains from Chainlist", zap.Int("count", len(chains)))
	return chains, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
