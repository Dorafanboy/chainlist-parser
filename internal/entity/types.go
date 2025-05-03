package entity

// RPCDetail holds information about a specific RPC endpoint after checking.
type RPCDetail struct {
	URL       string `json:"url"`
	Protocol  string `json:"protocol"`
	IsWorking *bool  `json:"isWorking"`
	LatencyMs *int64 `json:"latencyMs,omitempty"`
}

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
	Network     string      `json:"network,omitempty"`
	RedFlags    []string    `json:"redFlags,omitempty"`
	CheckedRPCs []RPCDetail `json:"checkedRPCs,omitempty"`
}

type Currency struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type Explorer struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Standard string `json:"standard"`
	Icon     string `json:"icon,omitempty"`
}

type Ens struct {
	Registry string `json:"registry"`
}

type Feature struct {
	Name string `json:"name"`
}

type Parent struct {
	Type    string   `json:"type"`
	Chain   string   `json:"chain"`
	Bridges []Bridge `json:"bridges,omitempty"`
}

type Bridge struct {
	URL string `json:"url"`
}
