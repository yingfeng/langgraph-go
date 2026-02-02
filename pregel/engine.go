// Package pregel provides the Pregel execution algorithm for graph processing.
package pregel

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/langgraph-go/langgraph/channels"
	"github.com/langgraph-go/langgraph/errors"
	"github.com/langgraph-go/langgraph/graph"
	"github.com/langgraph-go/langgraph/types"
)

// Engine implements the Pregel execution model.
type Engine struct {
	graph          interface{} // StateGraph
	checkpointer   graph.Checkpointer
	interrupts     map[string]bool
	recursionLimit int
	debug          bool
	config         *types.RunnableConfig
}

// NewEngine creates a new Pregel engine.
func NewEngine(graph interface{}, opts ...EngineOption) *Engine {
	eng := &Engine{
		graph:          graph,
		interrupts:     make(map[string]bool),
		recursionLimit: 25,
		debug:          false,
		config:         types.NewRunnableConfig(),
	}
	
	for _, opt := range opts {
		opt(eng)
	}
	
	return eng
}

// EngineOption is an option for configuring the Pregel engine.
type EngineOption func(*Engine)

// WithCheckpointer sets the checkpointer.
func WithCheckpointer(cp graph.Checkpointer) EngineOption {
	return func(e *Engine) {
		e.checkpointer = cp
	}
}

// WithInterrupts sets the interrupt nodes.
func WithInterrupts(nodes ...string) EngineOption {
	return func(e *Engine) {
		for _, node := range nodes {
			e.interrupts[node] = true
		}
	}
}

// WithRecursionLimit sets the recursion limit.
func WithRecursionLimit(limit int) EngineOption {
	return func(e *Engine) {
		e.recursionLimit = limit
	}
}

// WithDebug enables debug mode.
func WithDebug(debug bool) EngineOption {
	return func(e *Engine) {
		e.debug = debug
	}
}

// WithConfig sets the runnable config.
func WithConfig(cfg *types.RunnableConfig) EngineOption {
	return func(e *Engine) {
		e.config = cfg
	}
}

// ExecuteResult represents the result of graph execution.
type ExecuteResult struct {
	// Final state of the graph.
	State interface{}
	// Checkpoint ID for this execution.
	CheckpointID string
	// Metadata about the execution.
	Metadata map[string]interface{}
}

