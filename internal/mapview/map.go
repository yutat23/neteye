package mapview

import (
	"fmt"
	"sort"
	"strings"

	"github.com/example/neteye/pkg/types"
)

// GraphBuilder builds connection graphs from network connections
type GraphBuilder struct {
	collapsePorts   bool
	includeLoopback bool
}

// NewGraphBuilder creates a new graph builder
func NewGraphBuilder(collapsePorts, includeLoopback bool) *GraphBuilder {
	return &GraphBuilder{
		collapsePorts:   collapsePorts,
		includeLoopback: includeLoopback,
	}
}

// BuildGraph builds a connection graph from a list of connections
func (b *GraphBuilder) BuildGraph(connections []*types.Connection) *types.ConnectionGraph {
	nodes := make(map[string]*types.GraphNode)
	edges := make(map[string]*types.GraphEdge)

	for _, conn := range connections {
		// Skip loopback connections if not included
		if !b.includeLoopback && b.isLoopback(conn) {
			continue
		}

		// Generate nodes and edges
		fromNode, toNode := b.generateNodes(conn)
		edge := b.generateEdge(conn, fromNode.ID, toNode.ID)

		// Add nodes to map
		nodes[fromNode.ID] = fromNode
		if toNode.ID != fromNode.ID {
			nodes[toNode.ID] = toNode
		}

		// Add edge to map (use a key to avoid duplicates)
		edgeKey := b.edgeKey(edge)
		if existingEdge, exists := edges[edgeKey]; exists {
			// Merge edge information if needed
			edges[edgeKey] = b.mergeEdges(existingEdge, edge)
		} else {
			edges[edgeKey] = edge
		}
	}

	// Convert maps to slices
	var nodeList []*types.GraphNode
	var edgeList []*types.GraphEdge

	for _, node := range nodes {
		nodeList = append(nodeList, node)
	}

	for _, edge := range edges {
		edgeList = append(edgeList, edge)
	}

	// Sort for consistent output
	b.sortNodes(nodeList)
	b.sortEdges(edgeList)

	return &types.ConnectionGraph{
		Nodes: nodeList,
		Edges: edgeList,
	}
}

// generateNodes creates nodes for the source and destination of a connection
func (b *GraphBuilder) generateNodes(conn *types.Connection) (*types.GraphNode, *types.GraphNode) {
	var fromNode, toNode *types.GraphNode

	if conn.State == types.StateListen {
		// For listening connections, create a process node and a port node
		fromNode = &types.GraphNode{
			ID:    fmt.Sprintf("proc:%d", conn.PID),
			Label: b.formatProcessLabel(conn),
			Kind:  types.NodeKindProc,
		}

		if b.collapsePorts {
			toNode = &types.GraphNode{
				ID:    fmt.Sprintf("host:%s", conn.LocalIP),
				Label: b.formatHostLabel(conn.LocalIP),
				Kind:  types.NodeKindHost,
			}
		} else {
			toNode = &types.GraphNode{
				ID:    fmt.Sprintf("port:%s:%d", conn.LocalIP, conn.LocalPort),
				Label: b.formatPortLabel(conn.LocalIP, conn.LocalPort),
				Kind:  types.NodeKindPort,
			}
		}
	} else {
		// For established connections, create host nodes
		if b.collapsePorts {
			fromNode = &types.GraphNode{
				ID:    fmt.Sprintf("host:%s", conn.LocalIP),
				Label: b.formatHostLabel(conn.LocalIP),
				Kind:  types.NodeKindHost,
			}
			toNode = &types.GraphNode{
				ID:    fmt.Sprintf("host:%s", conn.RemoteIP),
				Label: b.formatHostLabel(conn.RemoteIP),
				Kind:  types.NodeKindHost,
			}
		} else {
			fromNode = &types.GraphNode{
				ID:    fmt.Sprintf("endpoint:%s:%d", conn.LocalIP, conn.LocalPort),
				Label: b.formatEndpointLabel(conn.LocalIP, conn.LocalPort, conn),
				Kind:  types.NodeKindHost,
			}
			toNode = &types.GraphNode{
				ID:    fmt.Sprintf("endpoint:%s:%d", conn.RemoteIP, conn.RemotePort),
				Label: b.formatEndpointLabel(conn.RemoteIP, conn.RemotePort, nil),
				Kind:  types.NodeKindHost,
			}
		}
	}

	return fromNode, toNode
}

