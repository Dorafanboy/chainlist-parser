package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"chainlist-parser/internal/config"
	"chainlist-parser/internal/entity"

	// "chainlist-parser/internal/pkg/apperrors" // Commenting out as this was part of the changes to be reverted

	"go.uber.org/zap"
)

var (
	// ErrChainNotFound означает, что запрошенная цепочка не найдена.
	ErrChainNotFound = errors.New("chain not found")
	// ErrNoRPCsAvailable означает, что для цепочки нет доступных или работающих RPC.
	ErrNoRPCsAvailable = errors.New("no RPCs available for the chain")
	// ErrUpstreamSourceFailure означает ошибку при получении данных из основного источника (например, chainid.network).
	ErrUpstreamSourceFailure = errors.New("upstream source failure")
	// ErrCacheFailure означает внутреннюю ошибку при работе с кешем (не cache miss).
	ErrCacheFailure = errors.New("cache operation failed")
)

// Compile-time check to ensure chainUseCase implements ChainService
var _ ChainService = (*chainUseCase)(nil)

// Compile-time check to ensure chainUseCase implements ChainUseCase (old interface name if it exists)
// var _ ChainUseCase = (*chainUseCase)(nil) // REMOVED/COMMENTED: Old check if it existed and used struct name

// chainUseCase implements the ChainService interface orchestrating chain data operations.
type chainUseCase struct {
	chainRepo  ChainRepository
	cacheRepo  CacheRepository
	rpcChecker RPCChecker
	logger     *zap.Logger
	cfg        config.Config
	rootCtx    context.Context // Context for background tasks
	isChecking *atomic.Bool    // Atomic flag to prevent multiple concurrent checks of all chains
}

// NewChainUseCase creates a new instance of the chain use case.
func NewChainUseCase(
	rootCtx context.Context,
	chainRepo ChainRepository,
	cacheRepo CacheRepository,
	rpcChecker RPCChecker,
	logger *zap.Logger,
	cfg config.Config,
) ChainUseCase { // Assuming ChainUseCase is the interface type returned
	uc := &chainUseCase{
		chainRepo:  chainRepo,
		cacheRepo:  cacheRepo,
		rpcChecker: rpcChecker,
		logger:     logger.Named("ChainUseCase"),
		cfg:        cfg,
		rootCtx:    rootCtx,
		isChecking: new(atomic.Bool),
	}

	// Start background checker without waiting for it to complete
	go uc.startBackgroundChecker()

	return uc
}

// GetAllChainsChecked gets all chains, returning cached data or triggering background checks.
// This method aims to provide a quick response, potentially with slightly stale data
// if a background check is in progress.
func (uc *chainUseCase) GetAllChainsChecked(ctx context.Context) ([]entity.Chain, error) {
	// Attempt to get from cache first
	// This cache should ideally store the chains *with* their checked RPCs.
	cachedChains, found, err := uc.cacheRepo.GetChains(ctx) // Assuming GetChains returns []entity.Chain with CheckedRPCs
	if err != nil {
		// Log cache error but proceed, as cache is not the source of truth.
		uc.logger.Warn("Cache error when getting all chains", zap.Error(err))
		// No return here, proceed to fetch
	}
	if found {
		uc.logger.Debug("Cache hit for all chains (potentially checked)")
		return cachedChains, nil
	}

	// Cache miss, fetch raw data
	uc.logger.Debug("Cache miss for all chains, fetching raw data...")
	rawChains, fetchErr := uc.chainRepo.GetAllChains(ctx)
	if fetchErr != nil {
		// Don't log again here if chainRepo already logged it.
		// uc.logger.Error("Failed to fetch raw chain data from repository", zap.Error(fetchErr)) // REMOVED
		return nil, fmt.Errorf("failed to fetch all chains from repository: %w", fetchErr)
	}

	// If chains are fetched successfully, trigger a background check if not already running.
	// The current request will return the raw (unchecked or stale) data immediately.
	// Subsequent requests will get updated data from cache once the background check completes.
	if len(rawChains) > 0 {
		// Use atomic bool to prevent multiple concurrent performRpcChecksAndUpdateCache calls
		if uc.isChecking.CompareAndSwap(false, true) {
			uc.logger.Info("Starting background check process after cache miss")
			// Pass the rootCtx for the background task, not the request-specific ctx
			go func(chainsToCheck []entity.Chain) {
				defer uc.isChecking.Store(false) // Reset the flag when done
				uc.performRpcChecksAndUpdateCache(uc.rootCtx, chainsToCheck)
			}(rawChains) // Pass a copy if rawChains could be modified elsewhere, though it's local here
		} else {
			uc.logger.Debug("Background check already in progress, skipping new check start")
		}
	}

	// Return raw chains immediately; background task will update cache with checked RPCs.
	// The `CheckedRPCs` field in `rawChains` will be empty or reflect very old data here.
	uc.logger.Info("Fetched raw chains, returning immediately", zap.Int("count", len(rawChains)))
	return rawChains, nil
}

