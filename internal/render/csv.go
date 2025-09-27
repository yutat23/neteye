package render

import (
	"encoding/csv"
	"io"
	"strconv"

	"github.com/yutat23/neteye/pkg/types"
)

// CSVRenderer handles CSV output formatting
type CSVRenderer struct {
	writer        io.Writer
	csvWriter     *csv.Writer
	headerWritten bool
}

// NewCSVRenderer creates a new CSV renderer
func NewCSVRenderer(writer io.Writer) *CSVRenderer {
	csvWriter := csv.NewWriter(writer)

	return &CSVRenderer{
		writer:        writer,
		csvWriter:     csvWriter,
		headerWritten: false,
	}
}

// RenderConnections renders connections as CSV format with headers
func (r *CSVRenderer) RenderConnections(connections []*types.Connection) error {
	// Write header if not already written
	if !r.headerWritten {
		if err := r.writeHeader(); err != nil {
			return err
		}
		r.headerWritten = true
	}

	// Write connection rows
	for _, conn := range connections {
		if err := r.writeConnectionRow(conn); err != nil {
			return err
		}
	}

	// Flush the writer to ensure all data is written
	r.csvWriter.Flush()
	return r.csvWriter.Error()
}

// RenderWatchEvent renders a single watch event as CSV row
func (r *CSVRenderer) RenderWatchEvent(event *types.WatchEvent) error {
	// Write header if not already written
	if !r.headerWritten {
		if err := r.writeWatchHeader(); err != nil {
			return err
		}
		r.headerWritten = true
	}

	// Write event row
	row := []string{
		event.Type,
		event.Timestamp.Format("2006-01-02T15:04:05Z"),
		string(event.Connection.Proto),
		event.Connection.LocalIP,
		strconv.Itoa(event.Connection.LocalPort),
		event.Connection.RemoteIP,
		strconv.Itoa(event.Connection.RemotePort),
		string(event.Connection.State),
		strconv.Itoa(event.Connection.PID),
		event.Connection.Process,
	}

	if err := r.csvWriter.Write(row); err != nil {
		return err
	}

	// Flush immediately for streaming output
	r.csvWriter.Flush()
	return r.csvWriter.Error()
}

// writeHeader writes the CSV header for connections
func (r *CSVRenderer) writeHeader() error {
	// Column order as specified: proto,local_ip,local_port,remote_ip,remote_port,state,pid,process,created_at
	header := []string{
		"proto",
		"local_ip",
		"local_port",
		"remote_ip",
		"remote_port",
		"state",
		"pid",
		"process",
		"created_at",
	}

	return r.csvWriter.Write(header)
}

// writeWatchHeader writes the CSV header for watch events
func (r *CSVRenderer) writeWatchHeader() error {
	header := []string{
		"event_type",
		"timestamp",
		"proto",
		"local_ip",
		"local_port",
		"remote_ip",
		"remote_port",
		"state",
		"pid",
		"process",
	}

	return r.csvWriter.Write(header)
}

// writeConnectionRow writes a single connection as CSV row
func (r *CSVRenderer) writeConnectionRow(conn *types.Connection) error {
	row := []string{
		string(conn.Proto),
		conn.LocalIP,
		strconv.Itoa(conn.LocalPort),
		conn.RemoteIP,
		strconv.Itoa(conn.RemotePort),
		string(conn.State),
		strconv.Itoa(conn.PID),
		conn.Process,
		conn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	return r.csvWriter.Write(row)
}

// Flush ensures all buffered data is written
func (r *CSVRenderer) Flush() error {
	r.csvWriter.Flush()
	return r.csvWriter.Error()
}

// Close finalizes the CSV output
func (r *CSVRenderer) Close() error {
	return r.Flush()
}
