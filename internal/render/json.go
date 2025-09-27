package render

import (
	"encoding/json"
	"io"
	"time"

	"github.com/example/neteye/pkg/types"
)

// JSONRenderer handles JSON output formatting
type JSONRenderer struct {
	writer io.Writer
	pretty bool
}

// NewJSONRenderer creates a new JSON renderer
func NewJSONRenderer(writer io.Writer, pretty bool) *JSONRenderer {
	return &JSONRenderer{
		writer: writer,
		pretty: pretty,
	}
}

// RenderConnections renders connections as JSON array
func (r *JSONRenderer) RenderConnections(connections []*types.Connection) error {
	var output []byte
	var err error

	if r.pretty {
		output, err = json.MarshalIndent(connections, "", "  ")
	} else {
		output, err = json.Marshal(connections)
	}

	if err != nil {
		return err
	}

	// Add newline for better readability
	output = append(output, '\n')

	_, err = r.writer.Write(output)
	return err
}

// RenderWatchEvent renders a single watch event as NDJSON (newline-delimited JSON)
func (r *JSONRenderer) RenderWatchEvent(event *types.WatchEvent) error {
	output, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Add newline for NDJSON format
	output = append(output, '\n')

	_, err = r.writer.Write(output)
	return err
}

// RenderConnectionGraph renders a connection graph as JSON
func (r *JSONRenderer) RenderConnectionGraph(graph *types.ConnectionGraph) error {
	var output []byte
	var err error

	if r.pretty {
		output, err = json.MarshalIndent(graph, "", "  ")
	} else {
		output, err = json.Marshal(graph)
	}

	if err != nil {
		return err
	}

	// Add newline for better readability
	output = append(output, '\n')

	_, err = r.writer.Write(output)
	return err
}

// RenderError renders an error as JSON log entry
func (r *JSONRenderer) RenderError(operation, message, code string, err error) error {
	logEntry := &types.LogEntry{
		Level:     "error",
		Timestamp: time.Now(),
		Operation: operation,
		Message:   message,
		Code:      code,
	}

	if err != nil {
		logEntry.Error = err.Error()
	}

	output, jsonErr := json.Marshal(logEntry)
	if jsonErr != nil {
		return jsonErr
	}

	// Add newline
	output = append(output, '\n')

	_, writeErr := r.writer.Write(output)
	return writeErr
}

// RenderInfo renders an info message as JSON log entry
func (r *JSONRenderer) RenderInfo(operation, message string, data interface{}) error {
	logEntry := &types.LogEntry{
		Level:     "info",
		Timestamp: time.Now(),
		Operation: operation,
		Message:   message,
		Data:      data,
	}

	output, err := json.Marshal(logEntry)
	if err != nil {
		return err
	}

	// Add newline
	output = append(output, '\n')

	_, err = r.writer.Write(output)
	return err
}

// RenderRaw renders raw JSON data
func (r *JSONRenderer) RenderRaw(data interface{}) error {
	var output []byte
	var err error

	if r.pretty {
		output, err = json.MarshalIndent(data, "", "  ")
	} else {
		output, err = json.Marshal(data)
	}

	if err != nil {
		return err
	}

	// Add newline
	output = append(output, '\n')

	_, err = r.writer.Write(output)
	return err
}
