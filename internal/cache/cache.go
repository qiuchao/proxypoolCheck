package cache

import (
	"github.com/qiuchao/proxypool/pkg/proxy"
	"github.com/patrickmn/go-cache"
	"time"
)

var Cache = cache.New(cache.NoExpiration, 10*time.Minute)

// Get proxies from Cache
func GetProxies(key string) proxy.ProxyList {
	result, found := Cache.Get(key)
	if found {
		return result.(proxy.ProxyList) //Get返回的是interface
	}
	return nil
}

// Set proxies to cache
func SetProxies(key string, proxies proxy.ProxyList) {
	Cache.Set(key, proxies, cache.NoExpiration)
}

// Set string to cache
func SetString(key, value string) {
	Cache.Set(key, value, cache.NoExpiration)
}

func SetBadProxies(badProxies map[string]int) {
	Cache.Set("badProxies", badProxies, cache.NoExpiration)
}

func GetBadProxies() map[string]int {
	result, found := Cache.Get("badProxies")
	if found {
		return result.(map[string]int)
	}
	return make(map[string]int)
}

// Get string from cache
func GetString(key string) string {
	result, found := Cache.Get(key)
	if found {
		return result.(string)
	}
	return ""
}
