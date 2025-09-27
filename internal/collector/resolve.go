package collector

import (
	"context"
	"net"
	"sync"
	"time"
)

// DNSCacheEntry represents a cached DNS resolution result
type DNSCacheEntry struct {
	hostname  string
	timestamp time.Time
	ttl       time.Duration
}

// IsValid checks if the cache entry is still valid
func (e *DNSCacheEntry) IsValid() bool {
	return time.Since(e.timestamp) < e.ttl
}

// DNSResolver handles DNS resolution with caching
type DNSResolver struct {
	cache   map[string]*DNSCacheEntry
	mutex   sync.RWMutex
	timeout time.Duration
	maxSize int
	ttl     time.Duration
}

// NewDNSResolver creates a new DNS resolver with the specified timeout and cache size
func NewDNSResolver(timeout time.Duration, maxSize int) *DNSResolver {
	return &DNSResolver{
		cache:   make(map[string]*DNSCacheEntry),
		timeout: timeout,
		maxSize: maxSize,
		ttl:     5 * time.Minute, // Default TTL of 5 minutes
	}
}

// SetTTL sets the TTL for cached entries
func (r *DNSResolver) SetTTL(ttl time.Duration) {
	r.ttl = ttl
}

// Resolve resolves an IP address to a hostname with caching
func (r *DNSResolver) Resolve(ctx context.Context, ip string) string {
	// Check cache first
	r.mutex.RLock()
	if entry, exists := r.cache[ip]; exists && entry.IsValid() {
		r.mutex.RUnlock()
		return entry.hostname
	}
	r.mutex.RUnlock()

	// Create context with timeout
	resolveCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Perform DNS lookup
	hostname := r.performLookup(resolveCtx, ip)

	// Cache the result
	r.cacheResult(ip, hostname)

	return hostname
}

// performLookup performs the actual DNS lookup
func (r *DNSResolver) performLookup(ctx context.Context, ip string) string {
	names, err := net.DefaultResolver.LookupAddr(ctx, ip)
	if err != nil || len(names) == 0 {
		return ""
	}

	// Return the first resolved name
	hostname := names[0]

	// Remove trailing dot if present
	if len(hostname) > 0 && hostname[len(hostname)-1] == '.' {
		hostname = hostname[:len(hostname)-1]
	}

	return hostname
}

// cacheResult caches the DNS resolution result
func (r *DNSResolver) cacheResult(ip, hostname string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if cache is full
	if len(r.cache) >= r.maxSize {
		r.evictOldest()
	}

	// Cache the result
	r.cache[ip] = &DNSCacheEntry{
		hostname:  hostname,
		timestamp: time.Now(),
		ttl:       r.ttl,
	}
}

// evictOldest removes the oldest cache entry
func (r *DNSResolver) evictOldest() {
	var oldestIP string
	var oldestTime time.Time
	first := true

	for ip, entry := range r.cache {
		if first || entry.timestamp.Before(oldestTime) {
			oldestIP = ip
			oldestTime = entry.timestamp
			first = false
		}
	}

	if oldestIP != "" {
		delete(r.cache, oldestIP)
	}
}

// ClearCache clears all cached entries
func (r *DNSResolver) ClearCache() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.cache = make(map[string]*DNSCacheEntry)
}

// CacheStats returns statistics about the DNS cache
func (r *DNSResolver) CacheStats() (size int, validEntries int) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	size = len(r.cache)
	for _, entry := range r.cache {
		if entry.IsValid() {
			validEntries++
		}
	}

	return size, validEntries
}

// CleanupExpired removes expired entries from the cache
func (r *DNSResolver) CleanupExpired() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for ip, entry := range r.cache {
		if !entry.IsValid() {
			delete(r.cache, ip)
		}
	}
}
