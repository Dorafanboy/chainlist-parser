package chainlist_dto

// NetworkTypeRaw defines the type for network classifications (e.g., mainnet, testnet) from raw data.
type NetworkTypeRaw string

// Constants for known network types from raw data.
const (
	NetworkMainnetRaw NetworkTypeRaw = "mainnet"
	NetworkTestnetRaw NetworkTypeRaw = "testnet"
)

// ChainRaw represents the data structure for a blockchain network as received from Chainlist.
type ChainRaw struct {
	Name      string         `json:"name"`
	Chain     string         `json:"chain"`
	Icon      string         `json:"icon,omitempty"`
	RPC       []string       `json:"rpc"`
	Features  []FeatureRaw   `json:"features,omitempty"`
	Faucets   []string       `json:"faucets,omitempty"`
	Currency  CurrencyRaw    `json:"nativeCurrency"`
	InfoURL   string         `json:"infoURL"`
	ShortName string         `json:"shortName"`
	ChainID   int64          `json:"chainId"`
	NetworkID int64          `json:"networkId"`
	Slip44    int64          `json:"slip44,omitempty"`
	Ens       *EnsRaw        `json:"ens,omitempty"`
	Explorers []ExplorerRaw  `json:"explorers,omitempty"`
	Title     string         `json:"title,omitempty"`
	Parent    *ParentRaw     `json:"parent,omitempty"`
	Network   NetworkTypeRaw `json:"network,omitempty"`
	RedFlags  []string       `json:"redFlags,omitempty"`
}

// CurrencyRaw defines the native currency details of a chain from raw data.
type CurrencyRaw struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

// ExplorerRaw defines details about a block explorer for a chain from raw data.
type ExplorerRaw struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Standard string `json:"standard"`
	Icon     string `json:"icon,omitempty"`
}

// EnsRaw defines ENS registry details from raw data.
type EnsRaw struct {
	Registry string `json:"registry"`
}

// FeatureRaw defines a feature supported by a chain from raw data.
type FeatureRaw struct {
	Name string `json:"name"`
}

// ParentRaw defines details about a parent chain (for L2s) from raw data.
type ParentRaw struct {
	Type    string      `json:"type"`
	Chain   string      `json:"chain"`
	Bridges []BridgeRaw `json:"bridges,omitempty"`
}

// BridgeRaw defines a bridge associated with a parent chain from raw data.
type BridgeRaw struct {
	URL string `json:"url"`
}
