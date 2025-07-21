package itunes

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
)

const (
	// Default cache settings
	DefaultCacheExpiration = 10 * time.Minute
	DefaultCleanupInterval = 20 * time.Minute
	CacheDir               = "itunes-cache"
)

// CacheManager handles both memory and file-based caching
type CacheManager struct {
	memCache *cache.Cache
	cacheDir string
}

// CacheEntry represents a cached search result with metadata
type CacheEntry struct {
	Query     string    `json:"query"`
	Tracks    []Track   `json:"tracks"`
	Timestamp time.Time `json:"timestamp"`
	Hash      string    `json:"hash"`
}

// NewCacheManager creates a new cache manager
func NewCacheManager() *CacheManager {
	return NewCacheManagerWithConfig(DefaultCacheExpiration, DefaultCleanupInterval)
}

// NewCacheManagerWithConfig creates a cache manager with custom settings
func NewCacheManagerWithConfig(expiration, cleanup time.Duration) *CacheManager {
	// Get cache directory in $TMPDIR
	tmpDir := os.TempDir()
	cacheDir := filepath.Join(tmpDir, CacheDir)

	// Create cache directory if it doesn't exist
	os.MkdirAll(cacheDir, 0755)

	return &CacheManager{
		memCache: cache.New(expiration, cleanup),
		cacheDir: cacheDir,
	}
}

// normalizeQuery cleans and normalizes a query string for consistent caching
func normalizeQuery(query string) string {
	// Convert to lowercase, trim spaces, and remove extra whitespace
	normalized := strings.ToLower(strings.TrimSpace(query))
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

// hashQuery creates a SHA256 hash of the normalized query for use as cache key
func hashQuery(query string) string {
	normalized := normalizeQuery(query)
	hash := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", hash)[:16] // Use first 16 characters for shorter filenames
}

// Get retrieves cached search results by query
func (cm *CacheManager) Get(query string) ([]Track, bool) {
	key := hashQuery(query)

	// Try memory cache first
	if cached, found := cm.memCache.Get(key); found {
		if entry, ok := cached.(CacheEntry); ok {
			return entry.Tracks, true
		}
	}

	// Try file cache if memory cache miss
	if tracks, found := cm.getFromFile(key); found {
		// Restore to memory cache
		entry := CacheEntry{
			Query:     normalizeQuery(query),
			Tracks:    tracks,
			Timestamp: time.Now(),
			Hash:      key,
		}
		cm.memCache.Set(key, entry, cache.DefaultExpiration)
		return tracks, true
	}

	return nil, false
}

// Set stores search results in both memory and file cache
func (cm *CacheManager) Set(query string, tracks []Track) error {
	key := hashQuery(query)
	entry := CacheEntry{
		Query:     normalizeQuery(query),
		Tracks:    tracks,
		Timestamp: time.Now(),
		Hash:      key,
	}

	// Store in memory cache
	cm.memCache.Set(key, entry, cache.DefaultExpiration)

	// Store in file cache
	return cm.setToFile(key, entry)
}

// getFromFile retrieves cached data from file system
func (cm *CacheManager) getFromFile(key string) ([]Track, bool) {
	filePath := filepath.Join(cm.cacheDir, "searches", key+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	// Check if file cache entry is still valid (within expiration time)
	if time.Since(entry.Timestamp) > DefaultCacheExpiration {
		// Remove expired file
		os.Remove(filePath)
		return nil, false
	}

	return entry.Tracks, true
}

// setToFile stores cached data to file system
func (cm *CacheManager) setToFile(key string, entry CacheEntry) error {
	searchesDir := filepath.Join(cm.cacheDir, "searches")
	if err := os.MkdirAll(searchesDir, 0755); err != nil {
		return fmt.Errorf("failed to create searches directory: %w", err)
	}

	filePath := filepath.Join(searchesDir, key+".json")
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// SaveLatestResults saves the latest search results for CLI compatibility
func (cm *CacheManager) SaveLatestResults(tracks []Track) error {
	filePath := filepath.Join(cm.cacheDir, "search_results.json")
	data, err := json.MarshalIndent(tracks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tracks: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// GetCacheStats returns basic cache statistics
func (cm *CacheManager) GetCacheStats() map[string]interface{} {
	stats := map[string]interface{}{
		"memory_items": cm.memCache.ItemCount(),
		"cache_dir":    cm.cacheDir,
	}

	// Count file cache entries
	searchesDir := filepath.Join(cm.cacheDir, "searches")
	if entries, err := os.ReadDir(searchesDir); err == nil {
		stats["file_items"] = len(entries)
	} else {
		stats["file_items"] = 0
	}

	return stats
}

// GetCacheDir returns the cache directory path
func (cm *CacheManager) GetCacheDir() string {
	return cm.cacheDir
}

// CleanupExpired removes expired entries from file cache
func (cm *CacheManager) CleanupExpired() error {
	searchesDir := filepath.Join(cm.cacheDir, "searches")
	entries, err := os.ReadDir(searchesDir)
	if err != nil {
		return nil // Directory doesn't exist or can't be read
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(searchesDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var cacheEntry CacheEntry
		if err := json.Unmarshal(data, &cacheEntry); err != nil {
			continue
		}

		// Remove if expired
		if time.Since(cacheEntry.Timestamp) > DefaultCacheExpiration {
			os.Remove(filePath)
		}
	}

	return nil
}

// GetAllCachedQueries returns all cached queries with metadata
func (cm *CacheManager) GetAllCachedQueries() []map[string]interface{} {
	var queries []map[string]interface{}

	// Get from memory cache
	for _, item := range cm.memCache.Items() {
		if entry, ok := item.Object.(CacheEntry); ok {
			queries = append(queries, map[string]interface{}{
				"query":       entry.Query,
				"hash":        entry.Hash,
				"timestamp":   entry.Timestamp,
				"track_count": len(entry.Tracks),
				"source":      "memory",
			})
		}
	}

	// Get from file cache (entries not in memory)
	searchDir := filepath.Join(cm.cacheDir, "searches")
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return queries
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		hash := strings.TrimSuffix(entry.Name(), ".json")

		// Skip if already in memory cache
		if _, found := cm.memCache.Get(hash); found {
			continue
		}

		// Read file to get metadata
		filePath := filepath.Join(searchDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var cacheEntry CacheEntry
		if err := json.Unmarshal(data, &cacheEntry); err != nil {
			continue
		}

		queries = append(queries, map[string]interface{}{
			"query":       cacheEntry.Query,
			"hash":        cacheEntry.Hash,
			"timestamp":   cacheEntry.Timestamp,
			"track_count": len(cacheEntry.Tracks),
			"source":      "file",
		})
	}

	return queries
}

// GetCachedResults returns the tracks for a specific query
func (cm *CacheManager) GetCachedResults(query string) ([]Track, bool) {
	return cm.Get(query)
}
