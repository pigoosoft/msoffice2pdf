package metrics

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"msoffice2pdf/internal/domain"
	"msoffice2pdf/internal/queue"
	"msoffice2pdf/internal/repo"
)

const samplePollInterval = 2 * time.Second

type Sampler struct {
	Interval time.Duration
	Queue    *queue.Queue
	Uploads  *repo.UploadRepo
	Samples  *repo.PressureSampleRepo

	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopOnce sync.Once
}

func (s *Sampler) Start() {
	if s == nil {
		return
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.wg.Add(1)
	go s.loop()
}

func (s *Sampler) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Wait()
	})
}

func (s *Sampler) loop() {
	defer s.wg.Done()
	iv := s.Interval
	if iv <= 0 {
		iv = 10 * time.Second
	}
	poll := samplePollInterval
	if poll > iv {
		poll = iv
	}

	s.insert(Collect(time.Now(), s.Queue, s.Uploads))

	var acc *domain.PressureSample
	pollT := time.NewTicker(poll)
	writeT := time.NewTicker(iv)
	defer pollT.Stop()
	defer writeT.Stop()

	accumulate := func() {
		row := Collect(time.Now(), s.Queue, s.Uploads)
		if acc == nil {
			cpy := row
			acc = &cpy
			return
		}
		mergeLoadPeak(acc, row)
	}
	flush := func() {
		if acc == nil {
			accumulate()
		}
		if acc != nil {
			s.insert(*acc)
			acc = nil
		}
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-pollT.C:
			accumulate()
		case <-writeT.C:
			flush()
		}
	}
}

func (s *Sampler) insert(row domain.PressureSample) {
	if s.Samples == nil {
		return
	}
	if err := s.Samples.Insert(&row); err != nil {
		slog.Warn("metrics sample insert failed", "err", err)
	}
}

func (s *Sampler) CollectNow() domain.PressureSample {
	return Collect(time.Now(), s.Queue, s.Uploads)
}
