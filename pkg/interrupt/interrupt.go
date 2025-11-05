package interrupt

import (
	"context"

	"github.com/Phillezi/interrupt/pkg/manager"
)

type InterruptConfig struct {
	baseContext context.Context
	manOpts     []manager.Option
}

type Option func(ic *InterruptConfig)

func WithBaseContext(ctx context.Context) Option {
	return func(ic *InterruptConfig) {
		ic.baseContext = ctx
	}
}

func WithManagerOpts(opts ...manager.Option) Option {
	return func(ic *InterruptConfig) {
		ic.manOpts = opts
	}
}

// Constructor for getting an interrupt manager that you can cancel with the provided context.CancelFunc
// Calling the cancel will omit the graceful wait
// note you will need to call wait on the returned manager on the main thread
func Cancellable(opts ...Option) (manager.Manager, context.CancelFunc) {
	var ic InterruptConfig = InterruptConfig{
		baseContext: context.Background(),
	}
	for _, opt := range opts {
		opt(&ic)
	}
	ctx, cancel := context.WithCancel(ic.baseContext)
	m := manager.NewManager(append(ic.manOpts, manager.WithContext(ctx))...)
	return m, cancel
}

// run in main on main thread
func Main(mainFunc func(m manager.ManagedManager, cancel context.CancelFunc), opts ...Option) {
	assertMainGoroutine()

	m, cancel := Cancellable(opts...)
	defer cancel()

	go mainFunc(m, cancel)

	m.Run()
}
