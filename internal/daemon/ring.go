package daemon

import (
	"sync"

	"github.com/bx-team/irori/internal/models"
)

type Ring struct {
	mu    sync.RWMutex
	buf   []models.LogLine
	start int
	size  int
}

func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{buf: make([]models.LogLine, capacity)}
}

func (r *Ring) Append(l models.LogLine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	idx := (r.start + r.size) % len(r.buf)
	r.buf[idx] = l
	if r.size < len(r.buf) {
		r.size++
	} else {
		r.start = (r.start + 1) % len(r.buf)
	}
}

func (r *Ring) Tail(n int) []models.LogLine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n <= 0 || n > r.size {
		n = r.size
	}
	out := make([]models.LogLine, 0, n)
	for i := r.size - n; i < r.size; i++ {
		out = append(out, r.buf[(r.start+i)%len(r.buf)])
	}
	return out
}

func (r *Ring) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}
