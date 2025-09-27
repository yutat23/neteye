package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/example/neteye/internal/collector"
	"github.com/example/neteye/internal/mapview"
	"github.com/example/neteye/internal/render"
	"github.com/example/neteye/internal/watch"
	"github.com/example/neteye/pkg/types"
	"github.com/spf13/cobra"
)

var (
	version = "v0.1.0"

	// Global flags - using strings for flag parsing
	globalFormat   = "table"
	globalNoColor  = false
	globalResolve  = false
	globalSortBy   = "proto"
	globalLogLevel = "error"

	// ASCII art logo
	logo = `███╗   ██╗███████╗████████╗███████╗██╗   ██╗███████╗
████╗  ██║██╔════╝╚══██╔══╝██╔════╝╚██╗ ██╔╝██╔════╝
██╔██╗ ██║█████╗     ██║   █████╗   ╚████╔╝ █████╗  
██║╚██╗██║██╔══╝     ██║   ██╔══╝    ╚██╔╝  ██╔══╝  
██║ ╚████║███████╗   ██║   ███████╗   ██║   ███████╗
╚═╝  ╚═══╝╚══════╝   ╚═╝   ╚══════╝   ╚═╝   ╚══════╝`
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		logError("main", "command execution failed", "E_COMMAND", err)
		os.Exit(int(types.ExitGeneralError))
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "neteye",
		Short: "Network connection investigation and monitoring tool",
		Long: `neteye is a cross-platform CLI tool for investigating and monitoring network connections.
It provides simple, fast, and visualizable network connection analysis.`,
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Setup global configuration
			setupGlobal()
		},
	}

	// Set custom usage template with logo
	cmd.SetUsageTemplate(getUsageTemplate())

	// Add global flags
	cmd.PersistentFlags().StringVar(&globalFormat, "format", "table", "Output format: table|json|csv")
	cmd.PersistentFlags().BoolVar(&globalNoColor, "no-color", false, "Disable ANSI colors")
	cmd.PersistentFlags().BoolVar(&globalResolve, "resolve", false, "Resolve remote IPs to hostnames")
	cmd.PersistentFlags().StringVar(&globalSortBy, "sort", "proto", "Sort by: proto|local|remote|state|pid|proc")

	// Add subcommands
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newWatchCmd())
	cmd.AddCommand(newMapCmd())

	return cmd
}

// getUsageTemplate returns a custom usage template with logo
func getUsageTemplate() string {
	coloredLogo := logo
	if !globalNoColor {
		// Add cyan color to the logo
		coloredLogo = fmt.Sprintf("\033[36m%s\033[0m", logo)
	}

	// Custom template that avoids infinite recursion
	return fmt.Sprintf(`%s

{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
{{end}}`, coloredLogo)
}

func newListCmd() *cobra.Command {
	opts := &types.CollectorOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List current TCP/UDP connections",
		Long:  "Display current network connections with optional filtering",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(opts)
		},
	}

	// Set custom usage template with logo for subcommand
	cmd.SetUsageTemplate(getUsageTemplate())

	// Add list-specific flags
	cmd.Flags().IntVar(&opts.PID, "pid", 0, "Filter by process ID")
	cmd.Flags().StringVar(&opts.RemoteHost, "remote", "", "Filter by remote IP or glob pattern")
	cmd.Flags().StringVar((*string)(&opts.Proto), "proto", "", "Filter by protocol: tcp|udp")
	cmd.Flags().BoolVar(&opts.ListenOnly, "listen-only", false, "Show only listening connections")
	cmd.Flags().BoolVar(&opts.Resolve, "resolve", false, "Resolve remote IPs to hostnames")

	return cmd
}

