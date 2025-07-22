package database

import (
	"crypto/md5"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SearchCache implements a simple in-memory cache for search results
type SearchCache struct {
	mu      sync.RWMutex
	cache   map[string]*CachedResult
	maxSize int
	ttl     time.Duration
}

// CachedResult holds cached search results with timestamp
type CachedResult struct {
	Tracks    []Track
	Timestamp time.Time
}

// NewSearchCache creates a new search cache
func NewSearchCache(maxSize int, ttl time.Duration) *SearchCache {
	return &SearchCache{
		cache:   make(map[string]*CachedResult),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves cached results if available and not expired
func (sc *SearchCache) Get(key string) ([]Track, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if result, exists := sc.cache[key]; exists {
		if time.Since(result.Timestamp) < sc.ttl {
			return result.Tracks, true
		}
		// Expired, will be cleaned up later
	}
	return nil, false
}

// Set stores search results in cache
func (sc *SearchCache) Set(key string, tracks []Track) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Simple eviction: remove oldest entries if cache is full
	if len(sc.cache) >= sc.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range sc.cache {
			if oldestKey == "" || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
			}
		}
		delete(sc.cache, oldestKey)
	}

	sc.cache[key] = &CachedResult{
		Tracks:    tracks,
		Timestamp: time.Now(),
	}
}

// Clear removes all cached results
func (sc *SearchCache) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cache = make(map[string]*CachedResult)
}

// SearchQueryBuilder builds optimized SQL queries for search
type SearchQueryBuilder struct {
	query        string
	filters      *SearchFilters
	useFTS       bool
	includeStats bool
}

// NewSearchQueryBuilder creates a new query builder
func NewSearchQueryBuilder(query string, filters *SearchFilters) *SearchQueryBuilder {
	if filters == nil {
		filters = &SearchFilters{Limit: 15}
	}
	if filters.Limit <= 0 {
		filters.Limit = 15
	}
	return &SearchQueryBuilder{
		query:   query,
		filters: filters,
		useFTS:  true, // Default to FTS for text queries
	}
}

// WithFTS enables or disables FTS search
func (sqb *SearchQueryBuilder) WithFTS(useFTS bool) *SearchQueryBuilder {
	sqb.useFTS = useFTS
	return sqb
}

// WithStats includes additional statistics in results
func (sqb *SearchQueryBuilder) WithStats(includeStats bool) *SearchQueryBuilder {
	sqb.includeStats = includeStats
	return sqb
}

