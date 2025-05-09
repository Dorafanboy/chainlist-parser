package port

import (
	"context"

	"chainlist-parser/internal/domain/entity"
)

// ChainService defines the interface for the core business logic related to fetching and checking chain data.
type ChainService interface {
	// GetAllChainsChecked gets all chains, returning cached data or triggering background checks.
	GetAllChainsChecked(ctx context.Context) ([]entity.Chain, error)

	// GetCheckedRPCsForChain gets checked RPC details for a specific chain.
	GetCheckedRPCsForChain(ctx context.Context, chainID int64) ([]entity.RPCDetail, error)
}
