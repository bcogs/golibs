// Package iotest provides helpers to test io.Reader and io.Writer
// implementations, and the code that uses them, by wrapping them to control
// where reads and writes are split and where errors occur.
//
// Example 1: check that a parser copes with a read that stops after 4 bytes
//
//	if err := parse(&RInjector{Reader: r, SplitOffset: 4}); err != nil { ...
//
// Example 2: check that a serializer copes with a write failing after 4 bytes
//
//	err := serialize(&WInjector{Writer: w, SplitOffset: 4, Err: syscall.EINTR})
//	// check that it recovers from err
package iotest

import "io"

// RInjector wraps a Reader and injects Err once, at SplitOffset.  Reads never
// cross that offset: the first one that would reach it stops exactly there and
// returns Err, and reads resume normally afterwards.
//
// Err can legitimately be nil, in which case the injection merely splits the
// reads at SplitOffset, which is useful to test readers at various read split
// points.
type RInjector struct {
	Reader      io.Reader // the wrapped Reader
	SplitOffset int       // offset in Reader at which Err is injected
	Err         error     // error to inject, possibly nil
	offset      int
	injected    bool
}

// Offset returns how many bytes have been read from Reader so far.
func (ri *RInjector) Offset() int { return ri.offset }

// Read implements io.Reader.
func (ri *RInjector) Read(p []byte) (int, error) {
	if !ri.injected && ri.offset <= ri.SplitOffset {
		if ri.offset == ri.SplitOffset {
			ri.injected = true
			return 0, ri.Err
		}
		if remaining := ri.SplitOffset - ri.offset; len(p) > remaining {
			p = p[:remaining]
		}
	}
	n, err := ri.Reader.Read(p)
	ri.offset += n
	if err == nil && !ri.injected && ri.offset == ri.SplitOffset {
		ri.injected = true
		err = ri.Err
	}
	return n, err
}

// WInjector wraps a Writer and injects Err once, at SplitOffset.  Writes to the
// wrapped Writer never cross that offset: the first one that would reach it
// stops exactly there and returns Err, and writes resume normally afterwards.
//
// Err can legitimately be nil, in which case the injection merely splits the
// writes to the wrapped Writer at SplitOffset - the whole slice is still
// written and Write reports no error, which is useful to test writers at
// various write split points.
type WInjector struct {
	Writer      io.Writer // the wrapped Writer
	SplitOffset int       // offset in Writer at which Err is injected
	Err         error     // error to inject, possibly nil
	offset      int
	injected    bool
}

// Offset returns how many bytes have been written to Writer so far.
func (wi *WInjector) Offset() int { return wi.offset }

// Write implements io.Writer.
func (wi *WInjector) Write(p []byte) (int, error) {
	written := 0
	if remaining := wi.SplitOffset - wi.offset; !wi.injected && remaining >= 0 && len(p) > remaining {
		if remaining > 0 {
			n, err := wi.Writer.Write(p[:remaining])
			wi.offset += n
			written = n
			if err != nil || n < remaining {
				return written, err
			}
		}
		wi.injected = true
		if wi.Err != nil {
			return written, wi.Err
		}
		p = p[remaining:]
	}
	n, err := wi.Writer.Write(p)
	wi.offset += n
	written += n
	if err == nil && !wi.injected && wi.offset == wi.SplitOffset {
		wi.injected = true
		err = wi.Err
	}
	return written, err
}