// Build constructs the SQL query and arguments
func (sqb *SearchQueryBuilder) Build() (string, []interface{}, error) {
	var conditions []string
	var args []interface{}
	var scoreColumns []string
	var orderByColumns []string

	// Base relevance scoring
	scoreColumns = append(scoreColumns, "0") // Base score

	// FTS match condition for text queries
	if sqb.query != "" && sqb.useFTS {
		conditions = append(conditions, "t.id IN (SELECT rowid FROM tracks_fts WHERE tracks_fts MATCH ?)")
		// Prepare FTS query - handle special characters and add wildcards
		ftsQuery := prepareFTSQuery(sqb.query)
		args = append(args, ftsQuery)

		// Add FTS rank to scoring (already matched by the condition above)
		scoreColumns = append(scoreColumns, "10")
	} else if sqb.query != "" {
		// Fallback to LIKE queries
		conditions = append(conditions, "(t.name LIKE ? OR ar.name LIKE ? OR al.name LIKE ?)")
		searchPattern := "%" + sqb.query + "%"
		args = append(args, searchPattern, searchPattern, searchPattern)

		// Add LIKE scoring
		scoreColumns = append(scoreColumns, `
			CASE WHEN LOWER(t.name) = LOWER(?) THEN 20
			     WHEN LOWER(t.name) LIKE LOWER(?) THEN 10
			     ELSE 0 END
		`)
		args = append(args, sqb.query, "%"+sqb.query+"%")

		scoreColumns = append(scoreColumns, `
			CASE WHEN LOWER(ar.name) = LOWER(?) THEN 15
			     WHEN LOWER(ar.name) LIKE LOWER(?) THEN 8
			     ELSE 0 END
		`)
		args = append(args, sqb.query, "%"+sqb.query+"%")
	}

	// Apply filters
	if sqb.filters.Genre != "" {
		conditions = append(conditions, "g.name = ?")
		args = append(args, sqb.filters.Genre)
	}

	if sqb.filters.Artist != "" {
		conditions = append(conditions, "ar.name = ?")
		args = append(args, sqb.filters.Artist)
	}

	if sqb.filters.Album != "" {
		conditions = append(conditions, "al.name = ?")
		args = append(args, sqb.filters.Album)
	}

	if sqb.filters.Playlist != "" {
		if sqb.filters.UsePlaylistID {
			// Use persistent ID for playlist lookup
			conditions = append(conditions, `
				EXISTS (
					SELECT 1 FROM playlist_tracks pt
					JOIN playlists p ON p.id = pt.playlist_id
					WHERE pt.track_id = t.id AND p.persistent_id = ?
				)
			`)
		} else {
			// Use playlist name
			conditions = append(conditions, `
				EXISTS (
					SELECT 1 FROM playlist_tracks pt
					JOIN playlists p ON p.id = pt.playlist_id
					WHERE pt.track_id = t.id AND p.name = ?
				)
			`)
		}
		args = append(args, sqb.filters.Playlist)
	}

	if sqb.filters.Starred != nil && *sqb.filters.Starred {
		conditions = append(conditions, "t.starred = 1")
		scoreColumns = append(scoreColumns, "5") // Boost starred tracks
	}

	if sqb.filters.MinRating > 0 {
		conditions = append(conditions, "t.rating >= ?")
		args = append(args, sqb.filters.MinRating)
	}

	// Add popularity scoring (play count and recent plays)
	scoreColumns = append(scoreColumns, "COALESCE(t.play_count * 0.1, 0)")
	scoreColumns = append(scoreColumns, `
		CASE WHEN t.last_played > datetime('now', '-7 days') THEN 5
		     WHEN t.last_played > datetime('now', '-30 days') THEN 3
		     WHEN t.last_played > datetime('now', '-90 days') THEN 1
		     ELSE 0 END
	`)

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Calculate total score
	totalScore := "(" + strings.Join(scoreColumns, " + ") + ")"

	// Build ORDER BY clause
	orderByColumns = append(orderByColumns, totalScore+" DESC")
	orderByColumns = append(orderByColumns, "t.ranking DESC")
	orderByColumns = append(orderByColumns, "t.name ASC")
	orderBy := "ORDER BY " + strings.Join(orderByColumns, ", ")

	// Build final query
	query := fmt.Sprintf(`
		SELECT
			t.id, t.persistent_id, t.name, al.name, t.collection,
			ar.name, g.name, t.rating, t.starred, t.ranking,
			t.duration, t.play_count, t.last_played, t.date_added,
			%s as relevance_score
		FROM tracks t
		LEFT JOIN artists ar ON ar.id = t.artist_id
		LEFT JOIN albums al ON al.id = t.album_id
		LEFT JOIN genres g ON g.id = t.genre_id
		%s
		%s
		LIMIT ?
	`, totalScore, whereClause, orderBy)

	args = append(args, sqb.filters.Limit)

	return query, args, nil
}

// prepareFTSQuery prepares a query string for FTS5
func prepareFTSQuery(query string) string {
	// Escape special FTS characters
	query = strings.ReplaceAll(query, "\"", "\"\"")

	// Split into terms and create a phrase query
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return ""
	}

	// For multiple terms, use AND by default
	if len(terms) > 1 {
		// Quote each term to handle special characters
		quotedTerms := make([]string, len(terms))
		for i, term := range terms {
			quotedTerms[i] = "\"" + term + "\""
		}
		return strings.Join(quotedTerms, " AND ")
	}

	// For single terms, add wildcard suffix for prefix matching
	return "\"" + terms[0] + "\"*"
}

// SearchMetrics tracks search performance
type SearchMetrics struct {
	Query       string
	Duration    time.Duration
	ResultCount int
	CacheHit    bool
	Method      string // "fts" or "like"
}

// SearchManager extends DatabaseManager with advanced search capabilities
type SearchManager struct {
	*DatabaseManager
	cache   *SearchCache
	metrics []SearchMetrics
	mu      sync.Mutex
}

// NewSearchManager creates a new search manager with caching
func NewSearchManager(dm *DatabaseManager) *SearchManager {
	return &SearchManager{
		DatabaseManager: dm,
		cache:           NewSearchCache(100, 5*time.Minute),
		metrics:         make([]SearchMetrics, 0, 1000),
	}
}

