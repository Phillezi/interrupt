package manager

import "context"

// Manager controls application lifecycle and signal handling.
// This manager is unmanaged and run is needed to be called on this on the main thread
// unlike the ManagedManager, which is managed.
type Manager interface {
	ManagedManager
	// Run starts signal monitoring and blocks until shutdown is complete.
	Run() error
}

// Managed manager is a manager that is being managed
type ManagedManager interface {
	Graceful
	// Context returns a context that is cancelled when shutdown begins.
	Context() context.Context
}

// Graceful manages graceful shutdown coordination.
type Graceful interface {
	// Add registers a component that must complete before shutdown finishes.
	// It returns a DoneFunc that should be called when the component has shut down.
	Add() DoneFunc

	Go(func(ctx context.Context))

	// Wait returns a channel that is closed when all components have completed.
	Wait() <-chan struct{}

	// Done returns true if shutdown has completed.
	Done() bool
}

// DoneFunc signals that a registered component has completed shutdown.
type DoneFunc func()
