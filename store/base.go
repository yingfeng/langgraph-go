package store

import (
	"context"
)

// BaseStore is the abstract interface for storing and retrieving data.
// It supports namespaced storage with get/put/search/index operations.
type BaseStore interface {
	// Get retrieves a value from the store by namespace and key.
	// Returns the value if found, nil if not found.
	Get(ctx context.Context, namespace []string, key string) (map[string]interface{}, error)

	// Put stores a value in the store under the given namespace and key.
	Put(ctx context.Context, namespace []string, key string, value map[string]interface{}) error

	// Delete removes a value from the store by namespace and key.
	Delete(ctx context.Context, namespace []string, key string) error

	// Search searches for values in the given namespace that match the query.
	// The query format is implementation-specific.
	Search(ctx context.Context, namespace []string, query string, limit int) ([]map[string]interface{}, error)

	// List lists all keys in the given namespace.
	List(ctx context.Context, namespace []string, limit int) ([]string, error)
}

// PutOperation represents a single put operation.
type PutOperation struct {
	Namespace []string
	Key       string
	Value     map[string]interface{}
}

// SearchOptions provides options for search operations.
type SearchOptions struct {
	Limit    int
	Offset   int
	Filter   map[string]interface{}
	SortBy   string
	SortDesc bool
}