// Run executes the graph using the Pregel algorithm.
func (e *Engine) Run(ctx context.Context, input interface{}, mode types.StreamMode) (<-chan interface{}, <-chan error) {
	outputCh := make(chan interface{}, 100)
	errCh := make(chan error, 1)
	
	go func() {
		defer close(outputCh)
		defer close(errCh)
		
		// Initialize channels
		channelRegistry := channels.NewRegistry()
		graphChannels := e.getGraphChannels()
		for name, ch := range graphChannels {
			channelRegistry.Register(name, ch.Copy())
		}
		
		// Apply input to channels
		if err := e.applyInput(channelRegistry, input); err != nil {
			errCh <- fmt.Errorf("failed to apply input: %w", err)
			return
		}
		
		// Get thread ID for checkpointing
		threadID := e.getThreadID()
		
		// Load checkpoint if available
		if e.checkpointer != nil {
			checkpoint, err := e.checkpointer.Get(ctx, map[string]interface{}{
				"thread_id": threadID,
			})
			if err == nil && checkpoint != nil {
				if err := channelRegistry.RestoreFromCheckpoint(checkpoint); err != nil {
					errCh <- fmt.Errorf("failed to restore from checkpoint: %w", err)
					return
				}
			}
		}
		
		// Execute Pregel loop
		step := 0
		completedTasks := make(map[string]bool)
		lastCompletedNode := ""
		lastState := input
		
		for {
			// Check recursion limit
			if step >= e.recursionLimit {
				errCh <- &errors.GraphRecursionError{Limit: e.recursionLimit}
				return
			}
			
			// Emit checkpoint event if in debug or checkpoints mode
			if mode == types.StreamModeCheckpoints || mode == types.StreamModeDebug {
				checkpointData := channelRegistry.CreateCheckpoint()
				outputCh <- map[string]interface{}{
					"type":      "checkpoint",
					"step":      step,
					"checkpoint": checkpointData,
					"timestamp": time.Now().Format(time.RFC3339Nano),
				}
			}
			
			// Determine next tasks
			tasks, triggers, err := e.prepareNextTasks(channelRegistry, completedTasks, lastCompletedNode, lastState)
			if err != nil {
				errCh <- fmt.Errorf("failed to prepare next tasks: %w", err)
				return
			}
			
			// Emit tasks event if in debug or tasks mode
			if mode == types.StreamModeTasks || mode == types.StreamModeDebug {
				taskNames := make([]string, len(tasks))
				for i, task := range tasks {
					taskNames[i] = task.Name
				}
				outputCh <- map[string]interface{}{
					"type":  "tasks",
					"step":  step,
					"names": taskNames,
					"count": len(tasks),
				}
			}
			
			// If no tasks, we're done
			if len(tasks) == 0 {
				break
			}
			
			// Check for interrupts
			interruptedTasks := e.shouldInterrupt(channelRegistry, tasks, triggers)
			if len(interruptedTasks) > 0 {
				// Save checkpoint
				if e.checkpointer != nil {
					checkpoint := channelRegistry.CreateCheckpoint()
					if err := e.checkpointer.Put(ctx, map[string]interface{}{
						"thread_id": threadID,
					}, checkpoint); err != nil {
						errCh <- fmt.Errorf("failed to save checkpoint: %w", err)
						return
					}
				}
				
				// Emit interrupt event
				if mode == types.StreamModeDebug {
					interruptNames := make([]string, len(interruptedTasks))
					for i, task := range interruptedTasks {
						interruptNames[i] = task.Name
					}
					outputCh <- map[string]interface{}{
						"type":  "interrupt",
						"step":  step,
						"tasks": interruptNames,
					}
				}
				
				errCh <- &errors.GraphInterrupt{}
				return
			}
			
			// Execute tasks
			results, err := e.executeTasks(ctx, tasks, channelRegistry)
			if err != nil {
				errCh <- fmt.Errorf("failed to execute tasks: %w", err)
				return
			}
			
			// Mark tasks as completed and track last state
			for _, result := range results {
				if result.Err == nil {
					completedTasks[result.Name] = true
					lastCompletedNode = result.Name
					// Merge result into lastState
					lastState = e.mergeStates(lastState, result.Output)
				}
			}
			
			// Apply writes to channels
			_, err = e.applyWrites(channelRegistry, results, triggers)
			if err != nil {
				errCh <- fmt.Errorf("failed to apply writes: %w", err)
				return
			}
			
			// Emit updates if in updates mode
			if mode == types.StreamModeUpdates {
				for _, result := range results {
					if result.Err == nil {
						outputCh <- map[string]interface{}{
							"type":    "update",
							"node":    result.Name,
							"output":  result.Output,
							"step":    step,
						}
					}
				}
			}
			
			// Emit values if in values mode
			if mode == types.StreamModeValues {
				values, _ := channelRegistry.GetValues()
				outputCh <- map[string]interface{}{
					"type":  "values",
					"values": values,
					"step":  step,
				}
			}
			
			// Save checkpoint
			if e.checkpointer != nil {
				checkpoint := channelRegistry.CreateCheckpoint()
				checkpointID := uuid.New().String()
				if err := e.checkpointer.Put(ctx, map[string]interface{}{
					"thread_id": threadID,
					"checkpoint_id": checkpointID,
					"step": step,
				}, checkpoint); err != nil {
					errCh <- fmt.Errorf("failed to save checkpoint: %w", err)
					return
				}
			}
			
			step++
		}
		
		// Get final state
		finalState, err := e.buildOutput(channelRegistry, lastState)
		if err != nil {
			errCh <- fmt.Errorf("failed to build output: %w", err)
			return
		}
		
		// Emit final result
		outputCh <- map[string]interface{}{
			"type":  "final",
			"state": finalState,
		}
	}()
	
	return outputCh, errCh
}

