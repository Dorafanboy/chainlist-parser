package entity

// Based on structure from https://chainid.network/chains.json
// We might only need a subset of these fields.

type Chain struct {
	Name      string     `json:"name"`
	Chain     string     `json:"chain"`
	Icon      string     `json:"icon,omitempty"` // Optional icon identifier
	RPC       []string   `json:"rpc"`            // List of RPC URLs
	Features  []Feature  `json:"features,omitempty"`
	Faucets   []string   `json:"faucets,omitempty"`
	Currency  Currency   `json:"nativeCurrency"`
	InfoURL   string     `json:"infoURL"`
	ShortName string     `json:"shortName"`
	ChainID   int64      `json:"chainId"` // Use int64 for potentially large IDs
	NetworkID int64      `json:"networkId"`
	Slip44    int64      `json:"slip44,omitempty"`
	Ens       *Ens       `json:"ens,omitempty"`
	Explorers []Explorer `json:"explorers,omitempty"`
	Title     string     `json:"title,omitempty"`
	Parent    *Parent    `json:"parent,omitempty"`
	Network   string     `json:"network,omitempty"`  // e.g., "mainnet", "testnet"
	RedFlags  []string   `json:"redFlags,omitempty"` // List of potential issues

	// --- Fields added by our service ---
	WorkingRPCs    []string `json:"workingRPCs,omitempty"`    // Populated after checking
	NonWorkingRPCs []string `json:"nonWorkingRPCs,omitempty"` // Optional: Populated after checking
}

type Currency struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type Explorer struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Standard string `json:"standard"` // e.g., "EIP3091"
	Icon     string `json:"icon,omitempty"`
}

type Ens struct {
	Registry string `json:"registry"`
}

type Feature struct {
	Name string `json:"name"` // e.g., "EIP155", "EIP1559"
}

type Parent struct {
	Type    string   `json:"type"`  // e.g., "L2"
	Chain   string   `json:"chain"` // e.g., "eip155-1"
	Bridges []Bridge `json:"bridges,omitempty"`
}

type Bridge struct {
	URL string `json:"url"`
}

// RPCInfo represents a checked RPC endpoint result
type RPCInfo struct {
	URL       string `json:"url"`
	IsWorking bool   `json:"isWorking"`
	LatencyMs int64  `json:"latencyMs,omitempty"` // Optional latency
	CheckedAt int64  `json:"checkedAt,omitempty"` // Optional timestamp
}
