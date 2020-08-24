package gows

// queue defines a queue that can be consumed as a channel, with pause/resume and requeue functionality
type queue struct {

	// Lifecycle channels
	stopChannel   chan struct{}
	pauseChannel  chan struct{}
	resumeChannel chan struct{}

	// IO channels
	primaryInChannel   chan []byte
	secondaryInChannel chan []byte
	outputChannel      chan []byte
}

// newQueue constructs a new queue and starts its main loop
func newQueue() *queue {
	q := &queue{
		stopChannel:        make(chan struct{}),
		pauseChannel:       make(chan struct{}),
		resumeChannel:      make(chan struct{}),
		primaryInChannel:   make(chan []byte),
		secondaryInChannel: make(chan []byte),
		outputChannel:      make(chan []byte),
	}
	go q.run()
	return q
}

// pop gets the read-only queue channel
func (q *queue) pop() <-chan []byte {
	return q.outputChannel
}

// add adds a message to the queue
func (q *queue) push(entry []byte) {
	go q.handleInput(q.secondaryInChannel, entry)
}

// unPop adds a message back to the front of a queue
func (q *queue) unPop(entry []byte) {
	go q.handleInput(q.primaryInChannel, entry)
}

// pause temporarily pauses the queue
func (q *queue) pause() {
	q.pauseChannel <- struct{}{}
}

// resume resumes the queue
func (q *queue) resume() {
	q.resumeChannel <- struct{}{}
}

// run starts the queue loop
func (q *queue) run() {
	for {
		select {

		// If we're shut down, exit
		case <-q.stopChannel:
			return

		// If we're paused, await the resume or stop
		case <-q.pauseChannel:
			if !q.awaitResume() {
				return
			}

		// If we have a primary message, send it
		case entry := <-q.primaryInChannel:
			if !q.writeOutput(entry) {
				return
			}

		// No pending primary message, pause, or stop. Wait for any of those to happen
		default:
			select {

			// If we're shut down, exit
			case <-q.stopChannel:
				return

			// If we're paused, await the resume or stop
			case <-q.pauseChannel:
				if !q.awaitResume() {
					return
				}

			// If a primary messages comes in, send it
			case entry := <-q.primaryInChannel:
				if !q.writeOutput(entry) {
					return
				}

			// If a secondary message comes in, send it
			case entry := <-q.secondaryInChannel:
				if !q.writeOutput(entry) {
					return
				}
			}
		}
	}
}

// awaitResume awaits a resume or stop
func (q *queue) awaitResume() bool {
	select {
	case <-q.resumeChannel:
		return true
	case <-q.stopChannel:
		return false
	}
}

// shutdown shuts the queue down, stopping the main loop and any pending writes
func (q *queue) shutdown() {
	close(q.stopChannel)
}

// handleInput writes the supplied entry to the supplied input channel, unless the queue has been shut down
func (q *queue) handleInput(channel chan []byte, entry []byte) {
	select {

	// Queue has been shut down, do nothing
	case <-q.stopChannel:
		return

	// No pending stop, wait for the input channel to accept the message (or for a shutdown)
	default:
		select {
		case channel <- entry:
		case <-q.stopChannel:
		}
	}
}

// writeOutput writes the supplied entry to the output channel
func (q *queue) writeOutput(entry []byte) bool {
	select {

	// Queue has been shut down, do nothing
	case <-q.stopChannel:
		return false

	// Queue has been paused, wait for the resume and try this again
	case <-q.pauseChannel:
		if !q.awaitResume() {
			return false
		}
		return q.writeOutput(entry)

	// No pending stop or pause. Attempt to write the message to the output channel, unless a shut down or pause happens
	default:
		select {

		// Successfully wrote to channel
		case q.outputChannel <- entry:
			return true

		// Shut down while awaiting sender
		case <-q.stopChannel:
			return false

		// Paused while awaiting sender. Wait for the resume and then retry this entry
		case <-q.pauseChannel:
			if !q.awaitResume() {
				return false
			}
			return q.writeOutput(entry)
		}
	}
}
