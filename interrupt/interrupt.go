// Package interrupt provides interrupt functionality for LangGraph Go.
package interrupt

import (
	"context"
	"fmt"

	"github.com/langgraph-go/langgraph/errors"
	"github.com/langgraph-go/langgraph/types"
)

// Interrupt interrupts the graph with a resumable exception from within a node.
// The value is surfaced to the client and can be used to request input required to resume execution.
//
// In a given node, the first invocation of this function raises a GraphInterrupt
// exception, halting execution. The provided value is included with the exception
// and sent to the client executing the graph.
//
// A client resuming the graph must use the Command primitive to specify a value
// for the interrupt and continue execution.
// The graph resumes from the start of the node, re-executing all logic.
//
// If a node contains multiple interrupt calls, LangGraph matches resume values
// to interrupts based on their order in the node.
//
// To use an interrupt, you must enable a checkpointer, as the feature relies
// on persisting the graph state.
func Interrupt(value interface{}) (interface{}, error) {
	// Get the current context/config
	ctx := context.Background()
	_ = ctx
	
	// Get scratchpad and config
	// In the actual implementation, this would be retrieved from context
	
	// Check for resume values
	resumeValues := getResumeValues()
	idx := getInterruptIndex()
	
	if idx < len(resumeValues) {
		// Return the resume value
		incrementInterruptIndex()
		return resumeValues[idx], nil
	}
	
	// Check for current resume value
	v := getNullResume()
	if v != nil {
		appendResumeValue(v)
		return v, nil
	}
	
	// No resume value found, raise interrupt
	return nil, &errors.GraphInterrupt{
		Interrupts: []interface{}{
			&types.Interrupt{
				Value: value,
				ID:    generateInterruptID(value),
			},
		},
	}
}

// interruptContext holds the context for interrupts.
type interruptContext struct {
	resumeValues []interface{}
	index        int
}

var currentContext *interruptContext

func init() {
	currentContext = &interruptContext{
		resumeValues: make([]interface{}, 0),
		index:        0,
	}
}

// getResumeValues returns the current resume values.
func getResumeValues() []interface{} {
	if currentContext == nil {
		return nil
	}
	return currentContext.resumeValues
}

// getInterruptIndex returns the current interrupt index.
func getInterruptIndex() int {
	if currentContext == nil {
		return 0
	}
	return currentContext.index
}

// incrementInterruptIndex increments the interrupt index.
func incrementInterruptIndex() {
	if currentContext != nil {
		currentContext.index++
	}
}

// getNullResume checks for a null resume value.
func getNullResume() interface{} {
	// In actual implementation, this would check config
	return nil
}

// appendResumeValue appends a resume value.
func appendResumeValue(v interface{}) {
	if currentContext != nil {
		currentContext.resumeValues = append(currentContext.resumeValues, v)
	}
}

// generateInterruptID generates a unique ID for an interrupt.
func generateInterruptID(value interface{}) string {
	// In actual implementation, this would use a hash of the namespace
	return fmt.Sprintf("%v", value)
}

// Reset resets the interrupt context.
func Reset() {
	currentContext = &interruptContext{
		resumeValues: make([]interface{}, 0),
		index:        0,
	}
}

// SetResumeValues sets the resume values for testing.
func SetResumeValues(values []interface{}) {
	if currentContext == nil {
		Reset()
	}
	currentContext.resumeValues = values
}

// IsInterrupt checks if an error is a GraphInterrupt.
func IsInterrupt(err error) bool {
	return errors.IsGraphInterrupt(err)
}

// GetInterruptValue extracts the interrupt value from a GraphInterrupt error.
func GetInterruptValue(err error) (interface{}, bool) {
	if !errors.IsGraphInterrupt(err) {
		return nil, false
	}
	
	if gi, ok := err.(*errors.GraphInterrupt); ok && len(gi.Interrupts) > 0 {
		return gi.Interrupts[0], true
	}
	
	return nil, false
}
