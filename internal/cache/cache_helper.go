package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheHelper provides common caching operations for repositories
type CacheHelper struct {
	client *redis.Client
	prefix string
}

// NewCacheHelper creates a new cache helper instance
func NewCacheHelper(client *redis.Client, prefix string) *CacheHelper {
	return &CacheHelper{
		client: client,
		prefix: prefix,
	}
}

// CacheConfig defines cache configuration for different data types
type CacheConfig struct {
	TTL    time.Duration
	Prefix string
}

// Default cache configurations based on docs.txt performance requirements
var (
	// Short-lived cache for frequently accessed data
	FastCacheConfig = CacheConfig{
		TTL:    5 * time.Minute,
		Prefix: "fast:",
	}

	// Medium-lived cache for assessment data
	AssessmentCacheConfig = CacheConfig{
		TTL:    5 * time.Minute,
		Prefix: "assessment:",
	}

	// Long-lived cache for question data (less frequently changed)
	QuestionCacheConfig = CacheConfig{
		TTL:    5 * time.Minute,
		Prefix: "question:",
	}

	// Very short cache for existence checks
	ExistsCacheConfig = CacheConfig{
		TTL:    2 * time.Minute,
		Prefix: "exists:",
	}

	// Stats cache for expensive queries
	StatsCacheConfig = CacheConfig{
		TTL:    5 * time.Minute,
		Prefix: "stats:",
	}
)

// GetCacheKey generates a cache key with prefix
func (c *CacheHelper) GetCacheKey(key string) string {
	return fmt.Sprintf("%s%s", c.prefix, key)
}

// Get retrieves and unmarshals data from cache
func (c *CacheHelper) Get(ctx context.Context, key string, dest interface{}) error {
	if c.client == nil {
		return ErrCacheNotAvailable
	}

	cacheKey := c.GetCacheKey(key)
	data, err := c.client.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return ErrCacheNotFound
		}
		// Sanitize error to prevent log injection
		return fmt.Errorf("cache get error for key type: %w", err)
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return fmt.Errorf("cache unmarshal error: %w", err)
	}

	return nil
}

// Set marshals and stores data in cache
func (c *CacheHelper) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c.client == nil {
		return nil // Graceful degradation when cache not available
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}

	cacheKey := c.GetCacheKey(key)
	return c.client.Set(ctx, cacheKey, data, ttl).Err()
}

// SetString stores string data in cache
func (c *CacheHelper) SetString(ctx context.Context, key string, value string, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}

	cacheKey := c.GetCacheKey(key)
	return c.client.Set(ctx, cacheKey, value, ttl).Err()
}

// GetString retrieves string data from cache
func (c *CacheHelper) GetString(ctx context.Context, key string) (string, error) {
	if c.client == nil {
		return "", ErrCacheNotAvailable
	}

	cacheKey := c.GetCacheKey(key)
	result, err := c.client.Get(ctx, cacheKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrCacheNotFound
		}
		return "", fmt.Errorf("cache get string error: %w", err)
	}

	return result, nil
}

// Delete removes data from cache using pipeline for multiple keys
func (c *CacheHelper) Delete(ctx context.Context, keys ...string) error {
	if c.client == nil {
		return nil
	}

	if len(keys) == 0 {
		return nil
	}

	cacheKeys := make([]string, len(keys))
	for i, key := range keys {
		cacheKeys[i] = c.GetCacheKey(key)
	}

	// Use pipeline for multiple keys
	if len(cacheKeys) > 1 {
		pipe := c.client.Pipeline()
		pipe.Del(ctx, cacheKeys...)
		_, err := pipe.Exec(ctx)
		return err
	}

	return c.client.Del(ctx, cacheKeys...).Err()
}

// Exists checks if a key exists in cache
func (c *CacheHelper) Exists(ctx context.Context, key string) (bool, error) {
	if c.client == nil {
		return false, ErrCacheNotAvailable
	}

	cacheKey := c.GetCacheKey(key)
	count, err := c.client.Exists(ctx, cacheKey).Result()
	if err != nil {
		return false, fmt.Errorf("cache exists error: %w", err)
	}

	return count > 0, nil
}

// SetMultiple stores multiple key-value pairs in a pipeline
func (c *CacheHelper) SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()

	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("cache marshal error for key %s: %w", key, err)
		}

		cacheKey := c.GetCacheKey(key)
		pipe.Set(ctx, cacheKey, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetMultiple retrieves multiple values from cache
func (c *CacheHelper) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if c.client == nil {
		return nil, ErrCacheNotAvailable
	}

	if len(keys) == 0 {
		return map[string]string{}, nil
	}

	cacheKeys := make([]string, len(keys))
	for i, key := range keys {
		cacheKeys[i] = c.GetCacheKey(key)
	}

	values, err := c.client.MGet(ctx, cacheKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("cache mget error: %w", err)
	}

	result := make(map[string]string)
	for i, value := range values {
		if value != nil {
			if str, ok := value.(string); ok {
				result[keys[i]] = str
			}
		}
	}

	return result, nil
}

// InvalidatePattern removes all keys matching a pattern using SCAN instead of KEYS
func (c *CacheHelper) InvalidatePattern(ctx context.Context, pattern string) error {
	if c.client == nil {
		return nil
	}

	fullPattern := c.GetCacheKey(pattern)
	var cursor uint64
	var keys []string

	// Use SCAN instead of KEYS for better performance
	for {
		var scanKeys []string
		var err error
		scanKeys, cursor, err = c.client.Scan(ctx, cursor, fullPattern, 100).Result()
		if err != nil {
			slog.ErrorContext(ctx, "Cache scan pattern error",
				"error", err,
				"pattern", fullPattern)
			return fmt.Errorf("cache scan pattern error: %w", err)
		}
		keys = append(keys, scanKeys...)
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete using pipeline for better performance
	pipe := c.client.Pipeline()
	const batchSize = 100
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		pipe.Del(ctx, keys[i:end]...)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		slog.ErrorContext(ctx, "Cache pipeline delete error",
			"error", err,
			"total_keys", len(keys))
		return fmt.Errorf("cache pipeline delete error: %w", err)
	}

	return nil
}

