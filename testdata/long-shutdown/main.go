package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	shutdownWait := 5
	if s := os.Getenv("SHUTDOWN_WAIT"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			shutdownWait = v
		}
	}

	log("Server starting (PID: %d)", os.Getpid())
	log("SHUTDOWN_WAIT=%ds", shutdownWait)
	log("Ready and waiting for signals...")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	s := <-sig
	log("Received %s - starting graceful shutdown", s)
	log("Simulating cleanup work for %d seconds...", shutdownWait)
	time.Sleep(time.Duration(shutdownWait) * time.Second)
	log("Cleanup complete - exiting")
}

func log(format string, args ...any) {
	fmt.Printf("[%s] %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}
