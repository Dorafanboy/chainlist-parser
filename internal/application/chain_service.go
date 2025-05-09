package application

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"chainlist-parser/internal/application/port"
	"chainlist-parser/internal/config"
	"chainlist-parser/internal/domain"
	"chainlist-parser/internal/domain/entity"
	domainRepo "chainlist-parser/internal/domain/repository"
	domainService "chainlist-parser/internal/domain/service"

	"go.uber.org/zap"
)

// Compile-time check to ensure chainUseCase implements ChainService
var _ port.ChainService = (*chainService)(nil)

// chainService implements the port.ChainService interface orchestrating chain data operations.
type chainService struct {
	chainRepo  domainRepo.ChainRepository
	cacheRepo  domainRepo.CacheRepository
	rpcChecker domainService.RPCChecker
	logger     *zap.Logger
	cfg        config.Config
	rootCtx    context.Context
	isChecking *atomic.Bool
}

// NewChainService creates a new instance of the chain service.
func NewChainService(
	rootCtx context.Context,
	chainRepo domainRepo.ChainRepository,
	cacheRepo domainRepo.CacheRepository,
	rpcChecker domainService.RPCChecker,
	logger *zap.Logger,
	cfg config.Config,
) port.ChainService {
	uc := &chainService{
		chainRepo:  chainRepo,
		cacheRepo:  cacheRepo,
		rpcChecker: rpcChecker,
		logger:     logger.Named("ChainService"),
		cfg:        cfg,
		rootCtx:    rootCtx,
		isChecking: new(atomic.Bool),
	}

	go uc.startBackgroundChecker()

	return uc
}

// GetAllChainsChecked retrieves all chains, prioritizing cache, and falls back to the repository.
func (uc *chainService) GetAllChainsChecked(ctx context.Context) ([]entity.Chain, error) {
	cachedChains, found, err := uc.cacheRepo.GetChains(ctx)
	if err != nil {
		uc.logger.Warn("Cache error when getting all chains", zap.Error(err))
	}
	if found {
		uc.logger.Debug("Cache hit for all chains (potentially checked)")
		return cachedChains, nil
	}

	uc.logger.Debug("Cache miss for all chains, fetching raw data...")
	rawChains, fetchErr := uc.chainRepo.GetAllChains(ctx)
	if fetchErr != nil {
		return nil, fmt.Errorf("failed to fetch all chains from repository: %w", fetchErr)
	}

	if len(rawChains) > 0 {
		if uc.isChecking.CompareAndSwap(false, true) {
			uc.logger.Info("Starting background check process after cache miss")
			go func(chainsToCheck []entity.Chain) {
				defer uc.isChecking.Store(false)
				uc.performRpcChecksAndUpdateCache(uc.rootCtx, chainsToCheck)
			}(rawChains)
		} else {
			uc.logger.Debug("Background check already in progress, skipping new check start")
		}
	}
	uc.logger.Info("Fetched raw chains, returning immediately", zap.Int("count", len(rawChains)))
	return rawChains, nil
}

// GetCheckedRPCsForChain retrieves actively working RPCs for a specific chain ID.
func (uc *chainService) GetCheckedRPCsForChain(ctx context.Context, chainID int64) ([]entity.RPCDetail, error) {
	checkedRPCs, found, err := uc.cacheRepo.GetChainCheckedRPCs(ctx, chainID)
	if err != nil {
		uc.logger.Warn("Cache error when getting checked RPCs for chain",
			zap.Int64("chainId", chainID), zap.Error(err),
		)
	}
	if found {
		uc.logger.Debug("Cache hit for chain checked RPCs", zap.Int64("chainId", chainID))
		if len(checkedRPCs) == 0 {
			return nil, fmt.Errorf("%w: no working RPCs found for chain %d (cached result)",
				domain.ErrNoRPCsAvailable, chainID,
			)
		}
		return checkedRPCs, nil
	}

	uc.logger.Debug("Cache miss for chain checked RPCs, fetching from repository",
		zap.Int64("chainId", chainID),
	)
	rawChains, fetchErr := uc.chainRepo.GetAllChains(ctx)
	if fetchErr != nil {
		return nil, fmt.Errorf("repository.GetAllChains for chain %d failed: %w", chainID, fetchErr)
	}

	var foundChain *entity.Chain
	for i := range rawChains {
		if rawChains[i].ChainID == chainID {
			foundChain = &rawChains[i]
			break
		}
	}

	if foundChain == nil {
		uc.logger.Warn("Chain not found in repository data", zap.Int64("chainId", chainID))
		return nil, fmt.Errorf("%w: chain with ID %d not found in repository", domain.ErrChainNotFound, chainID)
	}

	uc.logger.Debug("Found chain, checking its RPCs",
		zap.Int64("chainId", chainID), zap.Int("rpcCount", len(foundChain.RPC)),
	)
	checkerTimeout := uc.cfg.Checker.GetTimeout()
	chainCheckedRPCsResult := uc.checkChainRPCs(checkerTimeout, foundChain.RPC)

	cacheTTL := uc.cfg.Checker.GetCacheTTL()
	if cacheErr := uc.cacheRepo.SetChainCheckedRPCs(ctx, chainID, chainCheckedRPCsResult, cacheTTL); cacheErr != nil {
		uc.logger.Error("Failed to cache checked RPCs for chain (after check)",
			zap.Int64("chainId", chainID), zap.Error(cacheErr))
	}

	if len(chainCheckedRPCsResult) == 0 {
		uc.logger.Debug("No working RPCs found after check for chain", zap.Int64("chainId", chainID))
		return nil, fmt.Errorf("%w for chain %d after check", domain.ErrNoRPCsAvailable, chainID)
	}

	uc.logger.Debug("Finished checking RPCs for specific chain",
		zap.Int64("chainId", chainID), zap.Int("checkedCount", len(chainCheckedRPCsResult)))
	return chainCheckedRPCsResult, nil
}

