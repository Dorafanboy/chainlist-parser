package usecase

import (
	"context"
	"time"

	"chainlist-parser/internal/entity"
)

//go:generate mockgen -destination=mocks/mock_interfaces.go -package=mocks . ChainRepository,CacheRepository,RPCChecker

// ChainRepository defines the interface for accessing chain data.
type ChainRepository interface {
	GetAllChains(ctx context.Context) ([]entity.Chain, error)
}

// CacheRepository defines the interface for caching checked chain data.
type CacheRepository interface {
	GetChains(ctx context.Context) ([]entity.Chain, error)
	SetChains(ctx context.Context, chains []entity.Chain, ttl time.Duration) error
	GetChainRPCs(ctx context.Context, chainID int64) ([]string, error)
	SetChainRPCs(ctx context.Context, chainID int64, rpcs []string, ttl time.Duration) error
}

// RPCChecker defines the interface for checking RPC endpoint status.
type RPCChecker interface {
	CheckRPC(ctx context.Context, rpcURL string) (bool, time.Duration, error)
}

// ChainUseCase defines the interface for chain related use cases.
type ChainUseCase interface {
	GetAllChainsWithWorkingRPCs(ctx context.Context) ([]entity.Chain, error)
	GetWorkingRPCsForChain(ctx context.Context, chainID int64) ([]string, error)
}
