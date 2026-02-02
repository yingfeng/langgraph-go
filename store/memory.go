package store

import (
	"context"
	"strings"
	"sync"
	"time"
)

// InMemoryStore is an in-memory implementation of BaseStore.
// It is not thread-safe by default, use NewInMemoryStore() for a thread-safe version.
type InMemoryStore struct {
	mu     sync.RWMutex
	data   map[string]map[string]map[string]interface{}
	ttl    map[string]time.Time
	closed bool
}

// NewInMemoryStore creates a new thread-safe in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		data: make(map[string]map[string]map[string]interface{}),
		ttl:  make(map[string]time.Time),
	}
}

// Get retrieves a value from the store.
func (s *InMemoryStore) Get(ctx context.Context, namespace []string, key string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, &StoreError{Message: "store is closed"}
	}

	nsKey := s.nsKey(namespace)
	if nsData, ok := s.data[nsKey]; ok {
		// Check TTL
		if fullKey := s.fullKey(nsKey, key); !s.checkTTL(fullKey) {
			return nil, nil
		}

		if value, ok := nsData[key]; ok {
			return s.copyValue(value), nil
		}
	}

	return nil, nil
}

// Put stores a value in the store.
func (s *InMemoryStore) Put(ctx context.Context, namespace []string, key string, value map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return &StoreError{Message: "store is closed"}
	}

	nsKey := s.nsKey(namespace)
	if _, ok := s.data[nsKey]; !ok {
		s.data[nsKey] = make(map[string]map[string]interface{})
	}

	s.data[nsKey][key] = s.copyValue(value)

	return nil
}

// Delete removes a value from the store.
func (s *InMemoryStore) Delete(ctx context.Context, namespace []string, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return &StoreError{Message: "store is closed"}
	}

	nsKey := s.nsKey(namespace)
	if nsData, ok := s.data[nsKey]; ok {
		delete(nsData, key)
		fullKey := s.fullKey(nsKey, key)
		delete(s.ttl, fullKey)
	}

	return nil
}

// Search searches for values in the store.
func (s *InMemoryStore) Search(ctx context.Context, namespace []string, query string, limit int) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, &StoreError{Message: "store is closed"}
	}

	nsKey := s.nsKey(namespace)
	nsData, ok := s.data[nsKey]
	if !ok {
		return nil, nil
	}

	results := make([]map[string]interface{}, 0)
	for key, value := range nsData {
		fullKey := s.fullKey(nsKey, key)
		if !s.checkTTL(fullKey) {
			continue
		}

		// Simple query matching (can be extended)
		if query == "" || s.matchQuery(value, query) {
			results = append(results, s.copyValue(value))
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// List lists all keys in the namespace.
func (s *InMemoryStore) List(ctx context.Context, namespace []string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, &StoreError{Message: "store is closed"}
	}

	nsKey := s.nsKey(namespace)
	nsData, ok := s.data[nsKey]
	if !ok {
		return nil, nil
	}

	keys := make([]string, 0, len(nsData))
	for key := range nsData {
		fullKey := s.fullKey(nsKey, key)
		if s.checkTTL(fullKey) {
			keys = append(keys, key)
			if limit > 0 && len(keys) >= limit {
				break
			}
		}
	}

	return keys, nil
}

// BatchPut performs multiple put operations atomically.
func (s *InMemoryStore) BatchPut(ctx context.Context, ops []PutOperation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return &StoreError{Message: "store is closed"}
	}

	for _, op := range ops {
		nsKey := s.nsKey(op.Namespace)
		if _, ok := s.data[nsKey]; !ok {
			s.data[nsKey] = make(map[string]map[string]interface{})
		}
		s.data[nsKey][op.Key] = s.copyValue(op.Value)
	}

	return nil
}

// SetTTL sets a time-to-live for a key.
func (s *InMemoryStore) SetTTL(ctx context.Context, namespace []string, key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return &StoreError{Message: "store is closed"}
	}

	if ttl <= 0 {
		return nil
	}

	fullKey := s.fullKey(s.nsKey(namespace), key)
	s.ttl[fullKey] = time.Now().Add(ttl)

	return nil
}

// Clear clears all data from the store.
func (s *InMemoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]map[string]map[string]interface{})
	s.ttl = make(map[string]time.Time)

	return nil
}

// Close closes the store.
func (s *InMemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

// Helper methods

func (s *InMemoryStore) nsKey(namespace []string) string {
	return strings.Join(namespace, "|")
}

func (s *InMemoryStore) fullKey(nsKey, key string) string {
	return nsKey + ":" + key
}

func (s *InMemoryStore) checkTTL(fullKey string) bool {
	if expiry, ok := s.ttl[fullKey]; ok {
		return time.Now().Before(expiry)
	}
	return true
}

func (s *InMemoryStore) copyValue(value map[string]interface{}) map[string]interface{} {
	copied := make(map[string]interface{}, len(value))
	for k, v := range value {
		copied[k] = v
	}
	return copied
}

func (s *InMemoryStore) matchQuery(value map[string]interface{}, query string) bool {
	// Simple substring matching across all values
	// Can be extended to support more complex queries
	for _, v := range value {
		if str, ok := v.(string); ok {
			if strings.Contains(strings.ToLower(str), strings.ToLower(query)) {
				return true
			}
		}
	}
	return false
}

// StoreError represents a store error.
type StoreError struct {
	Message string
	Code    string
}

func (e *StoreError) Error() string {
	if e.Code != "" {
		return e.Code + ": " + e.Message
	}
	return e.Message
}
