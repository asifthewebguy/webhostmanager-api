package scheduler

import (
	"github.com/robfig/cron/v3"
)

// Scheduler wraps robfig/cron for periodic background job execution.
type Scheduler struct {
	c *cron.Cron
}

// New creates a new Scheduler instance.
func New() *Scheduler {
	return &Scheduler{c: cron.New()}
}

// Every30s registers a function to run every 30 seconds.
func (s *Scheduler) Every30s(fn func()) {
	s.c.AddFunc("@every 30s", fn) //nolint:errcheck — spec string is constant
}

// Start begins executing scheduled jobs in the background.
func (s *Scheduler) Start() {
	s.c.Start()
}

// Stop gracefully shuts down the scheduler, waiting for running jobs to finish.
func (s *Scheduler) Stop() {
	ctx := s.c.Stop()
	<-ctx.Done()
}
