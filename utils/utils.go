// Package utils provides utility functions for LangGraph Go.
package utils

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// Contains checks if a string slice contains a value.
func Contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// Filter filters a string slice based on a predicate.
func Filter(slice []string, predicate func(string) bool) []string {
	result := make([]string, 0)
	for _, v := range slice {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

// Map applies a function to each element of a slice.
func Map(slice []string, fn func(string) string) []string {
	result := make([]string, len(slice))
	for i, v := range slice {
		result[i] = fn(v)
	}
	return result
}

// MergeMaps merges multiple maps into one.
func MergeMaps(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// GetFromMap gets a value from a nested map using dot notation.
func GetFromMap(data map[string]interface{}, path string) (interface{}, bool) {
	keys := strings.Split(path, ".")
	current := data
	
	for i, key := range keys {
		if i == len(keys)-1 {
			val, ok := current[key]
			return val, ok
		}
		
		next, ok := current[key].(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = next
	}
	
	return nil, false
}

// SetInMap sets a value in a nested map using dot notation.
func SetInMap(data map[string]interface{}, path string, value interface{}) {
	keys := strings.Split(path, ".")
	current := data
	
	for i, key := range keys {
		if i == len(keys)-1 {
			current[key] = value
			return
		}
		
		if _, ok := current[key]; !ok {
			current[key] = make(map[string]interface{})
		}
		
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			// Cannot navigate further
			return
		}
	}
}

// StructToMap converts a struct to a map[string]interface{}.
func StructToMap(v interface{}) (map[string]interface{}, error) {
	if v == nil {
		return nil, fmt.Errorf("nil value")
	}
	
	result := make(map[string]interface{})
	
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("value is not a struct")
	}
	
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue // Skip unexported fields
		}
		
		value := rv.Field(i).Interface()
		result[field.Name] = value
	}
	
	return result, nil
}

// MapToStruct converts a map to a struct.
func MapToStruct(data map[string]interface{}, target interface{}) error {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer")
	}
	
	rv = rv.Elem()
	rt := rv.Type()
	
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}
		
		if value, ok := data[field.Name]; ok {
			fieldValue := rv.Field(i)
			if fieldValue.CanSet() {
				fieldValue.Set(reflect.ValueOf(value))
			}
		}
	}
	
	return nil
}

// DeepEqual checks if two values are deeply equal.
func DeepEqual(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}

// IsZero checks if a value is the zero value for its type.
func IsZero(v interface{}) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsZero()
}

// TypeName returns the name of a type.
func TypeName(v interface{}) string {
	if v == nil {
		return "nil"
	}
	
	t := reflect.TypeOf(v)
	return t.String()
}

// Retry executes a function with retry logic.
func Retry(attempts int, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
	}
	return err
}

// WithContext executes a function with a context.
func WithContext(ctx context.Context, fn func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()
	
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// Coalesce returns the first non-zero value.
func Coalesce(values ...interface{}) interface{} {
	for _, v := range values {
		if !IsZero(v) {
			return v
		}
	}
	return nil
}