// generateEdge creates an edge for the connection
func (b *GraphBuilder) generateEdge(conn *types.Connection, fromID, toID string) *types.GraphEdge {
	meta := &types.EdgeMeta{
		Proto: conn.Proto,
		State: conn.State,
	}

	// Set port based on connection type
	if conn.State == types.StateListen {
		meta.Port = conn.LocalPort
	} else {
		// For established connections, use the service port (usually the lower port)
		if conn.LocalPort < conn.RemotePort {
			meta.Port = conn.LocalPort
		} else {
			meta.Port = conn.RemotePort
		}
	}

	return &types.GraphEdge{
		From: fromID,
		To:   toID,
		Meta: meta,
	}
}

// formatProcessLabel creates a label for a process node
func (b *GraphBuilder) formatProcessLabel(conn *types.Connection) string {
	if conn.Process != "" {
		return fmt.Sprintf("%s(%d)", conn.Process, conn.PID)
	}
	return fmt.Sprintf("PID:%d", conn.PID)
}

// formatHostLabel creates a label for a host node
func (b *GraphBuilder) formatHostLabel(ip string) string {
	if ip == "0.0.0.0" || ip == "::" {
		return "Any Host"
	}
	if ip == "127.0.0.1" || ip == "::1" {
		return "Localhost"
	}
	return ip
}

// formatPortLabel creates a label for a port node
func (b *GraphBuilder) formatPortLabel(ip string, port int) string {
	hostLabel := b.formatHostLabel(ip)
	return fmt.Sprintf("%s:%d", hostLabel, port)
}

// formatEndpointLabel creates a label for an endpoint node
func (b *GraphBuilder) formatEndpointLabel(ip string, port int, conn *types.Connection) string {
	hostLabel := b.formatHostLabel(ip)
	label := fmt.Sprintf("%s:%d", hostLabel, port)

	if conn != nil && conn.Process != "" {
		label += fmt.Sprintf(" (%s)", conn.Process)
	}

	return label
}

// edgeKey generates a unique key for an edge
func (b *GraphBuilder) edgeKey(edge *types.GraphEdge) string {
	return fmt.Sprintf("%s->%s:%s:%d:%s",
		edge.From, edge.To, edge.Meta.Proto, edge.Meta.Port, edge.Meta.State)
}

// mergeEdges merges two edges with the same key
func (b *GraphBuilder) mergeEdges(existing, new *types.GraphEdge) *types.GraphEdge {
	// For now, just return the existing edge
	// In the future, we could merge metadata or track connection counts
	return existing
}

// isLoopback checks if a connection is a loopback connection
func (b *GraphBuilder) isLoopback(conn *types.Connection) bool {
	return (conn.LocalIP == "127.0.0.1" || conn.LocalIP == "::1") &&
		(conn.RemoteIP == "127.0.0.1" || conn.RemoteIP == "::1" ||
			conn.RemoteIP == "0.0.0.0" || conn.RemoteIP == "::")
}

// sortNodes sorts nodes by ID for consistent output
func (b *GraphBuilder) sortNodes(nodes []*types.GraphNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
}

// sortEdges sorts edges by from->to for consistent output
func (b *GraphBuilder) sortEdges(edges []*types.GraphEdge) {
	sort.Slice(edges, func(i, j int) bool {
		edgeA := fmt.Sprintf("%s->%s", edges[i].From, edges[i].To)
		edgeB := fmt.Sprintf("%s->%s", edges[j].From, edges[j].To)
		return edgeA < edgeB
	})
}

