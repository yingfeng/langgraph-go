// Package managed provides managed value types for runtime injection.
package managed

import (
	"fmt"
)

// ManagedValue represents a value that is managed by the runtime.
type ManagedValue interface {
	// Get returns the current value of the managed value.
	Get(scratchpad interface{}) (interface{}, error)
	// Copy creates a copy of this managed value.
	Copy() ManagedValue
}

// ManagedValueMapping is a collection of managed values keyed by name.
type ManagedValueMapping map[string]ManagedValue

// NewManagedValueMapping creates a new managed value mapping.
func NewManagedValueMapping() ManagedValueMapping {
	return make(ManagedValueMapping)
}

// Register registers a managed value.
func (m ManagedValueMapping) Register(name string, value ManagedValue) {
	m[name] = value
}

// Get gets a managed value by name.
func (m ManagedValueMapping) Get(name string) (ManagedValue, bool) {
	val, ok := m[name]
	return val, ok
}

// Contains checks if a managed value exists.
func (m ManagedValueMapping) Contains(name string) bool {
	_, ok := m[name]
	return ok
}

// Names returns all managed value names.
func (m ManagedValueMapping) Names() []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}

// IsLastStep provides information about whether the current step is the last step.
type IsLastStep struct {
	// Value indicates if this is the last step.
	Value bool
}

// NewIsLastStep creates a new IsLastStep managed value.
func NewIsLastStep() *IsLastStep {
	return &IsLastStep{
		Value: false,
	}
}

// Get returns the current value.
func (v *IsLastStep) Get(scratchpad interface{}) (interface{}, error) {
	if sd, ok := scratchpad.(map[string]interface{}); ok {
		if val, exists := sd["is_last_step"]; exists {
			if bl, ok := val.(bool); ok {
				v.Value = bl
			}
		}
	}
	return v.Value, nil
}

// Set sets the value.
func (v *IsLastStep) Set(value bool) {
	v.Value = value
}

// Copy creates a copy of this managed value.
func (v *IsLastStep) Copy() ManagedValue {
	return &IsLastStep{
		Value: v.Value,
	}
}

// PregelScratchpad provides temporary storage for graph execution.
type PregelScratchpad map[string]interface{}

// NewPregelScratchpad creates a new scratchpad.
func NewPregelScratchpad() PregelScratchpad {
	return make(PregelScratchpad)
}

// GetCallCounter returns the call counter.
func (p PregelScratchpad) GetCallCounter() int {
	if val, ok := p["call_counter"].(int); ok {
		return val
	}
	return 0
}

// IncrementCallCounter increments the call counter.
func (p PregelScratchpad) IncrementCallCounter() {
	p["call_counter"] = p.GetCallCounter() + 1
}

// SetCallCounter sets the call counter.
func (p PregelScratchpad) SetCallCounter(value int) {
	p["call_counter"] = value
}

// GetInterruptCounter returns the interrupt counter.
func (p PregelScratchpad) GetInterruptCounter() int {
	if val, ok := p["interrupt_counter"].(int); ok {
		return val
	}
	return 0
}

// IncrementInterruptCounter increments the interrupt counter.
func (p PregelScratchpad) IncrementInterruptCounter() {
	p["interrupt_counter"] = p.GetInterruptCounter() + 1
}

// GetSubgraphCounter returns the subgraph counter.
func (p PregelScratchpad) GetSubgraphCounter() int {
	if val, ok := p["subgraph_counter"].(int); ok {
		return val
	}
	return 0
}

// IncrementSubgraphCounter increments the subgraph counter.
func (p PregelScratchpad) IncrementSubgraphCounter() {
	p["subgraph_counter"] = p.GetSubgraphCounter() + 1
}

// Get returns a value from the scratchpad.
func (p PregelScratchpad) Get(key string) (interface{}, bool) {
	val, ok := p[key]
	return val, ok
}

// Set sets a value in the scratchpad.
func (p PregelScratchpad) Set(key string, value interface{}) {
	p[key] = value
}

// Delete removes a value from the scratchpad.
func (p PregelScratchpad) Delete(key string) {
	delete(p, key)
}

// Clear removes all values from the scratchpad.
func (p PregelScratchpad) Clear() {
	for k := range p {
		delete(p, k)
	}
}