// SetWithConfig stores data using predefined config
func (c *CacheHelper) SetWithConfig(ctx context.Context, key string, value interface{}, config CacheConfig) error {
	fullKey := config.Prefix + key
	return c.Set(ctx, fullKey, value, config.TTL)
}

// GetWithConfig retrieves data using predefined config
func (c *CacheHelper) GetWithConfig(ctx context.Context, key string, dest interface{}, config CacheConfig) error {
	fullKey := config.Prefix + key
	return c.Get(ctx, fullKey, dest)
}

// CacheOrExecute implements cache-aside pattern with proper error handling
func (c *CacheHelper) CacheOrExecute(ctx context.Context, key string, dest interface{}, ttl time.Duration, fetchFunc func() (interface{}, error)) error {
	// Try cache first
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil // Found in cache
	}

	if err != ErrCacheNotFound && err != ErrCacheNotAvailable {
		// Cache error occurred but continue with fetch
		slog.Info("Cache get error, proceeding to fetch", "error", err, "key", key)
	}

	// Execute fetch function
	value, err := fetchFunc()
	if err != nil {
		return fmt.Errorf("fetch function error: %w", err)
	}

	// Store in cache asynchronously to not block the response
	go func(parentCtx context.Context) {
		// Use parent context with timeout
		ctxWithTimeout, cancel := context.WithTimeout(parentCtx, 5*time.Second)
		defer cancel()
		if err := c.Set(ctxWithTimeout, key, value, ttl); err != nil {
			slog.Error("Cache set error", "error", err, "key", key)
		}
	}(ctx)

	// Set the result to destination directly without re-marshaling
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal result error: %w", err)
	}

	return json.Unmarshal(data, dest)
}

// Cache errors
var (
	ErrCacheNotAvailable = fmt.Errorf("cache not available")
	ErrCacheNotFound     = fmt.Errorf("cache not found")
)

// CacheManager manages multiple cache helpers
type CacheManager struct {
	Assessment *CacheHelper
	Question   *CacheHelper
	User       *CacheHelper
	Stats      *CacheHelper
	Exists     *CacheHelper
	Fast       *CacheHelper
}

// NewCacheManager creates cache manager with all cache helpers
func NewCacheManager(client *redis.Client) *CacheManager {
	if client == nil {
		return &CacheManager{
			Assessment: NewCacheHelper(nil, ""),
			Question:   NewCacheHelper(nil, ""),
			User:       NewCacheHelper(nil, ""),
			Stats:      NewCacheHelper(nil, ""),
			Exists:     NewCacheHelper(nil, ""),
			Fast:       NewCacheHelper(nil, ""),
		}
	}

	return &CacheManager{
		Assessment: NewCacheHelper(client, AssessmentCacheConfig.Prefix),
		Question:   NewCacheHelper(client, QuestionCacheConfig.Prefix),
		User:       NewCacheHelper(client, "user:"),
		Stats:      NewCacheHelper(client, StatsCacheConfig.Prefix),
		Exists:     NewCacheHelper(client, ExistsCacheConfig.Prefix),
		Fast:       NewCacheHelper(client, FastCacheConfig.Prefix),
	}
}

// HealthCheck verifies cache connectivity
func (cm *CacheManager) HealthCheck(ctx context.Context) error {
	if cm.Fast.client == nil {
		return ErrCacheNotAvailable
	}

	_, err := cm.Fast.client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("cache health check failed: %w", err)
	}

	return nil
}

// ClearAll clears all caches (use with caution)
func (cm *CacheManager) ClearAll(ctx context.Context) error {
	if cm.Fast.client == nil {
		return nil
	}

	return cm.Fast.client.FlushAll(ctx).Err()
}

// InvalidateAssessment invalidates all assessment-related caches
func (cm *CacheManager) InvalidateAssessment(ctx context.Context, assessmentID uint) error {
	patterns := []string{
		fmt.Sprintf("assessment:id:%d*", assessmentID),
		fmt.Sprintf("assessment:creator:%d*", assessmentID),
		fmt.Sprintf("stats:assessment:%d*", assessmentID),
		"assessment:list:*",
	}

	for _, pattern := range patterns {
		if err := cm.Assessment.InvalidatePattern(ctx, pattern); err != nil {
			// Log error but continue
			continue
		}
	}

	return nil
}

// InvalidateQuestion invalidates all question-related caches
func (cm *CacheManager) InvalidateQuestion(ctx context.Context, questionID uint) error {
	patterns := []string{
		fmt.Sprintf("question:id:%d*", questionID),
		fmt.Sprintf("question:creator:*"),
		fmt.Sprintf("stats:question:%d*", questionID),
		"question:list:*",
	}

	for _, pattern := range patterns {
		if err := cm.Question.InvalidatePattern(ctx, pattern); err != nil {
			continue
		}
	}

	return nil
}

// WarmupCache preloads frequently accessed data
func (cm *CacheManager) WarmupCache(ctx context.Context) error {
	// This would be called during application startup
	// to preload frequently accessed data

	// For now, just verify connectivity
	return cm.HealthCheck(ctx)
}
