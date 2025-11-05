package manager

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Phillezi/interrupt/pkg/nopr"

	"github.com/go-logr/logr"
)

// Option defines a functional option for Manager.
type Option func(*ManagerImpl)

// ManagerImpl is the concrete implementation of Manager.
type ManagerImpl struct {
	*graceful
	parentCtx      context.Context
	ctx            context.Context
	cancel         context.CancelFunc
	gracefulCtx    context.Context
	gracefulCancel context.CancelFunc

	logger   logr.Logger
	prompt   io.Writer
	onExit   func(code int)
	signalCh <-chan os.Signal
}

// NewManager creates a new manager with functional options.
func NewManager(opts ...Option) Manager {
	m := &ManagerImpl{
		parentCtx: context.Background(),
		logger:    nopr.NopLogger(), // default NOP logger
	}

	for _, opt := range opts {
		opt(m)
	}

	if m.ctx == nil || m.gracefulCtx == nil {
		m.ctx, m.cancel = context.WithCancel(m.parentCtx)
		m.gracefulCtx, m.gracefulCancel = context.WithCancel(m.parentCtx)
	}

	if m.graceful == nil {
		m.graceful = newGraceful(m.gracefulCtx)
	}

	return m
}

// WithLogger sets a custom logger.
func WithLogger(l logr.Logger) Option {
	return func(m *ManagerImpl) {
		m.logger = l
	}
}

// WithPrompt enables prompt output to the given writer.
func WithPrompt(enabled bool, w ...io.Writer) Option {
	return func(m *ManagerImpl) {
		if enabled {
			var wr io.Writer = os.Stderr
			for _, ww := range w {
				if ww != nil {
					wr = ww
					break
				}
			}
			m.prompt = wr
		}
	}
}

// WithOnExit sets a callback for exit code.
func WithOnExit(f func(code int)) Option {
	return func(m *ManagerImpl) {
		m.onExit = f
	}
}

// WithSignalChannel allows using a custom channel for shutdown signals (useful for tests)
func WithSignalChannel(ch <-chan os.Signal) Option {
	return func(m *ManagerImpl) {
		m.signalCh = ch
	}
}

func WithContext(ctx context.Context) Option {
	return func(mi *ManagerImpl) {
		mi.parentCtx = ctx
	}
}

// Context returns the manager's context.
func (m *ManagerImpl) Context() context.Context {
	return m.ctx
}

func (m *ManagerImpl) Go(f func(ctx context.Context)) {
	go func() {
		done := m.Add()
		defer done()
		f(m.ctx)
	}()
}

// Run starts signal monitoring and blocks until shutdown is complete.
func (m *ManagerImpl) Run() error {

	var sigCh <-chan os.Signal
	if m.signalCh != nil {
		// Use provided channel (mocked for tests)
		sigCh = m.signalCh
	} else {
		// Default: register OS signals
		c := make(chan os.Signal, 2)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(c)
		sigCh = c
	}

	// Cancel context when a signal is received
	go func() {
		defer m.gracefulCancel()

		select {
		case sig := <-sigCh:
			m.logger.Info("received shutdown signal", "signal", sig)
			m.cancel()
		case <-m.ctx.Done():
			m.logger.Info("context canceled externally")
			return
		}

		if m.prompt != nil {
			gracefulShutdownPrompt(m.prompt)
		}

		select {
		case sig := <-sigCh:
			m.logger.Info("received second shutdown signal", "signal", sig)
		case <-m.parentCtx.Done():
			m.logger.Info("context canceled externally")
			return
		}
	}()

	<-m.graceful.Wait()

	if m.onExit != nil {
		m.onExit(0)
	}

	return nil
}
