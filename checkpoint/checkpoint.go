// Package checkpoint provides production-grade checkpoint management.
package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CheckpointMetadata contains metadata about a checkpoint.
type CheckpointMetadata struct {
	// ID is the unique identifier for this checkpoint
	ID string
	// ParentID is the ID of the parent checkpoint
	ParentID string
	// ThreadID is the thread this checkpoint belongs to
	ThreadID string
	// Step is the step number when this checkpoint was created
	Step int
	// CreatedAt is the timestamp when this checkpoint was created
	CreatedAt time.Time
	// Source indicates where this checkpoint came from
	Source CheckpointSource
	// Custom metadata
	Metadata map[string]interface{}
}

// CheckpointSource indicates the source of a checkpoint.
type CheckpointSource string

const (
	SourceNode     CheckpointSource = "node"     // Created after node execution
	SourceEdge     CheckpointSource = "edge"     // Created after edge traversal
	SourceInterrupt CheckpointSource = "interrupt" // Created on interrupt
	SourceManual   CheckpointSource = "manual"   // Manually created
	SourceResume   CheckpointSource = "resume"   // Created on resume
)

// PendingWrite represents a write that hasn't been applied yet.
type PendingWrite struct {
	// Channel is the channel to write to
	Channel string
	// Value is the value to write
	Value interface{}
	// Overwrite indicates if this should bypass reducers
	Overwrite bool
	// Node that initiated this write
	Node string
}

// Checkpoint represents a complete checkpoint with versioning.
type Checkpoint struct {
	// ID is the unique identifier
	ID string
	// Version is the checkpoint version (monotonically increasing)
	Version int
	// ParentID is the ID of the parent checkpoint (for lineage tracking)
	ParentID string
	// ChannelVersions tracks versions of each channel
	ChannelVersions map[string]int
	// VersionsSeen tracks which channel versions each node has seen
	VersionsSeen map[string]map[string]int
	// State is the current state
	State map[string]interface{}
	// PendingWrites are writes that haven't been applied yet
	PendingWrites []PendingWrite
	// Metadata about this checkpoint
	Metadata CheckpointMetadata
}

// NewCheckpoint creates a new checkpoint.
func NewCheckpoint(threadID string, step int) *Checkpoint {
	id := uuid.New().String()
	return &Checkpoint{
		ID:              id,
		Version:         0,
		ChannelVersions:  make(map[string]int),
		VersionsSeen:    make(map[string]map[string]int),
		State:           make(map[string]interface{}),
		PendingWrites:   make([]PendingWrite, 0),
		Metadata: CheckpointMetadata{
			ID:        id,
			ThreadID:  threadID,
			Step:      step,
			CreatedAt: time.Now(),
			Source:    SourceNode,
			Metadata:  make(map[string]interface{}),
		},
	}
}

// Clone creates a deep copy of the checkpoint.
func (c *Checkpoint) Clone() *Checkpoint {
	clone := &Checkpoint{
		ID:             uuid.New().String(),
		Version:        c.Version,
		ParentID:       c.ID,
		ChannelVersions: make(map[string]int, len(c.ChannelVersions)),
		VersionsSeen:   make(map[string]map[string]int, len(c.VersionsSeen)),
		State:          make(map[string]interface{}, len(c.State)),
		PendingWrites:  make([]PendingWrite, len(c.PendingWrites)),
		Metadata: CheckpointMetadata{
			ID:        uuid.New().String(),
			ParentID:  c.ID,
			ThreadID:  c.Metadata.ThreadID,
			Step:      c.Metadata.Step,
			CreatedAt: time.Now(),
			Source:    c.Metadata.Source,
			Metadata:  make(map[string]interface{}),
		},
	}
	
	// Copy channel versions
	for k, v := range c.ChannelVersions {
		clone.ChannelVersions[k] = v
	}
	
	// Copy versions seen
	for node, versions := range c.VersionsSeen {
		clone.VersionsSeen[node] = make(map[string]int)
		for k, v := range versions {
			clone.VersionsSeen[node][k] = v
		}
	}
	
	// Copy state
	for k, v := range c.State {
		clone.State[k] = deepCopy(v)
	}
	
	// Copy pending writes
	copy(clone.PendingWrites, c.PendingWrites)
	
	// Copy metadata
	for k, v := range c.Metadata.Metadata {
		clone.Metadata.Metadata[k] = v
	}
	
	return clone
}

// IncrementChannel increments the version of a channel.
func (c *Checkpoint) IncrementChannel(channel string) {
	c.ChannelVersions[channel]++
}

// MarkSeen marks that a node has seen a channel's current version.
func (c *Checkpoint) MarkSeen(node, channel string) {
	if _, ok := c.VersionsSeen[node]; !ok {
		c.VersionsSeen[node] = make(map[string]int)
	}
	c.VersionsSeen[node][channel] = c.ChannelVersions[channel]
}

