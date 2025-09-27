//go:build linux

package collector

// newPlatformCollector creates a new collector for Linux
func newPlatformCollector() (Collector, error) {
	return newLinuxCollector()
}
