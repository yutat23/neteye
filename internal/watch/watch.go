package watch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yutat23/neteye/internal/collector"
	"github.com/yutat23/neteye/pkg/types"
)

// Watcher monitors network connections for changes
type Watcher struct {
	collector collector.Collector
	options   *types.WatchOptions
	callbacks WatchCallbacks

	// Internal state
	lastConnections map[string]*types.Connection
	mutex           sync.RWMutex
}

// WatchCallbacks defines callback functions for watch events
type WatchCallbacks struct {
	OnNew    func(*types.WatchEvent) error
	OnClosed func(*types.WatchEvent) error
	OnError  func(error)
}

// NewWatcher creates a new network connection watcher
func NewWatcher(collector collector.Collector, options *types.WatchOptions, callbacks WatchCallbacks) *Watcher {
	return &Watcher{
		collector:       collector,
		options:         options,
		callbacks:       callbacks,
		lastConnections: make(map[string]*types.Connection),
	}
}

// Start begins monitoring network connections
func (w *Watcher) Start(ctx context.Context, collectorOpts *types.CollectorOptions) error {
	// Validate options
	if w.options.Interval <= 0 {
		w.options.Interval = 500 * time.Millisecond
	}

	// Create a ticker for periodic checking
	ticker := time.NewTicker(w.options.Interval)
	defer ticker.Stop()

	// Create a context with timeout if duration is specified
	var watchCtx context.Context
	var cancel context.CancelFunc

	if w.options.Duration > 0 {
		watchCtx, cancel = context.WithTimeout(ctx, w.options.Duration)
		defer cancel()
	} else {
		watchCtx = ctx
	}

	// Perform initial collection to establish baseline
	if err := w.performInitialCollection(watchCtx, collectorOpts); err != nil {
		return fmt.Errorf("failed initial collection: %w", err)
	}

	// Watch for specific port if specified
	if w.options.Port > 0 {
		return w.watchSpecificPort(watchCtx, ticker, collectorOpts)
	}

	// Watch all connections for changes
	return w.watchAllConnections(watchCtx, ticker, collectorOpts)
}

// performInitialCollection establishes the baseline of current connections
func (w *Watcher) performInitialCollection(ctx context.Context, opts *types.CollectorOptions) error {
	connections, err := w.collector.Collect(ctx, opts)
	if err != nil {
		return err
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Store current connections as baseline
	for _, conn := range connections {
		key := w.connectionKey(conn)
		w.lastConnections[key] = conn
	}

	return nil
}

// watchSpecificPort monitors new connections to a specific port
func (w *Watcher) watchSpecificPort(ctx context.Context, ticker *time.Ticker, opts *types.CollectorOptions) error {
	targetPort := w.options.Port

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.checkForNewPortConnections(ctx, targetPort, opts); err != nil {
				if w.callbacks.OnError != nil {
					w.callbacks.OnError(err)
				}
			}
		}
	}
}

// watchAllConnections monitors all connection changes
func (w *Watcher) watchAllConnections(ctx context.Context, ticker *time.Ticker, opts *types.CollectorOptions) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.checkForAllConnectionChanges(ctx, opts); err != nil {
				if w.callbacks.OnError != nil {
					w.callbacks.OnError(err)
				}
			}
		}
	}
}

