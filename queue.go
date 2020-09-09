package gows

import "sync"

// queue defines a basic thread-safe queue structure that can be paused
type queue struct {
	lock     *sync.Mutex
	messages [][]byte
	paused   bool
}

// newQueue constructs a new queue
func newQueue() *queue {
	return &queue{
		lock:     &sync.Mutex{},
		messages: make([][]byte, 0),
	}
}

// push pushes a message onto the the back of the queue
func (q *queue) push(msg []byte) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.messages = append(q.messages, msg)
}

// pop pops a message from the queue, unless it's paused
func (q *queue) pop() ([]byte, int) {
	q.lock.Lock()
	defer q.lock.Unlock()

	// If the queue is paused, return nothing
	if q.paused {
		return nil, 0
	}

	// If there are no messages, return nothing
	if len(q.messages) == 0 {
		return nil, 0
	}

	// Pop the first element and return that and the remaining length
	msg, remaining := q.messages[0], q.messages[1:]
	q.messages = remaining
	return msg, len(q.messages)
}

// requeue adds a message back to the front of the queue
func (q *queue) requeue(msg []byte) {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.messages = append([][]byte{msg}, q.messages...)
}

// pause temporarily blocks sending
func (q *queue) pause() {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.paused = true
}

// resume unblocks sending
func (q *queue) resume() {
	q.lock.Lock()
	defer q.lock.Unlock()

	q.paused = false
}
