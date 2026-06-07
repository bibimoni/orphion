package download

import (
	"context"
	"sync"
)

// Status represents the outcome of a download.
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusSkipped
	StatusCancelled
)

// Job represents a single download job.
type Job struct {
	ID      string
	Episode string
	Title   string
	URL     string
	Status  Status
	Err     error
}

// Result holds the outcome of a single download.
type Result struct {
	JobID  string
	Status Status
	Err    error
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

// RunAll executes all jobs, limiting concurrency. On context cancellation,
// it stops scheduling new jobs but waits for started jobs to complete.
func (s *Scheduler) RunAll(ctx context.Context, jobs []Job, runner Runner) []Result {
	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		sem       = make(chan struct{}, s.concurrency)
		cancelled bool
	)

	results := make([]Result, len(jobs))

	for i := range jobs {
		mu.Lock()
		if cancelled {
			mu.Unlock()
			results[i] = Result{JobID: jobs[i].ID, Status: StatusCancelled}
			continue
		}
		mu.Unlock()

		// Block until a slot is available.
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			mu.Lock()
			cancelled = true
			mu.Unlock()
			results[i] = Result{JobID: jobs[i].ID, Status: StatusCancelled}
			continue
		}

		wg.Add(1)
		go func(j Job, idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			// Check for cancellation before starting.
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

	// Start a goroutine that waits for all jobs, then closes done.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	<-done
	return results
}

// AggregateExitCode returns the exit code for a set of results.
func AggregateExitCode(results []Result) int {
	hasFailure := false
	for _, r := range results {
		if r.Status == StatusFailed {
			hasFailure = true
		}
	}
	if hasFailure {
		return 1
	}
	return 0
}
