// Package message provides message handling utilities for LangGraph Go.
package message

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// REMOVE_ALL_MESSAGES is a special marker to remove all messages.
const REMOVE_ALL_MESSAGES = "__remove_all__"

// Message represents a base message type.
type Message struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Content   interface{}            `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	ToolCalls []ToolCall            `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool call in a message.
type ToolCall struct {
	ID   string                 `json:"id"`
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// ToolMessage represents a tool response message.
type ToolMessage struct {
	Message
	ToolCallID string `json:"tool_call_id"`
}

// RemoveMessage represents a message to remove.
type RemoveMessage struct {
	ID string `json:"id"`
}

// AddMessages merges two lists of messages, updating existing messages by ID.
// This ensures state is "append-only", unless a new message has the same ID as an existing message.
func AddMessages(left, right []interface{}, format string) ([]interface{}, error) {
	// Convert to list if single messages
	if len(left) == 1 && !isList(left[0]) {
		left = []interface{}{left[0]}
	}
	if len(right) == 1 && !isList(right[0]) {
		right = []interface{}{right[0]}
	}

	// Convert all to Message objects
	leftMsgs, err := toMessages(left)
	if err != nil {
		return nil, fmt.Errorf("error converting left messages: %w", err)
	}
	rightMsgs, err := toMessages(right)
	if err != nil {
		return nil, fmt.Errorf("error converting right messages: %w", err)
	}

	// Assign missing IDs
	for i := range leftMsgs {
		if msg, ok := leftMsgs[i].(Message); ok && msg.ID == "" {
			msg.ID = uuid.New().String()
			leftMsgs[i] = msg
		}
	}
	for i := range rightMsgs {
		if msg, ok := rightMsgs[i].(Message); ok && msg.ID == "" {
			msg.ID = uuid.New().String()
			rightMsgs[i] = msg
		}
	}

	// Check for REMOVE_ALL_MESSAGES
	var removeAllIdx = -1
	for i, msg := range rightMsgs {
		if rm, ok := msg.(RemoveMessage); ok && rm.ID == REMOVE_ALL_MESSAGES {
			removeAllIdx = i
			break
		}
	}

	if removeAllIdx >= 0 {
		// Return messages after the REMOVE_ALL_MESSAGES marker
		result := make([]interface{}, 0)
		for i := removeAllIdx + 1; i < len(rightMsgs); i++ {
			result = append(result, rightMsgs[i])
		}
		return result, nil
	}

	// Merge messages
	merged := make([]interface{}, len(leftMsgs))
	copy(merged, leftMsgs)

	mergedByID := make(map[string]int)
	for i, msg := range merged {
		if m, ok := msg.(Message); ok {
			mergedByID[m.ID] = i
		}
	}

	idsToRemove := make(map[string]bool)
	for _, msg := range rightMsgs {
		if rm, ok := msg.(RemoveMessage); ok {
			if _, exists := mergedByID[rm.ID]; exists {
				idsToRemove[rm.ID] = true
			} else {
				return nil, fmt.Errorf("attempting to delete a message with an ID that doesn't exist: '%s'", rm.ID)
			}
		} else if m, ok := msg.(Message); ok {
			if idx, exists := mergedByID[m.ID]; exists {
				idsToRemove[m.ID] = false // Don't remove this ID
				merged[idx] = m // Replace existing message
			} else {
				mergedByID[m.ID] = len(merged)
				merged = append(merged, m)
			}
		}
	}

	// Filter out removed messages
	result := make([]interface{}, 0)
	for _, msg := range merged {
		if m, ok := msg.(Message); ok {
			if !idsToRemove[m.ID] {
				result = append(result, m)
			}
		} else {
			result = append(result, msg)
		}
	}

	// Apply format if specified
	if format == "langchain-openai" {
		return formatMessages(result)
	} else if format != "" {
		return nil, fmt.Errorf("unrecognized format '%s', expected one of 'langchain-openai'", format)
	}

	return result, nil
}

// PushMessage writes a message manually to the stream.
func PushMessage(message interface{}, stateKey string) error {
	// This would integrate with the streaming system
	// For now, we just validate the message
	msg, err := toSingleMessage(message)
	if err != nil {
		return err
	}
	// Try to get ID from different message types
	var id string
	switch m := msg.(type) {
	case Message:
		id = m.ID
	case ToolMessage:
		id = m.ID
	case RemoveMessage:
		id = m.ID
	default:
		return fmt.Errorf("unsupported message type: %T", msg)
	}
	if id == "" {
		return fmt.Errorf("message ID is required")
	}
	return nil
}

// toMessages converts a list of message-like objects to Message objects.
func toMessages(msgs []interface{}) ([]interface{}, error) {
	result := make([]interface{}, len(msgs))
	for i, msg := range msgs {
		m, err := toSingleMessage(msg)
		if err != nil {
			return nil, err
		}
		result[i] = m
	}
	return result, nil
}

// toSingleMessage converts a message-like object to a Message.
func toSingleMessage(msg interface{}) (interface{}, error) {
	if msg == nil {
		return nil, fmt.Errorf("cannot convert nil to message")
	}

	switch v := msg.(type) {
	case Message:
		return v, nil
	case RemoveMessage:
		return v, nil
	case ToolMessage:
		return v, nil
	case map[string]interface{}:
		return mapToMessage(v)
	case string:
		return Message{
			ID:      uuid.New().String(),
			Type:    "text",
			Content: v,
		}, nil
	default:
		// Try to reflect and convert
		rv := reflect.ValueOf(msg)
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Struct {
			return structToMessage(rv)
		}
		return nil, fmt.Errorf("unsupported message type: %T", msg)
	}
}

// mapToMessage converts a map to a Message.
func mapToMessage(m map[string]interface{}) (interface{}, error) {
	// Check for remove marker
	if id, ok := m["id"].(string); ok && id == REMOVE_ALL_MESSAGES {
		return RemoveMessage{ID: REMOVE_ALL_MESSAGES}, nil
	}

	// Check for tool message
	if toolCallID, ok := m["tool_call_id"].(string); ok {
		return ToolMessage{
			Message: Message{
				ID:       getString(m, "id", uuid.New().String()),
				Type:     "tool",
				Content:  m["content"],
				Metadata: getMap(m, "metadata"),
			},
			ToolCallID: toolCallID,
		}, nil
	}

	// Regular message
	msg := Message{
		ID:       getString(m, "id", uuid.New().String()),
		Type:     getString(m, "type", "text"),
		Content:  m["content"],
		Metadata: getMap(m, "metadata"),
	}

	// Parse tool calls if present
	if tc, ok := m["tool_calls"].([]interface{}); ok {
		msg.ToolCalls = make([]ToolCall, len(tc))
		for i, t := range tc {
			if tm, ok := t.(map[string]interface{}); ok {
				msg.ToolCalls[i] = ToolCall{
					ID:   getString(tm, "id", ""),
					Name: getString(tm, "name", ""),
					Args: getMap(tm, "args"),
				}
			}
		}
	}

	return msg, nil
}

// structToMessage converts a struct to a Message via reflection.
func structToMessage(rv reflect.Value) (interface{}, error) {
	msg := Message{
		ID:      uuid.New().String(),
		Type:    "text",
		Content: nil,
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		value := rv.Field(i).Interface()

		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				fieldName = parts[0]
			}
		}

		switch fieldName {
		case "id":
			if str, ok := value.(string); ok && str != "" {
				msg.ID = str
			}
		case "type":
			if str, ok := value.(string); ok {
				msg.Type = str
			}
		case "content":
			msg.Content = value
		case "metadata":
			if m, ok := value.(map[string]interface{}); ok {
				msg.Metadata = m
			}
		case "tool_calls":
			if tc, ok := value.([]ToolCall); ok {
				msg.ToolCalls = tc
			}
		case "tool_call_id":
			// Tool message
			if tm, ok := value.(string); ok {
				return ToolMessage{
					Message:    msg,
					ToolCallID: tm,
				}, nil
			}
		}
	}

	return msg, nil
}