// ConnectionMapper provides higher-level mapping operations
type ConnectionMapper struct {
	builder *GraphBuilder
}

// NewConnectionMapper creates a new connection mapper
func NewConnectionMapper(collapsePorts bool) *ConnectionMapper {
	return &ConnectionMapper{
		builder: NewGraphBuilder(collapsePorts, false),
	}
}

// MapConnections maps connections to a graph
func (m *ConnectionMapper) MapConnections(connections []*types.Connection) *types.ConnectionGraph {
	return m.builder.BuildGraph(connections)
}

// MapWithOptions maps connections with specific options
func (m *ConnectionMapper) MapWithOptions(connections []*types.Connection, opts *types.MapOptions) *types.ConnectionGraph {
	// Update builder settings based on options
	m.builder.collapsePorts = opts.CollapsePorts

	return m.builder.BuildGraph(connections)
}

// GetGraphStatistics returns statistics about the graph
func (m *ConnectionMapper) GetGraphStatistics(graph *types.ConnectionGraph) GraphStatistics {
	stats := GraphStatistics{
		NodeCount:    len(graph.Nodes),
		EdgeCount:    len(graph.Edges),
		NodesByKind:  make(map[types.NodeKind]int),
		EdgesByProto: make(map[types.Protocol]int),
		EdgesByState: make(map[types.ConnectionState]int),
	}

	// Count nodes by kind
	for _, node := range graph.Nodes {
		stats.NodesByKind[node.Kind]++
	}

	// Count edges by protocol and state
	for _, edge := range graph.Edges {
		if edge.Meta != nil {
			stats.EdgesByProto[edge.Meta.Proto]++
			stats.EdgesByState[edge.Meta.State]++
		}
	}

	return stats
}

// GraphStatistics contains statistics about a connection graph
type GraphStatistics struct {
	NodeCount    int                           `json:"node_count"`
	EdgeCount    int                           `json:"edge_count"`
	NodesByKind  map[types.NodeKind]int        `json:"nodes_by_kind"`
	EdgesByProto map[types.Protocol]int        `json:"edges_by_proto"`
	EdgesByState map[types.ConnectionState]int `json:"edges_by_state"`
}

// VisualMapper provides graph visualization helpers
type VisualMapper struct{}

// NewVisualMapper creates a new visual mapper
func NewVisualMapper() *VisualMapper {
	return &VisualMapper{}
}

// GenerateGraphDescription generates a human-readable description of the graph
func (v *VisualMapper) GenerateGraphDescription(graph *types.ConnectionGraph) string {
	var parts []string

	stats := GraphStatistics{
		NodesByKind:  make(map[types.NodeKind]int),
		EdgesByProto: make(map[types.Protocol]int),
	}

	for _, node := range graph.Nodes {
		stats.NodesByKind[node.Kind]++
	}

	for _, edge := range graph.Edges {
		if edge.Meta != nil {
			stats.EdgesByProto[edge.Meta.Proto]++
		}
	}

	parts = append(parts, fmt.Sprintf("Network graph with %d nodes and %d edges:",
		len(graph.Nodes), len(graph.Edges)))

	if len(stats.NodesByKind) > 0 {
		var nodeTypes []string
		for kind, count := range stats.NodesByKind {
			nodeTypes = append(nodeTypes, fmt.Sprintf("%d %s", count, kind))
		}
		parts = append(parts, fmt.Sprintf("Nodes: %s", strings.Join(nodeTypes, ", ")))
	}

	if len(stats.EdgesByProto) > 0 {
		var protoTypes []string
		for proto, count := range stats.EdgesByProto {
			protoTypes = append(protoTypes, fmt.Sprintf("%d %s", count, proto))
		}
		parts = append(parts, fmt.Sprintf("Connections: %s", strings.Join(protoTypes, ", ")))
	}

	return strings.Join(parts, "\n")
}
