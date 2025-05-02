package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"chainlist-parser/internal/config"
	"chainlist-parser/internal/entity"
	"chainlist-parser/internal/usecase"
)

// Compile-time check
var _ usecase.ChainRepository = (*chainlistRepo)(nil)

type chainlistRepo struct {
	client *fasthttp.Client
	url    string
	logger *zap.Logger
}

func NewChainlistRepo(cfg config.ChainlistConfig, logger *zap.Logger) usecase.ChainRepository {
	return &chainlistRepo{
		client: &fasthttp.Client{},
		url:    cfg.URL,
		logger: logger.Named("ChainlistRepo"),
	}
}

func (r *chainlistRepo) GetAllChains(ctx context.Context) ([]entity.Chain, error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(r.url)
	req.Header.SetMethod(fasthttp.MethodGet)

	r.logger.Debug("Fetching chains from Chainlist", zap.String("url", r.url))

	// fasthttp doesn't directly support context cancellation for a single request easily.
	// You might need a more complex setup with timeouts or request abortion if strict context handling is needed here.
	err := r.client.Do(req, resp)
	if err != nil {
		r.logger.Error("Failed to execute request to Chainlist", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch from chainlist: %w", err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		r.logger.Error("Chainlist returned non-OK status",
			zap.Int("statusCode", resp.StatusCode()),
			zap.ByteString("body", resp.Body()))
		return nil, fmt.Errorf("chainlist returned status %d", resp.StatusCode())
	}

	var chains []entity.Chain
	err = json.Unmarshal(resp.Body(), &chains)
	if err != nil {
		r.logger.Error("Failed to unmarshal Chainlist response", zap.Error(err))
		return nil, fmt.Errorf("failed to parse chainlist response: %w", err)
	}

	r.logger.Info("Successfully fetched chains from Chainlist", zap.Int("count", len(chains)))
	return chains, nil
}