// GetCheckedRPCsForChain gets checked RPC details for a specific chain.
// It tries the cache first. If missed, it fetches all chains, checks RPCs for the specific chain,
// caches the result for that chain, and returns it.
func (uc *chainUseCase) GetCheckedRPCsForChain(ctx context.Context, chainID int64) ([]entity.RPCDetail, error) {
	// Try to get specifically checked RPCs for this chain from cache
	checkedRPCs, found, err := uc.cacheRepo.GetChainCheckedRPCs(ctx, chainID)
	if err != nil {
		// Log cache error but proceed.
		uc.logger.Warn("Cache error when getting checked RPCs for chain", zap.Int64("chainId", chainID), zap.Error(err))
		// No return here
	}
	if found {
		uc.logger.Debug("Cache hit for chain checked RPCs", zap.Int64("chainId", chainID))
		if len(checkedRPCs) == 0 {
			// This means we previously checked and found no working RPCs.
			return nil, fmt.Errorf("no working RPCs found for chain %d (cached result)", chainID)
		}
		return checkedRPCs, nil
	}

	// Cache miss for specific chain's RPCs.
	uc.logger.Debug("Cache miss for chain checked RPCs, fetching from repository", zap.Int64("chainId", chainID))
	rawChains, fetchErr := uc.chainRepo.GetAllChains(ctx) // Fetch all raw chains
	if fetchErr != nil {
		// Don't log again if chainRepo already logged it.
		// uc.logger.Error("Failed to get all chains from repo for specific chain RPC check", zap.Int64("chainId", chainID), zap.Error(fetchErr)) // REMOVED
		return nil, fmt.Errorf("repository.GetAllChains for chain %d failed: %w", chainID, fetchErr)
	}

	var foundChain *entity.Chain
	for i := range rawChains { // Iterate by index to take address
		if rawChains[i].ChainID == chainID {
			foundChain = &rawChains[i]
			break
		}
	}

	if foundChain == nil {
		// Log this specific use case error condition
		uc.logger.Warn("Chain not found in repository data", zap.Int64("chainId", chainID))
		return nil, fmt.Errorf("chain with ID %d not found in repository", chainID)
	}

	// Now check RPCs for this specific found chain
	uc.logger.Debug("Found chain, checking its RPCs", zap.Int64("chainId", chainID), zap.Int("rpcCount", len(foundChain.RPC)))
	checkerTimeout := uc.cfg.Checker.GetTimeout() // Get configured timeout for a single RPC check
	chainCheckedRPCsResult := uc.checkChainRPCs(checkerTimeout, foundChain.RPC)

	// Cache the checked RPCs for this specific chain
	cacheTTL := uc.cfg.Checker.GetCacheTTL()
	if cacheErr := uc.cacheRepo.SetChainCheckedRPCs(ctx, chainID, chainCheckedRPCsResult, cacheTTL); cacheErr != nil {
		// Log error originating from this action (cache write)
		uc.logger.Error("Failed to cache checked RPCs for chain (after check)",
			zap.Int64("chainId", chainID), zap.Error(cacheErr))
		// Don't fail the request for a cache write error.
	}

	if len(chainCheckedRPCsResult) == 0 {
		// Log this specific outcome
		uc.logger.Debug("No working RPCs found after check for chain", zap.Int64("chainId", chainID))
		return nil, fmt.Errorf("no working RPCs found for chain %d after check", chainID)
	}

	uc.logger.Debug("Finished checking RPCs for specific chain",
		zap.Int64("chainId", chainID), zap.Int("checkedCount", len(chainCheckedRPCsResult)))
	return chainCheckedRPCsResult, nil
}

