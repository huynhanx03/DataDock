package engines

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/huynhanx03/datadock/internal/core/apperror"
)

var errExecutionCancelled = errors.New("query execution cancelled")

type ExecutionRegistry struct {
	mu         sync.Mutex
	active     map[string]context.CancelCauseFunc
	semaphore  chan struct{}
	closed     bool
	wait       sync.WaitGroup
	closeOnce  sync.Once
	closedDone chan struct{}
}

func NewExecutionRegistry(maxConcurrency int) *ExecutionRegistry {
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	return &ExecutionRegistry{
		active:     make(map[string]context.CancelCauseFunc),
		semaphore:  make(chan struct{}, maxConcurrency),
		closedDone: make(chan struct{}),
	}
}

func (registry *ExecutionRegistry) Begin(ctx context.Context, executionID string) (context.Context, func(), error) {
	if strings.TrimSpace(executionID) == "" {
		return nil, nil, apperror.NewValidation("execution id is required", nil)
	}
	select {
	case registry.semaphore <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, executionContextError(ctx)
	case <-registry.closedDone:
		return nil, nil, apperror.NewConflict("query execution is shutting down", nil)
	}
	executionContext, cancel := context.WithCancelCause(ctx)
	registry.mu.Lock()
	if registry.closed {
		registry.mu.Unlock()
		cancel(errExecutionCancelled)
		<-registry.semaphore
		return nil, nil, apperror.NewConflict("query execution is shutting down", nil)
	}
	if _, exists := registry.active[executionID]; exists {
		registry.mu.Unlock()
		cancel(errExecutionCancelled)
		<-registry.semaphore
		return nil, nil, apperror.NewConflict("execution id is already active", nil)
	}
	registry.active[executionID] = cancel
	registry.wait.Add(1)
	registry.mu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			registry.mu.Lock()
			if current, exists := registry.active[executionID]; exists && current != nil {
				delete(registry.active, executionID)
			}
			registry.mu.Unlock()
			cancel(nil)
			<-registry.semaphore
			registry.wait.Done()
		})
	}
	return executionContext, release, nil
}

func (registry *ExecutionRegistry) Cancel(ctx context.Context, executionID string) error {
	if strings.TrimSpace(executionID) == "" {
		return apperror.NewValidation("execution id is required", nil)
	}
	if err := ctx.Err(); err != nil {
		return executionContextError(ctx)
	}
	registry.mu.Lock()
	cancel := registry.active[executionID]
	registry.mu.Unlock()
	if cancel == nil {
		return apperror.NewNotFound("active query execution was not found", nil)
	}
	cancel(errExecutionCancelled)
	return nil
}

func (registry *ExecutionRegistry) Close(ctx context.Context) error {
	registry.closeOnce.Do(func() {
		registry.mu.Lock()
		registry.closed = true
		cancellations := make([]context.CancelCauseFunc, 0, len(registry.active))
		for _, cancel := range registry.active {
			cancellations = append(cancellations, cancel)
		}
		close(registry.closedDone)
		registry.mu.Unlock()
		for _, cancel := range cancellations {
			cancel(errExecutionCancelled)
		}
	})
	done := make(chan struct{})
	go func() {
		registry.wait.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func executionContextError(ctx context.Context) error {
	cause := context.Cause(ctx)
	if errors.Is(cause, errExecutionCancelled) || errors.Is(cause, context.Canceled) {
		return apperror.NewCancellation("query was cancelled", cause)
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return apperror.NewTimeout("query timed out", cause)
	}
	return apperror.NewInternal("query execution failed", cause)
}
