//go:build darwin

package collector

// newPlatformCollector creates a new collector for Darwin (macOS)
func newPlatformCollector() (Collector, error) {
	return newDarwinCollector()
}
