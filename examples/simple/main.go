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
