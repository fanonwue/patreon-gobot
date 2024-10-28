package patreon

import (
	"github.com/patrickmn/go-cache"
	"time"
)

const cacheExpiration = 10 * time.Minute
const cacheCleanup = 15 * time.Minute

var rewardsCache *cache.Cache
var campaignsCache *cache.Cache

func init() {
	rewardsCache = cache.New(cacheExpiration, cacheCleanup)
	campaignsCache = cache.New(cacheExpiration, cacheCleanup)
}
