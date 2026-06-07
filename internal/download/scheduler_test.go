package download

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSchedulerConcurrencyLimit(t *testing.T) {
	mu := sync.Mutex{}
	active := 0
	maxActive := 0

	runner := RunnerFunc(func(ctx context.Context, job Job) error {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		return nil
	})

	sched := NewScheduler(2)
	jobs := make([]Job, 10)
	for i := range jobs {
		jobs[i] = Job{ID: fmt.Sprintf("j%d", i)}
	}

	results := sched.RunAll(context.Background(), jobs, runner)
	if len(results) != 10 {
		t.Fatalf("len results = %d, want 10", len(results))
	}
	for _, r := range results {
		if r.Status != StatusCompleted {
			t.Errorf("job %s status = %v, want completed", r.JobID, r.Status)
		}
	}
	if maxActive != 2 {
		t.Errorf("maxActive = %d, want 2", maxActive)
	}
}

func TestSchedulerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	jobs := make([]Job, 20)
	for i := range jobs {
		jobs[i] = Job{ID: fmt.Sprintf("j%d", i)}
	}

	runner := RunnerFunc(func(ctx context.Context, job Job) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Millisecond):
			return nil
		}
	})

	sched := NewScheduler(1)
	cancel() // Cancel before scheduling.
	results := sched.RunAll(ctx, jobs, runner)

	anyComplete := false
	for _, r := range results {
		if r.Status == StatusCompleted {
			anyComplete = true
		}
	}
	if anyComplete {
		t.Error("expected no completed jobs after cancellation")
	}
}

func TestSchedulerFailureIsolation(t *testing.T) {
	madeFailure := false

	runner := RunnerFunc(func(ctx context.Context, job Job) error {
		if job.ID == "fail" && !madeFailure {
			madeFailure = true
			return errors.New("simulated failure")
		}
		return nil
	})

	jobs := []Job{
		{ID: "j1"},
		{ID: "fail"},
		{ID: "j2"},
	}

	sched := NewScheduler(2)
	results := sched.RunAll(context.Background(), jobs, runner)

	if len(results) != 3 {
		t.Fatalf("len = %d, want 3", len(results))
	}
	if results[1].Status != StatusFailed {
		t.Errorf("fail status = %v, want failed", results[1].Status)
	}
	if results[2].Status != StatusCompleted {
		t.Errorf("j2 status = %v, want completed", results[2].Status)
	}
}

func TestAggregateExitCode(t *testing.T) {
	if AggregateExitCode([]Result{{Status: StatusCompleted}}) != 0 {
		t.Error("all success = 0")
	}
	if AggregateExitCode([]Result{{Status: StatusCompleted}, {Status: StatusFailed}}) != 1 {
		t.Error("mixed = 1")
	}
}
