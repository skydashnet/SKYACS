package monitoring

import (
	"context"
	"log"
	"sync"
	"time"
)

type Scheduler struct {
	interval    time.Duration
	workers     int
	targets     []*Target
	targetsMu   sync.RWMutex
	resultsChan chan *CheckResult
	stopChan    chan struct{}
	// TODO: Add database connection untuk persist results
}

func NewScheduler(interval time.Duration, workers int) *Scheduler {
	return &Scheduler{
		interval:    interval,
		workers:     workers,
		targets:     make([]*Target, 0),
		resultsChan: make(chan *CheckResult, 100),
		stopChan:    make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("Monitoring scheduler started (interval: %v, workers: %d)", s.interval, s.workers)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go s.worker(ctx, &wg, i)
	}

	// Start result collector
	go s.collectResults(ctx)

	// Main scheduling loop
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runChecks(ctx)
		case <-ctx.Done():
			close(s.stopChan)
			wg.Wait()
			close(s.resultsChan)
			log.Println("Monitoring scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) worker(ctx context.Context, wg *sync.WaitGroup, id int) {
	defer wg.Done()
	log.Printf("Monitoring worker %d started", id)

	for {
		select {
		case <-s.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			// Worker akan digunakan oleh runChecks via channel/queue
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (s *Scheduler) runChecks(ctx context.Context) {
	s.targetsMu.RLock()
	targets := make([]*Target, len(s.targets))
	copy(targets, s.targets)
	s.targetsMu.RUnlock()

	if len(targets) == 0 {
		return
	}

	log.Printf("Running checks for %d targets", len(targets))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, s.workers)

	for _, target := range targets {
		if !target.Enabled {
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(t *Target) {
			defer wg.Done()
			defer func() { <-semaphore }()

			checker := GetChecker(t.Type)
			result := checker.Check(ctx, t)

			select {
			case s.resultsChan <- result:
			default:
				log.Printf("Result channel full, dropping result for target %d", t.ID)
			}
		}(target)
	}

	wg.Wait()
}

func (s *Scheduler) collectResults(ctx context.Context) {
	for {
		select {
		case result, ok := <-s.resultsChan:
			if !ok {
				return
			}
			s.handleResult(result)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) handleResult(result *CheckResult) {
	log.Printf("Check result - Target: %d, Status: %s, RTT: %v",
		result.TargetID, result.Status, result.ResponseTime)

	// TODO: Persist hasil ke database
	// TODO: Check alert thresholds
	// TODO: Send notification jika status berubah
}

// AddTarget add target ke scheduler
func (s *Scheduler) AddTarget(target *Target) {
	s.targetsMu.Lock()
	defer s.targetsMu.Unlock()
	s.targets = append(s.targets, target)
}

// RemoveTarget remove target dari scheduler
func (s *Scheduler) RemoveTarget(id int64) {
	s.targetsMu.Lock()
	defer s.targetsMu.Unlock()

	for i, t := range s.targets {
		if t.ID == id {
			s.targets = append(s.targets[:i], s.targets[i+1:]...)
			return
		}
	}
}

// GetStats return monitoring stats
func (s *Scheduler) GetStats() *Stats {
	s.targetsMu.RLock()
	defer s.targetsMu.RUnlock()

	stats := &Stats{TotalTargets: len(s.targets)}
	for _, t := range s.targets {
		switch t.LastStatus {
		case StatusUp:
			stats.Up++
		case StatusDown:
			stats.Down++
		default:
			stats.Unknown++
		}
	}

	return stats
}
