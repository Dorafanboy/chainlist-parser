package usecase

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"chainlist-parser/internal/config"
	"chainlist-parser/internal/entity"
)

// Compile-time check to ensure chainUseCase implements ChainUseCase
var _ ChainUseCase = (*chainUseCase)(nil)

type chainUseCase struct {
	chainRepo  ChainRepository
	cacheRepo  CacheRepository
	rpcChecker RPCChecker
	logger     *zap.Logger
	cfg        config.Config
}

func NewChainUseCase(
	chainRepo ChainRepository,
	cacheRepo CacheRepository,
	rpcChecker RPCChecker,
	logger *zap.Logger,
	cfg config.Config,
) ChainUseCase {
	uc := &chainUseCase{
		chainRepo:  chainRepo,
		cacheRepo:  cacheRepo,
		rpcChecker: rpcChecker,
		logger:     logger.Named("ChainUseCase"),
		cfg:        cfg,
	}
	go uc.startBackgroundChecker()
	return uc
}

// GetAllChainsChecked gets all chains, potentially from cache or by fetching and checking.
func (uc *chainUseCase) GetAllChainsChecked(ctx context.Context) ([]entity.Chain, error) {
	// 1. Try to get from cache
	chains, err := uc.cacheRepo.GetChains(ctx)
	if err == nil && len(chains) > 0 {
		uc.logger.Debug("Cache hit for all chains")
		return chains, nil
	}
	uc.logger.Debug("Cache miss or error for all chains", zap.Error(err))

	// 2. If cache miss, fetch, check, and cache
	return uc.fetchCheckAndCacheAllChains(ctx)
}

// GetCheckedRPCsForChain gets checked RPC details for a specific chain, potentially from cache.
func (uc *chainUseCase) GetCheckedRPCsForChain(ctx context.Context, chainID int64) ([]entity.RPCDetail, error) {
	// 1. Try cache first for the specific chain's checked RPCs
	checkedRPCs, err := uc.cacheRepo.GetChainCheckedRPCs(ctx, chainID)
	if err == nil && len(checkedRPCs) > 0 {
		uc.logger.Debug("Cache hit for chain checked RPCs", zap.Int64("chainId", chainID))
		return checkedRPCs, nil
	}
	uc.logger.Debug("Cache miss or error for chain checked RPCs", zap.Int64("chainId", chainID), zap.Error(err))

	// 2. Fetch all chains from the repository to find the specific one
	uc.logger.Debug("Fetching all chains to find specific chain info for checking", zap.Int64("chainId", chainID))
	rawChains, err := uc.chainRepo.GetAllChains(ctx)
	if err != nil {
		uc.logger.Error("Failed to get all chains from repo while looking for specific chain RPCs",
			zap.Int64("chainId", chainID), zap.Error(err))
		return nil, err // Return error if fetching failed
	}

	// 3. Find the specific chain
	var foundChain *entity.Chain
	for i := range rawChains {
		if rawChains[i].ChainID == chainID {
			foundChain = &rawChains[i]
			break
		}
	}

	// 4. Handle chain not found
	if foundChain == nil {
		uc.logger.Warn("Chain not found in repository data", zap.Int64("chainId", chainID))
		return nil, nil // Return nil to indicate not found, handler will return 404 or appropriate response
	}

	uc.logger.Debug("Found chain, checking its RPCs", zap.Int64("chainId", chainID), zap.Int("rpcCount", len(foundChain.RPC)))

	// 5. Check RPCs only for the found chain
	checkerTimeout := uc.cfg.Checker.GetTimeout()
	chainCheckedRPCs := uc.checkChainRPCs(checkerTimeout, foundChain.RPC)
	uc.logger.Debug("Finished checking RPCs for chain", zap.Int64("chainId", chainID), zap.Int("checkedCount", len(chainCheckedRPCs)))

	// 6. Cache the result for this specific chain
	cacheTTL := uc.cfg.Checker.GetCacheTTL()
	err = uc.cacheRepo.SetChainCheckedRPCs(ctx, chainID, chainCheckedRPCs, cacheTTL)
	if err != nil {
		// Log caching error but don't fail the request
		uc.logger.Error("Failed to cache checked RPCs for chain", zap.Int64("chainId", chainID), zap.Error(err))
	}

	// 7. Return the checked RPC details
	return chainCheckedRPCs, nil
}

