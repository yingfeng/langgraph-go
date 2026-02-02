// Package task provides function decorators for LangGraph tasks.
package task

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/langgraph-go/langgraph/types"
)

// TaskDecorator wraps a function with retry, cache, and other policies.
type TaskDecorator struct {
	name        string
	retryPolicy *types.RetryPolicy
	cachePolicy *types.CachePolicy
	metadata    map[string]interface{}
}

// DecoratorOption configures a TaskDecorator.
type DecoratorOption func(*TaskDecorator)

// WithName sets the task name.
func WithName(name string) DecoratorOption {
	return func(d *TaskDecorator) {
		d.name = name
	}
}

// WithRetryPolicy sets the retry policy.
func WithRetryPolicy(policy *types.RetryPolicy) DecoratorOption {
	return func(d *TaskDecorator) {
		d.retryPolicy = policy
	}
}

// WithCachePolicy sets the cache policy.
func WithCachePolicy(policy *types.CachePolicy) DecoratorOption {
	return func(d *TaskDecorator) {
		d.cachePolicy = policy
	}
}

// WithMetadata sets task metadata.
func WithMetadata(metadata map[string]interface{}) DecoratorOption {
	return func(d *TaskDecorator) {
		d.metadata = metadata
	}
}

// NewDecorator creates a new task decorator.
func NewDecorator(opts ...DecoratorOption) *TaskDecorator {
	d := &TaskDecorator{
		name:     uuid.New().String(),
		metadata: make(map[string]interface{}),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Wrap wraps a function with the decorator's policies.
func (d *TaskDecorator) Wrap(fn types.NodeFunc) types.NodeFunc {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		// Create task context
		taskCtx := &TaskContext{
			Name:     d.name,
			ID:       uuid.New().String(),
			Input:    input,
			Metadata: d.metadata,
			Start:    time.Now(),
		}

		// Execute with retry if configured
		if d.retryPolicy != nil {
			return d.executeWithRetry(ctx, taskCtx, fn)
		}

		// Execute normally
		output, err := fn(ctx, input)
		taskCtx.End = time.Now()
		taskCtx.Output = output
		taskCtx.Error = err
		return output, err
	}
}

// executeWithRetry executes the function with retry logic.
func (d *TaskDecorator) executeWithRetry(ctx context.Context, taskCtx *TaskContext, fn types.NodeFunc) (interface{}, error) {
	policy := d.retryPolicy
	var lastErr error

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		taskCtx.Attempt = attempt

		output, err := fn(ctx, taskCtx.Input)
		if err == nil {
			taskCtx.End = time.Now()
			taskCtx.Output = output
			return output, nil
		}

		// Check if retryable
		if policy.RetryOn != nil && !policy.RetryOn(err) {
			return nil, fmt.Errorf("task %s failed with non-retryable error: %w", d.name, err)
		}

		lastErr = err

		if attempt >= policy.MaxAttempts {
			break
		}

		// Calculate backoff
		backoff := calculateBackoff(attempt, policy)

		// Wait before retry
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			// Continue
		}
	}

	taskCtx.End = time.Now()
	taskCtx.Error = lastErr
	return nil, fmt.Errorf("task %s failed after %d attempts: %w", d.name, policy.MaxAttempts, lastErr)
}

// TaskContext holds context information about a task execution.
type TaskContext struct {
	Name     string
	ID       string
	Input    interface{}
	Output   interface{}
	Error    error
	Attempt  int
	Metadata map[string]interface{}
	Start    time.Time
	End      time.Time
}

// Duration returns the execution duration.
func (tc *TaskContext) Duration() time.Duration {
	return tc.End.Sub(tc.Start)
}

// calculateBackoff calculates the backoff duration.
func calculateBackoff(attempt int, policy *types.RetryPolicy) time.Duration {
	backoff := time.Duration(float64(policy.InitialInterval) * pow(policy.BackoffFactor, attempt-1))
	if backoff > policy.MaxInterval {
		backoff = policy.MaxInterval
	}

	if policy.Jitter {
		backoff = addJitter(backoff)
	}

	return backoff
}

// pow calculates base^exp.
func pow(base float64, exp int) float64 {
	result := 1.0
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}

// addJitter adds random jitter to a duration.
func addJitter(d time.Duration) time.Duration {
	jitter := float64(d) * 0.5 * float64(time.Now().UnixNano()%100)/100.0
	return d - time.Duration(jitter)
}

// Task wraps a function with the given options.
func Task(fn types.NodeFunc, opts ...DecoratorOption) types.NodeFunc {
	decorator := NewDecorator(opts...)
	return decorator.Wrap(fn)
}

// Entrypoint marks a function as a graph entrypoint.
type Entrypoint struct {
	name     string
	fn       types.NodeFunc
	metadata map[string]interface{}
}

// NewEntrypoint creates a new entrypoint.
func NewEntrypoint(name string, fn types.NodeFunc, metadata map[string]interface{}) *Entrypoint {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	return &Entrypoint{
		name:     name,
		fn:       fn,
		metadata: metadata,
	}
}

// Name returns the entrypoint name.
func (e *Entrypoint) Name() string {
	return e.name
}

// Execute executes the entrypoint.
func (e *Entrypoint) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	return e.fn(ctx, input)
}

// EntrypointDecorator creates an entrypoint decorator.
func EntrypointDecorator(name string, metadata map[string]interface{}) func(types.NodeFunc) types.NodeFunc {
	return func(fn types.NodeFunc) types.NodeFunc {
		entry := NewEntrypoint(name, fn, metadata)
		return func(ctx context.Context, input interface{}) (interface{}, error) {
			return entry.Execute(ctx, input)
		}
	}
}

// Retryable marks a function as retryable with the given policy.
func Retryable(fn types.NodeFunc, maxAttempts int, backoffFactor float64) types.NodeFunc {
	policy := types.DefaultRetryPolicy()
	policy.MaxAttempts = maxAttempts
	policy.BackoffFactor = backoffFactor

	return Task(fn, WithRetryPolicy(&policy))
}

// Cached wraps a function with caching.
func Cached(fn types.NodeFunc, ttl time.Duration) types.NodeFunc {
	policy := &types.CachePolicy{
		TTL: &ttl,
	}
	return Task(fn, WithCachePolicy(policy))
}

// Named names a task.
func Named(name string, fn types.NodeFunc) types.NodeFunc {
	return Task(fn, WithName(name))
}

// WithTimeout adds timeout to a function.
func WithTimeout(fn types.NodeFunc, timeout time.Duration) types.NodeFunc {
	return func(ctx context.Context, input interface{}) (interface{}, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return fn(ctx, input)
	}
}

// Compose composes multiple decorators.
func Compose(decorators ...func(types.NodeFunc) types.NodeFunc) func(types.NodeFunc) types.NodeFunc {
	return func(fn types.NodeFunc) types.NodeFunc {
		for i := len(decorators) - 1; i >= 0; i-- {
			fn = decorators[i](fn)
		}
		return fn
	}
}