// HasSeen checks if a node has seen a channel's version.
func (c *Checkpoint) HasSeen(node, channel string) bool {
	if versions, ok := c.VersionsSeen[node]; ok {
		if version, ok := versions[channel]; ok {
			return version == c.ChannelVersions[channel]
		}
	}
	return false
}

// AddPendingWrite adds a pending write.
func (c *Checkpoint) AddPendingWrite(channel string, value interface{}, overwrite bool, node string) {
	c.PendingWrites = append(c.PendingWrites, PendingWrite{
		Channel:   channel,
		Value:     value,
		Overwrite: overwrite,
		Node:      node,
	})
}

// ClearPendingWrites clears all pending writes.
func (c *Checkpoint) ClearPendingWrites() {
	c.PendingWrites = make([]PendingWrite, 0)
}

// ToMap converts the checkpoint to a map for storage.
func (c *Checkpoint) ToMap() map[string]interface{} {
	result := make(map[string]interface{})
	
	result["id"] = c.ID
	result["version"] = c.Version
	result["parent_id"] = c.ParentID
	
	channelVersions := make(map[string]int)
	for k, v := range c.ChannelVersions {
		channelVersions[k] = v
	}
	result["channel_versions"] = channelVersions
	
	versionsSeen := make(map[string]map[string]int)
	for node, versions := range c.VersionsSeen {
		versionsSeen[node] = make(map[string]int)
		for k, v := range versions {
			versionsSeen[node][k] = v
		}
	}
	result["versions_seen"] = versionsSeen
	
	state := make(map[string]interface{})
	for k, v := range c.State {
		state[k] = deepCopy(v)
	}
	result["state"] = state
	
	pendingWrites := make([]map[string]interface{}, len(c.PendingWrites))
	for i, pw := range c.PendingWrites {
		pendingWrites[i] = map[string]interface{}{
			"channel":   pw.Channel,
			"value":     pw.Value,
			"overwrite": pw.Overwrite,
			"node":      pw.Node,
		}
	}
	result["pending_writes"] = pendingWrites
	
	metadata := map[string]interface{}{
		"id":         c.Metadata.ID,
		"parent_id":  c.Metadata.ParentID,
		"thread_id":  c.Metadata.ThreadID,
		"step":       c.Metadata.Step,
		"created_at": c.Metadata.CreatedAt.Format(time.RFC3339Nano),
		"source":     string(c.Metadata.Source),
	}
	for k, v := range c.Metadata.Metadata {
		metadata[k] = v
	}
	result["metadata"] = metadata
	
	return result
}

// FromMap creates a checkpoint from a map.
func FromMap(data map[string]interface{}) (*Checkpoint, error) {
	c := &Checkpoint{
		ID:              getString(data, "id"),
		ParentID:        getString(data, "parent_id"),
		Version:         getInt(data, "version", 0),
		ChannelVersions:  make(map[string]int),
		VersionsSeen:    make(map[string]map[string]int),
		State:           make(map[string]interface{}),
		PendingWrites:   make([]PendingWrite, 0),
	}
	
	// Parse channel versions
	if cv, ok := data["channel_versions"].(map[string]interface{}); ok {
		for k, v := range cv {
			if num, ok := v.(float64); ok {
				c.ChannelVersions[k] = int(num)
			}
		}
	}
	
	// Parse versions seen
	if vs, ok := data["versions_seen"].(map[string]interface{}); ok {
		for node, versions := range vs {
			if vMap, ok := versions.(map[string]interface{}); ok {
				c.VersionsSeen[node] = make(map[string]int)
				for k, v := range vMap {
					if num, ok := v.(float64); ok {
						c.VersionsSeen[node][k] = int(num)
					}
				}
			}
		}
	}
	
	// Parse state
	if state, ok := data["state"].(map[string]interface{}); ok {
		for k, v := range state {
			c.State[k] = deepCopy(v)
		}
	}
	
	// Parse pending writes
	if pws, ok := data["pending_writes"].([]interface{}); ok {
		for _, pw := range pws {
			if pwMap, ok := pw.(map[string]interface{}); ok {
				c.PendingWrites = append(c.PendingWrites, PendingWrite{
					Channel:   getString(pwMap, "channel"),
					Overwrite: getBool(pwMap, "overwrite", false),
					Node:      getString(pwMap, "node"),
					Value:     pwMap["value"],
				})
			}
		}
	}
	
	// Parse metadata
	if md, ok := data["metadata"].(map[string]interface{}); ok {
		c.Metadata = CheckpointMetadata{
			ID:        getString(md, "id"),
			ParentID:  getString(md, "parent_id"),
			ThreadID:  getString(md, "thread_id"),
			Step:      getInt(md, "step", 0),
			Source:    CheckpointSource(getString(md, "source")),
			Metadata:  make(map[string]interface{}),
		}
		
		if createdAtStr := getString(md, "created_at"); createdAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
				c.Metadata.CreatedAt = t
			}
		}
		
		// Copy custom metadata
		for k, v := range md {
			if k != "id" && k != "parent_id" && k != "thread_id" && k != "step" && k != "created_at" && k != "source" {
				c.Metadata.Metadata[k] = v
			}
		}
	}
	
	return c, nil
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}

