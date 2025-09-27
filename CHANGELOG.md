# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial implementation of neteye CLI tool
- Cross-platform network connection collection (Linux/Windows/macOS)
- Multiple output formats: table, JSON, CSV
- Real-time connection monitoring with `watch` command
- Connection graph visualization with `map` command
- Graphviz DOT format support for graph output
- DNS resolution with caching
- Comprehensive CLI with cobra framework
- Configuration file support
- CI/CD pipeline with GitHub Actions
- Cross-platform build scripts
- Docker container support

### Features

#### Core Commands
- `neteye list`: List current TCP/UDP connections
- `neteye watch`: Monitor connections for changes in real-time
- `neteye map`: Generate connection relationship graphs

#### Output Formats
- **Table**: Human-readable tabular output with colors
- **JSON**: Machine-readable JSON format
- **CSV**: Comma-separated values for data processing
- **DOT**: Graphviz format for network visualization

#### Filtering Options
- Filter by process ID (`--pid`)
- Filter by protocol (`--proto tcp|udp`)
- Filter by remote host (`--remote`)
- Show only listening connections (`--listen-only`)

#### Monitoring Features
- Configurable check interval (`--interval`)
- Duration-limited monitoring (`--duration`)
- Port-specific monitoring
- Event-based output (NEW/CLOSED connections)

#### Graph Features
- JSON graph format with nodes and edges
- DOT format for Graphviz visualization
- Port collapsing option (`--collapse-ports`)
- DNS resolution for hostnames

### Technical Specifications
- Go 1.22+ compatibility
- Cross-platform support (Linux/Windows/macOS)
- Performance: `list` command executes within 300ms
- Memory efficient with structured logging
- Exit codes: 0=OK, 1=General Error, 2=Usage Error, 3=Permission Error

### Documentation
- Comprehensive README with usage examples
- Configuration file examples
- API documentation for all components
- Build and development instructions

### Infrastructure
- GitHub Actions CI/CD pipeline
- GoReleaser configuration for automated releases
- Cross-platform build matrix
- Security scanning with gosec
- Code coverage reporting
- Linting with golangci-lint

## [0.1.0] - 2025-09-27

### Added
- Initial release of neteye
- Basic network connection listing functionality
- Cross-platform collector implementations
- Table and JSON output formats
- Real-time monitoring capabilities
- Connection graph generation
- Documentation and examples

### Notes
- This is the initial development release
- All core features are implemented and functional
- Ready for testing and feedback
