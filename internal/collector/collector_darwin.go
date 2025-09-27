//go:build darwin

package collector

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yutat23/neteye/pkg/types"
)

// darwinCollector implements network connection collection for macOS
type darwinCollector struct {
	*baseCollector
}

// newDarwinCollector creates a new Darwin collector
func newDarwinCollector() (Collector, error) {
	return &darwinCollector{
		baseCollector: newBaseCollector(),
	}, nil
}

// SupportsRealtime returns true as Darwin supports real-time monitoring
func (c *darwinCollector) SupportsRealtime() bool {
	return true
}

// Collect retrieves network connections using lsof command
func (c *darwinCollector) Collect(ctx context.Context, opts *types.CollectorOptions) ([]*types.Connection, error) {
	var connections []*types.Connection

	// Use lsof -i to get network connections
	args := []string{"-i"}

	// Add protocol filter if specified
	if opts != nil && opts.Proto != "" {
		if opts.Proto == types.ProtocolTCP {
			args = []string{"-iTCP"}
		} else if opts.Proto == types.ProtocolUDP {
			args = []string{"-iUDP"}
		}
	}

	// Add PID filter if specified
	if opts != nil && opts.PID > 0 {
		args = append(args, "-p", strconv.Itoa(opts.PID))
	}

	cmd := exec.CommandContext(ctx, "lsof", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lsof command failed: %w", err)
	}

	connections, err = c.parseLsofOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse lsof output: %w", err)
	}

	// Filter connections based on options
	filtered := filterConnections(connections, opts)

	// Resolve hostnames if requested
	if opts != nil && opts.Resolve {
		for _, conn := range filtered {
			conn.RemoteIP = c.resolveHostname(ctx, conn.RemoteIP, true)
		}
	}

	return filtered, nil
}

// parseLsofOutput parses the output of lsof command
func (c *darwinCollector) parseLsofOutput(output string) ([]*types.Connection, error) {
	var connections []*types.Connection
	lines := strings.Split(output, "\n")

	// Regex to parse lsof output
	// Example: ssh     1234 user   3u  IPv4 0x1234567890      0t0  TCP 192.168.1.100:22->192.168.1.50:54321 (ESTABLISHED)
	re := regexp.MustCompile(`^(\S+)\s+(\d+)\s+(\S+)\s+\d+\S+\s+(IPv[46])\s+\S+\s+\d+\S+\s+(TCP|UDP)\s+([^\s]+)(?:\s+\(([^)]+)\))?`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "COMMAND") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) < 7 {
			continue
		}

		command := matches[1]
		pidStr := matches[2]
		proto := matches[5]
		addressInfo := matches[6]
		state := ""
		if len(matches) > 7 && matches[7] != "" {
			state = matches[7]
		}

		// Parse PID
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Parse addresses
		localIP, localPort, remoteIP, remotePort, err := c.parseLsofAddress(addressInfo, proto)
		if err != nil {
			continue
		}

		// Determine connection state
		var connState types.ConnectionState
		if proto == "UDP" {
			connState = types.StateUnknown
		} else if state == "" {
			connState = types.StateListen
		} else {
			connState = parseState(state)
		}

		conn := &types.Connection{
			Proto:      parseProtocol(proto),
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   remoteIP,
			RemotePort: remotePort,
			State:      connState,
			PID:        pid,
			Process:    command,
			CreatedAt:  time.Now(),
		}

		connections = append(connections, conn)
	}

	return connections, nil
}

// parseLsofAddress parses address information from lsof output
func (c *darwinCollector) parseLsofAddress(addressInfo, proto string) (string, int, string, int, error) {
	// Handle different address formats:
	// 1. For listening sockets: *:22 or localhost:22
	// 2. For established connections: 192.168.1.100:22->192.168.1.50:54321
	// 3. For UDP: *:53 or 192.168.1.100:53

	var localIP, remoteIP string
	var localPort, remotePort int

	if strings.Contains(addressInfo, "->") {
		// Established connection format
		parts := strings.Split(addressInfo, "->")
		if len(parts) != 2 {
			return "", 0, "", 0, fmt.Errorf("invalid established connection format")
		}

		// Parse local address
		localAddr := parts[0]
		localIP, localPort = c.parseAddress(localAddr)

		// Parse remote address
		remoteAddr := parts[1]
		remoteIP, remotePort = c.parseAddress(remoteAddr)
	} else {
		// Listening socket or UDP format
		localIP, localPort = c.parseAddress(addressInfo)
		remoteIP = "0.0.0.0"
		remotePort = 0

		// For UDP, we might have a remote address without ->
		if proto == "UDP" && !strings.Contains(addressInfo, "*") {
			// This might be a UDP connection with a specific remote
			remoteIP = localIP
			remotePort = localPort
		}
	}

	return localIP, localPort, remoteIP, remotePort, nil
}

