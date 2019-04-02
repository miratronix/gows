package gows

import (
	"fmt"
	"github.com/gorilla/websocket"
	"time"
)

// connect connects the websocket, either indefinitely or using the maximum number of retries
func (ws *Websocket) connect(retries bool) (*websocket.Conn, error) {
	attempt := 0

	for {
		url := ws.configuration.URL
		ws.configuration.Logger.Info("Attempting connection to", url)

		if len(ws.configuration.Query) != 0 {
			url = fmt.Sprintf("%s?%s", url, ws.configuration.Query)
		}

		connection, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err == nil {
			ws.configuration.Logger.Info("Successfully connected websocket")
			return connection, nil
		}

		// Keep trying if retrying is allowed and the configured retries are set to 0, or if we have attempts left
		keepTrying := retries && (ws.configuration.ConnectionRetries == 0 || attempt < (ws.configuration.ConnectionRetries-1))

		if !keepTrying {
			return nil, err
		}

		// Sleep for the retry interval
		time.Sleep(ws.configuration.getRetryDuration(attempt))
		attempt++
	}
}

// reviver is a Goroutine responsible for initializing the websocket connection and reconnecting it when the connection is dropped
func (ws *Websocket) reviver(initialConnectionErrorChannel chan error) {

	connection, err := ws.connect(false)
	if err != nil {
		initialConnectionErrorChannel <- err
		return
	}

	// Save the connection
	ws.setConnection(connection)

	// Connected successfully, no error to push onto the channel
	close(initialConnectionErrorChannel)

	// Loop indefinitely on reconnects (unless we're stopped)
	for {
		select {

		case <-ws.stopChannel:
			ws.clearConnection()
			return

		case err := <-ws.connectionDroppedChannel:

			// A nil error means the channel was closed (or someone pushed a nil)
			if err == nil {
				break
			}

			ws.clearConnection()
			ws.configuration.Logger.Warn("Websocket connection lost:", err)
			connection, _ := ws.connect(true)

			// Set the connection, starting the message consumer
			ws.setConnection(connection)
		}
	}
}

// getConnection gets the websocket connection if it's present. Otherwise, blocks until the connection is established
func (ws *Websocket) getConnection() *websocket.Conn {

	// If we're already connected, just return the connection
	if ws.connected.IsSet() {
		return ws.connection
	}

	// Lock the await channels slice to prevent multiple writes
	ws.connectionAwaitChannelsLock.Lock()

	// If we just received the lock after the connection has been completed, we have our connection
	if ws.connected.IsSet() {
		ws.connectionAwaitChannelsLock.Unlock()
		return ws.connection
	}

	// Otherwise, we'll add ourselves to the connection await slice
	connectionAwaitChannel := make(chan struct{})
	ws.connectionAwaitChannels = append(ws.connectionAwaitChannels, connectionAwaitChannel)

	// All done with the lock
	ws.connectionAwaitChannelsLock.Unlock()

	// Block until we're connected
	<-connectionAwaitChannel

	return ws.connection
}

// setConnection initializes the websocket, starting up the reader and unblocking any goroutines trying to send stuff
func (ws *Websocket) setConnection(connection *websocket.Conn) {
	ws.connection = connection

	// Start the message consumer
	ws.startConsumer()

	// Lock the connection await channels slice
	ws.connectionAwaitChannelsLock.Lock()

	// Unblock any goroutines awaiting a connection
	for _, connectionAwaitChannel := range ws.connectionAwaitChannels {
		close(connectionAwaitChannel)
	}
	ws.connectionAwaitChannels = []chan struct{}{}

	// Indicate that the connection was successful
	ws.connected.SetTo(true)

	// Call the connection handler before unlocking the await lock and sending queued messages
	ws.connectedHandlerLock.Lock()
	ws.connectedHandler()
	ws.connectedHandlerLock.Unlock()

	// Unlock the connection await lock
	ws.connectionAwaitChannelsLock.Unlock()

	// Set up a heartbeat if required
	ws.heartbeatTicker = time.NewTicker(ws.configuration.HeartbeatInterval)
	go func() {
		for {
			select {
			case <-ws.heartbeatStopChannel:
				return

			case <-ws.heartbeatTicker.C:
				ws.Send(ws.configuration.HeartbeatMessage)
			}
		}
	}()

	// Add a close listener that writes on the connection drop channel
	ws.connectionDroppedChannel = make(chan error)
	ws.connection.SetCloseHandler(func(code int, message string) error {
		ws.connectionDroppedChannel <- fmt.Errorf("websocket closed with code %d:%s", code, message)
		return nil
	})
}

// clearConnection terminates the connection, cleaning up the consumer and closing the connection if present
func (ws *Websocket) clearConnection() {

	// Stop the heartbeat and its ticker
	close(ws.heartbeatStopChannel)
	ws.heartbeatTicker.Stop()

	// Set the connection state to false
	ws.connected.SetTo(false)

	// Stop the consumer if it's running
	ws.stopConsumer()

	// Close the connection
	if ws.connection != nil {
		ws.connection.Close()
	}
	ws.connection = nil

	// Call the disconnect handler
	ws.disconnectedHandlerLock.Lock()
	ws.disconnectedHandler()
	ws.disconnectedHandlerLock.Unlock()
}
