package iotest

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

var errInjected = errors.New("injected")

const data = "0123456789"

// recorder records the size of every Read/Write it's asked to perform.
type recorder struct {
	buf   bytes.Buffer
	sizes []int
}

func (r *recorder) Read(p []byte) (int, error) {
	n, err := r.buf.Read(p)
	r.sizes = append(r.sizes, n)
	return n, err
}

func (r *recorder) Write(p []byte) (int, error) {
	r.sizes = append(r.sizes, len(p))
	return r.buf.Write(p)
}

// readAll reads from r bufSize bytes at a time, tolerating one occurrence of
// errInjected, and returns what it read and how many times it saw the error.
func readAll(t *testing.T, r io.Reader, bufSize int) (string, int) {
	t.Helper()
	var out bytes.Buffer
	injected := 0
	buf := make([]byte, bufSize)
	for {
		n, err := r.Read(buf)
		out.Write(buf[:n])
		switch {
		case errors.Is(err, errInjected):
			injected++
		case err == io.EOF:
			return out.String(), injected
		default:
			require.NoError(t, err)
		}
		require.LessOrEqual(t, injected, 1)
	}
}

func TestRInjectorNoError(t *testing.T) {
	t.Parallel()
	for split := 0; split <= len(data)+1; split++ {
		for _, bufSize := range []int{1, 3, len(data), len(data) * 2} {
			rec := &recorder{}
			rec.buf.WriteString(data)
			ri := &RInjector{Reader: rec, SplitOffset: split}
			got, injected := readAll(t, ri, bufSize)
			require.Equal(t, data, got)
			require.Equal(t, 0, injected)
			require.Equal(t, len(data), ri.Offset())
			// no read of the wrapped Reader ever crossed SplitOffset
			offset := 0
			for _, n := range rec.sizes {
				require.False(t, offset < split && offset+n > split, "read of %d bytes at offset %d crosses split %d", n, offset, split)
				offset += n
			}
		}
	}
}

func TestRInjectorError(t *testing.T) {
	t.Parallel()
	for split := 0; split <= len(data)+1; split++ {
		for _, bufSize := range []int{1, 3, len(data), len(data) * 2} {
			var buf bytes.Buffer
			buf.WriteString(data)
			ri := &RInjector{Reader: &buf, SplitOffset: split, Err: errInjected}
			got, injected := readAll(t, ri, bufSize)
			require.Equal(t, data, got)
			require.Equal(t, len(data), ri.Offset())
			if split > len(data) {
				require.Equal(t, 0, injected) // the split is never reached
			} else {
				require.Equal(t, 1, injected)
			}
		}
	}
}

func TestRInjectorErrorOffset(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	buf.WriteString(data)
	ri := &RInjector{Reader: &buf, SplitOffset: 4, Err: errInjected}
	p := make([]byte, len(data))
	n, err := ri.Read(p)
	require.ErrorIs(t, err, errInjected)
	require.Equal(t, "0123", string(p[:n]))
	require.Equal(t, 4, ri.Offset())
	// the error is injected only once, and reading resumes where it stopped
	n, err = ri.Read(p)
	require.NoError(t, err)
	require.Equal(t, "456789", string(p[:n]))
	require.Equal(t, len(data), ri.Offset())
}

func TestRInjectorSplitOffsetZero(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	buf.WriteString(data)
	ri := &RInjector{Reader: &buf, SplitOffset: 0, Err: errInjected}
	p := make([]byte, len(data))
	n, err := ri.Read(p)
	require.ErrorIs(t, err, errInjected)
	require.Equal(t, 0, n)
	require.Equal(t, 0, ri.Offset())
	n, err = ri.Read(p)
	require.NoError(t, err)
	require.Equal(t, data, string(p[:n]))
}

// writeAll writes data to w in chunkSize slices, retrying whatever a short
// write left over, and returns how many times it saw errInjected.
func writeAll(t *testing.T, w io.Writer, chunkSize int) int {
	t.Helper()
	injected := 0
	for todo := []byte(data); len(todo) > 0; {
		chunk := todo
		if len(chunk) > chunkSize {
			chunk = chunk[:chunkSize]
		}
		n, err := w.Write(chunk)
		if errors.Is(err, errInjected) {
			injected++
		} else {
			require.NoError(t, err)
			require.Equal(t, len(chunk), n)
		}
		require.LessOrEqual(t, injected, 1)
		todo = todo[n:]
	}
	return injected
}

func TestWInjectorNoError(t *testing.T) {
	t.Parallel()
	for split := 0; split <= len(data)+1; split++ {
		for _, chunkSize := range []int{1, 3, len(data)} {
			rec := &recorder{}
			wi := &WInjector{Writer: rec, SplitOffset: split}
			require.Equal(t, 0, writeAll(t, wi, chunkSize))
			require.Equal(t, data, rec.buf.String())
			require.Equal(t, len(data), wi.Offset())
			// no write to the wrapped Writer ever crossed SplitOffset
			offset := 0
			for _, n := range rec.sizes {
				require.False(t, offset < split && offset+n > split, "write of %d bytes at offset %d crosses split %d", n, offset, split)
				offset += n
			}
		}
	}
}

func TestWInjectorError(t *testing.T) {
	t.Parallel()
	for split := 0; split <= len(data)+1; split++ {
		for _, chunkSize := range []int{1, 3, len(data)} {
			var buf bytes.Buffer
			wi := &WInjector{Writer: &buf, SplitOffset: split, Err: errInjected}
			injected := writeAll(t, wi, chunkSize)
			require.Equal(t, data, buf.String())
			require.Equal(t, len(data), wi.Offset())
			if split > len(data) {
				require.Equal(t, 0, injected) // the split is never reached
			} else {
				require.Equal(t, 1, injected)
			}
		}
	}
}

func TestWInjectorErrorOffset(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	wi := &WInjector{Writer: &buf, SplitOffset: 4, Err: errInjected}
	n, err := wi.Write([]byte(data))
	require.ErrorIs(t, err, errInjected)
	require.Equal(t, 4, n)
	require.Equal(t, "0123", buf.String())
	require.Equal(t, 4, wi.Offset())
	// the error is injected only once, and writing resumes where it stopped
	n, err = wi.Write([]byte(data[4:]))
	require.NoError(t, err)
	require.Equal(t, len(data)-4, n)
	require.Equal(t, data, buf.String())
	require.Equal(t, len(data), wi.Offset())
}

func TestWInjectorSplitOffsetZero(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	wi := &WInjector{Writer: &buf, SplitOffset: 0, Err: errInjected}
	n, err := wi.Write([]byte(data))
	require.ErrorIs(t, err, errInjected)
	require.Equal(t, 0, n)
	require.Equal(t, 0, wi.Offset())
	n, err = wi.Write([]byte(data))
	require.NoError(t, err)
	require.Equal(t, len(data), n)
	require.Equal(t, data, buf.String())
}

// TestWInjectorNoErrorIoCopy checks that a nil Err doesn't break io.Copy, which
// rejects short writes.
func TestWInjectorNoErrorIoCopy(t *testing.T) {
	t.Parallel()
	for split := 0; split <= len(data)+1; split++ {
		var buf bytes.Buffer
		wi := &WInjector{Writer: &buf, SplitOffset: split}
		n, err := io.Copy(wi, bytes.NewReader([]byte(data)))
		require.NoError(t, err)
		require.Equal(t, int64(len(data)), n)
		require.Equal(t, data, buf.String())
	}
}
