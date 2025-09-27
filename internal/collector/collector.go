package collector

import (
	"context"
	"net"
	"time"

	"github.com/example/neteye/pkg/types"
)

// Collector defines the interface for collecting network connection information
type Collector interface {
	// Collect retrieves current network connections based on options
	Collect(ctx context.Context, opts *types.CollectorOptions) ([]*types.Connection, error)

	// SupportsRealtime returns true if the collector supports real-time monitoring
	SupportsRealtime() bool
}

// Factory creates a new collector for the current operating system
func New() (Collector, error) {
	return newPlatformCollector()
}

// baseCollector provides common functionality for all collectors
type baseCollector struct {
	resolver *DNSResolver
}

// newBaseCollector creates a new base collector with DNS resolver
func newBaseCollector() *baseCollector {
	return &baseCollector{
		resolver: NewDNSResolver(300*time.Millisecond, 1000),
	}
}

// resolveHostname resolves an IP address to hostname if resolve is enabled
func (b *baseCollector) resolveHostname(ctx context.Context, ip string, resolve bool) string {
	if !resolve || ip == "0.0.0.0" || ip == "::" || ip == "127.0.0.1" || ip == "::1" {
		return ip
	}

	if hostname := b.resolver.Resolve(ctx, ip); hostname != "" {
		return hostname
	}
	return ip
}

// parseState converts OS-specific state string to standard ConnectionState
func parseState(state string) types.ConnectionState {
	switch state {
	case "LISTEN", "LISTENING":
		return types.StateListen
	case "ESTABLISHED":
		return types.StateEstablished
	case "TIME_WAIT":
		return types.StateTimeWait
	case "CLOSE_WAIT":
		return types.StateCloseWait
	case "FIN_WAIT1":
		return types.StateFinWait1
	case "FIN_WAIT2":
		return types.StateFinWait2
	case "CLOSING":
		return types.StateClosing
	case "LAST_ACK":
		return types.StateLastAck
	case "SYN_SENT":
		return types.StateSynSent
	case "SYN_RECV":
		return types.StateSynRecv
	default:
		return types.StateUnknown
	}
}

// parseProtocol converts protocol string to Protocol type
func parseProtocol(proto string) types.Protocol {
	switch proto {
	case "tcp", "tcp4", "tcp6":
		return types.ProtocolTCP
	case "udp", "udp4", "udp6":
		return types.ProtocolUDP
	default:
		return types.ProtocolTCP // default fallback
	}
}

// filterConnections applies the collector options to filter connections
func filterConnections(connections []*types.Connection, opts *types.CollectorOptions) []*types.Connection {
	if opts == nil {
		return connections
	}

	var filtered []*types.Connection
	for _, conn := range connections {
		// Filter by PID
		if opts.PID > 0 && conn.PID != opts.PID {
			continue
		}

		// Filter by protocol
		if opts.Proto != "" && conn.Proto != opts.Proto {
			continue
		}

		// Filter by listen-only
		if opts.ListenOnly && conn.State != types.StateListen {
			continue
		}

		// Filter by remote host (simple glob pattern matching)
		if opts.RemoteHost != "" {
			if !matchGlob(conn.RemoteIP, opts.RemoteHost) {
				continue
			}
		}

		filtered = append(filtered, conn)
	}

	return filtered
}

// matchGlob performs simple glob pattern matching
func matchGlob(str, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Simple implementation - can be enhanced later
	if pattern == str {
		return true
	}

	// Check if pattern contains wildcards
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix
	}

	return false
}

// isValidIP checks if a string is a valid IP address
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// normalizeIP normalizes IP address format
func normalizeIP(ip string) string {
	if parsed := net.ParseIP(ip); parsed != nil {
		return parsed.String()
	}
	return ip
}
