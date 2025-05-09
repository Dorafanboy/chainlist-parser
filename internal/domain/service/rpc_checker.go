package service

import (
	"context"
	"time"

	"chainlist-parser/internal/domain/entity"
)

// RPCChecker defines the interface for checking RPC endpoint status.
type RPCChecker interface {
	CheckRPC(ctx context.Context, rpcURL entity.RPCURL) (bool, time.Duration, error)
}
