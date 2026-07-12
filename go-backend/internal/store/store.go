package store

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

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
	dataPath := filepath.Join(s.dataDir, "sub-store.json")
	if data, err := os.ReadFile(dataPath); err == nil {
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err == nil {
			s.cache = result
		} else {
			if decoded, err := base64.StdEncoding.DecodeString(string(data)); err == nil {
				if err := json.Unmarshal(decoded, &result); err == nil {
					s.cache = result
				}
			}
		}
	}

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

	dataPath := filepath.Join(s.dataDir, "sub-store.json")
	if data, err := json.MarshalIndent(s.cache, "", "  "); err == nil {
		os.WriteFile(dataPath, data, 0644)
	}

	rootPath := filepath.Join(s.dataDir, "root.json")
	if data, err := json.MarshalIndent(s.root, "", "  "); err == nil {
		os.WriteFile(rootPath, data, 0644)
	}

	s.modified = false
}

func (s *Store) Read(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(key) > 0 && key[0] == '#' {
		key = key[1:]
		return s.root[key]
	}
	return s.cache[key]
}

func (s *Store) Write(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(key) > 0 && key[0] == '#' {
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

	if len(key) > 0 && key[0] == '#' {
		key = key[1:]
		delete(s.root, key)
	} else {
		delete(s.cache, key)
	}
	s.modified = true
	s.persist()
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persist()
	return nil
}

func (s *Store) Migrate() error {
	schemaVersion, _ := s.cache["schemaVersion"].(float64)
	if schemaVersion < 2.0 {
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

func (s *Store) GetRawCache() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache
}

func (s *Store) GetRawRoot() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

func (s *Store) SetRawCache(data map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = data
	s.modified = true
	s.persist()
}

func (s *Store) SetRawRoot(data map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root = data
	s.modified = true
	s.persist()
}