// Clone creates a copy of the scratchpad.
func (p PregelScratchpad) Clone() PregelScratchpad {
	clone := make(PregelScratchpad, len(p))
	for k, v := range p {
		clone[k] = v
	}
	return clone
}

// ConfigKey represents keys used in runtime configuration.
const (
	ConfigKeyTaskID      = "__task_id__"
	ConfigKeyRuntime     = "__runtime__"
	ConfigKeyRead        = "__read__"
	ConfigKeySend        = "__send__"
	ConfigKeyWriter      = "__writer__"
	ConfigKeyStore       = "__store__"
	ConfigKeyPrevious    = "__previous__"
	ConfigKeyCheckpointNS = "__checkpoint_ns__"
	ConfigKeyConfigurable = "__configurable__"
)

// Runtime provides runtime information for graph execution.
type Runtime struct {
	// TaskID is the ID of the current task.
	TaskID string
	// NodeName is the name of the current node.
	NodeName string
	// Step is the current step number.
	Step int
	// Configurable is the configurable parameters.
	Configurable map[string]interface{}
	// CheckpointNS is the checkpoint namespace.
	CheckpointNS string
}

// NewRuntime creates a new runtime.
func NewRuntime() *Runtime {
	return &Runtime{
		TaskID:      "",
		NodeName:     "",
		Step:         0,
		Configurable:  make(map[string]interface{}),
		CheckpointNS: "",
	}
}

// Clone creates a copy of the runtime.
func (r *Runtime) Clone() *Runtime {
	return &Runtime{
		TaskID:      r.TaskID,
		NodeName:     r.NodeName,
		Step:         r.Step,
		Configurable:  cloneMap(r.Configurable),
		CheckpointNS: r.CheckpointNS,
	}
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	clone := make(map[string]interface{}, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

// GetTaskID returns the task ID from config.
func GetTaskID(config map[string]interface{}) string {
	if config == nil {
		return ""
	}
	if val, ok := config[ConfigKeyTaskID].(string); ok {
		return val
	}
	return ""
}

// GetRuntime returns the runtime from config.
func GetRuntime(config map[string]interface{}) *Runtime {
	if config == nil {
		return NewRuntime()
	}
	if val, ok := config[ConfigKeyRuntime].(*Runtime); ok {
		return val
	}
	return NewRuntime()
}

// SetRuntime sets the runtime in config.
func SetRuntime(config map[string]interface{}, runtime *Runtime) {
	if config == nil {
		return
	}
	config[ConfigKeyRuntime] = runtime
}

// GetReader returns the read function from config.
func GetReader(config map[string]interface{}) interface{} {
	if config == nil {
		return nil
	}
	return config[ConfigKeyRead]
}

// SetReader sets the read function in config.
func SetReader(config map[string]interface{}, reader interface{}) {
	if config == nil {
		return
	}
	config[ConfigKeyRead] = reader
}

// GetSend returns the send function from config.
func GetSend(config map[string]interface{}) func(...interface{}) {
	if config == nil {
		return nil
	}
	if val, ok := config[ConfigKeySend]; ok {
		if fn, ok := val.(func(...interface{})); ok {
			return fn
		}
	}
	return nil
}

// SetSend sets the send function in config.
func SetSend(config map[string]interface{}, send func(...interface{})) {
	if config == nil {
		return
	}
	config[ConfigKeySend] = send
}

// GetWriter returns the writer from config.
func GetWriter(config map[string]interface{}) interface{} {
	if config == nil {
		return nil
	}
	return config[ConfigKeyWriter]
}

// SetWriter sets the writer in config.
func SetWriter(config map[string]interface{}, writer interface{}) {
	if config == nil {
		return
	}
	config[ConfigKeyWriter] = writer
}





// PatchConfig patches a config with new values.
func PatchConfig(base map[string]interface{}, updates map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = make(map[string]interface{})
	}
	if updates == nil {
		return base
	}
	
	result := make(map[string]interface{}, len(base))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range updates {
		result[k] = v
	}
	return result
}

// PatchConfigurable patches the configurable section of a config.
func PatchConfigurable(base map[string]interface{}, updates map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = make(map[string]interface{})
	}
	if updates == nil {
		return base
	}
	
	// Get or create configurable section
	configurable := make(map[string]interface{})
	if cfg, ok := base[ConfigKeyConfigurable]; ok {
		if cfgMap, ok := cfg.(map[string]interface{}); ok {
			configurable = cfgMap
		}
	}
	
	// Merge updates
	for k, v := range updates {
		configurable[k] = v
	}
	
	// Update base config
	result := make(map[string]interface{}, len(base)+1)
	for k, v := range base {
		result[k] = v
	}
	result[ConfigKeyConfigurable] = configurable
	
	return result
}