// SearchWithCache performs a cached search
func (sm *SearchManager) SearchWithCache(query string, filters *SearchFilters) ([]Track, error) {
	start := time.Now()

	// Generate cache key
	cacheKey := sm.generateCacheKey(query, filters)

	// Check cache
	if tracks, hit := sm.cache.Get(cacheKey); hit {
		sm.recordMetrics(SearchMetrics{
			Query:       query,
			Duration:    time.Since(start),
			ResultCount: len(tracks),
			CacheHit:    true,
			Method:      "cache",
		})
		return tracks, nil
	}

	// Perform search
	tracks, err := sm.SearchTracksOptimized(query, filters)
	if err != nil {
		return nil, err
	}

	// Cache results
	sm.cache.Set(cacheKey, tracks)

	sm.recordMetrics(SearchMetrics{
		Query:       query,
		Duration:    time.Since(start),
		ResultCount: len(tracks),
		CacheHit:    false,
		Method:      "fts",
	})

	return tracks, nil
}

// SearchTracksOptimized performs an optimized search with relevance scoring
func (sm *SearchManager) SearchTracksOptimized(query string, filters *SearchFilters) ([]Track, error) {
	builder := NewSearchQueryBuilder(query, filters)

	// Try FTS first
	sqlQuery, args, err := builder.WithFTS(true).Build()
	if err != nil {
		return nil, err
	}

	tracks, err := sm.executeSearchQuery(sqlQuery, args)
	if err != nil {
		// Fallback to LIKE queries if FTS fails
		sqlQuery, args, err = builder.WithFTS(false).Build()
		if err != nil {
			return nil, err
		}
		tracks, err = sm.executeSearchQuery(sqlQuery, args)
	}

	return tracks, err
}

// executeSearchQuery executes a search query and returns tracks
func (sm *SearchManager) executeSearchQuery(query string, args []interface{}) ([]Track, error) {
	rows, err := sm.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		track := Track{}
		var relevanceScore float64
		err := rows.Scan(
			&track.ID, &track.PersistentID, &track.Name, &track.Album, &track.Collection,
			&track.Artist, &track.Genre, &track.Rating, &track.Starred, &track.Ranking,
			&track.Duration, &track.PlayCount, &track.LastPlayed, &track.DateAdded,
			&relevanceScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan track: %w", err)
		}

		// Store relevance score in Ranking for now (could add a separate field)
		track.Ranking = relevanceScore

		// Get playlist associations
		playlists, err := sm.getTrackPlaylists(int(track.ID))
		if err == nil {
			track.Playlists = playlists
		}

		tracks = append(tracks, track)
	}

	return tracks, nil
}

// getTrackPlaylists retrieves playlist names for a track
func (sm *SearchManager) getTrackPlaylists(trackID int) ([]string, error) {
	rows, err := sm.DB.Query(`
		SELECT p.name
		FROM playlists p
		JOIN playlist_tracks pt ON p.id = pt.playlist_id
		WHERE pt.track_id = ?
		ORDER BY p.name
	`, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		playlists = append(playlists, name)
	}

	return playlists, nil
}

// generateCacheKey creates a unique key for search results
func (sm *SearchManager) generateCacheKey(query string, filters *SearchFilters) string {
	key := fmt.Sprintf("%s|%+v", query, filters)
	hash := md5.Sum([]byte(key))
	return fmt.Sprintf("%x", hash)
}

// recordMetrics records search performance metrics
func (sm *SearchManager) recordMetrics(metric SearchMetrics) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.metrics = append(sm.metrics, metric)

	// Keep only last 1000 metrics
	if len(sm.metrics) > 1000 {
		sm.metrics = sm.metrics[len(sm.metrics)-1000:]
	}
}

// GetMetrics returns recent search metrics
func (sm *SearchManager) GetMetrics() []SearchMetrics {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	result := make([]SearchMetrics, len(sm.metrics))
	copy(result, sm.metrics)
	return result
}

// GetAverageSearchTime returns the average search time for recent queries
func (sm *SearchManager) GetAverageSearchTime() time.Duration {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.metrics) == 0 {
		return 0
	}

	var total time.Duration
	for _, m := range sm.metrics {
		total += m.Duration
	}

	return total / time.Duration(len(sm.metrics))
}

// ClearCache clears the search cache
func (sm *SearchManager) ClearCache() {
	sm.cache.Clear()
}
