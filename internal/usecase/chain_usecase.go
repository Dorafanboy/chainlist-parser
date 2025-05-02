package usecase

import (
	"context"
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
	// Start background worker
	go uc.startBackgroundChecker()
	return uc
}

func (uc *chainUseCase) GetAllChainsWithWorkingRPCs(ctx context.Context) ([]entity.Chain, error) {
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

func (uc *chainUseCase) GetWorkingRPCsForChain(ctx context.Context, chainID int64) ([]string, error) {
	// 1. Try cache first
	rpcs, err := uc.cacheRepo.GetChainRPCs(ctx, chainID)
	if err == nil && len(rpcs) > 0 {
		uc.logger.Debug("Cache hit for chain RPCs", zap.Int64("chainId", chainID))
		return rpcs, nil
	}
	uc.logger.Debug("Cache miss or error for chain RPCs", zap.Int64("chainId", chainID), zap.Error(err))

	// 2. Fetch all chains from the repository to find the specific one
	uc.logger.Debug("Fetching all chains to find specific chain info", zap.Int64("chainId", chainID))
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
		// Return nil, nil to indicate not found, handler will return 404
		return nil, nil
	}

	uc.logger.Debug("Found chain, checking its RPCs", zap.Int64("chainId", chainID), zap.Int("rpcCount", len(foundChain.RPC)))

	// 5. Check RPCs only for the found chain
	checkerTimeout := uc.cfg.Checker.GetTimeout()
	workingRPCs := uc.checkChainRPCs(checkerTimeout, foundChain.RPC)
	uc.logger.Debug("Finished checking RPCs for chain", zap.Int64("chainId", chainID), zap.Int("workingCount", len(workingRPCs)))

	// 6. Cache the result for this specific chain
	cacheTTL := uc.cfg.Cache.GetTTL()
	err = uc.cacheRepo.SetChainRPCs(ctx, chainID, workingRPCs, cacheTTL)
	if err != nil {
		// Log caching error but don't fail the request
		uc.logger.Error("Failed to cache working RPCs for chain", zap.Int64("chainId", chainID), zap.Error(err))
	}

	// 7. Return the found working RPCs
	return workingRPCs, nil

	// uc.logger.Warn("Chain not found after fetching all chains", zap.Int64("chainId", chainID))
	// // Consider returning a specific error like ErrNotFound
	// return nil, nil // Or return an error indicating not found
}

// fetchCheckAndCacheAllChains fetches from source, checks RPCs, and updates cache.
func (uc *chainUseCase) fetchCheckAndCacheAllChains(ctx context.Context) ([]entity.Chain, error) {
	uc.logger.Info("Fetching and checking all chains...")

	// 1. Fetch raw chain data
	rawChains, err := uc.chainRepo.GetAllChains(ctx)
	if err != nil {
		uc.logger.Error("Failed to fetch chains from repository", zap.Error(err))
		return nil, err
	}

	checkedChains := make([]entity.Chain, len(rawChains))
	var wg sync.WaitGroup
	resultsChan := make(chan struct {
		index       int
		workingRPCs []string
	}, len(rawChains))
	checkerTimeout := uc.cfg.Checker.GetTimeout()

	// 2. Check RPCs in parallel
	for i, chain := range rawChains {
		wg.Add(1)
		go func(index int, c entity.Chain) {
			defer wg.Done()
			workingRPCs := uc.checkChainRPCs(checkerTimeout, c.RPC)
			resultsChan <- struct {
				index       int
				workingRPCs []string
			}{index: index, workingRPCs: workingRPCs}
		}(i, chain)
	}

	// Wait for all checkers to finish
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for result := range resultsChan {
		checkedChains[result.index] = rawChains[result.index] // Copy original data
		checkedChains[result.index].WorkingRPCs = result.workingRPCs
		// Optionally populate NonWorkingRPCs here as well
	}

	uc.logger.Info("Finished checking chains", zap.Int("totalChains", len(checkedChains)))

	// 3. Cache results
	cacheTTL := uc.cfg.Cache.GetTTL()
	err = uc.cacheRepo.SetChains(ctx, checkedChains, cacheTTL)
	if err != nil {
		uc.logger.Error("Failed to cache checked chains", zap.Error(err))
		// Continue even if caching fails, return the data
	}

	// Also cache individual chain RPCs for faster lookup
	for _, chain := range checkedChains {
		err = uc.cacheRepo.SetChainRPCs(ctx, chain.ChainID, chain.WorkingRPCs, cacheTTL)
		if err != nil {
			uc.logger.Error("Failed to cache individual chain RPCs", zap.Int64("chainId", chain.ChainID), zap.Error(err))
		}
	}

	return checkedChains, nil
}

// checkChainRPCs checks a list of RPC URLs for a single chain.
func (uc *chainUseCase) checkChainRPCs(timeout time.Duration, rpcs []string) []string {
	if len(rpcs) == 0 {
		return nil
	}

	var workingRPCs []string
	var wg sync.WaitGroup
	mutex := sync.Mutex{}
	rpcChan := make(chan string, len(rpcs))

	for _, rpc := range rpcs {
		wg.Add(1)
		go func(rpcURL string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), timeout) // Use background context for checks
			defer cancel()

			isWorking, latency, err := uc.rpcChecker.CheckRPC(ctx, rpcURL)
			if err != nil {
				uc.logger.Debug("RPC check failed", zap.String("rpc", rpcURL), zap.Error(err))
				return
			}
			if isWorking {
				uc.logger.Debug("RPC is working", zap.String("rpc", rpcURL), zap.Duration("latency", latency))
				rpcChan <- rpcURL
			} else {
				uc.logger.Debug("RPC is not working", zap.String("rpc", rpcURL))
			}
		}(rpc)
	}

	go func() {
		wg.Wait()
		close(rpcChan)
	}()

	for rpc := range rpcChan {
		mutex.Lock()
		workingRPCs = append(workingRPCs, rpc)
		mutex.Unlock()
	}

	return workingRPCs
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

	// Run once immediately on start?
	// go uc.fetchCheckAndCacheAllChains(context.Background())

	for range ticker.C {
		uc.logger.Info("Background checker running...")
		_, err := uc.fetchCheckAndCacheAllChains(context.Background()) // Use background context
		if err != nil {
			uc.logger.Error("Error during background check", zap.Error(err))
		}
	}
}
