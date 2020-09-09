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
		ws.configuration.Logger.Trace("CONSUMER: No connection on startup, shutting down")
		return
	}

	// Set up the read deadline and a pong handler that refreshes the deadline
	ws.configuration.Logger.Trace("CONSUMER: Setting read deadline...")
	_ = connection.SetReadDeadline(time.Now().Add(ws.configuration.ReadTimeout))
	connection.SetPongHandler(func(string) error {
		_ = connection.SetReadDeadline(time.Now().Add(ws.configuration.ReadTimeout))
		return nil
	})
	ws.configuration.Logger.Trace("CONSUMER: Successfully set read deadline")

	for {
		select {

		case <-ws.consumerStopChannel:
			ws.configuration.Logger.Trace("CONSUMER: Shutting down")
			return

		default:
			ws.configuration.Logger.Trace("CONSUMER: Reading message...")
			_, message, err := connection.ReadMessage()

			// Connection dropped, stop consuming, clear the consumer stop channel, and kill this goroutine
			if err != nil {

				// If the network connection was closed, clean up the logged message
				if strings.HasSuffix(err.Error(), "use of closed network connection") {
					err = errors.New("client was closed")
				}

				// Write an error to the connection error channel and kill this goroutine
				ws.configuration.Logger.Trace("CONSUMER: Failed to read message, flagging connection drop...")
				ws.handleConnectionError(err)
				ws.configuration.Logger.Trace("CONSUMER: Successfully flagged connection drop")
				return
			}

			// Handle the message in a goroutine
			ws.configuration.Logger.Trace("CONSUMER: Successfully read message")
			go func() {
				ws.configuration.Logger.Trace("CONSUMER: Calling message handler...")
				ws.messageHandler(message)
				ws.configuration.Logger.Trace("CONSUMER: Successfully called message handler")
			}()
		}
	}
}

// startConsumer starts the websocket consumer
func (ws *Websocket) startConsumer() {
	ws.configuration.Logger.Trace("Starting consumer goroutine...")
	ws.consumerStopChannel = make(chan struct{})
	go ws.consumer()
	ws.configuration.Logger.Trace("Successfully started consumer goroutine")
}

// stopConsumer stops the consumer
func (ws *Websocket) stopConsumer() {
	ws.configuration.Logger.Trace("Stopping consumer goroutine...")
	close(ws.consumerStopChannel)
	ws.configuration.Logger.Trace("Successfully stopped consumer goroutine")
}
