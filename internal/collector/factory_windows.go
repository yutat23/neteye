//go:build windows

package collector

// newPlatformCollector creates a new collector for Windows
func newPlatformCollector() (Collector, error) {
	return newWindowsCollector()
}