// prepareNextTasks determines which tasks to execute next.
func (e *Engine) prepareNextTasks(
	registry *channels.Registry,
	completedTasks map[string]bool,
	lastCompletedNode string,
	currentState interface{},
) ([]*Task, map[string]struct{}, error) {
	tasks := make([]*Task, 0)
	triggerToNodes := make(map[string]struct{})
	
	// If this is the first step
	if len(completedTasks) == 0 {
		entryPoint := e.getEntryPoint()
		if entryPoint == "" {
			return nil, nil, fmt.Errorf("no entry point set")
		}
		
		node := e.getNode(entryPoint)
		if node == nil {
			return nil, nil, &errors.NodeNotFoundError{NodeName: entryPoint}
		}
		
		task := e.createTask(node, currentState, []string{}, []string{})
		tasks = append(tasks, task)
		triggerToNodes["__start__"] = struct{}{}
		return tasks, triggerToNodes, nil
	}
	
	// Determine next nodes based on edges from last completed node
	nextNodes := e.getNextNodes(lastCompletedNode, currentState)
	
	for nodeName := range nextNodes {
		node := e.getNode(nodeName)
		if node == nil {
			continue
		}
		
		// Determine triggers for this node
		triggers := e.getTriggers(node)
		
		// Don't re-execute completed nodes
		if !completedTasks[nodeName] {
			task := e.createTask(node, currentState, triggers, []string{})
			tasks = append(tasks, task)
			
			// Build trigger to nodes mapping
			for _, trigger := range triggers {
				triggerToNodes[trigger] = struct{}{}
			}
		}
	}
	
	return tasks, triggerToNodes, nil
}

// shouldInterrupt checks if graph should be interrupted.
func (e *Engine) shouldInterrupt(
	registry *channels.Registry,
	tasks []*Task,
	triggerToNodes map[string]struct{},
) []*Task {
	interrupted := make([]*Task, 0)
	
	// Check if any triggered node should interrupt
	if len(e.interrupts) == 0 {
		return interrupted
	}
	
	// Check if "*" is set (interrupt all)
	interruptAll := e.interrupts[types.All]
	
	for _, task := range tasks {
		shouldInterrupt := false
		if interruptAll {
			shouldInterrupt = true
		} else {
			shouldInterrupt = e.interrupts[task.Name]
		}
		
		if shouldInterrupt {
			// Check if this task was triggered by a channel update
			triggered := false
			for trigger := range task.Triggers {
				if _, ok := triggerToNodes[trigger]; ok {
					triggered = true
					break
				}
			}
			
			if triggered {
				interrupted = append(interrupted, task)
			}
		}
	}
	
	return interrupted
}

// applyWrites applies task outputs to channels.
func (e *Engine) applyWrites(
	registry *channels.Registry,
	results []*TaskResult,
	triggerToNodes map[string]struct{},
) (map[string]struct{}, error) {
	updatedChannels := make(map[string]struct{})
	
	// Sort results for deterministic order
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	
	// Group writes by channel
	writesByChannel := make(map[string][]interface{})
	for _, result := range results {
		if result.Err != nil {
			continue
		}
		
		// Convert output to map of writes
		outputMap, err := toMap(result.Output)
		if err != nil {
			return nil, fmt.Errorf("failed to convert output to map: %w", err)
		}
		
		for key, value := range outputMap {
			// Check for Overwrite wrapper
			if ow, ok := value.(*types.Overwrite); ok {
				value = ow.Value
			}
			
			// Add to writes
			writesByChannel[key] = append(writesByChannel[key], value)
		}
	}
	
	// Apply writes to channels
	for channelName, values := range writesByChannel {
		if ch, ok := registry.Get(channelName); ok {
			updated, err := ch.Update(values)
			if err != nil {
				return nil, fmt.Errorf("failed to update channel %s: %w", channelName, err)
			}
			if updated && ch.IsAvailable() {
				updatedChannels[channelName] = struct{}{}
			}
		}
	}
	
	return updatedChannels, nil
}

