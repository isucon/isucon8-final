// copy from https://raw.githubusercontent.com/isucon/isucon7-qualify/master/bench/src/bench/urlcache/cache.go
package bench

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/marcw/cachecontrol"
)

// 2017/10/20 inada-s Expireを設定したほうがリクエスト数が減ってスコアが下がる現象がみられたため、Expiresは見ないことにした。

type CacheStore struct {
	sync.Mutex
	items map[string]*URLCache
}

func NewCacheStore() *CacheStore {
	return &CacheStore{
		items: make(map[string]*URLCache),
	}
}

func (c *CacheStore) Get(key string) (*URLCache, bool) {
	c.Lock()
	v, found := c.items[key]
	c.Unlock()
	return v, found
}

func (c *CacheStore) Set(key string, value *URLCache) {
	c.Lock()
	if value == nil {
		delete(c.items, key)
	} else {
		c.items[key] = value
	}
	c.Unlock()
}

func (c *CacheStore) Del(key string) {
	c.Lock()
	delete(c.items, key)
	c.Unlock()
}

type URLCache struct {
	LastModified string
	Etag         string
	CacheControl *cachecontrol.CacheControl
	Expire       time.Time
}

func NewURLCache(res *http.Response) (*URLCache, bool) {
	ccs := res.Header["Cache-Control"]
	directive := strings.Join(ccs, " ")
	cc := cachecontrol.Parse(directive)

	if cc.NoStore() {
		return nil, false
	}

	return &URLCache{
		LastModified: res.Header.Get("Last-Modified"),
		Etag:         res.Header.Get("ETag"),
		CacheControl: &cc,
		Expire:       time.Now().Add(cc.MaxAge()),
	}, true
}

func (c *URLCache) CanUseCache() bool {
	if c.CacheControl.NoStore() {
		return false
	}
	if noCache, _ := c.CacheControl.NoCache(); noCache {
		return false
	}
	if c.Expire.Before(time.Now()) {
		return false
	}
	return true
}

func (c *URLCache) ApplyRequest(req *http.Request) {
	if c.LastModified != "" {
		req.Header.Add("If-Modified-Since", c.LastModified)
	}
	if c.Etag != "" {
		req.Header.Add("If-None-Match", c.Etag)
	}
}
