// Package errors provides error types for LangGraph Go.
package errors

import (
	"fmt"
)

// ErrorCode represents specific error codes for LangGraph.
type ErrorCode string

const (
	// ErrorCodeGraphRecursionLimit is raised when the graph exhausts the maximum number of steps.
	ErrorCodeGraphRecursionLimit ErrorCode = "GRAPH_RECURSION_LIMIT"
	// ErrorCodeInvalidConcurrentGraphUpdate is raised for invalid concurrent graph updates.
	ErrorCodeInvalidConcurrentGraphUpdate ErrorCode = "INVALID_CONCURRENT_GRAPH_UPDATE"
	// ErrorCodeInvalidGraphNodeReturnValue is raised for invalid node return values.
	ErrorCodeInvalidGraphNodeReturnValue ErrorCode = "INVALID_GRAPH_NODE_RETURN_VALUE"
	// ErrorCodeMultipleSubgraphs is raised when multiple subgraphs are detected.
	ErrorCodeMultipleSubgraphs ErrorCode = "MULTIPLE_SUBGRAPHS"
	// ErrorCodeInvalidChatHistory is raised for invalid chat history.
	ErrorCodeInvalidChatHistory ErrorCode = "INVALID_CHAT_HISTORY"
)

// CreateErrorMessage creates an error message with troubleshooting information.
func CreateErrorMessage(message string, errorCode ErrorCode) string {
	return fmt.Sprintf(
		"%s\nFor troubleshooting, visit: https://docs.langchain.com/oss/python/langgraph/errors/%s",
		message,
		errorCode,
	)
}

// EmptyChannelError is raised when a channel is empty (never updated yet).
type EmptyChannelError struct {
	Message string
}

func (e *EmptyChannelError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "channel is empty"
}

// IsEmptyChannelError checks if an error is an EmptyChannelError.
func IsEmptyChannelError(err error) bool {
	_, ok := err.(*EmptyChannelError)
	return ok
}

// GraphRecursionError is raised when the graph has exhausted the maximum number of steps.
type GraphRecursionError struct {
	Limit int
}

func (e *GraphRecursionError) Error() string {
	return fmt.Sprintf(
		"Graph recursion limit of %d reached. To increase the limit, "+
			"run your graph with a config specifying a higher recursion_limit.",
		e.Limit,
	)
}

// IsGraphRecursionError checks if an error is a GraphRecursionError.
func IsGraphRecursionError(err error) bool {
	_, ok := err.(*GraphRecursionError)
	return ok
}

// InvalidUpdateError is raised when attempting to update a channel with an invalid set of updates.
type InvalidUpdateError struct {
	Message string
}

func (e *InvalidUpdateError) Error() string {
	return fmt.Sprintf("Invalid update: %s", e.Message)
}

// IsInvalidUpdateError checks if an error is an InvalidUpdateError.
func IsInvalidUpdateError(err error) bool {
	_, ok := err.(*InvalidUpdateError)
	return ok
}

// GraphBubbleUp is the base type for exceptions that bubble up from subgraphs.
type GraphBubbleUp struct {
	Message string
	Cause   error
}

func (e *GraphBubbleUp) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "graph bubble up"
}

func (e *GraphBubbleUp) Unwrap() error {
	return e.Cause
}

// GraphInterrupt is raised when a subgraph is interrupted.
type GraphInterrupt struct {
	Interrupts []interface{}
}

func (e *GraphInterrupt) Error() string {
	return fmt.Sprintf("graph interrupted with %d interrupt(s)", len(e.Interrupts))
}

// IsGraphInterrupt checks if an error is a GraphInterrupt.
func IsGraphInterrupt(err error) bool {
	_, ok := err.(*GraphInterrupt)
	return ok
}

// ParentCommand is raised when a command should be sent to the parent graph.
type ParentCommand struct {
	Command interface{}
}

func (e *ParentCommand) Error() string {
	return "parent command"
}

// IsParentCommand checks if an error is a ParentCommand.
func IsParentCommand(err error) bool {
	_, ok := err.(*ParentCommand)
	return ok
}

// EmptyInputError is raised when graph receives an empty input.
type EmptyInputError struct {
	Message string
}

func (e *EmptyInputError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "empty input"
}

// IsEmptyInputError checks if an error is an EmptyInputError.
func IsEmptyInputError(err error) bool {
	_, ok := err.(*EmptyInputError)
	return ok
}

// TaskNotFound is raised when the executor is unable to find a task.
type TaskNotFound struct {
	TaskID string
}

func (e *TaskNotFound) Error() string {
	return fmt.Sprintf("task not found: %s", e.TaskID)
}

// IsTaskNotFound checks if an error is a TaskNotFound.
func IsTaskNotFound(err error) bool {
	_, ok := err.(*TaskNotFound)
	return ok
}

// InvalidNodeError is raised when a node is invalid.
type InvalidNodeError struct {
	NodeName string
	Message  string
}

func (e *InvalidNodeError) Error() string {
	return fmt.Sprintf("invalid node '%s': %s", e.NodeName, e.Message)
}

// InvalidEdgeError is raised when an edge is invalid.
type InvalidEdgeError struct {
	From    string
	To      string
	Message string
}

func (e *InvalidEdgeError) Error() string {
	return fmt.Sprintf("invalid edge from '%s' to '%s': %s", e.From, e.To, e.Message)
}

// ChannelNotFoundError is raised when a channel is not found.
type ChannelNotFoundError struct {
	ChannelName string
}

func (e *ChannelNotFoundError) Error() string {
	return fmt.Sprintf("channel not found: %s", e.ChannelName)
}

// NodeNotFoundError is raised when a node is not found.
type NodeNotFoundError struct {
	NodeName string
}

func (e *NodeNotFoundError) Error() string {
	return fmt.Sprintf("node not found: %s", e.NodeName)
}
