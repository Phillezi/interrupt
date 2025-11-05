package interrupt

import (
	"runtime"
	"strings"
)

// assertMainGoroutine panics if not running on the main goroutine.
func assertMainGoroutine() {
	buf := make([]byte, 64)
	n := runtime.Stack(buf, false)
	if !strings.HasPrefix(string(buf[:n]), "goroutine 1 ") {
		panic("interrupt.Main must be called from the main goroutine (goroutine 1)")
	}
}
