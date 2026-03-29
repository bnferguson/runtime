package ui

import "io"

// TerminalError is an optional interface for errors that can render a rich,
// human-friendly representation to a terminal. Errors that implement this
// interface get colorized, multi-line output with source context when
// displayed through the CLI.
//
// The Error() method should still return a plain-text representation suitable
// for logging and wrapping. WriteForTerminal provides the enhanced version
// for interactive use.
//
// This follows the same pattern as io.WriterTo and http.Flusher — a type
// assertion at the display boundary unlocks richer behavior.
//
// Design note: this interface is rendering-only. The underlying error type
// should carry structured data (source locations, hints, etc.) as exported
// fields so that the data can be serialized over RPC. Client-side code can
// then reconstruct a TerminalError from the wire data for local rendering.
type TerminalError interface {
	error
	WriteForTerminal(w io.Writer)
}
