# gows
A basic reconnecting websocket library that supports:
- Query parameters
- Queueing during reconnects
- Automatic heartbeats
- Self-signed certificates for localhost connections

## Usage
Usage is as simple as configuring and connecting:
```go
import (
    "github.com/miratronix/gows"
	"github.com/miratronix/logpher"
)

// Initialize the websocket
ws := gows.New(&gows.Configuration{
	URL:                       "ws://some.url",         // The URL to connect to
	Query:                     "query_param=something", // Query parameters to add to the above URL
	Logger:                    logpher.NewLogger("ws"), // The logger for the websocket
	ConnectionRetries:         5,                       // The number of connection retries on initial connection
	ConnectionRetryFactor:     2,                       // The exponential retry factor
	ConnectionRetryTimeoutMin: 1 * time.Second,         // The minimum timeout for connection retries
	ConnectionRetryTimeoutMax: 5 * time.Second,         // The maximum timeout for connection retries
	ConnectionRetryRandomize:  false,                   // Whether to apply randomness to the timeout interval
	PingInterval               30 * time.Second,        // The interval to send pings at
    WriteTimeout               5 * time.Second,         // The timeout for write operations
    ReadTimeout                35 * time.Second,        // The timeout for read operations. Should be longer than the ping interval
	InsecureLocalhost:         false,                   // Whether to skip certificate validation for localhost connections
	RetryInitialConnection:    false,                   // Whether to apply retry logic to the initial connection attempt
})

// Attach handlers for various events
ws.OnConnected(func() {})
ws.OnMessage(func(msg []byte) {})
ws.OnDisconnected(func() {})

// Will return an error if the initial connection attempt fails ConnectionRetries times
err := ws.Connect()

// Returns immediately, but doesn't attempt to send until the socket is connected
ws.Send([]byte("Hello world!"))

// Queues outgoing packets (without making Send block)
ws.BlockSend()

// Unblocks outgoing packets and flushes any queued packets
ws.UnblockSend()

// Determines if the socket is currently connected (false during reconnects)
connected := ws.IsConnected()

// Disconnects the socket
err = ws.Disconnect()
```
