// Package pregel provides WebSocket-based remote execution support for Pregel.
package pregel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/langgraph-go/langgraph/types"
)

// WebSocketPregelProtocol implements PregelProtocol over WebSocket.
type WebSocketPregelProtocol struct {
	conn           *websocket.Conn
	mu             sync.Mutex
	sendCh         chan *PregelMessage
	receiveCh       chan *PregelMessage
	closeCh         chan struct{}
	url             string
	headers         map[string]string
	pingInterval    time.Duration
	reconnectDelay   time.Duration
	maxReconnects   int
}

// WebSocketConfig configures a WebSocket Pregel protocol.
type WebSocketConfig struct {
	URL           string
	Headers       map[string]string
	PingInterval  time.Duration
	ReconnectDelay time.Duration
	MaxReconnects int
}

// NewWebSocketPregelProtocol creates a new WebSocket Pregel protocol.
func NewWebSocketPregelProtocol(config *WebSocketConfig) (*WebSocketPregelProtocol, error) {
	if config.PingInterval == 0 {
		config.PingInterval = 30 * time.Second
	}
	if config.ReconnectDelay == 0 {
		config.ReconnectDelay = 1 * time.Second
	}
	if config.MaxReconnects == 0 {
		config.MaxReconnects = 5
	}

	// Connect to WebSocket server
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	headers := make(map[string][]string)
	for k, v := range config.Headers {
		headers[k] = []string{v}
	}

	conn, _, err := dialer.Dial(config.URL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	proto := &WebSocketPregelProtocol{
		conn:         conn,
		sendCh:       make(chan *PregelMessage, 256),
		receiveCh:     make(chan *PregelMessage, 256),
		closeCh:       make(chan struct{}),
		url:          config.URL,
		headers:      config.Headers,
		pingInterval: config.PingInterval,
		reconnectDelay: config.ReconnectDelay,
		maxReconnects: config.MaxReconnects,
	}

	// Start send/receive goroutines
	go proto.sendLoop()
	go proto.receiveLoop()
	go proto.pingLoop()

	return proto, nil
}

// Send sends a message to the remote peer.
func (p *WebSocketPregelProtocol) Send(ctx context.Context, message *PregelMessage) error {
	message.Timestamp = time.Now()
	
	select {
	case p.sendCh <- message:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("send cancelled: %w", ctx.Err())
	case <-p.closeCh:
		return fmt.Errorf("connection closed")
	}
}

// Receive receives a message from the remote peer.
func (p *WebSocketPregelProtocol) Receive(ctx context.Context) (*PregelMessage, error) {
	select {
	case msg := <-p.receiveCh:
		return msg, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("receive cancelled: %w", ctx.Err())
	case <-p.closeCh:
		return nil, fmt.Errorf("connection closed")
	}
}

// Stream receives a stream of messages from the remote peer.
func (p *WebSocketPregelProtocol) Stream(ctx context.Context) <-chan *PregelMessage {
	streamCh := make(chan *PregelMessage)
	
	go func() {
		defer close(streamCh)
		for {
			select {
			case msg := <-p.receiveCh:
				streamCh <- msg
			case <-ctx.Done():
				return
			case <-p.closeCh:
				return
			}
		}
	}()
	
	return streamCh
}

// Close closes the WebSocket connection.
func (p *WebSocketPregelProtocol) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	select {
	case <-p.closeCh:
		// Already closed
		return nil
	default:
		close(p.closeCh)
		close(p.sendCh)
		close(p.receiveCh)
		return p.conn.Close()
	}
}

// sendLoop handles sending messages to the WebSocket.
func (p *WebSocketPregelProtocol) sendLoop() {
	defer p.Close()
	
	for {
		select {
		case msg, ok := <-p.sendCh:
			if !ok {
				return
			}
			
			data, err := json.Marshal(msg)
			if err != nil {
				fmt.Printf("failed to marshal message: %v\n", err)
				continue
			}
			
			p.mu.Lock()
			err = p.conn.WriteMessage(websocket.TextMessage, data)
			p.mu.Unlock()
			
			if err != nil {
				fmt.Printf("failed to send message: %v\n", err)
				p.reconnect()
			}
			
		case <-p.closeCh:
			return
		}
	}
}

