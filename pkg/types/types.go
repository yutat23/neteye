package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// Protocol represents the network protocol type
type Protocol string

const (
	ProtocolTCP Protocol = "tcp"
	ProtocolUDP Protocol = "udp"
)

// ConnectionState represents the state of a network connection
type ConnectionState string

const (
	StateListen      ConnectionState = "LISTEN"
	StateEstablished ConnectionState = "ESTABLISHED"
	StateTimeWait    ConnectionState = "TIME_WAIT"
	StateCloseWait   ConnectionState = "CLOSE_WAIT"
	StateFinWait1    ConnectionState = "FIN_WAIT1"
	StateFinWait2    ConnectionState = "FIN_WAIT2"
	StateClosing     ConnectionState = "CLOSING"
	StateLastAck     ConnectionState = "LAST_ACK"
	StateSynSent     ConnectionState = "SYN_SENT"
	StateSynRecv     ConnectionState = "SYN_RECV"
	StateUnknown     ConnectionState = "UNKNOWN"
)

// Connection represents a network connection entry
// JSON Schema: https://example.com/schemas/neteye/connection.json
type Connection struct {
	Proto      Protocol        `json:"proto" yaml:"proto"`
	LocalIP    string          `json:"local_ip" yaml:"local_ip"`
	LocalPort  int             `json:"local_port" yaml:"local_port"`
	RemoteIP   string          `json:"remote_ip" yaml:"remote_ip"`
	RemotePort int             `json:"remote_port" yaml:"remote_port"`
	State      ConnectionState `json:"state" yaml:"state"`
	PID        int             `json:"pid" yaml:"pid"`
	Process    string          `json:"process" yaml:"process"`
	CreatedAt  time.Time       `json:"created_at" yaml:"created_at"`
}

// String returns a string representation of the connection
func (c *Connection) String() string {
	return fmt.Sprintf("%s %s:%d -> %s:%d [%s] PID:%d (%s)",
		c.Proto, c.LocalIP, c.LocalPort, c.RemoteIP, c.RemotePort,
		c.State, c.PID, c.Process)
}

// LocalAddress returns the local address as "ip:port"
func (c *Connection) LocalAddress() string {
	return fmt.Sprintf("%s:%d", c.LocalIP, c.LocalPort)
}

// RemoteAddress returns the remote address as "ip:port"
func (c *Connection) RemoteAddress() string {
	if c.RemoteIP == "0.0.0.0" || c.RemoteIP == "::" {
		return "*:*"
	}
	return fmt.Sprintf("%s:%d", c.RemoteIP, c.RemotePort)
}

// NodeKind represents the type of node in a graph
type NodeKind string

const (
	NodeKindHost NodeKind = "host"
	NodeKindProc NodeKind = "proc"
	NodeKindPort NodeKind = "port"
)

// GraphNode represents a node in the connection graph
type GraphNode struct {
	ID    string   `json:"id" yaml:"id"`
	Label string   `json:"label" yaml:"label"`
	Kind  NodeKind `json:"kind" yaml:"kind"`
}

// EdgeMeta contains metadata about a graph edge
type EdgeMeta struct {
	Proto Protocol        `json:"proto" yaml:"proto"`
	Port  int             `json:"port" yaml:"port"`
	State ConnectionState `json:"state" yaml:"state"`
}

// GraphEdge represents an edge in the connection graph
type GraphEdge struct {
	From string    `json:"from" yaml:"from"`
	To   string    `json:"to" yaml:"to"`
	Meta *EdgeMeta `json:"meta" yaml:"meta"`
}

// ConnectionGraph represents the complete connection graph
// JSON Schema: https://example.com/schemas/neteye/graph.json
type ConnectionGraph struct {
	Nodes []*GraphNode `json:"nodes" yaml:"nodes"`
	Edges []*GraphEdge `json:"edges" yaml:"edges"`
}

// WatchEvent represents a change event during monitoring
type WatchEvent struct {
	Type       string     `json:"type" yaml:"type"` // "NEW", "CLOSED"
	Connection Connection `json:"connection" yaml:"connection"`
	Timestamp  time.Time  `json:"timestamp" yaml:"timestamp"`
}

// OutputFormat represents the output format type
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatCSV   OutputFormat = "csv"
)

// GraphFormat represents the graph output format type
type GraphFormat string

const (
	GraphFormatJSON GraphFormat = "json"
	GraphFormatDOT  GraphFormat = "dot"
)

// SortKey represents the sort key for connections
type SortKey string

const (
	SortByProto  SortKey = "proto"
	SortByLocal  SortKey = "local"
	SortByRemote SortKey = "remote"
	SortByState  SortKey = "state"
	SortByPID    SortKey = "pid"
	SortByProc   SortKey = "proc"
)

// CollectorOptions contains options for the network collector
type CollectorOptions struct {
	PID        int      `yaml:"pid"`
	RemoteHost string   `yaml:"remote_host"`
	Proto      Protocol `yaml:"proto"`
	ListenOnly bool     `yaml:"listen_only"`
	Resolve    bool     `yaml:"resolve"`
}

// WatchOptions contains options for the watch command
type WatchOptions struct {
	Port     int           `yaml:"port"`
	Interval time.Duration `yaml:"interval"`
	Duration time.Duration `yaml:"duration"`
	Quiet    bool          `yaml:"quiet"`
}

// MapOptions contains options for the map command
type MapOptions struct {
	GraphFormat   GraphFormat `yaml:"graph_format"`
	CollapsePorts bool        `yaml:"collapse_ports"`
	Resolve       bool        `yaml:"resolve"`
}

// GlobalOptions contains global CLI options
type GlobalOptions struct {
	Format   OutputFormat `yaml:"format"`
	NoColor  bool         `yaml:"no_color"`
	Resolve  bool         `yaml:"resolve"`
	SortBy   SortKey      `yaml:"sort_by"`
	LogLevel string       `yaml:"log_level"`
}

// Config represents the application configuration
type Config struct {
	Global   GlobalOptions    `yaml:"global"`
	Collect  CollectorOptions `yaml:"collect"`
	Watch    WatchOptions     `yaml:"watch"`
	Map      MapOptions       `yaml:"map"`
	DNSCache struct {
		TTL     time.Duration `yaml:"ttl"`
		MaxSize int           `yaml:"max_size"`
	} `yaml:"dns_cache"`
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Level     string    `json:"level"`
	Timestamp time.Time `json:"ts"`
	Operation string    `json:"op"`
	Message   string    `json:"msg"`
	Code      string    `json:"code,omitempty"`
	Error     string    `json:"error,omitempty"`
	Data      any       `json:"data,omitempty"`
}

// JSON returns the log entry as JSON
func (l *LogEntry) JSON() string {
	b, _ := json.Marshal(l)
	return string(b)
}

// ExitCode represents application exit codes
type ExitCode int

const (
	ExitOK           ExitCode = 0
	ExitGeneralError ExitCode = 1
	ExitUsageError   ExitCode = 2
	ExitPermError    ExitCode = 3
)
