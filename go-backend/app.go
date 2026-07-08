package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Store provides persistent JSON storage for all application data
type Store struct {
	mu       sync.RWMutex
	dataDir  string
	cache    map[string]interface{}
	root     map[string]interface{}
	modified bool
}

func NewStore(dataDir string) *Store {
	s := &Store{
		dataDir: dataDir,
		cache:   make(map[string]interface{}),
		root:    make(map[string]interface{}),
	}
	s.load()
	return s
}

func (s *Store) load() {
	// Load main data
	dataPath := filepath.Join(s.dataDir, "sub-store.json")
	if data, err := os.ReadFile(dataPath); err == nil {
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err == nil {
			s.cache = result
		} else {
			// Try base64 decode
			if decoded, err := base64Decode(string(data)); err == nil {
				if err := json.Unmarshal(decoded, &result); err == nil {
					s.cache = result
				}
			}
		}
	}

	// Load root data
	rootPath := filepath.Join(s.dataDir, "root.json")
	if data, err := os.ReadFile(rootPath); err == nil {
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err == nil {
			s.root = result
		}
	}
}

func (s *Store) persist() {
	if !s.modified {
		return
	}

	// Save main data
	dataPath := filepath.Join(s.dataDir, "sub-store.json")
	if data, err := json.MarshalIndent(s.cache, "", "  "); err == nil {
		os.WriteFile(dataPath, data, 0644)
	}

	// Save root data
	rootPath := filepath.Join(s.dataDir, "root.json")
	if data, err := json.MarshalIndent(s.root, "", "  "); err == nil {
		os.WriteFile(rootPath, data, 0644)
	}

	s.modified = false
}

func (s *Store) Read(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if key[0] == '#' {
		key = key[1:]
		return s.root[key]
	}
	return s.cache[key]
}

func (s *Store) Write(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key[0] == '#' {
		key = key[1:]
		s.root[key] = value
	} else {
		s.cache[key] = value
	}
	s.modified = true
	s.persist()
}

func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key[0] == '#' {
		key = key[1:]
		delete(s.root, key)
	} else {
		delete(s.cache, key)
	}
	s.modified = true
	s.persist()
}

func (s *Store) Migrate() error {
	// Check schema version
	schemaVersion, _ := s.cache["schemaVersion"].(float64)
	if schemaVersion < 2.0 {
		// Migrate from old format (object keyed by name) to new format (array)
		for _, key := range []string{"subs", "collections", "artifacts", "rules", "files", "tokens"} {
			if data, ok := s.cache[key].(map[string]interface{}); ok {
				var arr []interface{}
				for _, v := range data {
					arr = append(arr, v)
				}
				s.cache[key] = arr
			}
		}
		s.cache["schemaVersion"] = 2.0
		s.modified = true
		s.persist()
		fmt.Println("[MIGRATION] Data migrated to version 2.0")
	}
	return nil
}

// App holds the application context
type App struct {
	Config *Config
	Store  *Store
	http   *HTTPClient
}

func NewApp(cfg *Config, store *Store) *App {
	return &App{
		Config: cfg,
		Store:  store,
		http:   NewHTTPClient(cfg),
	}
}

func (a *App) Info(msg string) {
	fmt.Printf("[sub-store] INFO: %s\n", msg)
}

func (a *App) Error(msg string) {
	fmt.Printf("[sub-store] ERROR: %s\n", msg)
}

func (a *App) Warn(msg string) {
	fmt.Printf("[sub-store] WARN: %s\n", msg)
}

func (a *App) Log(msg string) {
	fmt.Printf("[sub-store] LOG: %s\n", msg)
}

func (a *App) Notify(title, subtitle, content string) {
	fmt.Printf("[Notify] %s\n%s\n%s\n\n", title, subtitle, content)

	if a.Config.PushService != "" {
		go func() {
			if err := a.sendPushNotification(title, subtitle, content); err != nil {
				fmt.Printf("[Push Service] ERROR: %v\n", err)
			}
		}()
	}
}

func (a *App) sendPushNotification(title, subtitle, content string) error {
	if a.Config.PushService == "" {
		return nil
	}

	// Implement push notification logic (HTTP webhook or shoutrrr)
	// For now, just log it
	fmt.Printf("[Push Service] URL: %s\n", a.Config.PushService)
	return nil
}

func (a *App) SyncArtifacts() error {
	// Implementation will be added in sync handlers
	return nil
}

func (a *App) DownloadMMDB() {
	// Download MMDB files
	if a.Config.MMDBCountryURL != "" && a.Config.MMDBCountryPath != "" {
		a.Info(fmt.Sprintf("[MMDB CRON] downloading %s to %s", a.Config.MMDBCountryURL, a.Config.MMDBCountryPath))
	}
	if a.Config.MMDBASNURL != "" && a.Config.MMDBASNPath != "" {
		a.Info(fmt.Sprintf("[MMDB CRON] downloading %s to %s", a.Config.MMDBASNURL, a.Config.MMDBASNPath))
	}
}

// HTTPClient wraps HTTP operations
type HTTPClient struct {
	cfg *Config
}

func NewHTTPClient(cfg *Config) *HTTPClient {
	return &HTTPClient{cfg: cfg}
}
