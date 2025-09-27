package render

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/example/neteye/pkg/types"
	"github.com/olekukonko/tablewriter"
)

// TableRenderer handles table output formatting
type TableRenderer struct {
	writer  io.Writer
	noColor bool
	sortBy  types.SortKey
}

// NewTableRenderer creates a new table renderer
func NewTableRenderer(writer io.Writer, noColor bool, sortBy types.SortKey) *TableRenderer {
	return &TableRenderer{
		writer:  writer,
		noColor: noColor,
		sortBy:  sortBy,
	}
}

// RenderConnections renders connections as a formatted table
func (r *TableRenderer) RenderConnections(connections []*types.Connection) error {
	if len(connections) == 0 {
		return nil
	}

	// Sort connections based on sortBy option
	r.sortConnections(connections)

	// Create table
	table := tablewriter.NewWriter(r.writer)

	// Set table headers - fixed as per specification
	headers := []string{"PROTO", "LOCAL", "REMOTE", "STATE", "PID", "PROC"}
	table.SetHeader(headers)

	// Configure table appearance
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeaderLine(false)
	table.SetRowLine(false)
	table.SetColumnSeparator("  ")
	table.SetTablePadding("  ")

	// Disable colors if requested
	if r.noColor {
		table.SetColMinWidth(0, 6)  // PROTO
		table.SetColMinWidth(1, 18) // LOCAL
		table.SetColMinWidth(2, 18) // REMOTE
		table.SetColMinWidth(3, 12) // STATE
		table.SetColMinWidth(4, 6)  // PID
		table.SetColMinWidth(5, 8)  // PROC
	} else {
		// Enable colors for better readability
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.Bold}, // PROTO
			tablewriter.Colors{tablewriter.Bold}, // LOCAL
			tablewriter.Colors{tablewriter.Bold}, // REMOTE
			tablewriter.Colors{tablewriter.Bold}, // STATE
			tablewriter.Colors{tablewriter.Bold}, // PID
			tablewriter.Colors{tablewriter.Bold}, // PROC
		)
	}

	// Add rows
	for _, conn := range connections {
		row := []string{
			string(conn.Proto),
			conn.LocalAddress(),
			conn.RemoteAddress(),
			string(conn.State),
			strconv.Itoa(conn.PID),
			conn.Process,
		}

		// Apply colors based on connection state if colors are enabled
		if !r.noColor {
			r.colorizeRow(table, row, conn)
		} else {
			table.Append(row)
		}
	}

	table.Render()
	return nil
}

// colorizeRow applies color coding to table rows based on connection state
func (r *TableRenderer) colorizeRow(table *tablewriter.Table, row []string, conn *types.Connection) {
	var colors []tablewriter.Colors

	// Colorize based on protocol
	var protoColor tablewriter.Colors
	switch conn.Proto {
	case types.ProtocolTCP:
		protoColor = tablewriter.Colors{tablewriter.FgCyanColor}
	case types.ProtocolUDP:
		protoColor = tablewriter.Colors{tablewriter.FgYellowColor}
	default:
		protoColor = tablewriter.Colors{}
	}

	// Colorize based on state
	var stateColor tablewriter.Colors
	switch conn.State {
	case types.StateListen:
		stateColor = tablewriter.Colors{tablewriter.FgGreenColor}
	case types.StateEstablished:
		stateColor = tablewriter.Colors{tablewriter.FgBlueColor}
	case types.StateTimeWait, types.StateCloseWait, types.StateFinWait1, types.StateFinWait2:
		stateColor = tablewriter.Colors{tablewriter.FgYellowColor}
	default:
		stateColor = tablewriter.Colors{}
	}

	colors = []tablewriter.Colors{
		protoColor, // PROTO
		{},         // LOCAL
		{},         // REMOTE
		stateColor, // STATE
		{},         // PID
		{},         // PROC
	}

	table.Rich(row, colors)
}

