package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/example/neteye/pkg/types"
)

// DOTRenderer handles Graphviz DOT output formatting
type DOTRenderer struct {
	writer io.Writer
}

// NewDOTRenderer creates a new DOT renderer
func NewDOTRenderer(writer io.Writer) *DOTRenderer {
	return &DOTRenderer{
		writer: writer,
	}
}

// RenderConnectionGraph renders a connection graph as Graphviz DOT format
func (r *DOTRenderer) RenderConnectionGraph(graph *types.ConnectionGraph) error {
	// Start digraph
	if _, err := r.writer.Write([]byte("digraph neteye {\n")); err != nil {
		return err
	}

	// Set graph attributes
	attrs := []string{
		"  rankdir=LR;",
		"  node [shape=box, style=rounded];",
		"  edge [fontsize=10];",
		"  overlap=false;",
		"  splines=true;",
		"",
	}

	for _, attr := range attrs {
		if _, err := r.writer.Write([]byte(attr + "\n")); err != nil {
			return err
		}
	}

	// Write nodes
	if err := r.writeNodes(graph.Nodes); err != nil {
		return err
	}

	// Write edges
	if err := r.writeEdges(graph.Edges); err != nil {
		return err
	}

	// End digraph
	if _, err := r.writer.Write([]byte("}\n")); err != nil {
		return err
	}

	return nil
}

// writeNodes writes all nodes to the DOT output
func (r *DOTRenderer) writeNodes(nodes []*types.GraphNode) error {
	if len(nodes) == 0 {
		return nil
	}

	// Write nodes section header
	if _, err := r.writer.Write([]byte("  // Nodes\n")); err != nil {
		return err
	}

	for _, node := range nodes {
		if err := r.writeNode(node); err != nil {
			return err
		}
	}

	// Add blank line after nodes
	if _, err := r.writer.Write([]byte("\n")); err != nil {
		return err
	}

	return nil
}

// writeNode writes a single node to the DOT output
func (r *DOTRenderer) writeNode(node *types.GraphNode) error {
	// Escape node ID and label for DOT format
	nodeID := r.escapeDOTString(node.ID)
	label := r.escapeDOTString(node.Label)

	// Determine node style based on kind
	var style, color, shape string
	switch node.Kind {
	case types.NodeKindHost:
		style = "filled"
		color = "lightblue"
		shape = "ellipse"
	case types.NodeKindProc:
		style = "filled"
		color = "lightgreen"
		shape = "box"
	case types.NodeKindPort:
		style = "filled"
		color = "lightyellow"
		shape = "diamond"
	default:
		style = "solid"
		color = "white"
		shape = "box"
	}

	// Write node definition
	nodeDef := fmt.Sprintf("  \"%s\" [label=\"%s\", style=%s, fillcolor=%s, shape=%s];\n",
		nodeID, label, style, color, shape)

	_, err := r.writer.Write([]byte(nodeDef))
	return err
}

// writeEdges writes all edges to the DOT output
func (r *DOTRenderer) writeEdges(edges []*types.GraphEdge) error {
	if len(edges) == 0 {
		return nil
	}

	// Write edges section header
	if _, err := r.writer.Write([]byte("  // Edges\n")); err != nil {
		return err
	}

	for _, edge := range edges {
		if err := r.writeEdge(edge); err != nil {
			return err
		}
	}

	return nil
}

// writeEdge writes a single edge to the DOT output
func (r *DOTRenderer) writeEdge(edge *types.GraphEdge) error {
	// Escape node IDs
	fromID := r.escapeDOTString(edge.From)
	toID := r.escapeDOTString(edge.To)

	// Create edge label from metadata
	var labelParts []string
	if edge.Meta != nil {
		if edge.Meta.Proto != "" {
			labelParts = append(labelParts, string(edge.Meta.Proto))
		}
		if edge.Meta.Port > 0 {
			labelParts = append(labelParts, fmt.Sprintf(":%d", edge.Meta.Port))
		}
		if edge.Meta.State != "" {
			labelParts = append(labelParts, string(edge.Meta.State))
		}
	}

	label := strings.Join(labelParts, " ")
	if label == "" {
		label = "connection"
	}

	// Determine edge style based on protocol and state
	var color, style string
	if edge.Meta != nil {
		switch edge.Meta.Proto {
		case types.ProtocolTCP:
			color = "blue"
			if edge.Meta.State == types.StateListen {
				style = "dashed"
			} else {
				style = "solid"
			}
		case types.ProtocolUDP:
			color = "orange"
			style = "dotted"
		default:
			color = "black"
			style = "solid"
		}
	} else {
		color = "black"
		style = "solid"
	}

	// Write edge definition
	edgeDef := fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\", color=%s, style=%s];\n",
		fromID, toID, r.escapeDOTString(label), color, style)

	_, err := r.writer.Write([]byte(edgeDef))
	return err
}

// escapeDOTString escapes special characters for DOT format
func (r *DOTRenderer) escapeDOTString(s string) string {
	// Replace special characters that need escaping in DOT format
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

// RenderSimpleGraph renders a simplified graph directly from connections
func (r *DOTRenderer) RenderSimpleGraph(connections []*types.Connection, collapsePorts bool) error {
	// Start digraph
	if _, err := r.writer.Write([]byte("digraph neteye_simple {\n")); err != nil {
		return err
	}

	// Set graph attributes
	attrs := []string{
		"  rankdir=LR;",
		"  node [shape=box, style=rounded];",
		"  edge [fontsize=10];",
		"",
	}

	for _, attr := range attrs {
		if _, err := r.writer.Write([]byte(attr + "\n")); err != nil {
			return err
		}
	}

	// Track unique nodes and edges
	nodes := make(map[string]bool)
	edges := make(map[string]bool)

	// Process connections
	for _, conn := range connections {
		var fromNode, toNode string

		if collapsePorts {
			// Group by IP only
			fromNode = conn.LocalIP
			toNode = conn.RemoteIP
		} else {
			// Include port information
			fromNode = fmt.Sprintf("%s:%d", conn.LocalIP, conn.LocalPort)
			toNode = fmt.Sprintf("%s:%d", conn.RemoteIP, conn.RemotePort)
		}

		// Add nodes
		nodes[fromNode] = true
		if toNode != "0.0.0.0:0" && toNode != "*:*" {
			nodes[toNode] = true
		}

		// Create edge key
		var edgeKey string
		if toNode == "0.0.0.0:0" || toNode == "*:*" {
			// Listening socket - show as self-loop or special notation
			edgeKey = fmt.Sprintf("%s -> %s [%s %s]", fromNode, fromNode, conn.Proto, conn.State)
		} else {
			edgeKey = fmt.Sprintf("%s -> %s [%s %s]", fromNode, toNode, conn.Proto, conn.State)
		}
		edges[edgeKey] = true
	}

	// Write nodes
	if _, err := r.writer.Write([]byte("  // Nodes\n")); err != nil {
		return err
	}
	for node := range nodes {
		escapedNode := r.escapeDOTString(node)
		nodeDef := fmt.Sprintf("  \"%s\";\n", escapedNode)
		if _, err := r.writer.Write([]byte(nodeDef)); err != nil {
			return err
		}
	}

	// Write edges
	if _, err := r.writer.Write([]byte("\n  // Edges\n")); err != nil {
		return err
	}
	for edge := range edges {
		edgeDef := fmt.Sprintf("  %s;\n", edge)
		if _, err := r.writer.Write([]byte(edgeDef)); err != nil {
			return err
		}
	}

	// End digraph
	if _, err := r.writer.Write([]byte("}\n")); err != nil {
		return err
	}

	return nil
}
