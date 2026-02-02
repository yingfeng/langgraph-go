package channels

import (
	"fmt"
	"sync"

	"github.com/langgraph-go/langgraph/errors"
)

// NamedBarrierValue waits until all specified named nodes have written a value.
type NamedBarrierValue struct {
	BaseChannel
	waitingFor map[string]bool
	values    map[string]interface{}
	mu        sync.RWMutex
}

// NewNamedBarrierValue creates a new NamedBarrierValue channel.
func NewNamedBarrierValue(typ interface{}, waitFor []string) *NamedBarrierValue {
	m := make(map[string]bool)
	for _, name := range waitFor {
		m[name] = true
	}
	return &NamedBarrierValue{
		BaseChannel: BaseChannel{Typ: typ},
		waitingFor: m,
		values:     make(map[string]interface{}),
	}
}

// Get returns the value of the channel if all nodes have written.
func (c *NamedBarrierValue) Get() (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if len(c.values) == 0 {
		return nil, &errors.EmptyChannelError{}
	}
	
	// Check if all values are present
	for name := range c.waitingFor {
		if _, ok := c.values[name]; !ok {
			return nil, &errors.EmptyChannelError{
				Message: fmt.Sprintf("waiting for node '%s'", name),
			}
		}
	}
	
	// Return values as map
	return c.values, nil
}

// IsAvailable returns true if all nodes have written a value.
func (c *NamedBarrierValue) IsAvailable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if len(c.values) == 0 {
		return false
	}
	
	for name := range c.waitingFor {
		if _, ok := c.values[name]; !ok {
			return false
		}
	}
	
	return true
}

// Update updates the channel with new values from nodes.
func (c *NamedBarrierValue) Update(values []interface{}) (bool, error) {
	if len(values) == 0 {
		return false, nil
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	updated := false
	for _, val := range values {
		// Check if this is a named value (tuple with node name and value)
		if tuple, ok := val.([2]interface{}); ok && len(tuple) == 2 {
			if name, ok := tuple[0].(string); ok {
				if _, shouldWait := c.waitingFor[name]; shouldWait {
					// Store the value
					c.values[name] = tuple[1]
					updated = true
				}
			}
		} else if tuple, ok := val.([]interface{}); ok && len(tuple) == 2 {
			if name, ok := tuple[0].(string); ok {
				if _, shouldWait := c.waitingFor[name]; shouldWait {
					c.values[name] = tuple[1]
					updated = true
				}
			}
		}
	}
	
	return updated, nil
}

// Copy returns a copy of the channel.
func (c *NamedBarrierValue) Copy() Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	newCh := NewNamedBarrierValue(c.Typ, nil)
	newCh.Key = c.Key
	
	// Copy waitingFor map
	newCh.waitingFor = make(map[string]bool, len(c.waitingFor))
	for k, v := range c.waitingFor {
		newCh.waitingFor[k] = v
	}
	
	// Copy values map
	newCh.values = make(map[string]interface{}, len(c.values))
	for k, v := range c.values {
		newCh.values[k] = v
	}
	
	return newCh
}

// Checkpoint returns the current values.
func (c *NamedBarrierValue) Checkpoint() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if len(c.values) == 0 {
		return Missing
	}
	
	// Return a copy of values
	result := make(map[string]interface{}, len(c.values))
	for k, v := range c.values {
		result[k] = v
	}
	return result
}

// FromCheckpoint restores the channel from a checkpoint.
func (c *NamedBarrierValue) FromCheckpoint(checkpoint interface{}) Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	newCh := NewNamedBarrierValue(c.Typ, nil)
	newCh.Key = c.Key
	
	if checkpoint == nil {
		return newCh
	}
	
	// Restore values from checkpoint
	if cp, ok := checkpoint.(map[string]interface{}); ok {
		newCh.values = make(map[string]interface{}, len(cp))
		for k, v := range cp {
			newCh.values[k] = v
		}
	}
	
	return newCh
}

// Finish checks if this node has completed writing.
func (c *NamedBarrierValue) Finish() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if all nodes have completed
	updated := len(c.waitingFor) > 0
	c.waitingFor = make(map[string]bool)
	return updated
}

// Consume marks the channel as consumed.
func (c *NamedBarrierValue) Consume() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Clear values after consumption
	if len(c.values) > 0 {
		c.values = make(map[string]interface{})
		return true
	}
	return false
}

// NamedBarrierValueAfterFinish waits for all specified nodes, but only makes value available after finish().
type NamedBarrierValueAfterFinish struct {
	BaseChannel
	waitingFor map[string]bool
	values    map[string]interface{}
	finished  bool
	mu        sync.RWMutex
}

// NewNamedBarrierValueAfterFinish creates a new NamedBarrierValueAfterFinish channel.
func NewNamedBarrierValueAfterFinish(typ interface{}, waitFor []string) *NamedBarrierValueAfterFinish {
	m := make(map[string]bool)
	for _, name := range waitFor {
		m[name] = true
	}
	return &NamedBarrierValueAfterFinish{
		BaseChannel: BaseChannel{Typ: typ},
		waitingFor: m,
		values:     make(map[string]interface{}),
		finished:   false,
	}
}

// Get returns the value of the channel if finished and all nodes have written.
func (c *NamedBarrierValueAfterFinish) Get() (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.finished {
		return nil, &errors.EmptyChannelError{}
	}
	
	if len(c.values) == 0 {
		return nil, &errors.EmptyChannelError{}
	}
	
	// Check if all values are present
	for name := range c.waitingFor {
		if _, ok := c.values[name]; !ok {
			return nil, &errors.EmptyChannelError{}
		}
	}
	
	return c.values, nil
}

