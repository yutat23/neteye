//go:build windows

package collector

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/example/neteye/pkg/types"
)

// windowsCollector implements network connection collection for Windows
type windowsCollector struct {
	*baseCollector
	processCache map[int]string // Cache for PID -> process name mapping
}

// newWindowsCollector creates a new Windows collector
func newWindowsCollector() (Collector, error) {
	return &windowsCollector{
		baseCollector: newBaseCollector(),
		processCache:  make(map[int]string),
	}, nil
}

// SupportsRealtime returns true as Windows supports real-time monitoring
func (c *windowsCollector) SupportsRealtime() bool {
	return true
}

// Collect retrieves network connections using netstat command
func (c *windowsCollector) Collect(ctx context.Context, opts *types.CollectorOptions) ([]*types.Connection, error) {
	var connections []*types.Connection

	// Clear and populate process cache for this collection
	c.processCache = make(map[int]string)
	if err := c.populateProcessCache(ctx); err != nil {
		// Log error but continue - we can still get connections without process names
		// logWarning("failed to populate process cache", err)
	}

	// Use netstat -ano to get connections with PID information
	args := []string{"-ano"}
	
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

	connections, err = c.parseNetstatOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse netstat output: %w", err)
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

// parseNetstatOutput parses the output of netstat command
func (c *windowsCollector) parseNetstatOutput(output string) ([]*types.Connection, error) {
	var connections []*types.Connection
	lines := strings.Split(output, "\n")

	// Regex to parse netstat output
	// Example: TCP    192.168.1.100:445     0.0.0.0:0              LISTENING       4
	tcpRegex := regexp.MustCompile(`^\s*(TCP|UDP)\s+([^\s]+)\s+([^\s]+)\s+([^\s]+)\s+(\d+)\s*$`)

	// Alternative regex for UDP which doesn't have state
	// Example: UDP    192.168.1.100:137     *:*                                    4
	udpRegex := regexp.MustCompile(`^\s*UDP\s+([^\s]+)\s+([^\s]+)\s+(\d+)\s*$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "Proto") {
			continue
		}

		var conn *types.Connection
		var err error

		// Try TCP pattern first
		matches := tcpRegex.FindStringSubmatch(line)
		if len(matches) == 6 {
			conn, err = c.parseNetstatLine(matches[1], matches[2], matches[3], matches[4], matches[5])
		} else {
			// Try UDP pattern
			matches = udpRegex.FindStringSubmatch(line)
			if len(matches) == 4 {
				conn, err = c.parseNetstatLine("UDP", matches[1], matches[2], "", matches[3])
			}
		}

		if err != nil {
			continue // Skip invalid lines
		}

		if conn != nil {
			connections = append(connections, conn)
		}
	}

	return connections, nil
}

// parseNetstatLine parses a single line from netstat output
func (c *windowsCollector) parseNetstatLine(proto, local, remote, state, pidStr string) (*types.Connection, error) {
	// Parse PID
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PID: %w", err)
	}

	// Parse local address
	localIP, localPort, err := parseWindowsAddress(local)
	if err != nil {
		return nil, fmt.Errorf("failed to parse local address: %w", err)
	}

	// Parse remote address
	remoteIP, remotePort, err := parseWindowsAddress(remote)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote address: %w", err)
	}

	// Get process name
	processName := c.getProcessName(pid)

	// Parse state (UDP doesn't have state in netstat output)
	var connState types.ConnectionState
	if proto == "UDP" {
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
		PID:        pid,
		Process:    processName,
		CreatedAt:  time.Now(),
	}

	return conn, nil
}

// parseWindowsAddress parses address from Windows netstat output
func parseWindowsAddress(addr string) (string, int, error) {
	if addr == "*:*" {
		return "0.0.0.0", 0, nil
	}

	// Handle IPv6 addresses in brackets
	if strings.HasPrefix(addr, "[") {
		// Find the closing bracket
		closingBracket := strings.LastIndex(addr, "]")
		if closingBracket == -1 {
			return "", 0, fmt.Errorf("invalid IPv6 address format")
		}

		ip := addr[1:closingBracket]
		portStr := addr[closingBracket+2:] // Skip ]:

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("failed to parse port: %w", err)
		}

		return ip, port, nil
	}

	// Handle IPv4 addresses
	lastColon := strings.LastIndex(addr, ":")
	if lastColon == -1 {
		return "", 0, fmt.Errorf("invalid address format")
	}

	ip := addr[:lastColon]
	portStr := addr[lastColon+1:]

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse port: %w", err)
	}

	return ip, port, nil
}

// populateProcessCache populates the process cache with all running processes
func (c *windowsCollector) populateProcessCache(ctx context.Context) error {
	// Use PowerShell to get all processes in one command
	script := "Get-Process | ForEach-Object { \"$($_.Id):$($_.ProcessName)\" }"
	cmd := exec.CommandContext(ctx, "powershell", "-Command", script)
	output, err := cmd.Output()
	if err != nil {
		// Fallback to tasklist
		return c.populateProcessCacheWithTasklist(ctx)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse "PID:ProcessName" format
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			if pid, err := strconv.Atoi(parts[0]); err == nil {
				c.processCache[pid] = parts[1]
			}
		}
	}

	return nil
}

// populateProcessCacheWithTasklist fallback method using tasklist
func (c *windowsCollector) populateProcessCacheWithTasklist(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "tasklist", "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("tasklist command failed: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse CSV: "ProcessName","PID","SessionName","Session#","MemUsage"
		fields := parseCSVLine(line)
		if len(fields) >= 2 {
			processName := strings.Trim(fields[0], `"`)
			pidStr := strings.Trim(fields[1], `"`)
			
			if pid, err := strconv.Atoi(pidStr); err == nil {
				c.processCache[pid] = processName
			}
		}
	}

	return nil
}

// getProcessName gets process name from PID using the cache
func (c *windowsCollector) getProcessName(pid int) string {
	if pid == 0 {
		return ""
	}

	// Check cache first
	if processName, exists := c.processCache[pid]; exists {
		return processName
	}

	// If not in cache, return empty string
	// The cache should have been populated during Collect()
	return ""
}


// parseCSVLine parses a CSV line respecting quoted fields
func parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false

	for _, r := range line {
		switch r {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case ',':
			if inQuotes {
				current.WriteRune(r)
			} else {
				fields = append(fields, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	// Add the last field
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}

	return fields
}

// Alternative implementation using PowerShell for more detailed information
func (c *windowsCollector) collectWithPowerShell(ctx context.Context, opts *types.CollectorOptions) ([]*types.Connection, error) {
	// PowerShell script to get network connections with process information
	script := `Get-NetTCPConnection | Select-Object LocalAddress,LocalPort,RemoteAddress,RemotePort,State,OwningProcess | ConvertTo-Json`

	cmd := exec.CommandContext(ctx, "powershell", "-Command", script)
	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("PowerShell command failed: %w", err)
	}

	// Parse JSON output would go here
	// This is a simplified implementation
	return nil, fmt.Errorf("PowerShell implementation not yet complete")
}
