package stream

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	rpc "miren.dev/runtime/pkg/rpc"
)

type Thing struct {
	Name string `json:"name"`
}

func TestStream(t *testing.T) {
	t.Run("can send a stream of values", func(t *testing.T) {
		r := require.New(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
		r.NoError(err)

		serv := ss.Server()

		var vals []int

		serv.ExposeValue("stream", ReadStream(func(val int) error {
			vals = append(vals, val)
			return nil
		}))

		cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
		r.NoError(err)

		c, err := cs.Connect(ss.ListenAddr(), "stream")
		r.NoError(err)

		css, err := ClientSend[int](c)
		r.NoError(err)

		r.NoError(css.Send(ctx, 42))
		r.NoError(css.Send(ctx, 100))
		r.NoError(css.Send(ctx, 111))

		r.Equal([]int{42, 100, 111}, vals)
	})

	t.Run("can send a stream of structs", func(t *testing.T) {
		r := require.New(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
		r.NoError(err)

		serv := ss.Server()

		var vals []*Thing

		serv.ExposeValue("stream", ReadStream(func(val *Thing) error {
			vals = append(vals, val)
			return nil
		}))

		cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
		r.NoError(err)

		c, err := cs.Connect(ss.ListenAddr(), "stream")
		r.NoError(err)

		css, err := ClientSend[*Thing](c)
		r.NoError(err)

		r.NoError(css.Send(ctx, &Thing{Name: "foo"}))

		r.Equal([]*Thing{
			{Name: "foo"},
		}, vals)

	})

	t.Run("ChanWriter closes output channel when stream ends", func(t *testing.T) {
		r := require.New(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Server side: expose a RecvStream backed by a channel
		sourceCh := make(chan int)

		ss, err := rpc.NewState(ctx, rpc.WithSkipVerify)
		r.NoError(err)

		serv := ss.Server()
		serv.ExposeValue("stream", AdaptRecvStream(ChanReader(sourceCh)))

		// Client side: connect and wrap as RecvStreamClient
		cs, err := rpc.NewState(ctx, rpc.WithSkipVerify)
		r.NoError(err)

		c, err := cs.Connect(ss.ListenAddr(), "stream")
		r.NoError(err)

		rsc := NewRecvStreamClient[int](c)

		// Wire ChanWriter: reads from rsc, writes to outputCh
		outputCh := make(chan int, 10)
		ChanWriter(ctx, rsc, outputCh)

		// Send a value through and verify it arrives
		sourceCh <- 42
		select {
		case v := <-outputCh:
			r.Equal(42, v)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for value")
		}

		// Close the source channel, which should cause ChanWriter to
		// close the output channel (this is the bug fix for MIR-622)
		close(sourceCh)

		select {
		case _, ok := <-outputCh:
			r.False(ok, "output channel should be closed")
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for output channel to close")
		}
	})
}