// receiveLoop handles receiving messages from the WebSocket.
func (p *WebSocketPregelProtocol) receiveLoop() {
	defer p.Close()
	
	for {
		p.mu.Lock()
		messageType, data, err := p.conn.ReadMessage()
		p.mu.Unlock()
		
		if err != nil {
			if websocket.IsCloseError(err) || websocket.IsUnexpectedCloseError(err) {
				fmt.Printf("WebSocket connection closed: %v\n", err)
				p.reconnect()
				continue
			}
			fmt.Printf("failed to read message: %v\n", err)
			return
		}
		
		if messageType != websocket.TextMessage {
			fmt.Printf("unexpected message type: %d\n", messageType)
			continue
		}
		
		var msg PregelMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			fmt.Printf("failed to unmarshal message: %v\n", err)
			continue
		}
		
		select {
		case p.receiveCh <- &msg:
		case <-p.closeCh:
			return
		}
	}
}

// pingLoop sends periodic ping messages.
func (p *WebSocketPregelProtocol) pingLoop() {
	ticker := time.NewTicker(p.pingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pingMsg := &PregelMessage{
				Type:      MessageTypePing,
				ID:        generateMessageID(),
				Timestamp: time.Now(),
			}
			
			if err := p.Send(context.Background(), pingMsg); err != nil {
				fmt.Printf("failed to send ping: %v\n", err)
			}
			
		case <-p.closeCh:
			return
		}
	}
}

// reconnect attempts to reconnect to the WebSocket server.
func (p *WebSocketPregelProtocol) reconnect() {
	for i := 0; i < p.maxReconnects; i++ {
		time.Sleep(p.reconnectDelay)
		
		dialer := websocket.DefaultDialer
		dialer.HandshakeTimeout = 10 * time.Second
		
		headers := make(map[string][]string)
		for k, v := range p.headers {
			headers[k] = []string{v}
		}
		
		conn, _, err := dialer.Dial(p.url, headers)
		if err != nil {
			fmt.Printf("reconnect attempt %d failed: %v\n", i+1, err)
			continue
		}
		
		p.mu.Lock()
		p.conn = conn
		p.mu.Unlock()
		
		fmt.Printf("reconnected to WebSocket server\n")
		return
	}
	
	fmt.Printf("max reconnection attempts reached, closing connection\n")
	p.Close()
}

// StreamingRemoteRunnable executes nodes remotely via WebSocket with streaming support.
type StreamingRemoteRunnable struct {
	protocol PregelProtocol
	nodeName string
}

// NewStreamingRemoteRunnable creates a new streaming remote runnable.
func NewStreamingRemoteRunnable(protocol PregelProtocol, nodeName string) *StreamingRemoteRunnable {
	return &StreamingRemoteRunnable{
		protocol: protocol,
		nodeName: nodeName,
	}
}

// Execute executes a node remotely (non-streaming).
func (r *StreamingRemoteRunnable) Execute(ctx context.Context, input interface{}, config *types.RunnableConfig) (interface{}, error) {
	req := &PregelMessage{
		Type:      MessageTypeExecute,
		ID:        generateMessageID(),
		NodeName:  r.nodeName,
		Input:     input,
		Metadata:  map[string]interface{}{},
		Timestamp: time.Now(),
	}
	
	if config != nil {
		req.Metadata["config"] = config
	}
	
	if err := r.protocol.Send(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to send execution request: %w", err)
	}
	
	// Wait for response
	for {
		resp, err := r.protocol.Receive(ctx)
		if err != nil {
			return nil, err
		}
		
		if resp.ID != req.ID {
			// Not our response, continue waiting
			continue
		}
		
		switch resp.Type {
		case MessageTypeExecuteResponse:
			if resp.Error != "" {
				return nil, fmt.Errorf("remote execution error: %s", resp.Error)
			}
			return resp.Output, nil
			
		case MessageTypeInterrupt:
			// Handle interrupt
			return nil, fmt.Errorf("execution interrupted: %s", resp.Error)
			
		default:
			continue
		}
	}
}

