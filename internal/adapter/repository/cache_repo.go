package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	cache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"

	"chainlist-parser/internal/config"
	"chainlist-parser/internal/entity"
	"chainlist-parser/internal/usecase"
)

// Compile-time check
var _ usecase.CacheRepository = (*goCacheRepo)(nil)

const (
	// Cache keys
	allChainsKeyPrefix = "all_chains"
	chainRPCsKeyPrefix = "chain_rpcs_"
)

type goCacheRepo struct {
	cache  *cache.Cache
	logger *zap.Logger
	cfg    config.CacheConfig
}

func NewGoCacheRepo(cfg config.CacheConfig, logger *zap.Logger) usecase.CacheRepository {
	defaultTTL := cfg.GetTTL()
	cleanupInterval := cfg.GetCleanupInterval()

	c := cache.New(defaultTTL, cleanupInterval)
	logger.Info("Initialized go-cache",
		zap.Duration("defaultTTL", defaultTTL),
		zap.Duration("cleanupInterval", cleanupInterval))

	return &goCacheRepo{
		cache:  c,
		logger: logger.Named("GoCacheRepo"),
		cfg:    cfg,
	}
}

func (r *goCacheRepo) GetChains(ctx context.Context) ([]entity.Chain, error) {
	key := allChainsKeyPrefix
	if x, found := r.cache.Get(key); found {
		if chains, ok := x.([]entity.Chain); ok {
			r.logger.Debug("Cache hit", zap.String("key", key))
			return chains, nil
		}
		r.logger.Warn("Cache data type mismatch for key", zap.String("key", key), zap.Any("type", fmt.Sprintf("%T", x)))
		// Treat type mismatch as cache miss
	}
	r.logger.Debug("Cache miss", zap.String("key", key))
	// Return nil, nil for cache miss (usecase layer interprets this)
	return nil, nil
}

func (r *goCacheRepo) SetChains(ctx context.Context, chains []entity.Chain, ttl time.Duration) error {
	key := allChainsKeyPrefix
	if ttl <= 0 {
		ttl = r.cfg.GetTTL() // Use default TTL if zero or negative
	}
	r.cache.Set(key, chains, ttl)
	r.logger.Debug("Cache set", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

func (r *goCacheRepo) GetChainRPCs(ctx context.Context, chainID int64) ([]string, error) {
	key := r.getChainRPCsKey(chainID)
	if x, found := r.cache.Get(key); found {
		if rpcs, ok := x.([]string); ok {
			r.logger.Debug("Cache hit", zap.String("key", key))
			return rpcs, nil
		}
		r.logger.Warn("Cache data type mismatch for key", zap.String("key", key), zap.Any("type", fmt.Sprintf("%T", x)))
	}
	r.logger.Debug("Cache miss", zap.String("key", key))
	return nil, nil
}

func (r *goCacheRepo) SetChainRPCs(ctx context.Context, chainID int64, rpcs []string, ttl time.Duration) error {
	key := r.getChainRPCsKey(chainID)
	if ttl <= 0 {
		ttl = r.cfg.GetTTL()
	}
	r.cache.Set(key, rpcs, ttl)
	r.logger.Debug("Cache set", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

// Helper to generate consistent cache keys
func (r *goCacheRepo) getChainRPCsKey(chainID int64) string {
	return chainRPCsKeyPrefix + strconv.FormatInt(chainID, 10)
}
