package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"sub-store/internal/app"
)

const ScriptResourceCacheKey = "SCRIPT_RESOURCE_CACHE"

// ScriptResourceCache 是 Sub-Store 的持久化脚本资源缓存
// 它不同于内存级 TTLCache，会将数据持久化到 storage 中
type ScriptResourceCache struct {
	store        *app.Store
	mu           sync.RWMutex
	resourceCache map[string]cacheEntry
}

type cacheEntry struct {
	Time int64       `json:"time"`
	Data interface{} `json:"data"`
}

func NewScriptResourceCache(store *app.Store) *ScriptResourceCache {
	s := &ScriptResourceCache{
		store:         store,
		resourceCache: make(map[string]cacheEntry),
	}
	s.load()
	s.Cleanup("", 0)
	return s
}

func (s *ScriptResourceCache) load() {
	data := s.store.Read(ScriptResourceCacheKey)
	if data == "" {
		return
	}
	var loaded map[string]cacheEntry
	if err := json.Unmarshal([]byte(data), &loaded); err != nil {
		// 解析失败，重置为空
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

// Cleanup 清理过期条目（可选按前缀过滤）
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

// RevokeAll 清空所有缓存
func (s *ScriptResourceCache) RevokeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resourceCache = make(map[string]cacheEntry)
	s.persist()
}

// GetTime 获取条目的过期时间（毫秒时间戳），如果已过期或不存在返回 0
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

// Get 获取缓存，如果已过期且 remove=true 则删除
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

// Set 设置缓存
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

// 从环境变量读取默认 TTL（毫秒）
func currentDefaultTTL() *int64 {
	defaultTTL := int64(3600 * 1000) // 1 hour default
	return &defaultTTL
}

var globalScriptCache *ScriptResourceCache

func InitScriptResourceCache(store *app.Store) {
	globalScriptCache = NewScriptResourceCache(store)
}

func GetScriptCache() *ScriptResourceCache {
	if globalScriptCache == nil {
		panic("ScriptResourceCache not initialized")
	}
	return globalScriptCache
}
