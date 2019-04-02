package gows

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
	go func() {
		for {
			select {

			case <-ws.consumerStopChannel:
				return

			default:
				_, message, err := ws.connection.ReadMessage()

				// Connection dropped, stop consuming, clear the consumer stop channel, and kill this goroutine
				if err != nil {
					ws.consumerStopChannelLock.Lock()
					ws.consumerStopChannel = nil
					ws.consumerStopChannelLock.Unlock()
					ws.connectionDroppedChannel <- err
					return
				}

				// Handle the message in a goroutine
				go func() {
					ws.consumer(message)
				}()
			}
		}
	}()
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