// performRpcChecksAndUpdateCache is a core internal function that takes a list of raw chains,
// checks all their RPCs, and then updates the main cache with these fully checked chains.
// This function is intended to be called in a background goroutine.
func (uc *chainUseCase) performRpcChecksAndUpdateCache(ctx context.Context, rawChains []entity.Chain) {
	uc.logger.Info("Starting background RPC checks and cache update", zap.Int("chainCount", len(rawChains)))

	if len(rawChains) == 0 {
		uc.logger.Warn("performRpcChecksAndUpdateCache called with 0 chains")
		return
	}

	// Create a new slice to store chains with their checked RPCs.
	updatedChains := make([]entity.Chain, len(rawChains))

	// Channel to receive results from goroutines, one result per chain.
	// Each result contains the index and the checked RPCs for that chain.
	resultsChan := make(chan struct {
		index       int
		checkedRPCs []entity.RPCDetail
	}, len(rawChains))

	var wg sync.WaitGroup
	checkerTimeout := uc.cfg.Checker.GetTimeout() // Timeout for a single RPC check.

	// Semaphore to limit concurrent chain processing (i.e., how many chains are having their RPCs checked simultaneously)
	// This is different from maxWorkers for individual RPC checks within a chain.
	// Let's use a simpler approach: MaxWorkers limits total RPC checking goroutines.
	// The checkChainRPCs itself uses MaxWorkers for its internal RPC checks.
	// Here, we can iterate over chains sequentially and call checkChainRPCs for each,
	// or parallelize the iteration over chains as well.
	// For now, let's process chains sequentially, but each chain's RPCs are checked in parallel by checkChainRPCs.

	// If we want to parallelize processing of *chains* themselves:
	numChainWorkers := uc.cfg.Checker.MaxWorkers / 5 // Example: if MaxWorkers is 40, use 8 workers for chains. Adjust as needed.
	if numChainWorkers <= 0 {
		numChainWorkers = 1 // At least one worker
	}
	if numChainWorkers > len(rawChains) {
		numChainWorkers = len(rawChains)
	}
	chainJobs := make(chan int, len(rawChains)) // Send indices of chains to process

	for w := 0; w < numChainWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			uc.logger.Debug("Starting chain processing worker", zap.Int("workerID", workerID))
			for chainIndex := range chainJobs {
				select {
				case <-ctx.Done():
					uc.logger.Info("Context cancelled, chain processing worker shutting down", zap.Int("workerID", workerID))
					return
				default:
				}

				chainToProcess := rawChains[chainIndex]
				uc.logger.Debug("Chain processing worker checking chain", zap.Int("workerID", workerID), zap.String("chainName", chainToProcess.Name))

				checkedRPCs := uc.checkChainRPCs(checkerTimeout, chainToProcess.RPC)

				select {
				case resultsChan <- struct {
					index       int
					checkedRPCs []entity.RPCDetail
				}{index: chainIndex, checkedRPCs: checkedRPCs}:
				case <-ctx.Done():
					uc.logger.Info("Context cancelled before sending result for chain", zap.String("chainName", chainToProcess.Name))
					return
				}
			}
			uc.logger.Debug("Chain processing worker finished", zap.Int("workerID", workerID))
		}(w)
	}

	for i := 0; i < len(rawChains); i++ {
		select {
		case chainJobs <- i:
		case <-ctx.Done():
			uc.logger.Info("Context cancelled before sending all chain jobs")
			close(chainJobs)
			goto collectResults
		}
	}
	close(chainJobs)

collectResults:
	go func() {
		wg.Wait()
		close(resultsChan)
		uc.logger.Debug("All chain processing workers finished, results channel closed.")
	}()

	processedCount := 0
	for result := range resultsChan {
		select {
		case <-ctx.Done():
			uc.logger.Info("Context cancelled while collecting results")
			return
		default:
		}
		if result.index >= 0 && result.index < len(rawChains) {
			chainCopy := rawChains[result.index] // Make a copy
			chainCopy.CheckedRPCs = result.checkedRPCs
			updatedChains[result.index] = chainCopy
			processedCount++
		} else {
			uc.logger.Warn("Received result with out-of-bounds index", zap.Int("index", result.index), zap.Int("lenRawChains", len(rawChains)))
		}
	}
	uc.logger.Info("Finished collecting results for chain RPC checks", zap.Int("processedCount", processedCount), zap.Int("expectedCount", len(rawChains)))

	if ctx.Err() != nil {
		uc.logger.Warn("Context cancelled before caching results in background task, cache not updated.", zap.Error(ctx.Err()))
		return
	}

	uc.logger.Info("Attempting to set all checked chains to cache", zap.Int("count", len(updatedChains)))
	cacheTTL := uc.cfg.Checker.GetCacheTTL()
	if err := uc.cacheRepo.SetChains(ctx, updatedChains, cacheTTL); err != nil {
		uc.logger.Error("Failed to cache checked chains in background task", zap.Error(err))
	} else {
		uc.logger.Info("Successfully cached all chains with checked RPCs.")
	}

	for _, chain := range updatedChains {
		if ctx.Err() != nil {
			uc.logger.Info("Context cancelled before caching individual chain RPCs")
			return
		}
		if err := uc.cacheRepo.SetChainCheckedRPCs(ctx, chain.ChainID, chain.CheckedRPCs, cacheTTL); err != nil {
			uc.logger.Warn("Failed to cache individual chain's checked RPCs in background task",
				zap.Int64("chainId", chain.ChainID), zap.Error(err))
		}
	}

	uc.logger.Info("Background RPC check and cache update finished.")
}

