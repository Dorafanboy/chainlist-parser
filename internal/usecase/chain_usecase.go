package usecase

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"chainlist-parser/internal/config"
	"chainlist-parser/internal/entity"

	"go.uber.org/zap"
)

// Compile-time check to ensure chainUseCase implements ChainUseCase
var _ ChainUseCase = (*chainUseCase)(nil)

// chainUseCase implements the ChainUseCase interface orchestrating chain data operations.
type chainUseCase struct {
	chainRepo  ChainRepository
	cacheRepo  CacheRepository
	rpcChecker RPCChecker
	logger     *zap.Logger
	cfg        config.Config
	rootCtx    context.Context
	isChecking *atomic.Bool
}

// NewChainUseCase creates a new instance of the chain use case.
func NewChainUseCase(
	rootCtx context.Context,
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
		rootCtx:    rootCtx,
		isChecking: new(atomic.Bool),
	}
	go uc.startBackgroundChecker()
	return uc
}

// GetAllChainsChecked gets all chains, returning cached data or triggering background checks.
func (uc *chainUseCase) GetAllChainsChecked(ctx context.Context) ([]entity.Chain, error) {
	cachedChains, found, err := uc.cacheRepo.GetChains(ctx)
	if err == nil && found {
		uc.logger.Debug("Cache hit for all chains (potentially checked)")
		return cachedChains, nil
	}
	if err != nil {
		uc.logger.Warn("Cache error when getting all chains", zap.Error(err))
	}
	uc.logger.Debug("Cache miss for all chains, fetching raw data...")

	rawChains, fetchErr := uc.chainRepo.GetAllChains(ctx)
	if fetchErr != nil {
		uc.logger.Error("Failed to fetch raw chain data from repository", zap.Error(fetchErr))
		return nil, fetchErr
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

// GetCheckedRPCsForChain gets checked RPC details for a specific chain, using cache or fetching all chains.
func (uc *chainUseCase) GetCheckedRPCsForChain(ctx context.Context, chainID int64) ([]entity.RPCDetail, error) {
	checkedRPCs, found, err := uc.cacheRepo.GetChainCheckedRPCs(ctx, chainID)
	if err == nil && found {
		uc.logger.Debug("Cache hit for chain checked RPCs", zap.Int64("chainId", chainID))
		return checkedRPCs, nil
	}
	if err != nil {
		uc.logger.Warn("Cache error when getting checked RPCs", zap.Int64("chainId", chainID), zap.Error(err))
	}
	uc.logger.Debug("Cache miss for chain checked RPCs", zap.Int64("chainId", chainID))

	uc.logger.Debug(
		"Fetching all chains from repository to find specific chain info",
		zap.Int64("chainId", chainID),
	)
	rawChains, fetchErr := uc.chainRepo.GetAllChains(ctx)
	if fetchErr != nil {
		uc.logger.Error("Failed to get all chains from repo while looking for specific chain RPCs",
			zap.Int64("chainId", chainID), zap.Error(fetchErr))
		return nil, fetchErr
	}

	var foundChain *entity.Chain
	for i := range rawChains {
		if rawChains[i].ChainID == chainID {
			foundChain = &rawChains[i]
			break
		}
	}

	if foundChain == nil {
		uc.logger.Warn(
			"Chain not found in repository data (after fetching all)",
			zap.Int64("chainId", chainID),
		)
		return nil, nil
	}

	uc.logger.Debug(
		"Found chain in full list, checking its RPCs",
		zap.Int64("chainId", chainID),
		zap.Int("rpcCount", len(foundChain.RPC)),
	)

	checkerTimeout := uc.cfg.Checker.GetTimeout()
	chainCheckedRPCs := uc.checkChainRPCs(checkerTimeout, foundChain.RPC)
	uc.logger.Debug(
		"Finished checking RPCs for specific chain (from GetAllChains path)",
		zap.Int64("chainId", chainID),
		zap.Int("checkedCount",
			len(chainCheckedRPCs)),
	)

	cacheTTL := uc.cfg.Checker.GetCacheTTL()
	if cacheErr := uc.cacheRepo.SetChainCheckedRPCs(ctx, chainID, chainCheckedRPCs, cacheTTL); cacheErr != nil {
		uc.logger.Error(
			"Failed to cache checked RPCs for specific chain",
			zap.Int64("chainId", chainID),
			zap.Error(cacheErr),
		)
	}

	return chainCheckedRPCs, nil
}

// performRpcChecksAndUpdateCache performs RPC checks for the given chains and updates the cache.
func (uc *chainUseCase) performRpcChecksAndUpdateCache(ctx context.Context, rawChains []entity.Chain) {
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

	maxWorkers := uc.cfg.Checker.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 10
	}
	workerSem := make(chan struct{}, maxWorkers)

	for i, chain := range rawChains {
		wg.Add(1)
		workerSem <- struct{}{}
		go func(index int, c entity.Chain) {
			defer func() {
				<-workerSem
				wg.Done()
			}()

			select {
			case <-ctx.Done():
				uc.logger.Info(
					"Context cancelled before starting RPC check in background worker",
					zap.Int("chainIndex", index),
				)
				return
			default:
			}

			checkedRPCs := uc.checkChainRPCs(checkerTimeout, c.RPC)
			select {
			case resultsChan <- struct {
				index       int
				checkedRPCs []entity.RPCDetail
			}{index: index, checkedRPCs: checkedRPCs}:
			case <-ctx.Done():
				uc.logger.Info(
					"Context cancelled before sending RPC check result",
					zap.Int("chainIndex", index),
				)
				return
			}
		}(i, chain)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
		close(workerSem)
	}()

	for result := range resultsChan {
		select {
		case <-ctx.Done():
			uc.logger.Info("Context cancelled while collecting RPC check results")
			return
		default:
			if result.index >= 0 && result.index < len(rawChains) {
				updatedChains[result.index] = rawChains[result.index]
				updatedChains[result.index].CheckedRPCs = result.checkedRPCs
			} else {
				uc.logger.Warn(
					"Received result with out-of-bounds index",
					zap.Int("index", result.index),
					zap.Int("len", len(rawChains)),
				)
			}
		}
	}

	if ctx.Err() != nil {
		uc.logger.Info("Context cancelled before caching results in background task")
		return
	}

	uc.logger.Info("Finished checking chains in background task", zap.Int("totalChains", len(updatedChains)))

	cacheTTL := uc.cfg.Checker.GetCacheTTL()
	if err := uc.cacheRepo.SetChains(ctx, updatedChains, cacheTTL); err != nil {
		uc.logger.Error("Failed to cache checked chains in background task", zap.Error(err))
	}

	uc.logger.Info("Background RPC check and cache update finished.")
}

// checkChainRPCs performs parallel RPC checks for a list of URLs using a worker pool.
func (uc *chainUseCase) checkChainRPCs(timeout time.Duration, rpcs []string) []entity.RPCDetail {
	if len(rpcs) == 0 {
		return nil
	}

	checkedRPCs := make([]entity.RPCDetail, len(rpcs))
	var wg sync.WaitGroup

	maxWorkers := uc.cfg.Checker.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 10
	}
	workerSem := make(chan struct{}, maxWorkers)

	for i, rpcURL := range rpcs {
		wg.Add(1)
		workerSem <- struct{}{}
		go func(index int, url string) {
			defer func() {
				<-workerSem
				wg.Done()
			}()

			detail := entity.RPCDetail{URL: url}

			if strings.HasPrefix(url, "https://") {
				detail.Protocol = entity.ProtocolHTTPS
			} else if strings.HasPrefix(url, "http://") {
				detail.Protocol = entity.ProtocolHTTP
			} else if strings.HasPrefix(url, "wss://") {
				detail.Protocol = entity.ProtocolWSS
			} else if strings.HasPrefix(url, "ws://") {
				detail.Protocol = entity.ProtocolWS
			} else {
				detail.Protocol = entity.ProtocolUnknown
				notWorking := false
				detail.IsWorking = &notWorking
				checkedRPCs[index] = detail
				return
			}

			checkCtx, cancel := context.WithTimeout(uc.rootCtx, timeout)
			defer cancel()

			isWorking, latency, err := uc.rpcChecker.CheckRPC(checkCtx, url)
			if err != nil {
				uc.logger.Debug("RPC check failed", zap.String("rpc", url), zap.Error(err))
				notWorking := false
				detail.IsWorking = &notWorking
			} else {
				detail.IsWorking = &isWorking
				if isWorking {
					latencyMs := latency.Milliseconds()
					detail.LatencyMs = &latencyMs
					uc.logger.Debug(
						"RPC is working",
						zap.String("rpc", url),
						zap.Duration("latency",
							latency),
					)
				} else {
					uc.logger.Debug("RPC is not working", zap.String("rpc", url))
				}
			}
			checkedRPCs[index] = detail
		}(i, rpcURL)
	}

	wg.Wait()

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

	if uc.cfg.Checker.RunOnStartup {
		go func() {
			uc.logger.Info("Running initial background check...")
			rawChains, err := uc.chainRepo.GetAllChains(uc.rootCtx)
			if err != nil {
				if uc.rootCtx.Err() != nil {
					uc.logger.Warn("Initial background check fetch cancelled due to application shutdown")
				} else {
					uc.logger.Error("Error fetching chains during initial background check", zap.Error(err))
				}
				return
			}
			uc.performRpcChecksAndUpdateCache(uc.rootCtx, rawChains)
		}()
	}

	for {
		select {
		case <-ticker.C:
			uc.logger.Info("Background checker tick: running check...")
			rawChains, err := uc.chainRepo.GetAllChains(uc.rootCtx)
			if err != nil {
				if uc.rootCtx.Err() != nil {
					uc.logger.Warn("Periodic background check fetch cancelled due to application shutdown")
				} else {
					uc.logger.Error("Error fetching chains during periodic background check", zap.Error(err))
				}
				continue
			}
			uc.performRpcChecksAndUpdateCache(uc.rootCtx, rawChains)

		case <-uc.rootCtx.Done():
			uc.logger.Info("Background checker stopping due to context cancellation.")
			return
		}
	}
}
