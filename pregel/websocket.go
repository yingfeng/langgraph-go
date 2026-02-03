// Package pregel provides WebSocket support for remote graph execution.
package pregel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/langgraph-go/langgraph/types"
)

// WebSocketPregelProtocol implements PregelProtocol over WebSocket.
// This provides bidirectional streaming communication for remote graph execution.
type WebSocketPregelProtocol struct {
	conn            *websocket.Conn
	url             string
	headers         http.Header
	mu              sync.RWMutex
	messageChan     chan *PregelMessage
	doneChan        chan struct{}
	isClosed        bool
	readBufferSize  int
	writeBufferSize int
	pingInterval    time.Duration
	pongTimeout     time.Duration
}

// WebSocketConfig configures WebSocket connection.
type WebSocketConfig struct {
	URL            string
	Headers        http.Header
	ReadBufferSize int
	WriteBufferSize int
	PingInterval   time.Duration
	PongTimeout    time.Duration
}

// NewWebSocketPregelProtocol creates a new WebSocket Pregel protocol.
func NewWebSocketPregelProtocol(config *WebSocketConfig) *WebSocketPregelProtocol {
	if config.ReadBufferSize == 0 {
		config.ReadBufferSize = 1024
	}
	if config.WriteBufferSize == 0 {
		config.WriteBufferSize = 1024
	}
	if config.PingInterval == 0 {
		config.PingInterval = 30 * time.Second
	}
	if config.PongTimeout == 0 {
		config.PongTimeout = 60 * time.Second
	}

	return &WebSocketPregelProtocol{
		url:             config.URL,
		headers:         config.Headers,
		messageChan:     make(chan *PregelMessage, 100),
		doneChan:        make(chan struct{}),
		isClosed:        false,
		readBufferSize:  config.ReadBufferSize,
		writeBufferSize: config.WriteBufferSize,
		pingInterval:    config.PingInterval,
		pongTimeout:     config.PongTimeout,
	}
}

// Connect establishes the WebSocket connection.
func (p *WebSocketPregelProtocol) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		ReadBufferSize:  p.readBufferSize,
		WriteBufferSize: p.writeBufferSize,
	}

	conn, _, err := dialer.DialContext(ctx, p.url, p.headers)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	p.conn = conn

	// Start background goroutines
	go p.readLoop()
	go p.pingLoop()

	return nil
}

// readLoop continuously reads messages from the WebSocket.
func (p *WebSocketPregelProtocol) readLoop() {
	defer close(p.messageChan)

	for {
		select {
		case <-p.doneChan:
			return
		default:
		}

		_, data, err := p.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Log unexpected close
			}
			return
		}

		var message PregelMessage
		if err := json.Unmarshal(data, &message); err != nil {
			// Log unmarshal error
			continue
		}

		select {
		case p.messageChan <- &message:
		case <-p.doneChan:
			return
		}
	}
}

// pingLoop sends periodic ping messages to keep the connection alive.
func (p *WebSocketPregelProtocol) pingLoop() {
	ticker := time.NewTicker(p.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(p.pongTimeout)); err != nil {
				// Ping failed, connection might be dead
				return
			}
		case <-p.doneChan:
			return
		}
	}
}

// Send sends a message to the remote peer.
func (p *WebSocketPregelProtocol) Send(ctx context.Context, message *PregelMessage) error {
	p.mu.RLock()
	if p.isClosed {
		p.mu.RUnlock()
		return fmt.Errorf("websocket connection is closed")
	}
	conn := p.conn
	p.mu.RUnlock()

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// Receive receives a message from the remote peer.
func (p *WebSocketPregelProtocol) Receive(ctx context.Context) (*PregelMessage, error) {
	select {
	case message := <-p.messageChan:
		return message, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.doneChan:
		return nil, fmt.Errorf("websocket connection closed")
	}
}

// Close closes the protocol connection.
func (p *WebSocketPregelProtocol) Close() error {
	p.mu.Lock()
	if p.isClosed {
		p.mu.Unlock()
		return nil
	}
	p.isClosed = true
	close(p.doneChan)
	p.mu.Unlock()

	if p.conn != nil {
		// Send close message
		p.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		// Close connection
		return p.conn.Close()
	}
	return nil
}

// IsConnected returns true if the WebSocket is connected.
func (p *WebSocketPregelProtocol) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.isClosed && p.conn != nil
}

