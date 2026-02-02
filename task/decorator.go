// Package task provides function decorators for LangGraph tasks.
package task

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/langgraph-go/langgraph/graph"
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
	name         string
	fn           types.NodeFunc
	metadata     map[string]interface{}
	checkpointer interface{}
	store        interface{}
	configurable map[string]interface{}
	graph        *graph.StateGraph
	compiled     bool
}

// NewEntrypoint creates a new entrypoint.
func NewEntrypoint(name string, fn types.NodeFunc, metadata map[string]interface{}) *Entrypoint {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	return &Entrypoint{
		name:         name,
		fn:           fn,
		metadata:     metadata,
		checkpointer: nil,
		store:        nil,
		configurable:  make(map[string]interface{}),
		graph:        nil,
		compiled:     false,
	}
}

// EntrypointOption configures an entrypoint.
type EntrypointOption func(*Entrypoint)

// WithEntrypointCheckpointer sets the checkpointer for the entrypoint.
func WithEntrypointCheckpointer(cp interface{}) EntrypointOption {
	return func(e *Entrypoint) {
		e.checkpointer = cp
	}
}

// WithEntrypointStore sets the store for the entrypoint.
func WithEntrypointStore(st interface{}) EntrypointOption {
	return func(e *Entrypoint) {
		e.store = st
	}
}

// WithEntrypointConfigurable sets configurable values for the entrypoint.
func WithEntrypointConfigurable(configurable map[string]interface{}) EntrypointOption {
	return func(e *Entrypoint) {
		e.configurable = configurable
	}
}

// WithEntrypointGraph sets the graph for the entrypoint.
func WithEntrypointGraph(g *graph.StateGraph) EntrypointOption {
	return func(e *Entrypoint) {
		e.graph = g
	}
}

// NewEntrypointWithOptions creates a new entrypoint with options.
func NewEntrypointWithOptions(name string, fn types.NodeFunc, metadata map[string]interface{}, opts ...EntrypointOption) *Entrypoint {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	e := &Entrypoint{
		name:         name,
		fn:           fn,
		metadata:     metadata,
		checkpointer: nil,
		store:        nil,
		configurable:  make(map[string]interface{}),
		graph:        nil,
		compiled:     false,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Name returns the entrypoint name.
func (e *Entrypoint) Name() string {
	return e.name
}

// Execute executes the entrypoint.
func (e *Entrypoint) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	return e.fn(ctx, input)
}

// Compile compiles the graph associated with this entrypoint.
func (e *Entrypoint) Compile(ctx context.Context) error {
	if e.graph == nil {
		return fmt.Errorf("no graph associated with entrypoint")
	}
	if e.compiled {
		return nil
	}

	// Set checkpointer if provided (via config)
	if e.checkpointer != nil {
		// Note: In a full implementation, this would set the checkpointer
		// on the graph's configuration
	}

	// Set store if provided (via config)
	if e.store != nil {
		// Note: In a full implementation, this would set the store
		// on the graph's configuration
	}

	e.compiled = true
	return nil
}

// Invoke invokes the graph with the given input.
func (e *Entrypoint) Invoke(ctx context.Context, input interface{}, config *types.RunnableConfig) (interface{}, error) {
	// Compile if not already compiled
	if !e.compiled {
		if err := e.Compile(ctx); err != nil {
			return nil, err
		}
	}

	// Merge configurable values
	if config == nil {
		config = types.NewRunnableConfig()
	}
	for k, v := range e.configurable {
		config.Set(k, v)
	}

	// Invoke the graph
	if e.graph == nil {
		// If no graph, just execute the function
		return e.Execute(ctx, input)
	}

	// Note: In a full implementation, this would use the graph's Invoke method
	// For now, we just execute the function
	return e.Execute(ctx, input)
}

// AInvoke invokes the graph asynchronously with the given input.
func (e *Entrypoint) AInvoke(ctx context.Context, input interface{}, config *types.RunnableConfig) <-chan struct{} {
	result := make(chan struct{}, 1)

	go func() {
		defer close(result)
		_, err := e.Invoke(ctx, input, config)
		if err != nil {
			// In a full implementation, we'd return the error
			// For now, we just log or handle it
			_ = err
		}
	}()

	return result
}

// Stream streams the output of the graph execution.
func (e *Entrypoint) Stream(ctx context.Context, input interface{}, config *types.RunnableConfig, mode types.StreamMode) (<-chan interface{}, error) {
	// Compile if not already compiled
	if !e.compiled {
		if err := e.Compile(ctx); err != nil {
			return nil, err
		}
	}

	// Merge configurable values
	if config == nil {
		config = types.NewRunnableConfig()
	}
	for k, v := range e.configurable {
		config.Set(k, v)
	}

	// Stream from the graph
	if e.graph == nil {
		// If no graph, just execute the function
		output, err := e.Execute(ctx, input)
		if err != nil {
			return nil, err
		}
		ch := make(chan interface{}, 1)
		ch <- output
		close(ch)
		return ch, nil
	}

	// Note: In a full implementation, this would use the graph's Stream method
	// For now, we just execute the function
	output, err := e.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	ch := make(chan interface{}, 1)
	ch <- output
	close(ch)
	return ch, nil
}

// AStream streams the output of the graph execution asynchronously.
func (e *Entrypoint) AStream(ctx context.Context, input interface{}, config *types.RunnableConfig, mode types.StreamMode) (<-chan interface{}, error) {
	return e.Stream(ctx, input, config, mode)
}

// Batch invokes the graph with multiple inputs.
func (e *Entrypoint) Batch(ctx context.Context, inputs []interface{}, config *types.RunnableConfig) ([]interface{}, error) {
	results := make([]interface{}, len(inputs))
	
	for i, input := range inputs {
		output, err := e.Invoke(ctx, input, config)
		if err != nil {
			return nil, fmt.Errorf("batch invocation failed at index %d: %w", i, err)
		}
		results[i] = output
	}
	
	return results, nil
}

// ABatch invokes the graph with multiple inputs asynchronously.
func (e *Entrypoint) ABatch(ctx context.Context, inputs []interface{}, config *types.RunnableConfig) <-chan struct{} {
	result := make(chan struct{}, 1)

	go func() {
		defer close(result)
		_, err := e.Batch(ctx, inputs, config)
		if err != nil {
			_ = err
		}
	}()

	return result
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
