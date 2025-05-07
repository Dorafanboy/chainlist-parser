package entity

// NetworkType defines the type for network classifications (e.g., mainnet, testnet).
type NetworkType string

// Constants for known network types.
const (
	NetworkMainnet NetworkType = "mainnet"
	NetworkTestnet NetworkType = "testnet"
)

// Chain represents the data structure for a blockchain network from Chainlist.
type Chain struct {
	Name        string      `json:"name"`
	Chain       string      `json:"chain"`
	Icon        string      `json:"icon,omitempty"`
	RPC         []string    `json:"rpc"`
	Features    []Feature   `json:"features,omitempty"`
	Faucets     []string    `json:"faucets,omitempty"`
	Currency    Currency    `json:"nativeCurrency"`
	InfoURL     string      `json:"infoURL"`
	ShortName   string      `json:"shortName"`
	ChainID     int64       `json:"chainId"`
	NetworkID   int64       `json:"networkId"`
	Slip44      int64       `json:"slip44,omitempty"`
	Ens         *Ens        `json:"ens,omitempty"`
	Explorers   []Explorer  `json:"explorers,omitempty"`
	Title       string      `json:"title,omitempty"`
	Parent      *Parent     `json:"parent,omitempty"`
	Network     NetworkType `json:"network,omitempty"`
	RedFlags    []string    `json:"redFlags,omitempty"`
	CheckedRPCs []RPCDetail `json:"checkedRPCs,omitempty"` // This field will be populated by our service
}

// Currency defines the native currency details of a chain.
type Currency struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

// Explorer defines details about a block explorer for a chain.
type Explorer struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Standard string `json:"standard"`
	Icon     string `json:"icon,omitempty"`
}

// Ens defines ENS registry details.
type Ens struct {
	Registry string `json:"registry"`
}

// Feature defines a feature supported by a chain.
type Feature struct {
	Name string `json:"name"`
}

// Parent defines details about a parent chain (for L2s).
type Parent struct {
	Type    string   `json:"type"`
	Chain   string   `json:"chain"`
	Bridges []Bridge `json:"bridges,omitempty"`
}

// Bridge defines a bridge associated with a parent chain.
type Bridge struct {
	URL string `json:"url"`
}
