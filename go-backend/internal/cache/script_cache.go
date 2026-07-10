package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"sub-store/internal/store"
)

const ScriptResourceCacheKey = "SCRIPT_RESOURCE_CACHE"

// ScriptResourceCache 持久化脚本资源缓存，对标 Node.js ResourceCache
type ScriptResourceCache struct {
	store         *store.Store
	mu            sync.RWMutex
	resourceCache map[string]cacheEntry
}

type cacheEntry struct {
	Time int64       `json:"time"`
	Data interface{} `json:"data"`
}

func NewScriptResourceCache(s *store.Store) *ScriptResourceCache {
	src := &ScriptResourceCache{
		store:         s,
		resourceCache: make(map[string]cacheEntry),
	}
	src.load()
	src.Cleanup("", 0)
	return src
}

func (s *ScriptResourceCache) load() {
	val := s.store.Read(ScriptResourceCacheKey)
	if val == nil {
		return
	}
	var data []byte
	switch v := val.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data = []byte(fmt.Sprint(v))
	}
	if len(data) == 0 {
		return
	}
	var loaded map[string]cacheEntry
	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}
	s.resourceCache = loaded
}

func (s *ScriptResourceCache) persist() {
	s.store.Write(ScriptResourceCacheKey, s.marshal())
}

func (s *ScriptResourceCache) marshal() string {
	data, err := json.Marshal(s.resourceCache)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func (s *ScriptResourceCache) Cleanup(prefix string, ttl int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resolvedTTL := normalizeTTL(ttl)
	if resolvedTTL == nil {
		tmp := int64(0)
		resolvedTTL = &tmp
	}
	now := time.Now().UnixMilli()
	cleared := false
	for id, cached := range s.resourceCache {
		shouldDelete := cached.Time == 0 || cached.Time < now+*resolvedTTL
		if shouldDelete {
			if prefix == "" || (len(id) >= len(prefix) && id[:len(prefix)] == prefix) {
				delete(s.resourceCache, id)
				cleared = true
			}
		}
	}
	if cleared {
		s.persist()
	}
}

func (s *ScriptResourceCache) RevokeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resourceCache = make(map[string]cacheEntry)
	s.persist()
}

func (s *ScriptResourceCache) GetTime(id string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cached, ok := s.resourceCache[id]
	if !ok {
		return 0
	}
	if cached.Time > 0 && time.Now().UnixMilli() <= cached.Time {
		return cached.Time
	}
	return 0
}

func (s *ScriptResourceCache) Get(id string, ttl int64, remove bool) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	resolvedTTL := normalizeTTL(ttl)
	if resolvedTTL == nil {
		resolvedTTL = currentDefaultTTL()
	}
	cached, ok := s.resourceCache[id]
	if !ok {
		return nil
	}
	if cached.Time > 0 && time.Now().UnixMilli()+*resolvedTTL <= cached.Time {
		return cached.Data
	}
	if remove {
		delete(s.resourceCache, id)
		s.persist()
	}
	return nil
}

func (s *ScriptResourceCache) Set(id string, value interface{}, ttl int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resolvedTTL := normalizeTTL(ttl)
	if resolvedTTL == nil {
		resolvedTTL = currentDefaultTTL()
	}
	s.resourceCache[id] = cacheEntry{
		Time: time.Now().UnixMilli() + *resolvedTTL,
		Data: value,
	}
	s.persist()
}

func normalizeTTL(ttl int64) *int64 {
	if ttl > 0 {
		return &ttl
	}
	return nil
}

func currentDefaultTTL() *int64 {
	defaultTTL := int64(10 * 24 * 3600 * 1000) // 10 days default
	return &defaultTTL
}

var globalScriptCache *ScriptResourceCache

func InitScriptResourceCache(s *store.Store) {
	globalScriptCache = NewScriptResourceCache(s)
}

func GetScriptCache() *ScriptResourceCache {
	if globalScriptCache == nil {
		panic("ScriptResourceCache not initialized")
	}
	return globalScriptCache
}