// checkChainRPCs performs parallel RPC checks for a list of URLs.
func (uc *chainUseCase) checkChainRPCs(timeout time.Duration, rpcs []string) []entity.RPCDetail {
	if len(rpcs) == 0 {
		return nil
	}

	checkedRPCs := make([]entity.RPCDetail, len(rpcs))
	var wg sync.WaitGroup

	numWorkers := uc.cfg.Checker.MaxWorkers
	if numWorkers <= 0 {
		numWorkers = 10
	}
	if len(rpcs) < numWorkers {
		numWorkers = len(rpcs)
	}

	jobChan := make(chan struct {
		index int
		url   string
	}, len(rpcs))

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			uc.logger.Debug("Starting RPC check sub-worker", zap.Int("workerID", workerID))
			for job := range jobChan {
				detail := entity.RPCDetail{URL: job.url}

				if strings.HasPrefix(job.url, "https://") {
					detail.Protocol = entity.ProtocolHTTPS
				} else if strings.HasPrefix(job.url, "http://") {
					detail.Protocol = entity.ProtocolHTTP
				} else if strings.HasPrefix(job.url, "wss://") {
					detail.Protocol = entity.ProtocolWSS
				} else if strings.HasPrefix(job.url, "ws://") {
					detail.Protocol = entity.ProtocolWS
				} else {
					detail.Protocol = entity.ProtocolUnknown
					notWorking := false
					detail.IsWorking = &notWorking
					checkedRPCs[job.index] = detail
					uc.logger.Warn("Unknown protocol for RPC, marked as not working", zap.String("url", job.url))
					continue
				}

				checkCtx, cancel := context.WithTimeout(uc.rootCtx, timeout)

				isWorking, latency, err := uc.rpcChecker.CheckRPC(checkCtx, job.url)
				cancel()

				if err != nil {
					uc.logger.Debug("RPC check failed", zap.String("rpc", job.url), zap.Error(err))
					notWorkingStatus := false
					detail.IsWorking = &notWorkingStatus
				} else {
					detail.IsWorking = &isWorking
					if isWorking {
						latencyMs := latency.Milliseconds()
						detail.LatencyMs = &latencyMs
						uc.logger.Debug("RPC is working", zap.String("rpc", job.url), zap.Duration("latency", latency))
					} else {
						uc.logger.Debug("RPC is not working (checker reported)", zap.String("rpc", job.url))
					}
				}
				checkedRPCs[job.index] = detail
			}
			uc.logger.Debug("RPC check sub-worker finished", zap.Int("workerID", workerID))
		}(w)
	}

	for i, rpcURL := range rpcs {
		jobChan <- struct {
			index int
			url   string
		}{index: i, url: rpcURL}
	}
	close(jobChan)

	wg.Wait()

	if rpcs == nil && checkedRPCs != nil && len(checkedRPCs) == 0 {
		return []entity.RPCDetail{}
	}
	if checkedRPCs == nil && len(rpcs) > 0 {
		return make([]entity.RPCDetail, 0)
	}

	return checkedRPCs
}

// startBackgroundChecker runs a periodic task to fetch and check all chains.
func (uc *chainUseCase) startBackgroundChecker() {
	interval := uc.cfg.Checker.GetCheckInterval()
	if interval <= 0 {
		uc.logger.Info("Background checker disabled (interval <= 0)")
		return
	}

	uc.logger.Info("Starting background checker", zap.Duration("interval", interval))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if uc.isChecking.CompareAndSwap(false, true) {
				uc.logger.Info("Background checker tick: triggering new check...")
				rawChains, err := uc.chainRepo.GetAllChains(uc.rootCtx)
				if err != nil {
					if uc.rootCtx.Err() != nil {
						uc.logger.Warn("Periodic background check fetch cancelled due to application shutdown")
					} else {
						uc.logger.Error("Error fetching chains during periodic background check", zap.Error(err))
					}
					uc.isChecking.Store(false)
					continue
				}
				go func(chainsToCheck []entity.Chain) {
					defer uc.isChecking.Store(false)
					uc.performRpcChecksAndUpdateCache(uc.rootCtx, chainsToCheck)
				}(rawChains)
			} else {
				uc.logger.Debug("Background checker tick: check already in progress.")
			}

		case <-uc.rootCtx.Done():
			uc.logger.Info("Background checker stopping due to context cancellation.")
			return
		}
	}
}