// executeTasks executes the given tasks concurrently.
func (e *Engine) executeTasks(
	ctx context.Context,
	tasks []*Task,
	registry *channels.Registry,
) ([]*TaskResult, error) {
	results := make([]*TaskResult, len(tasks))
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t *Task) {
			defer wg.Done()
			
			result := e.executeTask(ctx, t, registry)
			
			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, task)
	}
	
	wg.Wait()
	
	return results, nil
}

// executeTask executes a single task with retry logic.
func (e *Engine) executeTask(
	ctx context.Context,
	task *Task,
	registry *channels.Registry,
) *TaskResult {
	retryPolicy := task.RetryPolicy
	if retryPolicy == nil {
		defaultPolicy := types.DefaultRetryPolicy()
		retryPolicy = &defaultPolicy
	}
	
	attempts := 0
	var lastErr error
	
	for attempts < retryPolicy.MaxAttempts {
		// Read input for this task
		input, err := e.readTaskInput(registry, task)
		if err != nil {
			return &TaskResult{
				Name: task.Name,
				Err:  fmt.Errorf("failed to read task input: %w", err),
			}
		}
		
		// Execute the task
		output, err := task.Func(ctx, input)
		if err == nil {
			// Success
			return &TaskResult{
				Name:   task.Name,
				Output: output,
				Err:    nil,
			}
		}
		
		// Check for interrupt
		if errors.IsGraphInterrupt(err) {
			return &TaskResult{
				Name: task.Name,
				Err:  err,
			}
		}
		
		lastErr = err
		attempts++
		
		if attempts >= retryPolicy.MaxAttempts {
			break
		}
		
		// Check if we should retry this error
		if retryPolicy.RetryOn != nil && !retryPolicy.RetryOn(err) {
			break
		}
		
		// Calculate wait time
		waitTime := retryPolicy.InitialInterval
		for i := 0; i < attempts-1; i++ {
			waitTime = time.Duration(float64(waitTime) * retryPolicy.BackoffFactor)
			if waitTime > retryPolicy.MaxInterval {
				waitTime = retryPolicy.MaxInterval
				break
			}
		}
		
		// Add jitter if configured
		if retryPolicy.Jitter {
			waitTime = time.Duration(float64(waitTime) * (0.5 + 0.5*float64(time.Now().UnixNano()%1000)/1000))
		}
		
		// Wait before retry
		select {
		case <-ctx.Done():
			return &TaskResult{
				Name: task.Name,
				Err:  ctx.Err(),
			}
		case <-time.After(waitTime):
			// Continue to next attempt
		}
	}
	
	return &TaskResult{
		Name: task.Name,
		Err:  fmt.Errorf("max retries exceeded (%d): %w", attempts, lastErr),
	}
}

// readTaskInput reads the input for a task from channels.
func (e *Engine) readTaskInput(registry *channels.Registry, task *Task) (interface{}, error) {
	if len(task.Channels) == 0 {
		return nil, nil
	}
	
	// Read values from specified channels
	values := make(map[string]interface{})
	for _, channelName := range task.Channels {
		if ch, ok := registry.Get(channelName); ok {
			value, err := ch.Get()
			if err != nil {
				if _, isEmpty := err.(*errors.EmptyChannelError); !isEmpty {
					return nil, err
				}
				// Empty channels are OK
				continue
			}
			values[channelName] = value
		}
	}
	
	return values, nil
}

