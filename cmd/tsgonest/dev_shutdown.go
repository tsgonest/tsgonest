package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// shutdownProc is what the signal handler needs from the child runner.
type shutdownProc interface {
	Stop() error
	Kill()
}

// signalCoalesceWindow covers the copies of one keypress. Under a task runner
// (turbo, pnpm run, concurrently) a single Ctrl-C reaches tsgonest more than
// once: the terminal delivers SIGINT to the whole foreground process group,
// and each wrapper forwards the signal to its child on top of that. Those
// copies land within milliseconds and mean one request to stop, not "force
// kill now".
const signalCoalesceWindow = 500 * time.Millisecond

// shutdownOnSignal runs once the first shutdown signal has arrived. It stops
// the child gracefully and treats a later, distinct signal as a request to
// force-kill and exit at once.
//
// Whatever path is taken, the child is dead before exit runs. The earlier
// version exited on the second signal straight after *launching* the stop in
// a goroutine; when the second signal was already buffered, the select took
// it before that goroutine had sent SIGTERM, and the child outlived tsgonest
// with no parent, still bound to its port. The next `tsgonest dev` then
// failed with EADDRINUSE, which reads as "the crash blocked the port".
func shutdownOnSignal(sigCh <-chan os.Signal, proc shutdownProc, stderr io.Writer, now func() time.Time, exit func(int)) {
	first := now()
	stopDone := make(chan struct{})
	go func() {
		if proc != nil {
			proc.Stop()
		}
		close(stopDone)
	}()
	for {
		select {
		case <-stopDone:
			return
		case <-sigCh:
			if now().Sub(first) < signalCoalesceWindow {
				continue
			}
			fmt.Fprintln(stderr, "\nforce killing...")
			if proc != nil {
				proc.Kill()
			}
			exit(1)
			return
		}
	}
}
