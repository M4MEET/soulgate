package process

import "sync"

// ringBuffer is a fixed-size circular byte buffer that retains the last N bytes
// written to it. It is safe for concurrent use.
type ringBuffer struct {
	buf  []byte
	size int
	pos  int
	full bool
	mu   sync.Mutex
}

// newRingBuffer allocates a ring buffer that holds up to size bytes.
func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

// Write appends p to the ring buffer, overwriting the oldest bytes when the
// buffer is full. Implements io.Writer.
func (r *ringBuffer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	n = len(p)

	// If the incoming slice is larger than the buffer, keep only the tail.
	if len(p) >= r.size {
		p = p[len(p)-r.size:]
		copy(r.buf, p)
		r.pos = 0
		r.full = true
		return n, nil
	}

	// How many bytes can we write before wrapping around?
	tail := r.size - r.pos
	if len(p) <= tail {
		copy(r.buf[r.pos:], p)
		r.pos += len(p)
		if r.pos == r.size {
			r.pos = 0
			r.full = true
		}
	} else {
		// Write in two parts: to the end of the buffer, then from the beginning.
		copy(r.buf[r.pos:], p[:tail])
		copy(r.buf, p[tail:])
		r.pos = len(p) - tail
		r.full = true
	}

	return n, nil
}

// String returns the contents of the buffer in chronological order (oldest
// byte first).
func (r *ringBuffer) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.full {
		return string(r.buf[:r.pos])
	}

	// Reassemble: from pos to end, then from start to pos.
	out := make([]byte, r.size)
	n := copy(out, r.buf[r.pos:])
	copy(out[n:], r.buf[:r.pos])
	return string(out)
}

// Len returns the number of bytes currently stored.
func (r *ringBuffer) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.full {
		return r.size
	}
	return r.pos
}