// Task represents a task to execute.
type Task struct {
	ID         string
	Name       string
	Func       types.NodeFunc
	Channels   []string
	Triggers   map[string]struct{}
	RetryPolicy *types.RetryPolicy
}

// TaskResult represents the result of executing a task.
type TaskResult struct {
	Name   string
	Output interface{}
	Err    error
}

// Helper methods that need to be implemented by the StateGraph
func (e *Engine) getGraphChannels() map[string]channels.Channel {
	// This will be implemented when we have the actual StateGraph interface
	return make(map[string]channels.Channel)
}

func (e *Engine) getEntryPoint() string {
	// This will be implemented when we have the actual StateGraph interface
	return ""
}

func (e *Engine) getNode(name string) interface{} {
	// This will be implemented when we have the actual StateGraph interface
	return nil
}

func (e *Engine) getNextNodes(node string, state interface{}) map[string]bool {
	// This will be implemented when we have the actual StateGraph interface
	return make(map[string]bool)
}

func (e *Engine) getTriggers(node interface{}) []string {
	// This will be implemented when we have the actual StateGraph interface
	return []string{}
}

func (e *Engine) createTask(node interface{}, state interface{}, channels []string, triggers []string) *Task {
	// This will be implemented when we have the actual StateGraph interface
	return &Task{
		ID:       uuid.New().String(),
		Name:     "",
		Channels: channels,
		Triggers: make(map[string]struct{}),
	}
}

func (e *Engine) applyInput(registry *channels.Registry, input interface{}) error {
	// Convert input to map
	inputMap, err := toMap(input)
	if err != nil {
		return err
	}
	
	// Apply each key to corresponding channel
	writes := make(map[string][]interface{})
	for key, value := range inputMap {
		writes[key] = []interface{}{value}
	}
	
	return registry.UpdateChannels(writes)
}

func (e *Engine) getThreadID() string {
	if e.config != nil && e.config.Configurable != nil {
		if tid, ok := e.config.Configurable["thread_id"].(string); ok {
			return tid
		}
	}
	return uuid.New().String()
}

func (e *Engine) buildOutput(registry *channels.Registry, lastState interface{}) (interface{}, error) {
	values, err := registry.GetValues()
	if err != nil {
		return lastState, nil
	}
	
	if len(values) > 0 {
		return values, nil
	}
	
	return lastState, nil
}

func (e *Engine) mergeStates(existing, new interface{}) interface{} {
	if existing == nil {
		return new
	}
	
	if new == nil {
		return existing
	}
	
	// Try to merge maps
	existingMap, ok1 := existing.(map[string]interface{})
	newMap, ok2 := new.(map[string]interface{})
	
	if ok1 && ok2 {
		result := make(map[string]interface{})
		for k, v := range existingMap {
			result[k] = v
		}
		for k, v := range newMap {
			result[k] = v
		}
		return result
	}
	
	return new
}

// toMap converts a struct or map to a map[string]interface{}.
func toMap(v interface{}) (map[string]interface{}, error) {
	if v == nil {
		return nil, fmt.Errorf("nil value")
	}
	
	// If it's already a map
	if m, ok := v.(map[string]interface{}); ok {
		return m, nil
	}
	
	// Use reflection to convert struct to map
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	
	if rv.Kind() != reflect.Struct && rv.Kind() != reflect.Map {
		return map[string]interface{}{"__root__": v}, nil
	}
	
	result := make(map[string]interface{})
	
	if rv.Kind() == reflect.Map {
		for _, key := range rv.MapKeys() {
			result[fmt.Sprintf("%v", key.Interface())] = rv.MapIndex(key).Interface()
		}
		return result, nil
	}
	
	// Struct
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}
		value := rv.Field(i).Interface()
		
		// Convert field name to snake_case for consistency
		fieldName := toSnakeCase(field.Name)
		result[fieldName] = value
	}
	
	return result, nil
}

// toSnakeCase converts CamelCase to snake_case.
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}
