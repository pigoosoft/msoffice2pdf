package queue

import "sync"

type slotLimiter struct {
	mu       sync.Mutex
	cond     *sync.Cond
	max      int
	min      int
	cur      int
	inflight int
	stopped  bool
}

func newSlotLimiter(max, min int) *slotLimiter {
	if max < 1 {
		max = 1
	}
	if min < 1 {
		min = 1
	}
	if min > max {
		min = max
	}
	s := &slotLimiter{max: max, min: min, cur: max}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func (s *slotLimiter) acquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		if s.stopped {
			return false
		}
		if s.inflight < s.cur {
			s.inflight++
			return true
		}
		s.cond.Wait()
	}
}

func (s *slotLimiter) release() {
	s.mu.Lock()
	if s.inflight > 0 {
		s.inflight--
	}
	s.cond.Signal()
	s.mu.Unlock()
}

func (s *slotLimiter) setCurrent(n int) (old int, changed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n < s.min {
		n = s.min
	}
	if n > s.max {
		n = s.max
	}
	old = s.cur
	if n == old {
		return old, false
	}
	s.cur = n
	s.cond.Broadcast()
	return old, true
}

func (s *slotLimiter) current() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

func (s *slotLimiter) snapshot() (inflight, max, min int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inflight, s.max, s.min
}

func (s *slotLimiter) wakeAll() {
	s.mu.Lock()
	s.stopped = true
	s.cond.Broadcast()
	s.mu.Unlock()
}