// formatMessages formats messages according to a specific format.
func formatMessages(msgs []interface{}) ([]interface{}, error) {
	// For "langchain-openai" format, we would convert to OpenAI-compatible format
	// This is a simplified version
	result := make([]interface{}, len(msgs))
	for i, msg := range msgs {
		if m, ok := msg.(Message); ok {
			// Format the message content
			if content, ok := m.Content.([]interface{}); ok {
				// Convert to OpenAI format
				formatted := formatContent(content)
				m.Content = formatted
			}
			result[i] = m
		} else {
			result[i] = msg
		}
	}
	return result, nil
}

// formatContent formats message content to OpenAI format.
func formatContent(content []interface{}) []interface{} {
	result := make([]interface{}, 0, len(content))
	for _, c := range content {
		if block, ok := c.(map[string]interface{}); ok {
			blockType, _ := block["type"].(string)
			switch blockType {
			case "text":
				result = append(result, map[string]interface{}{
					"type": "text",
					"text": block["text"],
				})
			case "image_url":
				result = append(result, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": block["source"],
					},
				})
			default:
				result = append(result, c)
			}
		} else {
			result = append(result, c)
		}
	}
	return result
}

// isList checks if a value is a list/slice.
func isList(v interface{}) bool {
	if v == nil {
		return false
	}
	_, ok := v.([]interface{})
	return ok
}

// getString gets a string value from a map with a default.
func getString(m map[string]interface{}, key string, defaultValue string) string {
	if v, ok := m[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return defaultValue
}

// getMap gets a map value from a map with a default.
func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if mv, ok := v.(map[string]interface{}); ok {
			return mv
		}
	}
	return nil
}

// MessagesState represents a state with messages.
type MessagesState struct {
	messages []interface{}
	mu       sync.RWMutex
}

// NewMessagesState creates a new MessagesState.
func NewMessagesState() *MessagesState {
	return &MessagesState{
		messages: make([]interface{}, 0),
	}
}

// GetMessages returns the messages.
func (s *MessagesState) GetMessages() []interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.messages
}

// SetMessages sets the messages.
func (s *MessagesState) SetMessages(msgs []interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = msgs
}

// Add adds messages to the state.
func (s *MessagesState) Add(msgs []interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	merged, err := AddMessages(s.messages, msgs, "")
	if err != nil {
		return err
	}
	s.messages = merged
	return nil
}

// MarshalJSON implements json.Marshaler.
func (s *MessagesState) MarshalJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(s.messages)
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *MessagesState) UnmarshalJSON(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, &s.messages)
}
