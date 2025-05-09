package entity

// NetworkType defines the type for network classifications (e.g., mainnet, testnet).
type NetworkType string

// Constants for known network types.
const (
	NetworkMainnet NetworkType = "mainnet"
	NetworkTestnet NetworkType = "testnet"
)

// Chain represents the data structure for a blockchain network.
type Chain struct {
	Name        string
	Chain       string
	Icon        string
	RPC         []RPCURL
	Features    []Feature
	Faucets     []string
	Currency    Currency
	InfoURL     string
	ShortName   string
	ChainID     int64
	NetworkID   int64
	Slip44      int64
	Ens         *Ens
	Explorers   []Explorer
	Title       string
	Parent      *Parent
	Network     NetworkType
	RedFlags    []string
	CheckedRPCs []RPCDetail
}

// Currency defines the native currency details of a chain.
type Currency struct {
	Name     string
	Symbol   string
	Decimals int
}

// Explorer defines details about a block explorer for a chain.
type Explorer struct {
	Name     string
	URL      string
	Standard string
	Icon     string // omitempty logic will be handled by mappers if needed
}

// Ens defines ENS registry details.
type Ens struct {
	Registry string
}

// Feature defines a feature supported by a chain.
type Feature struct {
	Name string
}

// Parent defines details about a parent chain (for L2s).
type Parent struct {
	Type    string
	Chain   string
	Bridges []Bridge // omitempty logic will be handled by mappers if needed
}

// Bridge defines a bridge associated with a parent chain.
type Bridge struct {
	URL string
}
