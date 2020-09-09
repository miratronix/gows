package gows

import (
	"github.com/gorilla/websocket"
	"time"
)

// sender defines A simple goroutine that ensures all message are sent sequentially
func (ws *Websocket) sender() {

	// Set up a ping interval and shut it down when we exit this goroutine
	pingTicker := time.NewTicker(ws.configuration.PingInterval)
	defer pingTicker.Stop()

	// Set up an interval for flushing messages
	flushTicker := time.NewTicker(50 * time.Millisecond)
	defer flushTicker.Stop()

	// Set up a channel to do another pop
	continueChannel := make(chan struct{}, 1)

	// Set up the function that sends the message. This function is responsible for popping the message out of the queue,
	// sending it with a write deadline, requeueing it if there's a send failure, and writing to the continueChannel if
	// there are more messages to send. It returns true if an error is encountered and the goroutine should be stopped.
	// Writing this function here allows us to call it from two different select cases, which ensures that we send pings
	// even when the queue contains messages.
	sendMessage := func() bool {

		// Pop a message from the queue. If there aren't any, we're done here
		msg, remaining := ws.sendQueue.pop()
		if msg == nil {
			return false
		}

		// Get the connection. If it's nil, we're about to be restarted. Requeue the message and kill this goroutine,
		// the reviver will restart us when a new connection is established
		connection := ws.getConnection()
		if connection == nil {
			ws.configuration.Logger.Trace("SENDER: Requeueing message, connection is nil...")
			ws.sendQueue.requeue(msg)
			ws.configuration.Logger.Trace("SENDER: Successfully requeued message")
			return true
		}

		// Write the message, returning true if there are more messages to send
		ws.configuration.Logger.Trace("SENDER: Writing message...")
		_ = connection.SetWriteDeadline(time.Now().Add(ws.configuration.WriteTimeout))
		err := connection.WriteMessage(websocket.BinaryMessage, msg)

		// There was a write timeout, re-queue the message and kill this goroutine. It will be revived and the message
		// will be sent when the connection is re-established
		if err != nil {
			ws.configuration.Logger.Trace("SENDER: Encountered write timeout, requeing message and flagging the websocket drop...")
			ws.sendQueue.requeue(msg)
			ws.handleConnectionError(err)
			ws.configuration.Logger.Trace("SENDER: Successfully requeued message and flagged websocket drop")
			return true
		}

		ws.configuration.Logger.Trace("SENDER: Successfully wrote message")

		// If there are no more messages to send, we're done here for now
		if remaining == 0 {
			ws.configuration.Logger.Trace("SENDER: No more messages to send, sleeping for 50ms")
			return false
		}

		// There are more messages to send, write onto the repeat channel
		ws.configuration.Logger.Trace("SENDER: More messages remaining, continuing the flush")
		select {
		case continueChannel <- struct{}{}:
		default:
		}

		return false
	}

	// For consistency with the above function, set up another function responsible for sending ping messages. Like the
	// function above, this function returns true when the goroutine should be stopped.
	sendPing := func() bool {

		// Get the connection. If it's nil, we're about to restarted. Ignore the ping and kill this goroutine, the
		// reviver will restart us when a new connection comes in
		connection := ws.getConnection()
		if connection == nil {
			ws.configuration.Logger.Trace("SENDER: No connection for ping, shutting down")
			return true
		}

		// Write the ping message. If there's a timeout, clean up the stop channel, write the error, and kill this goroutine
		ws.configuration.Logger.Trace("SENDER: Writing ping message")
		_ = connection.SetWriteDeadline(time.Now().Add(ws.configuration.WriteTimeout))
		err := connection.WriteMessage(websocket.PingMessage, nil)
		if err == nil {
			ws.configuration.Logger.Trace("SENDER: Successfully wrote ping")
			return false
		}

		// There was a write timeout, clean up the stop channel, write the error, and kill this goroutine
		ws.configuration.Logger.Trace("SENDER: Encountered ping timeout, flagging the websocket drop...")
		ws.handleConnectionError(err)
		ws.configuration.Logger.Trace("SENDER: Successfully flagged websocket drop")
		return true
	}

	// Run the main goroutine loop
	for {
		select {

		// Stopped, kill this goroutine
		case <-ws.senderStopChannel:
			ws.configuration.Logger.Trace("SENDER: Shutting down")
			return

		// Check the message queue every 50ms
		case <-flushTicker.C:
			if sendMessage() {
				return
			}

		// If we finished a send and there are still more queued messages, do the send again. Since this is part of the
		// select, it allows us to gracefully pause to send a ping or react to a shut down.
		case <-continueChannel:
			if sendMessage() {
				return
			}

		// Send a ping
		case <-pingTicker.C:
			if sendPing() {
				return
			}
		}
	}
}

// startSender starts the sender goroutine
func (ws *Websocket) startSender() {
	ws.configuration.Logger.Trace("Starting sender goroutine...")
	ws.senderStopChannel = make(chan struct{})
	go ws.sender()
	ws.configuration.Logger.Trace("Successfully started sender goroutine...")
}

// stopSender stops the sender goroutine
func (ws *Websocket) stopSender() {
	ws.configuration.Logger.Trace("Stopping sender goroutine...")
	close(ws.senderStopChannel)
	ws.configuration.Logger.Trace("Successfully stopped sender goroutine")
}
