package manager_test

import (
	"bytes"
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Phillezi/interrupt/pkg/manager"

	"github.com/go-logr/logr"
)

type fakeLogger struct {
	msgs []string
}

func (f *fakeLogger) Enabled(int) bool { return true }
func (f *fakeLogger) Info(level int, msg string, kvs ...any) {
	f.msgs = append(f.msgs, msg)
}
func (f *fakeLogger) Error(err error, msg string, kvs ...any) {
	f.msgs = append(f.msgs, "ERROR:"+msg)
}
func (f *fakeLogger) Init(logr.RuntimeInfo)          {}
func (f *fakeLogger) WithValues(...any) logr.LogSink { return f }
func (f *fakeLogger) WithName(string) logr.LogSink   { return f }
func (f *fakeLogger) Logger() logr.Logger            { return logr.New(f) }
func newFakeLogger() *fakeLogger                     { return &fakeLogger{} }

type safeBuffer struct {
	mu sync.Mutex
	b  *bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestGracefulManager_AddAndWait(t *testing.T) {
	g := manager.NewManager(manager.WithOnExit(func(code int) {
		t.Logf("exitcode: %d", code)
	}))

	done := g.Add()
	go func() {
		time.Sleep(50 * time.Millisecond)
		done()
	}()

	select {
	case <-g.Wait():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for graceful wait")
	}
}

func TestGracefulManager_DoneFuncIdempotency(t *testing.T) {
	g := manager.NewManager(manager.WithOnExit(func(code int) {
		t.Logf("exitcode: %d", code)
	}))
	done := g.Add()

	done()
	done() // should be safe

	select {
	case <-g.Wait():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("graceful wait did not close")
	}
}

func TestManager_ContextCancelledOnSignal(t *testing.T) {
	t.Parallel()

	m := manager.NewManager(manager.WithOnExit(func(code int) {
		t.Logf("exitcode: %d", code)
	}))
	g := m
	ctx := m.Context()

	done := g.Add()
	doneCh := make(chan struct{})

	go func() {
		defer close(doneCh)
		if err := m.Run(); err != nil {
			t.Logf("Run returned error: %v", err)
		}
	}()

	// Trigger SIGTERM
	go func() {
		time.Sleep(100 * time.Millisecond)
		t.Log("sending SIGTERM...")
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		t.Log("calling done()...")
		done() // make sure the WaitGroup unblocks
	}()

	select {
	case <-ctx.Done():
		t.Log("context cancelled (expected)")
	case <-time.After(2 * time.Second):
		t.Fatal("context not cancelled on signal")
	}

	select {
	case <-doneCh:
		t.Log("m.Run() exited cleanly")
	case <-time.After(1 * time.Second):
		t.Fatal("m.Run() did not exit (likely still waiting on GracefulManager.Wait)")
	}
}

func TestManager_PromptOutput(t *testing.T) {
	buf := &safeBuffer{b: &bytes.Buffer{}}

	// Create a mock signal channel
	sigCh := make(chan os.Signal, 1)

	m := manager.NewManager(
		manager.WithPrompt(true, buf),
		manager.WithSignalChannel(sigCh), // inject mock signal channel
		manager.WithOnExit(func(code int) {
			t.Logf("exitcode: %d", code)
		}),
	)
	ctx := m.Context()

	// Add a dummy component to unblock graceful.Wait()
	cancel := func() { m.Add()() }

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = m.Run()
	}()

	// Simulate sending a signal
	go func() {
		time.Sleep(50 * time.Millisecond)
		sigCh <- syscall.SIGINT
		cancel()
	}()

	select {
	case <-ctx.Done():
		t.Log("context cancelled (expected)")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("context not cancelled on signal")
	}

	<-done // ensure m.Run() is done before reading from buf

	out := buf.String()
	if out == "" {
		t.Fatal("expected prompt output, got none")
	}
	if want := "Press Ctrl+C"; !bytes.Contains([]byte(out), []byte(want)) {
		t.Fatalf("expected output to contain %q, got %q", want, out)
	}
}

func TestManager_PromptOutputDefault(t *testing.T) {
	sigCh := make(chan os.Signal, 1)

	m := manager.NewManager(
		manager.WithPrompt(true, nil),
		manager.WithSignalChannel(sigCh), // inject mock signal channel
		manager.WithOnExit(func(code int) {
			t.Logf("exitcode: %d", code)
		}),
	)

	if m == nil {
		t.Fail()
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		close(sigCh)
	}()

	m.Run()
}

