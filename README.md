# interrupt

A small Go package that provides a simple, structured way to handle graceful shutdowns using contexts and signal handling.

`interrupt` listens for operating system interrupt signals (e.g. `SIGINT` / Ctrl+C) and cancels a shared `context.Context`. All goroutines running under that context can then begin their graceful termination.

> [!TIP]: If you just need this then the built in `signal.NotifyContext(context.Background(), os.Interrupt)` might be enough.

When each worker completes, it calls a special `manager.DoneFunc`, and when all workers have exited, the main program can exit cleanly.
If a second interrupt signal is received before all workers finish, `Run()/Wait()` will return immediately, allowing the main thread to exit forcefully.

---

## Installation

```bash
go get github.com/Phillezi/interrupt
```

---

## Usage

There are two main APIs provided by this package.

---

### 1. The Manual API

You manage your own `manager.Manager` and register workers with `m.Add()` and `defer done()`, similar to how you would do with `sync.WaitGroup`.

#### Example

`examples/manual/main.go`

```go
package main

import (
	"fmt"
	"time"

	"github.com/Phillezi/interrupt/pkg/manager"

	"github.com/go-logr/stdr"
)

func main() {
	m := manager.NewManager(
		manager.WithLogger(stdr.New(nil)),
		manager.WithPrompt(true, nil),
	)

	done := m.Add()
	go func() {
		defer done()
		<-m.Context().Done()
		fmt.Println("worker shutting down...")
		time.Sleep(2 * time.Second)
		fmt.Println("worker done")
	}()

	fmt.Println("service running...")
	_ = m.Run()
}
```

Run this example and press `Ctrl+C`. The worker will receive the shutdown signal, complete its work, and exit gracefully. If you press `Ctrl+C` again before it finishes, `m.Run()` will return, causing the main thread exits immediately.

---

### 2. The Managed API

The managed API wraps the main function and handles everything for you.
It automatically spawns your main logic in a goroutine with a managed `context`, and handles waiting and shutdown from the main thread.

Use `interrupt.Main()` and spawn child goroutines with `m.Go()`.

> [!NOTE]: Use interrupt.Main in the main thread.

#### Example

`examples/managed/main.go`

```go
package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Phillezi/interrupt/pkg/interrupt"
	"github.com/Phillezi/interrupt/pkg/manager"

	"github.com/go-logr/stdr"
)

func main() {
	interrupt.Main(func(m manager.ManagedManager, cancel context.CancelFunc) {
		fmt.Println("Hello world!")

		var g atomic.Int32
		m.Go(func(ctx context.Context) {
			id := g.Add(1)
			fmt.Printf("[goroutine %d] Hi, I am a goroutine\n", id)

			<-ctx.Done()
			fmt.Printf("[goroutine %d] I was cancelled! Starting graceful shutdown...\n", id)

			fmt.Printf("[goroutine %d] Sleeping 5s for cleanup...\n", id)
			time.Sleep(5 * time.Second)
			fmt.Printf("[goroutine %d] Done.\n", id)
		})

		m.Go(func(ctx context.Context) {
			id := g.Add(1)
			fmt.Printf("[goroutine %d] Hi, I am a goroutine\n", id)

			<-ctx.Done()
			fmt.Printf("[goroutine %d] I was cancelled! Starting graceful shutdown...\n", id)

			fmt.Printf("[goroutine %d] Sleeping 10s for cleanup...\n", id)
			time.Sleep(10 * time.Second)
			fmt.Printf("[goroutine %d] Done.\n", id)
		})

		m.Go(func(ctx context.Context) {
			id := g.Add(1)
			fmt.Printf("[goroutine %d] Hi, I am a goroutine\n", id)

			context.AfterFunc(ctx, func() {
				fmt.Printf("[goroutine %d] I was cancelled! Starting graceful shutdown...\n", id)

				fmt.Printf("[goroutine %d] Sleeping 10s for cleanup...\n", id)
				time.Sleep(3 * time.Second)
				fmt.Printf("[goroutine %d] Done.\n", id)
			})

			<-ctx.Done()
		})
	}, interrupt.WithManagerOpts(manager.WithLogger(stdr.New(nil)), manager.WithPrompt(true)))
}

```

When you press `Ctrl+C`, all goroutines receive the cancellation signal and execute their cleanup logic.

If you press `Ctrl+C` again before they finish, the main thread exits immediately.