// GetConfigurable returns the configurable section from config.
func GetConfigurable(config map[string]interface{}) map[string]interface{} {
	if config == nil {
		return nil
	}
	if val, ok := config[ConfigKeyConfigurable]; ok {
		if cfgMap, ok := val.(map[string]interface{}); ok {
			return cfgMap
		}
	}
	return nil
}

// GetCheckpointNS returns the checkpoint namespace from config.
func GetCheckpointNS(config map[string]interface{}) string {
	if config == nil {
		return ""
	}
	if val, ok := config[ConfigKeyCheckpointNS].(string); ok {
		return val
	}
	return ""
}

// SetCheckpointNS sets the checkpoint namespace in config.
func SetCheckpointNS(config map[string]interface{}, ns string) {
	if config == nil {
		return
	}
	config[ConfigKeyCheckpointNS] = ns
}

// ParseCheckpointNS parses a checkpoint namespace to extract node path.
func ParseCheckpointNS(ns string) []string {
	if ns == "" {
		return []string{}
	}
	return splitCheckpointNS(ns)
}

// RecastCheckpointNS recasts a checkpoint namespace by removing task ID.
func RecastCheckpointNS(ns string) string {
	parts := splitCheckpointNS(ns)
	if len(parts) == 0 {
		return ""
	}
	
	// Remove task ID if present (usually the last part)
	lastPart := parts[len(parts)-1]
	if isTaskID(lastPart) {
		return joinCheckpointNS(parts[:len(parts)-1])
	}
	
	return ns
}

func splitCheckpointNS(ns string) []string {
	// Simple implementation - split by separator
	// In a full implementation, this would handle nested namespaces
	return []string{ns}
}

func joinCheckpointNS(parts []string) string {
	// Simple implementation - join with separator
	// In a full implementation, this would use proper namespace separator
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "|" + parts[i]
	}
	return result
}

func isTaskID(s string) bool {
	// Simple check - task IDs are usually UUIDs
	// In a full implementation, this would be more sophisticated
	return len(s) > 20 && (containsDash(s) || containsUnderscore(s))
}

func containsDash(s string) bool {
	for _, c := range s {
		if c == '-' {
			return true
		}
	}
	return false
}

func containsUnderscore(s string) bool {
	for _, c := range s {
		if c == '_' {
			return true
		}
	}
	return false
}

// StreamWriter is a function that writes to the output stream.
type StreamWriter func(interface{})

// NewStreamWriter creates a new stream writer.
func NewStreamWriter(fn func(interface{})) StreamWriter {
	return StreamWriter(fn)
}

// Write writes a value to the stream.
func (w StreamWriter) Write(value interface{}) {
	w(value)
}

// FormatCheckpoint formats a checkpoint for debug output.
func FormatCheckpoint(checkpoint map[string]interface{}) string {
	if checkpoint == nil {
		return "{}"
	}
	
	result := "{"
	first := true
	for k, v := range checkpoint {
		if !first {
			result += ", "
		}
		result += fmt.Sprintf("\"%s\": %v", k, v)
		first = false
	}
	result += "}"
	return result
}

// FormatTask formats a task for debug output.
func FormatTask(task interface{}) string {
	if task == nil {
		return "nil"
	}
	return fmt.Sprintf("%v", task)
}

// FormatValue formats a value for debug output.
func FormatValue(value interface{}) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprintf("%v", value)
}

// FormatDuration formats a duration for debug output.
func FormatDuration(d int64) string {
	if d < 1000 {
		return fmt.Sprintf("%dms", d)
	} else if d < 60000 {
		return fmt.Sprintf("%.1fs", float64(d)/1000)
	} else if d < 3600000 {
		return fmt.Sprintf("%.1fm", float64(d)/60000)
	} else {
		return fmt.Sprintf("%.1fh", float64(d)/3600000)
	}
}
