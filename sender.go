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

	for {
		select {

		// Stopped, kill this goroutine
		case <-ws.senderStopChannel:
			return

		// Got a new message to send
		case msg := <-ws.sendQueue.pop():

			// Get the connection. If it's nil, we're about to be restarted. Requeue the message and kill this goroutine,
			// the reviver will restart us when a new connection is established
			connection := ws.getConnection()
			if connection == nil {
				ws.sendQueue.unPop(msg)
				return
			}

			// Write the message
			_ = connection.SetWriteDeadline(time.Now().Add(ws.configuration.WriteTimeout))
			err := connection.WriteMessage(websocket.BinaryMessage, msg)
			if err == nil {
				continue
			}

			// There was a write timeout, re-queue the message and kill this goroutine. It will be revived and the message
			// will be sent when the connection is re-established
			ws.sendQueue.unPop(msg)
			ws.stopSender()
			ws.handleConnectionError(err)
			return

		// Send a ping
		case <-pingTicker.C:

			// Get the connection. If it's nil, we're about to restarted. Ignore the ping and kill this goroutine, the
			// reviver will restart us when a new connection comes in
			connection := ws.getConnection()
			if connection == nil {
				return
			}

			// Write the ping message. If there's a timeout, clean up the stop channel, write the error, and kill this goroutine
			_ = connection.SetWriteDeadline(time.Now().Add(ws.configuration.WriteTimeout))
			err := connection.WriteMessage(websocket.PingMessage, nil)
			if err == nil {
				continue
			}

			// There was a write timeout, clean up the stop channel, write the error, and kill this goroutine
			ws.stopSender()
			ws.handleConnectionError(err)
			return
		}
	}
}

// startSender starts the sender goroutine if it's not already started
func (ws *Websocket) startSender() {

	// If we're already started, do nothing
	ws.senderStopChannelLock.Lock()
	started := ws.senderStopChannel != nil
	ws.senderStopChannelLock.Unlock()
	if started {
		return
	}

	// Set up an exit channel for killing the sender
	ws.senderStopChannelLock.Lock()
	ws.senderStopChannel = make(chan struct{})
	ws.senderStopChannelLock.Unlock()

	// Start up the sender goroutine
	go ws.sender()
}

// stopSender stops the sender goroutine unless it's already stopped
func (ws *Websocket) stopSender() {
	ws.senderStopChannelLock.Lock()
	if ws.senderStopChannel != nil {
		close(ws.senderStopChannel)
		ws.senderStopChannel = nil
	}
	ws.senderStopChannelLock.Unlock()
}
