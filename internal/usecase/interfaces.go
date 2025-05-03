package usecase

import (
	"context"
	"time"

	"chainlist-parser/internal/entity"
)

// ChainRepository defines the interface for accessing chain data.
type ChainRepository interface {
	GetAllChains(ctx context.Context) ([]entity.Chain, error)
}

// CacheRepository defines the interface for caching checked chain data.
type CacheRepository interface {
	GetChains(ctx context.Context) ([]entity.Chain, bool, error)
	SetChains(ctx context.Context, chains []entity.Chain, ttl time.Duration) error
	GetChainCheckedRPCs(ctx context.Context, chainID int64) ([]entity.RPCDetail, bool, error)
	SetChainCheckedRPCs(ctx context.Context, chainID int64, rpcs []entity.RPCDetail, ttl time.Duration) error
}

// RPCChecker defines the interface for checking RPC endpoint status.
type RPCChecker interface {
	CheckRPC(ctx context.Context, rpcURL string) (bool, time.Duration, error)
}

// ChainUseCase defines the interface for chain related use cases.
type ChainUseCase interface {
	GetAllChainsChecked(ctx context.Context) ([]entity.Chain, error)
	GetCheckedRPCsForChain(ctx context.Context, chainID int64) ([]entity.RPCDetail, error)
}