// fetchCheckAndCacheAllChains fetches from source, checks RPCs, and updates cache.
func (uc *chainUseCase) fetchCheckAndCacheAllChains(ctx context.Context) ([]entity.Chain, error) {
	uc.logger.Info("Starting to fetch, check, and cache all chains")

	// 1. Fetch raw chain data
	rawChains, err := uc.chainRepo.GetAllChains(ctx)
	if err != nil {
		uc.logger.Error("Failed to fetch raw chain data", zap.Error(err))
		return nil, err
	}

	if len(rawChains) == 0 {
		uc.logger.Warn("Fetched 0 chains from repository")
		return []entity.Chain{}, nil // Return empty slice, not nil
	}
	uc.logger.Debug("Fetched raw chains", zap.Int("count", len(rawChains)))

	updatedChains := make([]entity.Chain, len(rawChains))
	resultsChan := make(chan struct {
		index       int
		checkedRPCs []entity.RPCDetail
	}, len(rawChains))
	var wg sync.WaitGroup
	checkerTimeout := uc.cfg.Checker.GetTimeout() // Use helper method from config

	// 2. Check RPCs for all chains in parallel
	for i, chain := range rawChains {
		wg.Add(1)
		go func(index int, c entity.Chain) {
			defer wg.Done()
			checkedRPCs := uc.checkChainRPCs(checkerTimeout, c.RPC)
			resultsChan <- struct {
				index       int
				checkedRPCs []entity.RPCDetail
			}{index: index, checkedRPCs: checkedRPCs}
		}(i, chain)
	}

	// Wait for all checkers to finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results and update the chain entities
	for result := range resultsChan {
		updatedChains[result.index] = rawChains[result.index]
		updatedChains[result.index].CheckedRPCs = result.checkedRPCs
	}

	uc.logger.Info("Finished checking chains", zap.Int("totalChains", len(updatedChains)))

	// 3. Cache results (both the full list and individual chain details)
	cacheTTL := uc.cfg.Checker.GetCacheTTL() // Correct: Use method from CheckerConfig
	err = uc.cacheRepo.SetChains(ctx, updatedChains, cacheTTL)
	if err != nil {
		uc.logger.Error("Failed to cache checked chains (full list)", zap.Error(err))
	}

	// Also cache individual chain checked RPCs for faster lookup
	for _, chain := range updatedChains {
		// Use the same TTL for individual entries
		err = uc.cacheRepo.SetChainCheckedRPCs(ctx, chain.ChainID, chain.CheckedRPCs, cacheTTL)
		if err != nil {
			uc.logger.Error("Failed to cache individual chain checked RPCs", zap.Int64("chainId", chain.ChainID), zap.Error(err))
		}
	}

	return updatedChains, nil
}

// checkChainRPCs checks a list of RPC URLs for a single chain and returns detailed results.
func (uc *chainUseCase) checkChainRPCs(timeout time.Duration, rpcs []string) []entity.RPCDetail {
	if len(rpcs) == 0 {
		return nil
	}

	checkedRPCs := make([]entity.RPCDetail, len(rpcs))
	var wg sync.WaitGroup

	// Check each RPC in parallel
	for i, rpcURL := range rpcs {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()

			detail := entity.RPCDetail{URL: url}

			// Determine protocol
			if strings.HasPrefix(url, "https://") {
				detail.Protocol = "https"
			} else if strings.HasPrefix(url, "http://") {
				detail.Protocol = "http"
			} else if strings.HasPrefix(url, "wss://") {
				detail.Protocol = "wss"
				// Do not check WSS, leave IsWorking and LatencyMs as nil
				checkedRPCs[index] = detail
				return
			} else {
				detail.Protocol = "unknown"
				// Mark as not working if protocol unknown/unsupported for check
				notWorking := false
				detail.IsWorking = &notWorking
				checkedRPCs[index] = detail
				return
			}

			// Perform the check for http/https
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			isWorking, latency, err := uc.rpcChecker.CheckRPC(ctx, url)
			if err != nil {
				uc.logger.Debug("RPC check failed", zap.String("rpc", url), zap.Error(err))
				// Mark as not working on error
				notWorking := false
				detail.IsWorking = &notWorking
			} else {
				detail.IsWorking = &isWorking
				if isWorking {
					latencyMs := latency.Milliseconds()
					detail.LatencyMs = &latencyMs
					uc.logger.Debug("RPC is working", zap.String("rpc", url), zap.Duration("latency", latency))
				} else {
					uc.logger.Debug("RPC is not working", zap.String("rpc", url))
				}
			}
			checkedRPCs[index] = detail
		}(i, rpcURL)
	}

	wg.Wait() // Wait for all goroutines to complete

	return checkedRPCs
}

// startBackgroundChecker periodically re-checks all chains.
func (uc *chainUseCase) startBackgroundChecker() {
	interval := uc.cfg.Checker.GetCheckInterval()
	if interval <= 0 {
		uc.logger.Info("Background checker disabled (interval <= 0)")
		return
	}

	uc.logger.Info("Starting background checker", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately on start only if configured
	if uc.cfg.Checker.RunOnStartup {
		go func() {
			uc.logger.Info("Running initial background check...")
			_, err := uc.fetchCheckAndCacheAllChains(context.Background())
			if err != nil {
				uc.logger.Error("Error during initial background check", zap.Error(err))
			}
		}()
	} else {
		uc.logger.Info("Skipping initial background check (RunOnStartup is false)")
	}

	for range ticker.C {
		uc.logger.Info("Background checker running...")
		_, err := uc.fetchCheckAndCacheAllChains(context.Background()) // Use background context
		if err != nil {
			uc.logger.Error("Error during background check", zap.Error(err))
		}
	}
}
