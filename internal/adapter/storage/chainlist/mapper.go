package chainlist

import (
	dto "chainlist-parser/internal/adapter/storage/chainlist/dto"
	"chainlist-parser/internal/domain/entity"

	"go.uber.org/zap"
)

// mapNetworkType converts a raw DTO network type to its domain entity counterpart.
func mapNetworkType(rawType dto.NetworkTypeRaw) entity.NetworkType {
	switch rawType {
	case dto.NetworkMainnetRaw:
		return entity.NetworkMainnet
	case dto.NetworkTestnetRaw:
		return entity.NetworkTestnet
	default:
		return entity.NetworkType(rawType)
	}
}

// toDomainChains converts a slice of raw DTO chain representations to a slice of domain entity chains.
func toDomainChains(rawChains []dto.ChainRaw, logger *zap.Logger) []entity.Chain {
	if rawChains == nil {
		return nil
	}
	domainChains := make([]entity.Chain, 0, len(rawChains))
	for _, raw := range rawChains {
		var domainRPCs []entity.RPCURL
		if raw.RPC != nil {
			domainRPCs = make([]entity.RPCURL, 0, len(raw.RPC))
			for _, rpcStr := range raw.RPC {
				rpcURL, err := entity.NewRPCURL(rpcStr)
				if err != nil {
					if logger != nil {
						logger.Warn("Skipping invalid RPC URL during mapping",
							zap.String("rawUrl", rpcStr),
							zap.Int64("chainId", raw.ChainID),
							zap.Error(err))
					}
					continue
				}
				domainRPCs = append(domainRPCs, rpcURL)
			}
		}

		var domainFeatures []entity.Feature
		if raw.Features != nil {
			domainFeatures = make([]entity.Feature, len(raw.Features))
			for j, fRaw := range raw.Features {
				domainFeatures[j] = entity.Feature{Name: fRaw.Name}
			}
		}

		var domainExplorers []entity.Explorer
		if raw.Explorers != nil {
			domainExplorers = make([]entity.Explorer, len(raw.Explorers))
			for j, eRaw := range raw.Explorers {
				domainExplorers[j] = entity.Explorer{
					Name:     eRaw.Name,
					URL:      eRaw.URL,
					Standard: eRaw.Standard,
					Icon:     eRaw.Icon,
				}
			}
		}

		var domainEns *entity.Ens
		if raw.Ens != nil {
			domainEns = &entity.Ens{Registry: raw.Ens.Registry}
		}

		var domainParent *entity.Parent
		if raw.Parent != nil {
			var domainBridges []entity.Bridge
			if raw.Parent.Bridges != nil {
				domainBridges = make([]entity.Bridge, len(raw.Parent.Bridges))
				for j, bRaw := range raw.Parent.Bridges {
					domainBridges[j] = entity.Bridge{URL: bRaw.URL}
				}
			}
			domainParent = &entity.Parent{
				Type:    raw.Parent.Type,
				Chain:   raw.Parent.Chain,
				Bridges: domainBridges,
			}
		}

		domainChain := entity.Chain{
			Name:     raw.Name,
			Chain:    raw.Chain,
			Icon:     raw.Icon,
			RPC:      domainRPCs,
			Features: domainFeatures,
			Faucets:  raw.Faucets,
			Currency: entity.Currency{
				Name:     raw.Currency.Name,
				Symbol:   raw.Currency.Symbol,
				Decimals: raw.Currency.Decimals,
			},
			InfoURL:     raw.InfoURL,
			ShortName:   raw.ShortName,
			ChainID:     raw.ChainID,
			NetworkID:   raw.NetworkID,
			Slip44:      raw.Slip44,
			Ens:         domainEns,
			Explorers:   domainExplorers,
			Title:       raw.Title,
			Parent:      domainParent,
			Network:     mapNetworkType(raw.Network),
			RedFlags:    raw.RedFlags,
			CheckedRPCs: nil,
		}
		domainChains = append(domainChains, domainChain)
	}
	return domainChains
}
