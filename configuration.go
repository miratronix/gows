package gows

import (
	"github.com/miratronix/logpher"
	"math"
	"math/rand"
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
	HeartbeatInterval         time.Duration
	HeartbeatMessage          []byte
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