// IsAvailable returns true if finished and all nodes have written a value.
func (c *NamedBarrierValueAfterFinish) IsAvailable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.finished {
		return false
	}
	
	if len(c.values) == 0 {
		return false
	}
	
	for name := range c.waitingFor {
		if _, ok := c.values[name]; !ok {
			return false
		}
	}
	
	return true
}

// Update updates the channel with new values from nodes.
func (c *NamedBarrierValueAfterFinish) Update(values []interface{}) (bool, error) {
	if len(values) == 0 {
		return false, nil
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	updated := false
	for _, val := range values {
		// Check if this is a named value (tuple with node name and value)
		if tuple, ok := val.([2]interface{}); ok && len(tuple) == 2 {
			if name, ok := tuple[0].(string); ok {
				if _, shouldWait := c.waitingFor[name]; shouldWait {
					c.values[name] = tuple[1]
					updated = true
				}
			}
		} else if tuple, ok := val.([]interface{}); ok && len(tuple) == 2 {
			if name, ok := tuple[0].(string); ok {
				if _, shouldWait := c.waitingFor[name]; shouldWait {
					c.values[name] = tuple[1]
					updated = true
				}
			}
		}
	}
	
	return updated, nil
}

// Copy returns a copy of the channel.
func (c *NamedBarrierValueAfterFinish) Copy() Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	newCh := NewNamedBarrierValueAfterFinish(c.Typ, nil)
	newCh.Key = c.Key
	newCh.finished = c.finished
	
	// Copy waitingFor map
	newCh.waitingFor = make(map[string]bool, len(c.waitingFor))
	for k, v := range c.waitingFor {
		newCh.waitingFor[k] = v
	}
	
	// Copy values map
	newCh.values = make(map[string]interface{}, len(c.values))
	for k, v := range c.values {
		newCh.values[k] = v
	}
	
	return newCh
}

// Checkpoint returns the current values.
func (c *NamedBarrierValueAfterFinish) Checkpoint() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if len(c.values) == 0 {
		return Missing
	}
	
	// Return a copy of values
	result := make(map[string]interface{}, len(c.values))
	for k, v := range c.values {
		result[k] = v
	}
	return result
}

// FromCheckpoint restores the channel from a checkpoint.
func (c *NamedBarrierValueAfterFinish) FromCheckpoint(checkpoint interface{}) Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	newCh := NewNamedBarrierValueAfterFinish(c.Typ, nil)
	newCh.Key = c.Key
	newCh.finished = c.finished
	
	if checkpoint == nil {
		return newCh
	}
	
	// Restore values from checkpoint
	if cp, ok := checkpoint.(map[string]interface{}); ok {
		newCh.values = make(map[string]interface{}, len(cp))
		for k, v := range cp {
			newCh.values[k] = v
		}
	}
	
	return newCh
}

// Finish marks the channel as finished.
func (c *NamedBarrierValueAfterFinish) Finish() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Remove nodes from waiting list
	updated := len(c.waitingFor) > 0
	c.waitingFor = make(map[string]bool)
	
	// If no more nodes to wait for, mark as finished
	if !updated && len(c.waitingFor) == 0 {
		c.finished = true
	}
	
	return updated
}

// Consume always returns false for this channel.
func (c *NamedBarrierValueAfterFinish) Consume() bool {
	return false
}

// LastValueAfterFinish stores the last value received, but only makes it available after finish().
type LastValueAfterFinish struct {
	BaseChannel
	value    interface{}
	finished bool
	mu        sync.RWMutex
}

// NewLastValueAfterFinish creates a new LastValueAfterFinish channel.
func NewLastValueAfterFinish(typ interface{}) *LastValueAfterFinish {
	return &LastValueAfterFinish{
		BaseChannel: BaseChannel{Typ: typ},
		value:      Missing,
		finished:    false,
	}
}

// Get returns the last value received after finish().
func (c *LastValueAfterFinish) Get() (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if !c.finished {
		return nil, &errors.EmptyChannelError{}
	}
	
	if IsMissing(c.value) {
		return nil, &errors.EmptyChannelError{}
	}
	
	return c.value, nil
}

// IsAvailable returns true if finished and has a value.
func (c *LastValueAfterFinish) IsAvailable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.finished && !IsMissing(c.value)
}

// Update updates the channel with new values.
func (c *LastValueAfterFinish) Update(values []interface{}) (bool, error) {
	if len(values) == 0 {
		return false, nil
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Only accept one value per step
	if len(values) > 1 {
		return false, &errors.InvalidUpdateError{
			Message: "Can receive only one value per step. Use a reducer to handle multiple values.",
		}
	}
	
	c.value = values[0]
	return true, nil
}

// Copy returns a copy of the channel.
func (c *LastValueAfterFinish) Copy() Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	newCh := NewLastValueAfterFinish(c.Typ)
	newCh.Key = c.Key
	newCh.value = c.value
	newCh.finished = c.finished
	return newCh
}

// Checkpoint returns the current value.
func (c *LastValueAfterFinish) Checkpoint() interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// FromCheckpoint restores the channel from a checkpoint.
func (c *LastValueAfterFinish) FromCheckpoint(checkpoint interface{}) Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	newCh := NewLastValueAfterFinish(c.Typ)
	newCh.Key = c.Key
	
	if !IsMissing(checkpoint) {
		newCh.value = checkpoint
	}
	
	return newCh
}

// Finish marks the channel as finished.
func (c *LastValueAfterFinish) Finish() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.finished {
		c.finished = true
		return true
	}
	return false
}

// Consume always returns false for this channel.
func (c *LastValueAfterFinish) Consume() bool {
	return false
}
