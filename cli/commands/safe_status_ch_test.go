package commands

import (
	"context"
	"sync"
	"testing"

	"github.com/moby/buildkit/client"
)

func TestSafeStatusChNoPanicOnConcurrentClose(t *testing.T) {
	const senders = 16
	const sendsPerGoroutine = 1000

	s := &safeStatusCh{ch: make(chan *client.SolveStatus, 4)}

	// Drain the channel so senders don't block forever.
	var drainerWG sync.WaitGroup
	drainerWG.Add(1)
	go func() {
		defer drainerWG.Done()
		for range s.ch {
		}
	}()

	ctx := context.Background()

	var senderWG sync.WaitGroup
	senderWG.Add(senders)
	for i := 0; i < senders; i++ {
		go func() {
			defer senderWG.Done()
			for j := 0; j < sendsPerGoroutine; j++ {
				if err := s.Send(ctx, &client.SolveStatus{}); err != nil {
					t.Errorf("Send returned error: %v", err)
					return
				}
			}
		}()
	}

	// Close concurrently with sends. The race detector + nopanic semantics are
	// what we're verifying here.
	go func() {
		s.Close()
	}()

	senderWG.Wait()
	// Final close in case the goroutine above lost the race.
	s.Close()
	// Idempotent Close.
	s.Close()
	drainerWG.Wait()
}

func TestSafeStatusChSendAfterCloseIsNoop(t *testing.T) {
	s := &safeStatusCh{ch: make(chan *client.SolveStatus, 1)}
	s.Close()

	if err := s.Send(context.Background(), &client.SolveStatus{}); err != nil {
		t.Fatalf("Send after Close returned error: %v", err)
	}
}

func TestSafeStatusChSendRespectsContext(t *testing.T) {
	// Unbuffered channel with no consumer → Send blocks until ctx is cancelled.
	s := &safeStatusCh{ch: make(chan *client.SolveStatus)}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.Send(ctx, &client.SolveStatus{})
	}()

	cancel()

	if err := <-done; err == nil {
		t.Fatal("Send did not return ctx.Err() after cancellation")
	}

	// Drain so a follow-up Close on a buffered scenario wouldn't leak; here the
	// channel has no buffered values so just closing is safe.
	s.Close()
}