// performRpcChecksAndUpdateCache is an internal method to check RPCs for a list of chains and update the cache.
func (uc *chainService) performRpcChecksAndUpdateCache(ctx context.Context, rawChains []entity.Chain) {
	uc.logger.Info("Starting background RPC checks and cache update", zap.Int("chainCount", len(rawChains)))

	if len(rawChains) == 0 {
		uc.logger.Warn("performRpcChecksAndUpdateCache called with 0 chains")
		return
	}

	updatedChains := make([]entity.Chain, len(rawChains))
	resultsChan := make(chan struct {
		index       int
		checkedRPCs []entity.RPCDetail
	}, len(rawChains))

	var wg sync.WaitGroup
	checkerTimeout := uc.cfg.Checker.GetTimeout()

	numChainWorkers := uc.cfg.Checker.MaxWorkers / 5
	if numChainWorkers <= 0 {
		numChainWorkers = 1
	}
	if numChainWorkers > len(rawChains) {
		numChainWorkers = len(rawChains)
	}
	chainJobs := make(chan int, len(rawChains))

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
				uc.logger.Debug("Chain processing worker checking chain",
					zap.Int("workerID", workerID), zap.String("chainName", chainToProcess.Name),
				)

				checkedRPCs := uc.checkChainRPCs(checkerTimeout, chainToProcess.RPC)

				select {
				case resultsChan <- struct {
					index       int
					checkedRPCs []entity.RPCDetail
				}{index: chainIndex, checkedRPCs: checkedRPCs}:
				case <-ctx.Done():
					uc.logger.Info("Context cancelled before sending result for chain",
						zap.String("chainName", chainToProcess.Name),
					)
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
			chainCopy := rawChains[result.index]
			chainCopy.CheckedRPCs = result.checkedRPCs
			updatedChains[result.index] = chainCopy
			processedCount++
		} else {
			uc.logger.Warn("Received result with out-of-bounds index",
				zap.Int("index", result.index), zap.Int("lenRawChains", len(rawChains)),
			)
		}
	}
	uc.logger.Info("Finished collecting results for chain RPC checks",
		zap.Int("processedCount", processedCount), zap.Int("expectedCount", len(rawChains)),
	)

	if ctx.Err() != nil {
		uc.logger.Warn("Context cancelled before caching results in background task, cache not updated.",
			zap.Error(ctx.Err()),
		)
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

// checkChainRPCs performs parallel RPC checks for a given list of RPC URLs and returns their details.
func (uc *chainService) checkChainRPCs(timeout time.Duration, rpcs []entity.RPCURL) []entity.RPCDetail {
	if len(rpcs) == 0 {
		return nil
	}

	checkedRPCDetails := make([]entity.RPCDetail, len(rpcs))
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
		url   entity.RPCURL
	}, len(rpcs))

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			uc.logger.Debug("Starting RPC check sub-worker", zap.Int("workerID", workerID))
			for job := range jobChan {
				detail := entity.RPCDetail{URL: job.url}
				rawURLStr := job.url.String()

				scheme, _, _ := strings.Cut(rawURLStr, "://")
				switch strings.ToLower(scheme) {
				case "http":
					detail.Protocol = entity.ProtocolHTTP
				case "https":
					detail.Protocol = entity.ProtocolHTTPS
				case "ws":
					detail.Protocol = entity.ProtocolWS
				case "wss":
					detail.Protocol = entity.ProtocolWSS
				default:
					detail.Protocol = entity.ProtocolUnknown
					notWorking := false
					detail.IsWorking = &notWorking
					checkedRPCDetails[job.index] = detail
					uc.logger.Error("RPCURL with unknown/unhandled protocol encountered in checkChainRPCs",
						zap.String("url", rawURLStr),
						zap.String("parsedScheme", scheme),
					)
					continue
				}

				checkCtx, cancel := context.WithTimeout(uc.rootCtx, timeout)

				isWorking, latency, err := uc.rpcChecker.CheckRPC(checkCtx, job.url)
				cancel()

				if err != nil {
					uc.logger.Debug("RPC check failed", zap.String("rpc", rawURLStr), zap.Error(err))
					notWorkingStatus := false
					detail.IsWorking = &notWorkingStatus
				} else {
					detail.IsWorking = &isWorking
					if isWorking {
						latencyMs := latency.Milliseconds()
						detail.LatencyMs = &latencyMs
						uc.logger.Debug("RPC is working",
							zap.String("rpc", rawURLStr), zap.Duration("latency", latency),
						)
					} else {
						uc.logger.Debug("RPC is not working (checker reported)", zap.String("rpc", rawURLStr))
					}
				}
				checkedRPCDetails[job.index] = detail
			}
			uc.logger.Debug("RPC check sub-worker finished", zap.Int("workerID", workerID))
		}(w)
	}

	for i, rpcURL := range rpcs {
		jobChan <- struct {
			index int
			url   entity.RPCURL
		}{index: i, url: rpcURL}
	}
	close(jobChan)

	wg.Wait()
	return checkedRPCDetails
}

// startBackgroundChecker initializes a ticker to periodically fetch all chains and update their RPC statuses in the cache.
func (uc *chainService) startBackgroundChecker() {
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
