package gows

import (
	"fmt"
	"github.com/gorilla/websocket"
	"strings"
	"time"
)

// connect connects the websocket, either indefinitely or using the maximum number of retries
func (ws *Websocket) connect(retries bool) (*websocket.Conn, error) {
	attempt := 0

	for {
		url := ws.configuration.URL
		ws.configuration.Logger.Info("Attempting connection to", url)

		// Append the provided query parameters
		if len(ws.configuration.Query) != 0 {
			url = fmt.Sprintf("%s?%s", url, ws.configuration.Query)
		}

		// Create the dialer
		dialer, err := ws.configuration.getDialer()
		if err != nil {
			return nil, err
		}

		// Dial the connection
		connection, _, err := dialer.Dial(url, nil)
		if err == nil {
			ws.configuration.Logger.Info("Successfully connected websocket")
			return connection, nil
		}

		// Keep trying if retrying is allowed and the configured retries are set to 0, or if we have attempts left
		keepTrying := retries && (ws.configuration.ConnectionRetries == 0 || attempt < (ws.configuration.ConnectionRetries-1))

		if !keepTrying {
			ws.configuration.Logger.Info("Failed to connect websocket after", retries, "attempts")
			return nil, err
		}

		// Sleep for the retry interval
		time.Sleep(ws.configuration.getRetryDuration(attempt))
		attempt++
	}
}

// reviver is a Goroutine responsible for initializing the websocket connection and reconnecting it when the connection is dropped
func (ws *Websocket) reviver(initialConnectionErrorChannel chan error) {

	connection, err := ws.connect(ws.configuration.RetryInitialConnection)
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

			// Clear out the connection
			ws.configuration.Logger.Warn("Websocket connection lost:", err)
			ws.clearConnection()

			// And establish a new one
			connection, _ := ws.connect(true)
			ws.setConnection(connection)
		}
	}
}

// setConnection initializes the websocket, starting up the reader and unblocking any goroutines trying to send stuff
func (ws *Websocket) setConnection(connection *websocket.Conn) {
	ws.configuration.Logger.Debug("Preparing new connection...")

	// Lock on the connection lock while modifying the connection
	ws.configuration.Logger.Trace("Initializing connection object...")
	ws.connectionLock.Lock()

	// Set the connection
	ws.connection = connection

	// Add a close listener that writes on the connection drop channel
	ws.connectionDroppedChannel = make(chan error)
	ws.connection.SetCloseHandler(func(code int, message string) error {
		ws.connectionDroppedChannel <- fmt.Errorf("websocket closed with code %d:%s", code, message)
		return nil
	})

	// Release the connection lock
	ws.connectionLock.Unlock()
	ws.configuration.Logger.Trace("Successfully initialized connection object")

	// Call the connection handler
	ws.configuration.Logger.Trace("Calling connection handler...")
	ws.connectedHandlerLock.Lock()
	ws.connectedHandler()
	ws.connectedHandlerLock.Unlock()
	ws.configuration.Logger.Trace("Successfully called connection handler")

	// Start the message consumer and sender after calling the connection handler, to ensure no events come in
	// before the connected handler has completed
	ws.configuration.Logger.Trace("Starting consumer/sender goroutines...")
	ws.startConsumer()
	ws.startSender()
	ws.configuration.Logger.Trace("Successfully started consumer/sender goroutines")

	ws.configuration.Logger.Debug("Successfully prepared new connection")
}

// clearConnection terminates the connection, cleaning up the consumer and closing the connection if present
func (ws *Websocket) clearConnection() {
	ws.configuration.Logger.Debug("Clearing out connection...")

	// Stop the consumer and sender
	ws.configuration.Logger.Trace("Stopping consumer/sender goroutines...")
	ws.stopConsumer()
	ws.stopSender()
	ws.configuration.Logger.Trace("Successfully stopped consumer/sender goroutines")

	// Lock on the connection lock while modifying the connection
	ws.configuration.Logger.Trace("Closing and removing connection object...")
	ws.connectionLock.Lock()

	// Close the connection and log an error if closing it failed
	if ws.connection != nil {
		err := ws.connection.Close()
		if err != nil && !strings.HasSuffix(err.Error(), "use of closed connection") {
			ws.configuration.Logger.Warn("Failed to close connection:", err)
		}
	}

	// Clear the connection
	ws.connection = nil

	// Release the connection lock
	ws.connectionLock.Unlock()
	ws.configuration.Logger.Trace("Successfully closed and removed connection object")

	// Call the disconnect handler
	ws.configuration.Logger.Trace("Calling disconnect handler...")
	ws.disconnectedHandlerLock.Lock()
	ws.disconnectedHandler()
	ws.disconnectedHandlerLock.Unlock()
	ws.configuration.Logger.Trace("Successfully called disconnect handler")

	ws.configuration.Logger.Debug("Successfully cleared out connection")
}

// getConnection gets the current websocket connection
func (ws *Websocket) getConnection() *websocket.Conn {

	// Lock on the connection lock
	ws.connectionLock.Lock()
	defer ws.connectionLock.Unlock()

	return ws.connection
}

// handleConnectionError writes the supplied connection error to the connection drop channel. If there are no goroutines
// currently waiting on the drop channel, it means that we're currently reviving already, so the error can be dropped
func (ws *Websocket) handleConnectionError(err error) {
	select {
	case ws.connectionDroppedChannel <- err:
	default:
	}
}
