# LangGraph Go

[![Go Reference](https://pkg.go.dev/badge/github.com/langgraph-go/langgraph.svg)](https://pkg.go.dev/github.com/langgraph-go/langgraph)
[![Go Report Card](https://goreportcard.com/badge/github.com/langgraph-go/langgraph)](https://goreportcard.com/report/github.com/langgraph-go/langgraph)

LangGraph Go is a Go port of [LangGraph](https://github.com/langchain-ai/langgraph), a library for building stateful, multi-agent applications with LLMs. It provides a graph-based execution model that supports cycles, branching, persistence, and human-in-the-loop workflows.

## Features

- **Stateful Computation**: Nodes communicate by reading and writing to shared state channels
- **Cyclic Graphs**: Support for loops and recursion with configurable limits
- **Checkpointing**: Persistent state storage with in-memory and SQLite backends
- **Human-in-the-Loop**: Interrupt flows for human approval or input
- **Streaming**: Real-time output streaming during graph execution
- **Type-Safe**: Strong typing with Go generics support

## Installation

```bash
go get github.com/langgraph-go/langgraph
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/langgraph-go/langgraph"
)

// Define your state
type State struct {
    Messages []string
    Counter  int
}

func main() {
    ctx := context.Background()

    // Create a graph builder
    builder := langgraph.NewStateGraph(State{})

    // Add nodes
    builder.AddNode("agent", func(ctx context.Context, state interface{}) (interface{}, error) {
        s := state.(State)
        s.Messages = append(s.Messages, "Hello from agent")
        s.Counter++
        return s, nil
    })

    // Add edges
    builder.AddEdge(langgraph.Start, "agent")
    builder.AddEdge("agent", langgraph.End)

    // Compile the graph
    graph, err := builder.Compile()
    if err != nil {
        log.Fatal(err)
    }

    // Run the graph
    result, err := graph.Invoke(ctx, State{
        Messages: []string{"Starting..."},
        Counter:  0,
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Result: %+v\n", result)
}
```

## Core Concepts

### StateGraph

The `StateGraph` is the main class for building graphs. Nodes in the graph communicate by reading and writing to a shared state object.

```go
builder := langgraph.NewStateGraph(State{})
```

### Nodes

Nodes are functions that read the current state and return updates to the state.

```go
builder.AddNode("my_node", func(ctx context.Context, state interface{}) (interface{}, error) {
    s := state.(MyState)
    // Process state...
    return s, nil
})
```

### Edges

Edges define the flow between nodes.

```go
// Simple edge
builder.AddEdge("node_a", "node_b")

// Conditional edges
builder.AddConditionalEdges("decision", conditionFunc, map[string]string{
    "yes": "node_a",
    "no":  "node_b",
})
```

### Channels

Channels define how state is stored and updated. Different channel types support different update semantics:

- **LastValue**: Stores only the most recent value
- **Topic**: Accumulates values in a list
- **BinaryOperatorAggregate**: Reduces values using a binary operator
- **EphemeralValue**: Clears after being read once

```go
builder.AddChannel("messages", langgraph.NewTopic(string, true))
```

### Checkpoints

Checkpoints enable persistence and resumption of graph execution.

```go
// In-memory checkpointer
saver := langgraph.NewMemorySaver()

// SQLite checkpointer
saver, err := langgraph.NewSqliteSaver("checkpoints.db")

// Compile with checkpointer
graph, err := builder.Compile(langgraph.WithCheckpointer(saver))
```

## Examples

### Conditional Routing

```go
builder.AddConditionalEdges("router", func(ctx context.Context, state interface{}) (interface{}, error) {
    s := state.(MyState)
    if s.Value > threshold {
        return "high", nil
    }
    return "low", nil
}, map[string]string{
    "high": "high_value_node",
    "low":  "low_value_node",
})
```

### Retry Policy

```go
retryPolicy := langgraph.RetryPolicy{
    MaxAttempts:     3,
    InitialInterval: 500 * time.Millisecond,
    BackoffFactor:   2.0,
}

builder.AddNodeWithOptions("risky_node", nodeFunc, langgraph.NodeOptions{
    RetryPolicy: &retryPolicy,
})
```

### Interrupts (Human-in-the-Loop)

```go
// Enable interrupts for specific nodes
graph, err := builder.Compile(langgraph.WithInterrupts("human_review"))

// In your node, use interrupt
func humanReviewNode(ctx context.Context, state interface{}) (interface{}, error) {
    // This will pause execution and return to the client
    result, err := langgraph.Interrupt("Please review and approve")
    if err != nil {
        return nil, err
    }
    // Resume with the result
    return processResult(result), nil
}

// Resume with a command
result, err := graph.Invoke(ctx, langgraph.NewCommand().WithResume(approval), config)
```

## Project Structure

```
langgraph-go/
├── channels/      # Channel implementations for state management
├── checkpoint/    # Checkpoint savers (memory, sqlite)
├── constants/     # Constants and reserved keys
├── errors/        # Error types
├── examples/      # Example applications
├── graph/         # Graph building and execution
├── interrupt/     # Human-in-the-loop functionality
├── types/         # Core type definitions
└── utils/         # Utility functions
```

## Architecture

LangGraph Go follows the [Pregel](https://research.google.com/pubs/pub37252.html) execution model:

1. **Build Phase**: Define nodes, edges, and state channels
2. **Compile Phase**: Validate and prepare the graph for execution
3. **Execution Phase**: Run the Pregel loop:
   - Determine which nodes are ready to execute
   - Execute nodes in parallel (when possible)
   - Apply writes to channels
   - Check for interrupts
   - Repeat until complete or recursion limit reached

## Differences from Python LangGraph

While this Go implementation aims to be functionally equivalent to the Python version, there are some differences due to language constraints:

- **Type System**: Go uses interface{} for dynamic typing instead of Python's Any
- **Generics**: Limited use of Go generics where type safety is important
- **Reflection**: More reliance on reflection for struct-to-map conversions
- **Channels**: Similar semantics but different implementation details

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

This is a Go port of the original [LangGraph](https://github.com/langchain-ai/langgraph) library by LangChain AI.
