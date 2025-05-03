package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chainlist-parser/internal/config"
	"chainlist-parser/internal/entity"
	"chainlist-parser/internal/usecase"

	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

// Compile-time check
var _ usecase.CacheRepository = (*goCacheRepo)(nil)

const (
	// Cache keys
	allChainsKeyPrefix        = "all_chains"
	chainCheckedRPCsKeyPrefix = "chain_checked_rpcs_"
)

type goCacheRepo struct {
	cache   *cache.Cache
	logger  *zap.Logger
	fullCfg config.Config
}

// NewGoCacheRepo Updated constructor to accept full config
func NewGoCacheRepo(cfg config.Config, logger *zap.Logger) usecase.CacheRepository {
	defaultExpiration := cfg.Cache.GetDefaultExpiration()
	cleanupInterval := cfg.Cache.GetCleanupInterval()

	c := cache.New(defaultExpiration, cleanupInterval)
	logger.Info("Initialized go-cache",
		zap.Duration("defaultExpiration", defaultExpiration),
		zap.Duration("cleanupInterval", cleanupInterval))

	return &goCacheRepo{
		cache:   c,
		logger:  logger.Named("GoCacheRepo"),
		fullCfg: cfg,
	}
}

// GetChains retrieves the full list of chains
func (r *goCacheRepo) GetChains(ctx context.Context) ([]entity.Chain, error) {
	key := allChainsKeyPrefix
	if x, found := r.cache.Get(key); found {
		if chains, ok := x.([]entity.Chain); ok {
			r.logger.Debug("Cache hit", zap.String("key", key))
			return chains, nil
		}
		r.logger.Warn("Cache data type mismatch for key", zap.String("key", key), zap.Any("type", fmt.Sprintf("%T", x)))
	}
	r.logger.Debug("Cache miss", zap.String("key", key))
	return nil, nil
}

// SetChains caches the full list of chains
func (r *goCacheRepo) SetChains(ctx context.Context, chains []entity.Chain, ttl time.Duration) error {
	key := allChainsKeyPrefix
	if ttl <= 0 {
		ttl = r.fullCfg.Checker.GetCacheTTL()
	}
	r.cache.Set(key, chains, ttl)
	r.logger.Debug("Cache set", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

// GetChainCheckedRPCs retrieves the cached list of checked RPC details for a specific chain.
func (r *goCacheRepo) GetChainCheckedRPCs(ctx context.Context, chainID int64) ([]entity.RPCDetail, error) {
	key := r.getChainCheckedRPCsKey(chainID)
	if x, found := r.cache.Get(key); found {
		if rpcs, ok := x.([]entity.RPCDetail); ok {
			r.logger.Debug("Cache hit", zap.String("key", key))
			return rpcs, nil
		}
		r.logger.Warn("Cache data type mismatch for key", zap.String("key", key), zap.Any("type", fmt.Sprintf("%T", x)))
	}
	r.logger.Debug("Cache miss", zap.String("key", key))
	return nil, nil
}

// SetChainCheckedRPCs caches the list of checked RPC details for a specific chain.
func (r *goCacheRepo) SetChainCheckedRPCs(ctx context.Context, chainID int64, rpcs []entity.RPCDetail, ttl time.Duration) error {
	key := r.getChainCheckedRPCsKey(chainID)
	if ttl <= 0 {
		ttl = r.fullCfg.Checker.GetCacheTTL()
	}
	r.cache.Set(key, rpcs, ttl)
	r.logger.Debug("Cache set", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

// Helper to generate consistent cache keys for checked RPC details.
func (r *goCacheRepo) getChainCheckedRPCsKey(chainID int64) string {
	return chainCheckedRPCsKeyPrefix + strconv.FormatInt(chainID, 10)
}
