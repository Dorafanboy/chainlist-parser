package entity

import (
	"fmt"
	"net/url"
	"strings"
)

// RPCURL represents a typed URL for an RPC endpoint.
type RPCURL string

// NewRPCURL creates a new RPCURL instance.
func NewRPCURL(rawURL string) (RPCURL, error) {
	if strings.TrimSpace(rawURL) == "" {
		return "", fmt.Errorf("rpc url cannot be empty")
	}

	u, err := url.ParseRequestURI(rawURL) // Checks general URI validity
	if err != nil {
		return "", fmt.Errorf("invalid rpc url format '%s': %w", rawURL, err)
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https", "ws", "wss":
		// Allowed schemes
	default:
		return "", fmt.Errorf("rpc url '%s' has unsupported scheme: '%s'", rawURL, scheme)
	}

	return RPCURL(rawURL), nil
}

// String returns the string representation of the RPCURL.
func (r RPCURL) String() string {
	return string(r)
}
