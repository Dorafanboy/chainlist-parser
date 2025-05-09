package repository

import (
	"context"
	"time"

	"chainlist-parser/internal/domain/entity"
)

// CacheRepository defines the interface for caching checked chain data.
type CacheRepository interface {
	// GetChains retrieves the cached list of all chains.
	GetChains(ctx context.Context) ([]entity.Chain, bool, error)

	// SetChains stores the list of all chains in the cache with a specified TTL.
	SetChains(ctx context.Context, chains []entity.Chain, ttl time.Duration) error

	// GetChainCheckedRPCs retrieves the cached list of checked RPC details for a specific chain ID.
	GetChainCheckedRPCs(ctx context.Context, chainID int64) ([]entity.RPCDetail, bool, error)
	
	// SetChainCheckedRPCs stores the list of checked RPC details for a specific chain ID in the cache with a specified TTL.
	SetChainCheckedRPCs(ctx context.Context, chainID int64, rpcs []entity.RPCDetail, ttl time.Duration) error
}