// StreamExecute executes a node remotely with streaming output.
func (r *StreamingRemoteRunnable) StreamExecute(ctx context.Context, input interface{}, config *types.RunnableConfig) (<-chan interface{}, error) {
	outputCh := make(chan interface{}, 100)
	
	req := &PregelMessage{
		Type:      MessageTypeExecute,
		ID:        generateMessageID(),
		NodeName:  r.nodeName,
		Input:     input,
		Metadata:  map[string]interface{}{},
		Timestamp: time.Now(),
	}
	
	if config != nil {
		req.Metadata["config"] = config
		req.Metadata["stream"] = true
	}
	
	if err := r.protocol.Send(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to send execution request: %w", err)
	}
	
	// Start receiving stream
	go func() {
		defer close(outputCh)
		
		streamCh := r.protocol.(*WebSocketPregelProtocol).Stream(ctx)
		for {
			select {
			case msg := <-streamCh:
				if msg.ID != req.ID {
					continue
				}
				
				switch msg.Type {
				case MessageTypeStateUpdate:
					// Streaming output
					outputCh <- msg.Output
					
				case MessageTypeExecuteResponse:
					if msg.Error != "" {
						outputCh <- fmt.Errorf("remote execution error: %s", msg.Error)
					} else {
						outputCh <- msg.Output
					}
					return
					
				case MessageTypeInterrupt:
					outputCh <- fmt.Errorf("execution interrupted: %s", msg.Error)
					return
				}
				
			case <-ctx.Done():
				return
			}
		}
	}()
	
	return outputCh, nil
}

// CheckpointMigrator handles checkpoint migration between local and remote graphs.
type CheckpointMigrator struct {
	localProtocol  PregelProtocol
	remoteProtocol PregelProtocol
}

// NewCheckpointMigrator creates a new checkpoint migrator.
func NewCheckpointMigrator(local, remote PregelProtocol) *CheckpointMigrator {
	return &CheckpointMigrator{
		localProtocol:  local,
		remoteProtocol: remote,
	}
}

// MigrateCheckpoint migrates a checkpoint from local to remote.
func (m *CheckpointMigrator) MigrateCheckpoint(ctx context.Context, checkpointID string) error {
	// Request checkpoint from local
	req := &PregelMessage{
		Type:      MessageTypeCheckpoint,
		ID:        generateMessageID(),
		Metadata: map[string]interface{}{
			"checkpoint_id": checkpointID,
			"action":       "get",
		},
		Timestamp: time.Now(),
	}
	
	if err := m.localProtocol.Send(ctx, req); err != nil {
		return fmt.Errorf("failed to request checkpoint: %w", err)
	}
	
	resp, err := m.localProtocol.Receive(ctx)
	if err != nil {
		return err
	}
	
	// Send checkpoint to remote
	migrateReq := &PregelMessage{
		Type:      MessageTypeCheckpoint,
		ID:        generateMessageID(),
		Output:    resp.Output,
		Metadata: map[string]interface{}{
			"checkpoint_id": checkpointID,
			"action":       "migrate",
		},
		Timestamp: time.Now(),
	}
	
	if err := m.remoteProtocol.Send(ctx, migrateReq); err != nil {
		return fmt.Errorf("failed to send checkpoint: %w", err)
	}
	
	// Wait for acknowledgment
	ack, err := m.remoteProtocol.Receive(ctx)
	if err != nil {
		return err
	}
	
	if ack.Type != MessageTypeCheckpoint || ack.Error != "" {
		return fmt.Errorf("checkpoint migration failed: %s", ack.Error)
	}
	
	return nil
}

// BidirectionalWebSocket creates a bidirectional WebSocket connection for remote graph execution.
type BidirectionalWebSocket struct {
	client   *WebSocketPregelProtocol
	server   *WebSocketPregelProtocol
	migrator *CheckpointMigrator
}

// NewBidirectionalWebSocket creates a new bidirectional WebSocket connection.
func NewBidirectionalWebSocket(clientURL string, serverURL string) (*BidirectionalWebSocket, error) {
	clientProto, err := NewWebSocketPregelProtocol(&WebSocketConfig{
		URL: clientURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client WebSocket: %w", err)
	}
	
	serverProto, err := NewWebSocketPregelProtocol(&WebSocketConfig{
		URL: serverURL,
	})
	if err != nil {
		clientProto.Close()
		return nil, fmt.Errorf("failed to create server WebSocket: %w", err)
	}
	
	return &BidirectionalWebSocket{
		client:   clientProto,
		server:   serverProto,
		migrator: NewCheckpointMigrator(clientProto, serverProto),
	}, nil
}

// Close closes both WebSocket connections.
func (b *BidirectionalWebSocket) Close() error {
	var errs []error
	
	if err := b.client.Close(); err != nil {
		errs = append(errs, err)
	}
	
	if err := b.server.Close(); err != nil {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("failed to close connections: %v", errs)
	}
	
	return nil
}

// generateMessageID generates a unique message ID.
func generateMessageID() string {
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

// ValidateWebSocketURL validates a WebSocket URL.
func ValidateWebSocketURL(wsURL string) error {
	u, err := url.Parse(wsURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf("URL scheme must be ws or wss, got %s", u.Scheme)
	}
	
	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	
	return nil
}
