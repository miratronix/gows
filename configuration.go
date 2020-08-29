package gows

import (
	"crypto/tls"
	"github.com/gorilla/websocket"
	"github.com/miratronix/logpher"
	"math"
	"math/rand"
	"net/url"
	"time"
)

// Configuration defines the options structure for the websocket connection
type Configuration struct {
	URL                       string
	Query                     string
	Logger                    *logpher.Logger
	ConnectionRetries         int
	ConnectionRetryFactor     float64
	ConnectionRetryTimeoutMin time.Duration
	ConnectionRetryTimeoutMax time.Duration
	ConnectionRetryRandomize  bool
	PingInterval              time.Duration
	WriteTimeout              time.Duration
	ReadTimeout               time.Duration
	InsecureLocalhost         bool
	RetryInitialConnection    bool

	dialer *websocket.Dialer
}

// getRetryDuration computes the retry duration for a reconnect attempt
func (c *Configuration) getRetryDuration(attempt int) time.Duration {
	random := float64(1)
	if c.ConnectionRetryRandomize {
		random = rand.Float64() + 1
	}
	min := float64(c.ConnectionRetryTimeoutMin)
	max := float64(c.ConnectionRetryTimeoutMax)
	retryInterval := int64(math.Min(random*min*math.Pow(c.ConnectionRetryFactor, float64(attempt)), max))

	return time.Duration(retryInterval)
}

// getDialer gets the websocket dialer
func (c *Configuration) getDialer() (*websocket.Dialer, error) {

	// Already have a dialer, re-use it
	if c.dialer != nil {
		return c.dialer, nil
	}

	// Parse the URL
	uri, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}

	// If insecure localhost is not set, we're not using wss, or we're not connecting to localhost, use the default dialer
	if !c.InsecureLocalhost || uri.Scheme != "wss" || uri.Host != "localhost" {
		c.dialer = websocket.DefaultDialer
		return c.dialer, nil
	}

	// Clone the TLS configuration and set the insecure skip flag
	tlsConfig := &tls.Config{}
	if websocket.DefaultDialer.TLSClientConfig != nil {
		tlsConfig = websocket.DefaultDialer.TLSClientConfig.Clone()
	}
	tlsConfig.InsecureSkipVerify = true

	// Clone the default dialer but modify the TLS config
	c.dialer = &websocket.Dialer{
		NetDial:           websocket.DefaultDialer.NetDial,
		NetDialContext:    websocket.DefaultDialer.NetDialContext,
		Proxy:             websocket.DefaultDialer.Proxy,
		HandshakeTimeout:  websocket.DefaultDialer.HandshakeTimeout,
		ReadBufferSize:    websocket.DefaultDialer.ReadBufferSize,
		WriteBufferSize:   websocket.DefaultDialer.WriteBufferSize,
		WriteBufferPool:   websocket.DefaultDialer.WriteBufferPool,
		Subprotocols:      websocket.DefaultDialer.Subprotocols,
		EnableCompression: websocket.DefaultDialer.EnableCompression,
		Jar:               websocket.DefaultDialer.Jar,
		TLSClientConfig:   tlsConfig,
	}

	return c.dialer, nil
}
