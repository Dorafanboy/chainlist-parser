package entity

// Protocol defines the type for RPC protocols.
type Protocol string

// Constants for known protocols.
const (
	ProtocolHTTP    Protocol = "http"
	ProtocolHTTPS   Protocol = "https"
	ProtocolWS      Protocol = "ws"
	ProtocolWSS     Protocol = "wss"
	ProtocolUnknown Protocol = "unknown"
)

// RPCDetail holds information about a specific RPC endpoint after checking.
type RPCDetail struct {
	URL       RPCURL   `json:"url"`
	Protocol  Protocol `json:"protocol"`
	IsWorking *bool    `json:"isWorking"`
	LatencyMs *int64   `json:"latencyMs,omitempty"`
}