// RemoteGraphClient provides a high-level client for remote graph execution over WebSocket.
type RemoteGraphClient struct {
	protocol    *WebSocketPregelProtocol
	streamModes []types.StreamMode
	mu          sync.RWMutex
	handlers    map[MessageType]func(*PregelMessage)
}

// NewRemoteGraphClient creates a new remote graph client.
func NewRemoteGraphClient(wsURL string, streamModes []types.StreamMode) *RemoteGraphClient {
	return &RemoteGraphClient{
		protocol: &WebSocketPregelProtocol{
			url:         wsURL,
			messageChan: make(chan *PregelMessage, 100),
			doneChan:    make(chan struct{}),
			isClosed:    false,
		},
		streamModes: streamModes,
		handlers:    make(map[MessageType]func(*PregelMessage)),
	}
}

// Connect establishes connection to the remote graph server.
func (c *RemoteGraphClient) Connect(ctx context.Context) error {
	return c.protocol.Connect(ctx)
}

// Invoke executes the graph remotely and returns the final result.
func (c *RemoteGraphClient) Invoke(ctx context.Context, input interface{}, config *types.RunnableConfig) (interface{}, error) {
	// Send execute request
	req := &PregelMessage{
		Type:      MessageTypeExecute,
		ID:        generateMessageID(),
		Input:     input,
		Metadata: map[string]interface{}{
			"config": config,
			"stream_modes": c.streamModes,
		},
		Timestamp: time.Now(),
	}

	if err := c.protocol.Send(ctx, req); err != nil {
		return nil, err
	}

	// Wait for response
	for {
		msg, err := c.protocol.Receive(ctx)
		if err != nil {
			return nil, err
		}

		switch msg.Type {
		case MessageTypeExecuteResponse:
			if msg.Error != "" {
				return nil, fmt.Errorf("remote execution error: %s", msg.Error)
			}
			return msg.Output, nil
		case MessageTypeCheckpoint:
			// Handle checkpoint if needed
		case MessageTypeStateUpdate:
			// Handle state update if needed
		case MessageTypeInterrupt:
			// Handle interrupt
			return nil, fmt.Errorf("execution interrupted: %v", msg.Metadata)
		}
	}
}

// Stream executes the graph remotely and streams results.
func (c *RemoteGraphClient) Stream(ctx context.Context, input interface{}, config *types.RunnableConfig) (<-chan *PregelMessage, error) {
	// Send execute request with streaming
	req := &PregelMessage{
		Type:      MessageTypeExecute,
		ID:        generateMessageID(),
		Input:     input,
		Metadata: map[string]interface{}{
			"config": config,
			"stream_modes": c.streamModes,
			"stream": true,
		},
		Timestamp: time.Now(),
	}

	if err := c.protocol.Send(ctx, req); err != nil {
		return nil, err
	}

	// Create output channel
	outputChan := make(chan *PregelMessage, 100)

	// Start goroutine to forward messages
	go func() {
		defer close(outputChan)
		for {
			msg, err := c.protocol.Receive(ctx)
			if err != nil {
				return
			}

			select {
			case outputChan <- msg:
			case <-ctx.Done():
				return
			}

			// Stop on final response
			if msg.Type == MessageTypeExecuteResponse {
				return
			}
		}
	}()

	return outputChan, nil
}

// OnMessage registers a handler for a specific message type.
func (c *RemoteGraphClient) OnMessage(msgType MessageType, handler func(*PregelMessage)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[msgType] = handler
}

// Close closes the client connection.
func (c *RemoteGraphClient) Close() error {
	return c.protocol.Close()
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// GetReadBufferSize returns the read buffer size.
func (p *WebSocketPregelProtocol) GetReadBufferSize() int {
	return p.readBufferSize
}

// GetWriteBufferSize returns the write buffer size.
func (p *WebSocketPregelProtocol) GetWriteBufferSize() int {
	return p.writeBufferSize
}

// GetPingInterval returns the ping interval.
func (p *WebSocketPregelProtocol) GetPingInterval() time.Duration {
	return p.pingInterval
}

// GetPongTimeout returns the pong timeout.
func (p *WebSocketPregelProtocol) GetPongTimeout() time.Duration {
	return p.pongTimeout
}
