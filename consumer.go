package gows

import (
	"errors"
	"strings"
	"time"
)

// consumer defines the goroutine responsible for reading messages from the connection
func (ws *Websocket) consumer() {

	// Get the current connection. If it's nil, it means that the connection dropped while we were starting up. Nothing
	// to do with this connection, so just exit and let the reviver start us up again
	connection := ws.getConnection()
	if connection == nil {
		return
	}

	// Set up the read deadline and a pong handler that refreshes the deadline
	_ = connection.SetReadDeadline(time.Now().Add(ws.configuration.ReadTimeout))
	connection.SetPongHandler(func(string) error {
		_ = connection.SetReadDeadline(time.Now().Add(ws.configuration.ReadTimeout))
		return nil
	})

	for {
		select {

		case <-ws.consumerStopChannel:
			return

		default:
			_, message, err := connection.ReadMessage()

			// Connection dropped, stop consuming, clear the consumer stop channel, and kill this goroutine
			if err != nil {

				// If the network connection was closed, clean up the logged message
				if strings.HasSuffix(err.Error(), "use of closed network connection") {
					err = errors.New("client was closed")
				}

				// Clean up the consumer stop channel, write a connection error, and kill this goroutine
				ws.stopConsumer()
				ws.handleConnectionError(err)
				return
			}

			// Handle the message in a goroutine
			go func() {
				ws.messageHandler(message)
			}()
		}
	}
}

// startConsumer starts the websocket consumer
func (ws *Websocket) startConsumer() {

	// If we're already started, do nothing
	ws.consumerStopChannelLock.Lock()
	started := ws.consumerStopChannel != nil
	ws.consumerStopChannelLock.Unlock()
	if started {
		return
	}

	// Set up an exit channel for killing the consumer
	ws.consumerStopChannelLock.Lock()
	ws.consumerStopChannel = make(chan struct{})
	ws.consumerStopChannelLock.Unlock()

	// Start up the consumer goroutine
	go ws.consumer()
}

// stopConsumer stops the consumer
func (ws *Websocket) stopConsumer() {
	ws.consumerStopChannelLock.Lock()
	if ws.consumerStopChannel != nil {
		close(ws.consumerStopChannel)
		ws.consumerStopChannel = nil
	}
	ws.consumerStopChannelLock.Unlock()
}