// sortConnections sorts connections based on the specified sort key
func (r *TableRenderer) sortConnections(connections []*types.Connection) {
	sort.Slice(connections, func(i, j int) bool {
		a, b := connections[i], connections[j]

		// Primary sort by the specified key
		switch r.sortBy {
		case types.SortByProto:
			if a.Proto != b.Proto {
				return a.Proto < b.Proto
			}
		case types.SortByLocal:
			localA := fmt.Sprintf("%s:%d", a.LocalIP, a.LocalPort)
			localB := fmt.Sprintf("%s:%d", b.LocalIP, b.LocalPort)
			if localA != localB {
				return localA < localB
			}
		case types.SortByRemote:
			remoteA := fmt.Sprintf("%s:%d", a.RemoteIP, a.RemotePort)
			remoteB := fmt.Sprintf("%s:%d", b.RemoteIP, b.RemotePort)
			if remoteA != remoteB {
				return remoteA < remoteB
			}
		case types.SortByState:
			if a.State != b.State {
				return a.State < b.State
			}
		case types.SortByPID:
			if a.PID != b.PID {
				return a.PID < b.PID
			}
		case types.SortByProc:
			if a.Process != b.Process {
				return a.Process < b.Process
			}
		}

		// Default secondary sort: proto asc, state asc, local asc, remote asc, pid asc
		if a.Proto != b.Proto {
			return a.Proto < b.Proto
		}
		if a.State != b.State {
			return a.State < b.State
		}

		localA := fmt.Sprintf("%s:%d", a.LocalIP, a.LocalPort)
		localB := fmt.Sprintf("%s:%d", b.LocalIP, b.LocalPort)
		if localA != localB {
			return localA < localB
		}

		remoteA := fmt.Sprintf("%s:%d", a.RemoteIP, a.RemotePort)
		remoteB := fmt.Sprintf("%s:%d", b.RemoteIP, b.RemotePort)
		if remoteA != remoteB {
			return remoteA < remoteB
		}

		return a.PID < b.PID
	})
}

// RenderWatchEvent renders a single watch event in table format
func (r *TableRenderer) RenderWatchEvent(event *types.WatchEvent) error {
	var prefix string
	switch event.Type {
	case "NEW":
		if r.noColor {
			prefix = "[NEW]"
		} else {
			prefix = "\033[32m[NEW]\033[0m" // Green
		}
	case "CLOSED":
		if r.noColor {
			prefix = "[CLOSED]"
		} else {
			prefix = "\033[31m[CLOSED]\033[0m" // Red
		}
	default:
		prefix = fmt.Sprintf("[%s]", event.Type)
	}

	conn := &event.Connection
	timestamp := event.Timestamp.Format("2006-01-02T15:04:05Z")

	// Format: [EVENT] remote -> local (PROTO) timestamp
	output := fmt.Sprintf("%s  %s -> %s (%s)  %s\n",
		prefix,
		conn.RemoteAddress(),
		conn.LocalAddress(),
		strings.ToUpper(string(conn.Proto)),
		timestamp,
	)

	_, err := r.writer.Write([]byte(output))
	return err
}

// RenderSummary renders a summary of connections
func (r *TableRenderer) RenderSummary(connections []*types.Connection) error {
	if len(connections) == 0 {
		_, err := r.writer.Write([]byte("No connections found.\n"))
		return err
	}

	// Count by protocol and state
	tcpCount := 0
	udpCount := 0
	stateCount := make(map[types.ConnectionState]int)

	for _, conn := range connections {
		if conn.Proto == types.ProtocolTCP {
			tcpCount++
		} else if conn.Proto == types.ProtocolUDP {
			udpCount++
		}
		stateCount[conn.State]++
	}

	// Create summary table
	table := tablewriter.NewWriter(r.writer)
	table.SetHeader([]string{"METRIC", "COUNT"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	// Add summary rows
	table.Append([]string{"Total Connections", strconv.Itoa(len(connections))})
	table.Append([]string{"TCP Connections", strconv.Itoa(tcpCount)})
	table.Append([]string{"UDP Connections", strconv.Itoa(udpCount)})

	// Add state breakdown
	for state, count := range stateCount {
		table.Append([]string{string(state), strconv.Itoa(count)})
	}

	table.Render()
	return nil
}