// parseAddress parses a single address from lsof output
func (c *darwinCollector) parseAddress(addr string) (string, int) {
	if addr == "*" {
		return "0.0.0.0", 0
	}

	// Handle IPv6 addresses in brackets
	if strings.HasPrefix(addr, "[") {
		closingBracket := strings.LastIndex(addr, "]")
		if closingBracket != -1 && len(addr) > closingBracket+2 {
			ip := addr[1:closingBracket]
			portStr := addr[closingBracket+2:] // Skip ]:
			port, _ := strconv.Atoi(portStr)
			return ip, port
		}
	}

	// Handle IPv4 addresses and hostnames
	lastColon := strings.LastIndex(addr, ":")
	if lastColon == -1 {
		return addr, 0
	}

	ip := addr[:lastColon]
	portStr := addr[lastColon+1:]

	// Handle special cases
	if ip == "*" {
		ip = "0.0.0.0"
	} else if ip == "localhost" {
		ip = "127.0.0.1"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return ip, 0
	}

	return ip, port
}

// Alternative implementation using netstat for Darwin
func (c *darwinCollector) collectWithNetstat(ctx context.Context, opts *types.CollectorOptions) ([]*types.Connection, error) {
	// Use netstat -an for basic connection info
	args := []string{"-an"}

	// Add protocol filter if specified
	if opts != nil && opts.Proto != "" {
		if opts.Proto == types.ProtocolTCP {
			args = append(args, "-p", "tcp")
		} else if opts.Proto == types.ProtocolUDP {
			args = append(args, "-p", "udp")
		}
	}

	cmd := exec.CommandContext(ctx, "netstat", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("netstat command failed: %w", err)
	}

	return c.parseNetstatOutput(string(output))
}

// parseNetstatOutput parses netstat output for Darwin
func (c *darwinCollector) parseNetstatOutput(output string) ([]*types.Connection, error) {
	var connections []*types.Connection
	lines := strings.Split(output, "\n")

	// Regex to parse netstat output
	// Example: tcp4       0      0  192.168.1.100.22       192.168.1.50.54321     ESTABLISHED
	re := regexp.MustCompile(`^(tcp[46]?|udp[46]?)\s+\d+\s+\d+\s+([^\s]+)\s+([^\s]+)\s+(\w+)?`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Proto") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) < 4 {
			continue
		}

		proto := matches[1]
		local := matches[2]
		remote := matches[3]
		state := ""
		if len(matches) > 4 {
			state = matches[4]
		}

		// Parse addresses (Darwin netstat uses . as separator for port)
		localIP, localPort := c.parseNetstatAddress(local)
		remoteIP, remotePort := c.parseNetstatAddress(remote)

		// Parse state
		var connState types.ConnectionState
		if strings.HasPrefix(proto, "udp") {
			connState = types.StateUnknown
		} else {
			connState = parseState(state)
		}

		conn := &types.Connection{
			Proto:      parseProtocol(proto),
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   remoteIP,
			RemotePort: remotePort,
			State:      connState,
			PID:        0, // netstat without -p doesn't show PID
			Process:    "",
			CreatedAt:  time.Now(),
		}

		connections = append(connections, conn)
	}

	return connections, nil
}

// parseNetstatAddress parses address from Darwin netstat output
func (c *darwinCollector) parseNetstatAddress(addr string) (string, int) {
	if addr == "*.*" {
		return "0.0.0.0", 0
	}

	// Darwin netstat uses . as port separator
	lastDot := strings.LastIndex(addr, ".")
	if lastDot == -1 {
		return addr, 0
	}

	ip := addr[:lastDot]
	portStr := addr[lastDot+1:]

	// Handle special cases
	if ip == "*" {
		ip = "0.0.0.0"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return ip, 0
	}

	return ip, port
}