// checkForNewPortConnections checks for new connections to a specific port
func (w *Watcher) checkForNewPortConnections(ctx context.Context, targetPort int, opts *types.CollectorOptions) error {
	connections, err := w.collector.Collect(ctx, opts)
	if err != nil {
		return err
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Look for new connections to the target port
	for _, conn := range connections {
		if conn.LocalPort == targetPort || conn.RemotePort == targetPort {
			key := w.connectionKey(conn)

			// Check if this is a new connection
			if _, exists := w.lastConnections[key]; !exists {
				// New connection found
				event := &types.WatchEvent{
					Type:       "NEW",
					Connection: *conn,
					Timestamp:  time.Now(),
				}

				if w.callbacks.OnNew != nil {
					if err := w.callbacks.OnNew(event); err != nil {
						return err
					}
				}

				// Add to tracking
				w.lastConnections[key] = conn
			}
		}
	}

	return nil
}

// checkForAllConnectionChanges checks for all connection changes
func (w *Watcher) checkForAllConnectionChanges(ctx context.Context, opts *types.CollectorOptions) error {
	connections, err := w.collector.Collect(ctx, opts)
	if err != nil {
		return err
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Create current connections map
	currentConnections := make(map[string]*types.Connection)
	for _, conn := range connections {
		key := w.connectionKey(conn)
		currentConnections[key] = conn
	}

	// Find new connections
	for key, conn := range currentConnections {
		if _, exists := w.lastConnections[key]; !exists {
			event := &types.WatchEvent{
				Type:       "NEW",
				Connection: *conn,
				Timestamp:  time.Now(),
			}

			if w.callbacks.OnNew != nil {
				if err := w.callbacks.OnNew(event); err != nil {
					return err
				}
			}
		}
	}

	// Find closed connections
	for key, conn := range w.lastConnections {
		if _, exists := currentConnections[key]; !exists {
			event := &types.WatchEvent{
				Type:       "CLOSED",
				Connection: *conn,
				Timestamp:  time.Now(),
			}

			if w.callbacks.OnClosed != nil {
				if err := w.callbacks.OnClosed(event); err != nil {
					return err
				}
			}
		}
	}

	// Update the last connections
	w.lastConnections = currentConnections

	return nil
}

// connectionKey generates a unique key for a connection
func (w *Watcher) connectionKey(conn *types.Connection) string {
	return fmt.Sprintf("%s:%s:%d:%s:%d:%s:%d",
		conn.Proto,
		conn.LocalIP,
		conn.LocalPort,
		conn.RemoteIP,
		conn.RemotePort,
		conn.State,
		conn.PID,
	)
}

// GetStatistics returns watching statistics
func (w *Watcher) GetStatistics() WatchStatistics {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return WatchStatistics{
		TrackedConnections: len(w.lastConnections),
		Interval:           w.options.Interval,
		Duration:           w.options.Duration,
		TargetPort:         w.options.Port,
	}
}

// WatchStatistics contains statistics about the watcher
type WatchStatistics struct {
	TrackedConnections int           `json:"tracked_connections"`
	Interval           time.Duration `json:"interval"`
	Duration           time.Duration `json:"duration"`
	TargetPort         int           `json:"target_port"`
}

// Stop gracefully stops the watcher
func (w *Watcher) Stop() {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Clear tracked connections
	w.lastConnections = make(map[string]*types.Connection)
}

// PortWatcher creates a specialized watcher for a specific port
func PortWatcher(collector collector.Collector, port int, interval time.Duration, onNewConnection func(*types.Connection)) *Watcher {
	options := &types.WatchOptions{
		Port:     port,
		Interval: interval,
		Duration: 0, // Infinite
		Quiet:    false,
	}

	callbacks := WatchCallbacks{
		OnNew: func(event *types.WatchEvent) error {
			if onNewConnection != nil {
				onNewConnection(&event.Connection)
			}
			return nil
		},
		OnError: func(err error) {
			// Default error handling - could log to stderr
			fmt.Printf("Watch error: %v\n", err)
		},
	}

	return NewWatcher(collector, options, callbacks)
}

// ConnectionDiffer compares two connection sets and returns differences
type ConnectionDiffer struct{}

// NewConnectionDiffer creates a new connection differ
func NewConnectionDiffer() *ConnectionDiffer {
	return &ConnectionDiffer{}
}

// Compare compares two sets of connections and returns the differences
func (d *ConnectionDiffer) Compare(oldConnections, newConnections []*types.Connection) (added, removed []*types.Connection) {
	// Create maps for efficient lookup
	oldMap := make(map[string]*types.Connection)
	newMap := make(map[string]*types.Connection)

	for _, conn := range oldConnections {
		key := d.connectionKey(conn)
		oldMap[key] = conn
	}

	for _, conn := range newConnections {
		key := d.connectionKey(conn)
		newMap[key] = conn
	}

	// Find added connections
	for key, conn := range newMap {
		if _, exists := oldMap[key]; !exists {
			added = append(added, conn)
		}
	}

	// Find removed connections
	for key, conn := range oldMap {
		if _, exists := newMap[key]; !exists {
			removed = append(removed, conn)
		}
	}

	return added, removed
}

// connectionKey generates a unique key for a connection (same as Watcher)
func (d *ConnectionDiffer) connectionKey(conn *types.Connection) string {
	return fmt.Sprintf("%s:%s:%d:%s:%d:%s:%d",
		conn.Proto,
		conn.LocalIP,
		conn.LocalPort,
		conn.RemoteIP,
		conn.RemotePort,
		conn.State,
		conn.PID,
	)
}
