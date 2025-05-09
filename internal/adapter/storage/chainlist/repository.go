package chainlist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	dto "chainlist-parser/internal/adapter/storage/chainlist/dto"
	"chainlist-parser/internal/config"
	"chainlist-parser/internal/domain/entity"
	domainRepo "chainlist-parser/internal/domain/repository"
	"chainlist-parser/internal/pkg/apperrors"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

// Compile-time check
var _ domainRepo.ChainRepository = (*Repository)(nil)

// Repository implements ChainRepository for fetching data from the Chainlist source.
type Repository struct {
	client *fasthttp.Client
	url    string
	logger *zap.Logger
}

// NewRepository creates a new Chainlist repository instance. It configures the HTTP client and stores the Chainlist URL.
func NewRepository(cfg config.ChainlistConfig, logger *zap.Logger) domainRepo.ChainRepository {
	return &Repository{
		client: &fasthttp.Client{},
		url:    cfg.URL,
		logger: logger.Named("ChainlistStorage"),
	}
}

// GetAllChains fetches the full list of chains from the configured Chainlist URL.
func (r *Repository) GetAllChains(ctx context.Context) ([]entity.Chain, error) {
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
		return nil, fmt.Errorf("%w: failed to execute request to Chainlist: %v",
			apperrors.ErrExternalServiceFailure, err,
		)
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
		return nil, fmt.Errorf("%w: chainlist returned status %d",
			apperrors.ErrExternalServiceFailure, resp.StatusCode(),
		)
	}

	var body []byte
	contentEncoding := resp.Header.Peek(fasthttp.HeaderContentEncoding)
	if bytes.EqualFold(contentEncoding, []byte("gzip")) {
		r.logger.Debug("Received gzipped response from Chainlist")
		body, err = resp.BodyGunzip()
		if err != nil {
			r.logger.Error("Failed to gunzip Chainlist response body", zap.Error(err))
			return nil, fmt.Errorf("%w: failed to decompress chainlist response: %v",
				apperrors.ErrExternalServiceFailure, err,
			)
		}
	} else {
		body = resp.Body()
	}

	var rawChains []dto.ChainRaw
	err = json.Unmarshal(body, &rawChains)
	if err != nil {
		r.logger.Error("Failed to unmarshal Chainlist response into raw DTOs",
			zap.Error(err), zap.ByteString("bodySample", body[:min(1024, len(body))]),
		)
		return nil, fmt.Errorf("%w: failed to parse chainlist response into raw DTOs: %v",
			apperrors.ErrExternalServiceFailure, err,
		)
	}

	r.logger.Info("Successfully fetched and unmarshaled raw chains from Chainlist",
		zap.Int("count", len(rawChains)),
	)

	domainChains := toDomainChains(rawChains, r.logger)
	r.logger.Info("Successfully mapped raw DTOs to domain entities", zap.Int("count", len(domainChains)))

	return domainChains, nil
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
