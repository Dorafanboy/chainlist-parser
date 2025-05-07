package usecase

import (
	"context"

	"chainlist-parser/internal/entity"
)

// ChainService defines the interface for the core business logic
// related to fetching and checking chain data.
// This is the primary interface used by delivery layers (e.g., HTTP handlers).
type ChainService interface {
	// GetAllChainsChecked gets all chains, returning cached data or triggering background checks.
	// The returned chains may have empty or stale CheckedRPCs if fetched from cache while checks are running.
	GetAllChainsChecked(ctx context.Context) ([]entity.Chain, error)

	// GetCheckedRPCsForChain gets checked RPC details for a specific chain.
	// It tries the cache first. If missed, it fetches necessary data and performs checks.
	GetCheckedRPCsForChain(ctx context.Context, chainID int64) ([]entity.RPCDetail, error)

	// Shutdown signals background tasks within the use case to stop gracefully.
	// Relying on context cancellation passed during initialization is often preferred.
	// Shutdown() // Keep commented out unless explicit shutdown is required beyond context.
}
