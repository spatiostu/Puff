package storage

import (
	"sync"
	"time"
)

// CacheItem 缓存项
type CacheItem struct {
	Data       interface{}
	ExpiresAt  time.Time
	AccessTime time.Time // 最后访问时间
}

// Cache 内存缓存
type Cache struct {
	data      map[string]*CacheItem
	duration  time.Duration
	mu        sync.RWMutex
	hitCount  int64 // 命中次数
	missCount int64 // 未命中次数
	maxSize   int   // 最大缓存项数
}

// NewCache 创建新的缓存
func NewCache(duration time.Duration) *Cache {
	cache := &Cache{
		data:     make(map[string]*CacheItem),
		duration: duration,
		maxSize:  1000, // 默认最大1000个缓存项
	}

	// 启动清理协程
	go cache.cleanup()

	return cache
}

// Set 设置缓存项（使用默认过期时间）
func (c *Cache) Set(key string, value interface{}) {
	c.SetWithDuration(key, value, c.duration)
}

// SetWithDuration 设置缓存项（使用自定义过期时间）
func (c *Cache) SetWithDuration(key string, value interface{}, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否需要清理空间
	if len(c.data) >= c.maxSize {
		c.evictLRU()
	}

	c.data[key] = &CacheItem{
		Data:       value,
		ExpiresAt:  time.Now().Add(duration),
		AccessTime: time.Now(),
	}
}

// Get 获取缓存项
func (c *Cache) Get(key string) interface{} {
	c.mu.Lock() // 改为写锁，因为要更新访问时间和统计
	defer c.mu.Unlock()

	item, exists := c.data[key]
	if !exists {
		c.missCount++
		return nil
	}

	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		delete(c.data, key)
		c.missCount++
		return nil
	}

	// 更新访问时间和命中统计
	item.AccessTime = time.Now()
	c.hitCount++

	return item.Data
}

// Delete 删除缓存项
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
}

// Clear 清空缓存
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]*CacheItem)
	c.hitCount = 0
	c.missCount = 0
}

// Size 获取缓存大小
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.data)
}

// Keys 获取所有键
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.data))
	for key := range c.data {
		keys = append(keys, key)
	}

	return keys
}

// cleanup 清理过期项
func (c *Cache) cleanup() {
	ticker := time.NewTicker(10 * time.Minute) // 每10分钟清理一次
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.data {
			if now.After(item.ExpiresAt) {
				delete(c.data, key)
			}
		}
		c.mu.Unlock()
	}
}

// evictLRU 淘汰最近最少使用的缓存项
func (c *Cache) evictLRU() {
	if len(c.data) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, item := range c.data {
		if first || item.AccessTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.AccessTime
			first = false
		}
	}

	if oldestKey != "" {
		delete(c.data, oldestKey)
	}
}

// SetMaxSize 设置最大缓存项数
func (c *Cache) SetMaxSize(size int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxSize = size
}

// GetStats 获取缓存统计信息
func (c *Cache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalItems := len(c.data)
	expiredItems := 0
	now := time.Now()

	for _, item := range c.data {
		if now.After(item.ExpiresAt) {
			expiredItems++
		}
	}

	total := c.hitCount + c.missCount
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hitCount) / float64(total) * 100
	}

	return map[string]interface{}{
		"total_items":   totalItems,
		"expired_items": expiredItems,
		"valid_items":   totalItems - expiredItems,
		"duration":      c.duration.String(),
		"hit_count":     c.hitCount,
		"miss_count":    c.missCount,
		"hit_rate":      hitRate,
		"max_size":      c.maxSize,
	}
}
