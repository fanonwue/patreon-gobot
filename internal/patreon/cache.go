package patreon

import (
	"context"
	"github.com/fanonwue/patreon-gobot/internal/logging"
	"sync"
	"time"
)

type (
	CacheEntry[T any] struct {
		value     T
		expiresAt time.Time
	}
	Cache[K comparable, T any] struct {
		name   string
		mu     sync.RWMutex
		ttl    time.Duration
		values map[K]CacheEntry[T]
	}
)

func (cv *CacheEntry[T]) isExpired() bool {
	return cv.isExpiredAt(time.Now())
}

func (cv *CacheEntry[T]) isExpiredAt(referenceTime time.Time) bool {
	return referenceTime.After(cv.expiresAt)
}

func (c *Cache[K, T]) Name() string {
	return c.name
}

func (c *Cache[K, T]) cleanupValues() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for key, value := range c.values {
		if value.isExpiredAt(now) {
			delete(c.values, key)
		}
	}
}

func (c *Cache[K, T]) startCleanupJob(interval time.Duration, ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer func() {
		logging.Debugf("cache cleanup stopped for: %s", c.name)
		ticker.Stop()
	}()
	logging.Debugf("cache cleanup started for: %s", c.name)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.cleanupValues()
		}
	}
}

func (c *Cache[K, T]) Get(key K) (value T, found bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, found := c.values[key]
	if !found || v.isExpired() {
		return *new(T), false
	}
	return v.value, true
}

func (c *Cache[K, T]) Set(key K, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = CacheEntry[T]{value: value, expiresAt: time.Now().Add(c.ttl)}
}

const cacheEnabled = true
const cacheTTL = 10 * time.Minute
const cacheCleanup = 15 * time.Minute

var rewardsCache *Cache[RewardId, *Reward]
var campaignsCache *Cache[CampaignId, *Campaign]
var onStartupCalled = false

func OnStartup(appContext context.Context) {
	if onStartupCalled {
		logging.Debug("OnStartup() already called")
		return
	}

	if cacheEnabled {
		rewardsCache = &Cache[RewardId, *Reward]{
			name:   "RewardsCache",
			ttl:    cacheTTL,
			mu:     sync.RWMutex{},
			values: make(map[RewardId]CacheEntry[*Reward]),
		}

		campaignsCache = &Cache[CampaignId, *Campaign]{
			name:   "CampaignsCache",
			ttl:    cacheTTL,
			mu:     sync.RWMutex{},
			values: make(map[CampaignId]CacheEntry[*Campaign]),
		}

		go func() {
			ctx, cancel := context.WithCancel(appContext)
			defer cancel()
			rewardsCache.startCleanupJob(cacheCleanup, ctx)
		}()

		go func() {
			ctx, cancel := context.WithCancel(appContext)
			defer cancel()
			campaignsCache.startCleanupJob(cacheCleanup, ctx)
		}()
	}

	onStartupCalled = true
}
