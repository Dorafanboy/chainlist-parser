package domain

import "errors"

var (
	// ErrChainNotFound means the requested chain was not found.
	ErrChainNotFound = errors.New("chain not found")

	// ErrNoRPCsAvailable means there are no available or working RPCs for the chain.
	ErrNoRPCsAvailable = errors.New("no RPCs available for the chain")

	// ErrUpstreamSourceFailure means an error occurred while fetching data from the upstream source (e.g., chainid.network).
	ErrUpstreamSourceFailure = errors.New("upstream source failure")

	// ErrCacheFailure means an internal error occurred while interacting with the cache (not a cache miss).
	ErrCacheFailure = errors.New("cache operation failed")
)