func newWatchCmd() *cobra.Command {
	opts := &types.WatchOptions{
		Interval: 500 * time.Millisecond,
		Duration: 0,
		Quiet:    false,
	}

	collectorOpts := &types.CollectorOptions{}

	cmd := &cobra.Command{
		Use:   "watch [port]",
		Short: "Monitor network connections for changes",
		Long: `Monitor network connections for new or closed connections.
If a port is specified, only monitor connections to/from that port.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				port, err := strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("invalid port number: %w", err)
				}
				opts.Port = port
			}
			return runWatch(opts, collectorOpts)
		},
	}

	// Set custom usage template with logo for subcommand
	cmd.SetUsageTemplate(getUsageTemplate())

	// Add watch-specific flags
	cmd.Flags().DurationVar(&opts.Interval, "interval", 500*time.Millisecond, "Check interval")
	cmd.Flags().DurationVar(&opts.Duration, "duration", 0, "Maximum watch duration (0=infinite)")
	cmd.Flags().BoolVar(&opts.Quiet, "quiet", false, "Suppress output except for changes")

	return cmd
}

func newMapCmd() *cobra.Command {
	opts := &types.MapOptions{
		GraphFormat:   types.GraphFormatJSON,
		CollapsePorts: false,
		Resolve:       false,
	}

	collectorOpts := &types.CollectorOptions{}

	cmd := &cobra.Command{
		Use:   "map",
		Short: "Generate connection relationship graph",
		Long:  "Generate a graph representation of network connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMap(opts, collectorOpts)
		},
	}

	// Set custom usage template with logo for subcommand
	cmd.SetUsageTemplate(getUsageTemplate())

	// Add map-specific flags
	cmd.Flags().StringVar((*string)(&opts.GraphFormat), "graph", "json", "Graph format: json|dot")
	cmd.Flags().BoolVar(&opts.CollapsePorts, "collapse-ports", false, "Group connections by IP only")
	cmd.Flags().BoolVar(&opts.Resolve, "resolve", false, "Resolve IPs to hostnames")

	return cmd
}

func runList(opts *types.CollectorOptions) error {
	// Override with global resolve setting if not set locally
	if !opts.Resolve {
		opts.Resolve = globalResolve
	}

	// Create collector
	coll, err := collector.New()
	if err != nil {
		logError("list", "failed to create collector", "E_COLLECTOR", err)
		os.Exit(int(types.ExitPermError))
	}

	// Collect connections
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connections, err := coll.Collect(ctx, opts)
	if err != nil {
		logError("list", "failed to collect connections", "E_COLLECT", err)
		os.Exit(int(types.ExitGeneralError))
	}

	// Render output
	return renderConnections(connections)
}

func runWatch(opts *types.WatchOptions, collectorOpts *types.CollectorOptions) error {
	// Override with global resolve setting if not set locally
	if !collectorOpts.Resolve {
		collectorOpts.Resolve = globalResolve
	}

	// Create collector
	coll, err := collector.New()
	if err != nil {
		logError("watch", "failed to create collector", "E_COLLECTOR", err)
		os.Exit(int(types.ExitPermError))
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	// Create watch callbacks
	callbacks := watch.WatchCallbacks{
		OnNew: func(event *types.WatchEvent) error {
			return renderWatchEvent(event)
		},
		OnClosed: func(event *types.WatchEvent) error {
			return renderWatchEvent(event)
		},
		OnError: func(err error) {
			logError("watch", "watch error", "E_WATCH", err)
		},
	}

	// Create and start watcher
	watcher := watch.NewWatcher(coll, opts, callbacks)

	err = watcher.Start(ctx, collectorOpts)
	if err != nil {
		if err == context.Canceled {
			os.Exit(int(types.ExitOK))
		} else if err == context.DeadlineExceeded {
			os.Exit(int(types.ExitUsageError))
		}
		logError("watch", "watch failed", "E_WATCH", err)
		os.Exit(int(types.ExitGeneralError))
	}

	return nil
}

func runMap(opts *types.MapOptions, collectorOpts *types.CollectorOptions) error {
	// Override with global resolve setting if not set locally
	if !opts.Resolve {
		opts.Resolve = globalResolve
	}
	if !collectorOpts.Resolve {
		collectorOpts.Resolve = globalResolve
	}

	// Create collector
	coll, err := collector.New()
	if err != nil {
		logError("map", "failed to create collector", "E_COLLECTOR", err)
		os.Exit(int(types.ExitPermError))
	}

	// Collect connections
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connections, err := coll.Collect(ctx, collectorOpts)
	if err != nil {
		logError("map", "failed to collect connections", "E_COLLECT", err)
		os.Exit(int(types.ExitGeneralError))
	}

	// Build graph
	mapper := mapview.NewConnectionMapper(opts.CollapsePorts)
	graph := mapper.MapConnections(connections)

	// Render graph
	return renderGraph(graph, opts.GraphFormat)
}

func renderConnections(connections []*types.Connection) error {
	switch types.OutputFormat(globalFormat) {
	case types.FormatTable:
		renderer := render.NewTableRenderer(os.Stdout, globalNoColor, types.SortKey(globalSortBy))
		return renderer.RenderConnections(connections)
	case types.FormatJSON:
		renderer := render.NewJSONRenderer(os.Stdout, false)
		return renderer.RenderConnections(connections)
	case types.FormatCSV:
		renderer := render.NewCSVRenderer(os.Stdout)
		return renderer.RenderConnections(connections)
	default:
		return fmt.Errorf("unsupported output format: %s", globalFormat)
	}
}

func renderWatchEvent(event *types.WatchEvent) error {
	switch types.OutputFormat(globalFormat) {
	case types.FormatTable:
		renderer := render.NewTableRenderer(os.Stdout, globalNoColor, types.SortKey(globalSortBy))
		return renderer.RenderWatchEvent(event)
	case types.FormatJSON:
		renderer := render.NewJSONRenderer(os.Stdout, false)
		return renderer.RenderWatchEvent(event)
	case types.FormatCSV:
		renderer := render.NewCSVRenderer(os.Stdout)
		return renderer.RenderWatchEvent(event)
	default:
		return fmt.Errorf("unsupported output format: %s", globalFormat)
	}
}

func renderGraph(graph *types.ConnectionGraph, format types.GraphFormat) error {
	switch format {
	case types.GraphFormatJSON:
		renderer := render.NewJSONRenderer(os.Stdout, true)
		return renderer.RenderConnectionGraph(graph)
	case types.GraphFormatDOT:
		renderer := render.NewDOTRenderer(os.Stdout)
		return renderer.RenderConnectionGraph(graph)
	default:
		return fmt.Errorf("unsupported graph format: %s", format)
	}
}

func setupGlobal() {
	// Validate format
	switch strings.ToLower(globalFormat) {
	case "table", "json", "csv":
		globalFormat = strings.ToLower(globalFormat)
	default:
		logError("setup", "invalid format", "E_FORMAT", fmt.Errorf("unsupported format: %s", globalFormat))
		os.Exit(int(types.ExitUsageError))
	}

	// Validate sort key
	switch strings.ToLower(globalSortBy) {
	case "proto", "local", "remote", "state", "pid", "proc":
		globalSortBy = strings.ToLower(globalSortBy)
	default:
		logError("setup", "invalid sort key", "E_SORT", fmt.Errorf("unsupported sort key: %s", globalSortBy))
		os.Exit(int(types.ExitUsageError))
	}
}

func logError(operation, message, code string, err error) {
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

	// Output to stderr as JSON
	output, _ := json.Marshal(logEntry)
	fmt.Fprintf(os.Stderr, "%s\n", output)
}

func parseInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}

	var result int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		result = result*10 + int(r-'0')
	}
	return result, nil
}