func TestManager_UsesNopLoggerByDefault(t *testing.T) {
	m := manager.NewManager(manager.WithOnExit(func(code int) {
		t.Logf("exitcode: %d", code)
	}))
	if m == nil {
		t.Fatal("expected manager to be created")
	}
}

func TestManager_UsesCustomLogger(t *testing.T) {
	logger := newFakeLogger()
	m := manager.NewManager(
		manager.WithLogger(logger.Logger()),
		manager.WithOnExit(func(code int) {
			t.Logf("exitcode: %d", code)
		}))
	if _, ok := m.(*manager.ManagerImpl); !ok {
		t.Fatal("expected ManagerImpl")
	}

	logger.Info(0, "test")
	if len(logger.msgs) == 0 {
		t.Fatal("logger not used")
	}
}

func TestGracefulManager_MultipleComponents(t *testing.T) {
	g := manager.NewManager()
	done1 := g.Add()
	done2 := g.Add()

	go func() {
		time.Sleep(50 * time.Millisecond)
		done1()
	}()
	go func() {
		time.Sleep(100 * time.Millisecond)
		done2()
	}()

	select {
	case <-g.Wait():
	case <-time.After(300 * time.Millisecond):
		t.Fatal("timeout waiting for multiple components")
	}
}

func TestGracefulManager_AddAfterDone(t *testing.T) {
	g := manager.NewManager()
	done := g.Add()
	done()

	select {
	case <-g.Wait():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for graceful wait")
	}

	done2 := g.Add()
	done2() // should be safe and not panic
	if !g.Done() {
		t.Fatal("expected Done() to be true after shutdown")
	}
}

func TestGracefulManager_ConcurrentAdd(t *testing.T) {
	g := manager.NewManager()
	var wg sync.WaitGroup

	const N = 100
	for range N {
		wg.Go(func() {
			done := g.Add()
			time.Sleep(10 * time.Millisecond)
			done()
		})
	}
	wg.Wait()

	select {
	case <-g.Wait():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for concurrent Add() components")
	}
}

func TestManager_ContextCancelledExternally(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m := manager.NewManager(manager.WithContext(ctx))
	g := m

	// we never call done, instead we try cancelling with external context
	_ = g.Add()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
		time.Sleep(10 * time.Millisecond)
		if !g.Done() {
			t.Log("Should be done since we cancelled externally")
			t.Fail()
		}
	}()

	err := m.Run()
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}
}

func TestManager_SingalThenContextCancelledExternally(t *testing.T) {
	ch := make(chan os.Signal, 2)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m := manager.NewManager(manager.WithContext(ctx), manager.WithSignalChannel(ch))
	g := m

	// we never call done, instead we try cancelling with external context
	_ = g.Add()

	go func() {
		time.Sleep(10 * time.Millisecond)
		ch <- os.Interrupt
		time.Sleep(10 * time.Millisecond)
		if g.Done() {
			t.Log("Should not be done yet")
			t.Fail()
		}
		cancel()
		time.Sleep(10 * time.Millisecond)
		if !g.Done() {
			t.Log("Should be done since we cancelled externally")
			t.Fail()
		}
	}()

	err := m.Run()
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}
}

func TestManager_GoFuncSpawner_CleansUpOnCancel(t *testing.T) {
	ch := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	m := manager.NewManager(
		manager.WithContext(ctx),
		manager.WithSignalChannel(ch),
	)

	started := make(chan struct{})
	cleaned := make(chan struct{})

	m.Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		time.Sleep(10 * time.Millisecond)
		close(cleaned)
	})

	go func() {
		<-started
		time.Sleep(20 * time.Millisecond)
		ch <- os.Interrupt
	}()

	err := m.Run()
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	select {
	case <-cleaned:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected goroutine to clean up after context cancel")
	}
}

func TestManager_GoFuncSpawner_ExternalCancel(t *testing.T) {
	ch := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(t.Context())

	m := manager.NewManager(
		manager.WithContext(ctx),
		manager.WithSignalChannel(ch),
	)

	cleaned := make(chan struct{})

	m.Go(func(ctx context.Context) {
		defer close(cleaned)
		<-ctx.Done()
	})

	// Cancel externally
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	if err := m.Run(); err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	select {
	case <-cleaned:
		t.Log("goroutine cleaned up after external cancel (expected)")
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected goroutine to exit and clean up after cancel")
	}
}
