package gows

import (
	"github.com/gorilla/websocket"
	"github.com/tevino/abool"
	"sync"
)

// Websocket defines a simple websocket structure
type Websocket struct {
	configuration *Configuration

	// Connection information
	connected                *abool.AtomicBool // Whether we are currently connected
	connection               *websocket.Conn   // The websocket connection
	stopChannel              chan struct{}     // The channel to send to when stopping the connection reviver
	connectionDroppedChannel chan error        // The connection drop channel to listen on for connection failures

	// Consumer stop information
	consumerStopChannel     chan struct{} // Stop channel for the consumer
	consumerStopChannelLock *sync.Mutex   // Lock for the consumer stop channel

	// Sender information
	sendQueue             *queue        // Queue of messages to send
	senderStopChannel     chan struct{} // Stop channel for the sender
	senderStopChannelLock *sync.Mutex   // Lock for the sender stop channel

	// Handler information
	messageHandler          func([]byte) // The websocket handler
	messageHandlerLock      *sync.Mutex  // Lock for the handler
	connectedHandler        func()       // The connected handler
	connectedHandlerLock    *sync.Mutex  // Lock for the connection handler
	disconnectedHandler     func()       // The disconnected handler
	disconnectedHandlerLock *sync.Mutex  // Lock for the disconnectedHandler
}

// New constructs a new websocket object
func New(configuration *Configuration) *Websocket {
	return &Websocket{
		configuration: configuration,

		// Connection information
		connected:                abool.New(),
		connection:               nil,
		stopChannel:              make(chan struct{}),
		connectionDroppedChannel: nil,

		// Consumer stop information
		consumerStopChannel:     nil,
		consumerStopChannelLock: &sync.Mutex{},

		// Sender information
		sendQueue:             newQueue(),
		senderStopChannel:     nil,
		senderStopChannelLock: &sync.Mutex{},

		// Handler information
		messageHandler:          func([]byte) {},
		messageHandlerLock:      &sync.Mutex{},
		connectedHandler:        func() {},
		connectedHandlerLock:    &sync.Mutex{},
		disconnectedHandler:     func() {},
		disconnectedHandlerLock: &sync.Mutex{},
	}
}

// Connect connects the websocket
func (ws *Websocket) Connect() error {
	initialConnectionErrorChannel := make(chan error)

	// Start up the reviver
	go ws.reviver(initialConnectionErrorChannel)

	return <-initialConnectionErrorChannel
}

// Reconnect forces a reconnection if currently connected, by closing the underlying connection. The consumer then fails
// to read and forces a revival as normal
func (ws *Websocket) Reconnect() error {
	if ws.connected.IsNotSet() {
		return nil
	}
	return ws.connection.Close()
}

// Send sends a binary message with the provided body
func (ws *Websocket) Send(msg []byte) {
	ws.sendQueue.push(msg)
}

// OnConnected sets the onConnected handler
func (ws *Websocket) OnConnected(handler func()) {
	ws.connectedHandlerLock.Lock()
	ws.connectedHandler = handler
	ws.connectedHandlerLock.Unlock()
}

// OnMessage sets the onMessage handler
func (ws *Websocket) OnMessage(handler func([]byte)) {
	ws.messageHandlerLock.Lock()
	ws.messageHandler = handler
	ws.messageHandlerLock.Unlock()
}

// OnDisconnected sets the onDisconnected handler
func (ws *Websocket) OnDisconnected(handler func()) {
	ws.disconnectedHandlerLock.Lock()
	ws.disconnectedHandler = handler
	ws.disconnectedHandlerLock.Unlock()
}

// IsConnected determines if the socket is currently connected
func (ws *Websocket) IsConnected() bool {
	return ws.connected.IsSet()
}

// BlockSend blocks message sending until UnblockSend() is called
func (ws *Websocket) BlockSend() {
	ws.sendQueue.pause()
}

// UnblockSend stops blocking message sending
func (ws *Websocket) UnblockSend() {
	ws.sendQueue.resume()
}

// Disconnect disconnects the websocket
func (ws *Websocket) Disconnect() {
	if ws.connected.IsSet() {
		close(ws.stopChannel)
	}
}
