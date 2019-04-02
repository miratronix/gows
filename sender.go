package gows

import "github.com/gorilla/websocket"

// A simple sender goroutine that ensure all sent message are done sequentially and handles send blocking functionality
func (ws *Websocket) sender() {
	for {
		select {

		// Stopped, end the loop
		case <-ws.senderStopChannel:
			return

		// Got a new message to send
		case msg := <-ws.sendChannel:

			// Wait on the send lock if there is one
			if ws.sendLockChannel != nil {
				<-ws.sendLockChannel
			}

			// Send the message
			err := ws.getConnection().WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				ws.configuration.Logger.Warn("Failed to send message", msg)
			}
		}
	}
}
