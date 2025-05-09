package repository

import (
	"context"

	"chainlist-parser/internal/domain/entity"
)

// ChainRepository defines the interface for accessing chain data.
type ChainRepository interface {
	// GetAllChains retrieves the list of all chains from the underlying data source.
	GetAllChains(ctx context.Context) ([]entity.Chain, error)
}
