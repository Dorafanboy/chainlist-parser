package usecase

import (
	"testing"
)

// TODO: Add tests for ChainUseCase

func TestChainUseCase_GetAllChainsWithWorkingRPCs(t *testing.T) {
	// Setup mocks (using go:generate)
	// ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	// mockChainRepo := mocks.NewMockChainRepository(ctrl)
	// mockCacheRepo := mocks.NewMockCacheRepository(ctrl)
	// mockRpcChecker := mocks.NewMockRPCChecker(ctrl)
	// logger := zap.NewNop()
	// cfg := config.Config{ /* ... */ }

	// uc := NewChainUseCase(mockChainRepo, mockCacheRepo, mockRpcChecker, logger, cfg)

	// Define expectations
	// mockCacheRepo.EXPECT().GetChains(gomock.Any()).Return(nil, nil) // Cache miss
	// mockChainRepo.EXPECT().GetAllChains(gomock.Any()).Return([]entity.Chain{...}, nil)
	// mockRpcChecker.EXPECT().CheckRPC(gomock.Any(), "rpc-url").Return(true, time.Second, nil)
	// mockCacheRepo.EXPECT().SetChains(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	// mockCacheRepo.EXPECT().SetChainRPCs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	// Call the method
	// _, err := uc.GetAllChainsWithWorkingRPCs(context.Background())
	// require.NoError(t, err)
	// Add assertions
}

func TestChainUseCase_GetWorkingRPCsForChain(t *testing.T) {
	// Similar setup as above
	// Test cache hit, cache miss, chain found, chain not found scenarios
}
