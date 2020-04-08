package gows

import (
	"github.com/gorilla/websocket"
	"github.com/tevino/abool"
	"sync"
	"time"
)

// Websocket defines a simple websocket structure
type Websocket struct {
	configuration *Configuration

	// Connection information
	connected                   *abool.AtomicBool // Whether we are currently connected
	connection                  *websocket.Conn   // The websocket connection
	stopChannel                 chan struct{}     // The channel to send to when stopping the connection reviver
	connectionDroppedChannel    chan error        // The connection drop channel to listen on for connection failures
	connectionAwaitChannels     []chan struct{}   // Slice of channels that are currently awaiting reconnect
	connectionAwaitChannelsLock *sync.Mutex       // Lock for the connection await channels slice

	// Consumer information
	consumer                func([]byte)  // The websocket consumer
	consumerLock            *sync.Mutex   // Lock for the consumer list
	consumerStopChannel     chan struct{} // Stop channel for the consumer
	consumerStopChannelLock *sync.Mutex   // Lock for the consumer stop channel

	// Handler information
	connectedHandler        func()
	connectedHandlerLock    *sync.Mutex
	disconnectedHandler     func()
	disconnectedHandlerLock *sync.Mutex

	// Heartbeat information
	heartbeatTicker      *time.Ticker
	heartbeatStopChannel chan struct{}

	// Sending information
	sendChannel       chan []byte
	sendLockChannel   chan struct{}
	senderStopChannel chan struct{}
}

// New constructs a new websocket object
func New(configuration *Configuration) *Websocket {
	return &Websocket{
		configuration:               configuration,
		connected:                   abool.New(),
		stopChannel:                 make(chan struct{}),
		connectionAwaitChannelsLock: &sync.Mutex{},
		consumerLock:                &sync.Mutex{},
		consumerStopChannelLock:     &sync.Mutex{},
		consumer:                    func([]byte) {},
		connectedHandler:            func() {},
		connectedHandlerLock:        &sync.Mutex{},
		disconnectedHandler:         func() {},
		disconnectedHandlerLock:     &sync.Mutex{},
		sendChannel:                 make(chan []byte),
		sendLockChannel:             nil,
		senderStopChannel:           make(chan struct{}),
	}
}

// Connect connects the websocket
func (ws *Websocket) Connect() error {
	initialConnectionErrorChannel := make(chan error)

	// Start up the message sender
	go ws.sender()

	// Start up the reviver
	go ws.reviver(initialConnectionErrorChannel)

	return <-initialConnectionErrorChannel
}

// Send sends a message with the provided details in a separate goroutine (so it doesn't block on reconnects)
func (ws *Websocket) Send(msg []byte) {
	go func() {
		ws.sendChannel <- msg
	}()
}

// OnConnected sets the onConnected handler
func (ws *Websocket) OnConnected(handler func()) {
	ws.connectedHandlerLock.Lock()
	ws.connectedHandler = handler
	ws.connectedHandlerLock.Unlock()
}

// OnMessage sets the onMessage handler
func (ws *Websocket) OnMessage(consumer func([]byte)) {
	ws.consumerLock.Lock()
	ws.consumer = consumer
	ws.consumerLock.Unlock()
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
	ws.sendLockChannel = make(chan struct{})
}

// UnblockSend stops blocking message sending
func (ws *Websocket) UnblockSend() {
	if ws.sendLockChannel == nil {
		return
	}
	close(ws.sendLockChannel)
	ws.sendLockChannel = nil
}

// Disconnect disconnects the websocket
func (ws *Websocket) Disconnect() error {
	if ws.connected.IsSet() {
		close(ws.stopChannel)
	}
	close(ws.senderStopChannel)
	return nil
}