func getBool(m map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultVal
}

// CheckpointManager manages checkpoints with versioning.
type CheckpointManager struct {
	mu          sync.RWMutex
	checkpoints map[string][]*Checkpoint // threadID -> checkpoints
	maxVersions int // Maximum versions to keep per thread
}

// NewCheckpointManager creates a new checkpoint manager.
func NewCheckpointManager(maxVersions int) *CheckpointManager {
	if maxVersions <= 0 {
		maxVersions = 100 // Default max versions
	}
	
	return &CheckpointManager{
		checkpoints: make(map[string][]*Checkpoint),
		maxVersions: maxVersions,
	}
}

// Save saves a checkpoint.
func (cm *CheckpointManager) Save(ctx context.Context, checkpoint *Checkpoint) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	threadID := checkpoint.Metadata.ThreadID
	
	// Append to thread's checkpoint history
	cm.checkpoints[threadID] = append(cm.checkpoints[threadID], checkpoint)
	
	// Trim if we have too many versions
	if len(cm.checkpoints[threadID]) > cm.maxVersions {
		cm.checkpoints[threadID] = cm.checkpoints[threadID][len(cm.checkpoints[threadID])-cm.maxVersions:]
	}
	
	return nil
}

// Load loads the latest checkpoint for a thread.
func (cm *CheckpointManager) Load(ctx context.Context, threadID string) (*Checkpoint, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	checkpoints := cm.checkpoints[threadID]
	if len(checkpoints) == 0 {
		return nil, nil
	}
	
	return checkpoints[len(checkpoints)-1].Clone(), nil
}

// LoadByCheckpointID loads a specific checkpoint by ID.
func (cm *CheckpointManager) LoadByCheckpointID(ctx context.Context, checkpointID string) (*Checkpoint, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	for _, checkpoints := range cm.checkpoints {
		for _, cp := range checkpoints {
			if cp.ID == checkpointID {
				return cp.Clone(), nil
			}
		}
	}
	
	return nil, fmt.Errorf("checkpoint not found: %s", checkpointID)
}

// List lists checkpoints for a thread.
func (cm *CheckpointManager) List(ctx context.Context, threadID string, limit int) ([]*Checkpoint, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	checkpoints := cm.checkpoints[threadID]
	if len(checkpoints) == 0 {
		return nil, nil
	}
	
	if limit <= 0 || limit > len(checkpoints) {
		limit = len(checkpoints)
	}
	
	// Return most recent checkpoints first
	result := make([]*Checkpoint, limit)
	start := len(checkpoints) - limit
	for i := 0; i < limit; i++ {
		result[i] = checkpoints[start+i].Clone()
	}
	
	return result, nil
}

// GetVersion gets a specific version of a checkpoint.
func (cm *CheckpointManager) GetVersion(ctx context.Context, threadID string, version int) (*Checkpoint, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	checkpoints := cm.checkpoints[threadID]
	for _, cp := range checkpoints {
		if cp.Version == version {
			return cp.Clone(), nil
		}
	}
	
	return nil, fmt.Errorf("version not found: %d", version)
}

// Delete deletes a checkpoint.
func (cm *CheckpointManager) Delete(ctx context.Context, checkpointID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for threadID, checkpoints := range cm.checkpoints {
		for i, cp := range checkpoints {
			if cp.ID == checkpointID {
				// Remove from slice
				cm.checkpoints[threadID] = append(checkpoints[:i], checkpoints[i+1:]...)
				return nil
			}
		}
	}
	
	return fmt.Errorf("checkpoint not found: %s", checkpointID)
}

// ClearThread clears all checkpoints for a thread.
func (cm *CheckpointManager) ClearThread(ctx context.Context, threadID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	delete(cm.checkpoints, threadID)
	return nil
}

// ClearAll clears all checkpoints.
func (cm *CheckpointManager) ClearAll(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	cm.checkpoints = make(map[string][]*Checkpoint)
	return nil
}

// deepCopy creates a deep copy of a value using JSON serialization.
func deepCopy(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	
	// Handle map[string]interface{}
	if m, ok := v.(map[string]interface{}); ok {
		return deepCopyMap(m)
	}
	
	// Handle []interface{}
	if s, ok := v.([]interface{}); ok {
		return deepCopySlice(s)
	}
	
	// For other types, try JSON marshaling
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return v
	}
	
	return result
}

// deepCopyMap creates a deep copy of a map.
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = deepCopy(v)
	}
	return result
}

// deepCopySlice creates a deep copy of a slice.
func deepCopySlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = deepCopy(v)
	}
	return result
}
