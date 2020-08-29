package gows

import (
	"errors"
	"strings"
	"time"
)

// consumer defines the goroutine responsible for reading messages from the conenction
func (ws *Websocket) consumer() {

	// Set up the read deadline and a pong handler that refreshes the deadline
	_ = ws.connection.SetReadDeadline(time.Now().Add(ws.configuration.ReadTimeout))
	ws.connection.SetPongHandler(func(string) error {
		_ = ws.connection.SetReadDeadline(time.Now().Add(ws.configuration.ReadTimeout))
		return nil
	})

	for {
		select {

		case <-ws.consumerStopChannel:
			return

		default:
			_, message, err := ws.connection.ReadMessage()

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
