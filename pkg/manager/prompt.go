package manager

import (
	"fmt"
	"io"
)

const AnsiClearLine = "\033[2K\n"

func gracefulShutdownPrompt(out io.Writer) {
	fmt.Fprint(out, AnsiClearLine+"Press Ctrl+C again to forcefully exit.\n")
}
