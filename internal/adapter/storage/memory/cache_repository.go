package memory

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chainlist-parser/internal/config"
	"chainlist-parser/internal/domain/entity"
	domainRepo "chainlist-parser/internal/domain/repository"

	"github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

// Compile-time check
var _ domainRepo.CacheRepository = (*CacheRepository)(nil)

// Cache keys
const (
	allChainsKeyPrefix        = "all_chains_v2"
	chainCheckedRPCsKeyPrefix = "chain_checked_rpcs_v2_"
)

// CacheRepository implements domainRepo.CacheRepository using the go-cache in-memory library.
type CacheRepository struct {
	cache   *cache.Cache
	logger  *zap.Logger
	fullCfg config.Config
}

// NewCacheRepository creates a new in-memory cache repository instance.
func NewCacheRepository(cfg config.Config, logger *zap.Logger) domainRepo.CacheRepository {
	defaultExpiration := cfg.Cache.GetDefaultExpiration()
	cleanupInterval := cfg.Cache.GetCleanupInterval()

	c := cache.New(defaultExpiration, cleanupInterval)
	logger.Info(
		"Initialized go-cache for memory storage",
		zap.Duration("defaultExpiration", defaultExpiration),
		zap.Duration("cleanupInterval", cleanupInterval),
	)

	return &CacheRepository{
		cache:   c,
		logger:  logger.Named("MemoryCacheStorage"),
		fullCfg: cfg,
	}
}

// GetChains retrieves the cached full list of chains, returning found status.
func (r *CacheRepository) GetChains(_ context.Context) ([]entity.Chain, bool, error) {
	key := allChainsKeyPrefix
	if x, found := r.cache.Get(key); found {
		if chains, ok := x.([]entity.Chain); ok {
			r.logger.Debug("Memory cache hit", zap.String("key", key))
			return chains, true, nil
		}
		r.logger.Warn(
			"Memory cache data type mismatch for key",
			zap.String("key", key), zap.Any("type", fmt.Sprintf("%T", x)),
		)
	}
	r.logger.Debug("Memory cache miss", zap.String("key", key))
	return nil, false, nil
}

// SetChains caches the full list of chains with a given TTL.
func (r *CacheRepository) SetChains(_ context.Context, chains []entity.Chain, ttl time.Duration) error {
	key := allChainsKeyPrefix
	if ttl <= 0 {
		ttl = r.fullCfg.Checker.GetCacheTTL()
		if ttl <= 0 {
			ttl = r.fullCfg.Cache.GetDefaultExpiration()
		}
	}
	r.cache.Set(key, chains, ttl)
	r.logger.Debug("Memory cache set", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

// GetChainCheckedRPCs retrieves cached checked RPCs for a chain, returning found status.
func (r *CacheRepository) GetChainCheckedRPCs(_ context.Context, chainID int64) ([]entity.RPCDetail, bool, error) {
	key := r.getChainCheckedRPCsKey(chainID)
	if x, found := r.cache.Get(key); found {
		if rpcs, ok := x.([]entity.RPCDetail); ok {
			r.logger.Debug("Memory cache hit", zap.String("key", key))
			return rpcs, true, nil
		}
		r.logger.Warn(
			"Memory cache data type mismatch for key",
			zap.String("key", key),
			zap.Any("type", fmt.Sprintf("%T", x)),
		)
	}
	r.logger.Debug("Memory cache miss", zap.String("key", key))
	return nil, false, nil
}

// SetChainCheckedRPCs caches the checked RPCs for a specific chain with a given TTL.
func (r *CacheRepository) SetChainCheckedRPCs(
	_ context.Context,
	chainID int64,
	rpcs []entity.RPCDetail,
	ttl time.Duration,
) error {
	key := r.getChainCheckedRPCsKey(chainID)
	if ttl <= 0 {
		ttl = r.fullCfg.Checker.GetCacheTTL()
		if ttl <= 0 {
			ttl = r.fullCfg.Cache.GetDefaultExpiration()
		}
	}
	r.cache.Set(key, rpcs, ttl)
	r.logger.Debug("Memory cache set", zap.String("key", key), zap.Duration("ttl", ttl))
	return nil
}

// getChainCheckedRPCsKey generates the cache key for a specific chain's checked RPCs.
func (r *CacheRepository) getChainCheckedRPCsKey(chainID int64) string {
	return chainCheckedRPCsKeyPrefix + strconv.FormatInt(chainID, 10)
}
