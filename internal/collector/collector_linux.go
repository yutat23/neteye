//go:build linux

package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yutat23/neteye/pkg/types"
)

// linuxCollector implements network connection collection for Linux
type linuxCollector struct {
	*baseCollector
}

// newLinuxCollector creates a new Linux collector
func newLinuxCollector() (Collector, error) {
	return &linuxCollector{
		baseCollector: newBaseCollector(),
	}, nil
}

// SupportsRealtime returns true as Linux supports real-time monitoring
func (c *linuxCollector) SupportsRealtime() bool {
	return true
}

// Collect retrieves network connections using /proc filesystem
func (c *linuxCollector) Collect(ctx context.Context, opts *types.CollectorOptions) ([]*types.Connection, error) {
	var connections []*types.Connection

	// Collect TCP connections
	tcpConns, err := c.collectFromProc(ctx, "tcp", opts)
	if err != nil {
		// Fallback to ss command
		tcpConns, err = c.collectWithSS(ctx, "tcp", opts)
		if err != nil {
			return nil, fmt.Errorf("failed to collect TCP connections: %w", err)
		}
	}
	connections = append(connections, tcpConns...)

	// Collect TCP6 connections
	tcp6Conns, err := c.collectFromProc(ctx, "tcp6", opts)
	if err == nil {
		connections = append(connections, tcp6Conns...)
	}

	// Collect UDP connections if not filtered to TCP only
	if opts == nil || opts.Proto == "" || opts.Proto == types.ProtocolUDP {
		udpConns, err := c.collectFromProc(ctx, "udp", opts)
		if err != nil {
			// Fallback to ss command
			udpConns, err = c.collectWithSS(ctx, "udp", opts)
			if err == nil {
				connections = append(connections, udpConns...)
			}
		} else {
			connections = append(connections, udpConns...)
		}

		// Collect UDP6 connections
		udp6Conns, err := c.collectFromProc(ctx, "udp6", opts)
		if err == nil {
			connections = append(connections, udp6Conns...)
		}
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

// collectFromProc reads connection information from /proc/net files
func (c *linuxCollector) collectFromProc(ctx context.Context, protocol string, opts *types.CollectorOptions) ([]*types.Connection, error) {
	procFile := fmt.Sprintf("/proc/net/%s", protocol)

	file, err := os.Open(procFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", procFile, err)
	}
	defer file.Close()

	var connections []*types.Connection
	scanner := bufio.NewScanner(file)

	// Skip header line
	if !scanner.Scan() {
		return connections, nil
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		conn, err := c.parseProcLine(ctx, line, protocol, opts)
		if err != nil {
			continue // Skip invalid lines
		}

		if conn != nil {
			connections = append(connections, conn)
		}
	}

	return connections, scanner.Err()
}

// parseProcLine parses a line from /proc/net/{tcp,udp} file
func (c *linuxCollector) parseProcLine(ctx context.Context, line, protocol string, opts *types.CollectorOptions) (*types.Connection, error) {
	fields := strings.Fields(line)
	if len(fields) < 10 {
		return nil, fmt.Errorf("invalid proc line format")
	}

	// Parse local address
	localIP, localPort, err := parseAddress(fields[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse local address: %w", err)
	}

	// Parse remote address
	remoteIP, remotePort, err := parseAddress(fields[2])
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote address: %w", err)
	}

	// Parse state
	stateHex := fields[3]
	state := parseStateFromHex(stateHex)

	// Parse inode
	inode, err := strconv.ParseUint(fields[9], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse inode: %w", err)
	}

	// Get process information from inode
	pid, process := c.getProcessFromInode(inode)

	conn := &types.Connection{
		Proto:      parseProtocol(protocol),
		LocalIP:    localIP,
		LocalPort:  localPort,
		RemoteIP:   remoteIP,
		RemotePort: remotePort,
		State:      state,
		PID:        pid,
		Process:    process,
		CreatedAt:  time.Now(),
	}

	return conn, nil
}

// parseAddress parses hex-encoded address from /proc/net format
func parseAddress(addrStr string) (string, int, error) {
	parts := strings.Split(addrStr, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid address format")
	}

	// Parse IP address
	ipHex := parts[0]
	var ip string

	if len(ipHex) == 8 {
		// IPv4
		ipBytes := make([]byte, 4)
		for i := 0; i < 4; i++ {
			b, err := strconv.ParseUint(ipHex[i*2:(i+1)*2], 16, 8)
			if err != nil {
				return "", 0, fmt.Errorf("failed to parse IP: %w", err)
			}
			ipBytes[3-i] = byte(b) // Little-endian
		}
		ip = fmt.Sprintf("%d.%d.%d.%d", ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3])
	} else if len(ipHex) == 32 {
		// IPv6
		ipBytes := make([]byte, 16)
		for i := 0; i < 16; i++ {
			b, err := strconv.ParseUint(ipHex[i*2:(i+1)*2], 16, 8)
			if err != nil {
				return "", 0, fmt.Errorf("failed to parse IPv6: %w", err)
			}
			ipBytes[i] = byte(b)
		}
		// Format as IPv6
		ip = fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x",
			uint16(ipBytes[0])<<8|uint16(ipBytes[1]),
			uint16(ipBytes[2])<<8|uint16(ipBytes[3]),
			uint16(ipBytes[4])<<8|uint16(ipBytes[5]),
			uint16(ipBytes[6])<<8|uint16(ipBytes[7]),
			uint16(ipBytes[8])<<8|uint16(ipBytes[9]),
			uint16(ipBytes[10])<<8|uint16(ipBytes[11]),
			uint16(ipBytes[12])<<8|uint16(ipBytes[13]),
			uint16(ipBytes[14])<<8|uint16(ipBytes[15]))
	} else {
		return "", 0, fmt.Errorf("unsupported IP format")
	}

	// Parse port
	portHex := parts[1]
	port, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return "", 0, fmt.Errorf("failed to parse port: %w", err)
	}

	return ip, int(port), nil
}

