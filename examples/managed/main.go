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

	}, interrupt.WithManagerOpts(manager.WithLogger(stdr.New(nil)), manager.WithPrompt(true)))

	time.Sleep(1 * time.Second)
}
