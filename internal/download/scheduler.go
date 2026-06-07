package download

import (
	"context"
	"fmt"
	"sync"
)

// Status represents the outcome of a download.
type Status int

const (
	StatusPending   Status = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusSkipped
	StatusCancelled
)

// Job represents a single download job.
type Job struct {
	ID     string
	Episode string
	URL    string
	Status Status
	Err    error
}

// Result holds the outcome of a single download.
type Result struct {
	JobID  string
	Status Status
	Err    error
}

// Scheduler manages concurrent download jobs.
type Scheduler struct {
	concurrency int
}

// NewScheduler creates a download scheduler with the given concurrency.
func NewScheduler(concurrency int) *Scheduler {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 4 {
		concurrency = 4
	}
	return &Scheduler{concurrency: concurrency}
}

// Runner executes a single job.
type Runner interface {
	Run(ctx context.Context, job Job) error
}

// RunnerFunc is an adapter for running jobs.
type RunnerFunc func(ctx context.Context, job Job) error

// Run calls rf(ctx, job).
func (rf RunnerFunc) Run(ctx context.Context, job Job) error {
	return rf(ctx, job)
}

// RunAll executes all jobs, limiting concurrency. On context cancellation,
// it stops scheduling new jobs and returns partial results.
func (s *Scheduler) RunAll(ctx context.Context, jobs []Job, runner Runner) []Result {
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		sem   = make(chan struct{}, s.concurrency)
		done  = make(chan struct{})
	)

	results := make([]Result, len(jobs))

	for i := range jobs {
		select {
		case <-ctx.Done():
			for j := i; j < len(jobs); j++ {
				results[j] = Result{JobID: jobs[j].ID, Status: StatusCancelled}
			}
			return results
		default:
		}

		wg.Add(1)
		go func(j Job, idx int) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				mu.Lock()
				results[idx] = Result{JobID: j.ID, Status: StatusCancelled}
				mu.Unlock()
				return
			default:
			}

			err := runner.Run(ctx, j)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				stats := StatusFailed
				if ctx.Err() != nil {
					stats = StatusCancelled
				}
				results[idx] = Result{JobID: j.ID, Status: stats, Err: err}
			} else {
				results[idx] = Result{JobID: j.ID, Status: StatusCompleted}
			}
		}(jobs[i], i)
	}

	// Close done channel when all goroutines finish.
	go func() {
		wg.Wait()
		close(done)
	}()

	<-done
	return results
}

// AggregateExitCode returns the exit code for a set of results.
// 0 = full success + skips, 1 = some failures, 2 = all cancelled.
func AggregateExitCode(results []Result) int {
	hasFailure := false
	hasSuccess := false
	for _, r := range results {
		if r.Status == StatusCompleted || r.Status == StatusSkipped {
			hasSuccess = true
		}
		if r.Status == StatusFailed {
			hasFailure = true
		}
	}
	if hasFailure && !hasSuccess {
		return 1
	}
	if hasFailure {
		return 1
	}
	return 0
}

// Summary returns a text summary of results.
func Summary(results []Result) string {
	var completed, failed, cancelled, skipped int
	for _, r := range results {
		switch r.Status {
		case StatusCompleted:
			completed++
		case StatusFailed:
			failed++
		case StatusCancelled:
			cancelled++
		case StatusSkipped:
			skipped++
		}
	}
	return fmt.Sprintf("completed %d, failed %d, skipped %d, cancelled %d",
		completed, failed, skipped, cancelled)
}