// parseStateFromHex converts hex state to ConnectionState
func parseStateFromHex(stateHex string) types.ConnectionState {
	stateNum, err := strconv.ParseUint(stateHex, 16, 8)
	if err != nil {
		return types.StateUnknown
	}

	switch stateNum {
	case 0x01:
		return types.StateEstablished
	case 0x02:
		return types.StateSynSent
	case 0x03:
		return types.StateSynRecv
	case 0x04:
		return types.StateFinWait1
	case 0x05:
		return types.StateFinWait2
	case 0x06:
		return types.StateTimeWait
	case 0x07:
		return types.StateCloseWait
	case 0x08:
		return types.StateLastAck
	case 0x09:
		return types.StateClosing
	case 0x0A:
		return types.StateListen
	default:
		return types.StateUnknown
	}
}

// getProcessFromInode gets process name and PID from socket inode
func (c *linuxCollector) getProcessFromInode(inode uint64) (int, string) {
	if inode == 0 {
		return 0, ""
	}

	inodeStr := fmt.Sprintf("socket:[%d]", inode)

	// Search through /proc/*/fd/ for matching socket
	procDirs, err := filepath.Glob("/proc/[0-9]*/fd")
	if err != nil {
		return 0, ""
	}

	for _, procDir := range procDirs {
		pidStr := strings.Split(procDir, "/")[2]
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		// Check if this PID has the socket
		fdEntries, err := os.ReadDir(procDir)
		if err != nil {
			continue
		}

		for _, entry := range fdEntries {
			fdPath := filepath.Join(procDir, entry.Name())
			link, err := os.Readlink(fdPath)
			if err != nil {
				continue
			}

			if link == inodeStr {
				// Found the process, get its name
				processName := c.getProcessName(pid)
				return pid, processName
			}
		}
	}

	return 0, ""
}

// getProcessName gets the process name from PID
func (c *linuxCollector) getProcessName(pid int) string {
	commFile := fmt.Sprintf("/proc/%d/comm", pid)
	data, err := os.ReadFile(commFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// collectWithSS collects connections using the ss command as fallback
func (c *linuxCollector) collectWithSS(ctx context.Context, protocol string, opts *types.CollectorOptions) ([]*types.Connection, error) {
	args := []string{"-tuln"}
	if protocol == "tcp" {
		args = []string{"-tln"}
	} else if protocol == "udp" {
		args = []string{"-uln"}
	}

	cmd := exec.CommandContext(ctx, "ss", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ss command failed: %w", err)
	}

	return c.parseSSOutput(string(output), protocol)
}

// parseSSOutput parses the output of ss command
func (c *linuxCollector) parseSSOutput(output, protocol string) ([]*types.Connection, error) {
	var connections []*types.Connection
	lines := strings.Split(output, "\n")

	// Regex to parse ss output
	// Example: tcp    LISTEN     0      128        0.0.0.0:22         0.0.0.0:*
	re := regexp.MustCompile(`(\w+)\s+(\w+)\s+\d+\s+\d+\s+([^\s]+)\s+([^\s]+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Netid") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) != 5 {
			continue
		}

		proto := matches[1]
		state := matches[2]
		local := matches[3]
		remote := matches[4]

		// Parse addresses
		localIP, localPort := parseSSAddress(local)
		remoteIP, remotePort := parseSSAddress(remote)

		conn := &types.Connection{
			Proto:      parseProtocol(proto),
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   remoteIP,
			RemotePort: remotePort,
			State:      parseState(state),
			PID:        0, // ss without -p doesn't show PID
			Process:    "",
			CreatedAt:  time.Now(),
		}

		connections = append(connections, conn)
	}

	return connections, nil
}

// parseSSAddress parses address from ss output
func parseSSAddress(addr string) (string, int) {
	if addr == "*" {
		return "0.0.0.0", 0
	}

	// Handle IPv6 addresses in brackets
	if strings.HasPrefix(addr, "[") && strings.Contains(addr, "]:") {
		parts := strings.Split(addr, "]:")
		if len(parts) == 2 {
			ip := strings.TrimPrefix(parts[0], "[")
			port, _ := strconv.Atoi(parts[1])
			return ip, port
		}
	}

	// Handle IPv4 addresses
	parts := strings.Split(addr, ":")
	if len(parts) >= 2 {
		ip := strings.Join(parts[:len(parts)-1], ":")
		port, _ := strconv.Atoi(parts[len(parts)-1])
		return ip, port
	}

	return addr, 0
}
