package queue

// Waiting reports how many Dequeue calls are blocked on an empty queue. Tests use it to know that a
// call has reached the point they want to interrupt.
func (q *Memory) Waiting() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.waiters)
}
