package tail

import (
	"bytes"
	"io"
)

// LineTailer reads line by line from an io.Reader and supports polling it when reaching EOF, in a tail -f fashion.
// Example use, mimicking tail -f:
//
//	file, _ := os.Open("my-file.txt")
//	tailer := NewLineTailer(file, 1024 * 1024)
//	for {
//		line, err := tailer.ReadLine()
//		// caveat: line is a reference to the tailer's internal buffer,
//		// later calls to ReadLine can corrupt it
//		switch err {
//		case nil: fmt.Println(string(line))
//		case io.EOF: time.Sleep(time.Second / 10)
//		default: panic(fmt.Errorf("error when reading file: %w", err))
//		}
//	}
type LineTailer struct {
	Reader     io.Reader
	buffer     []byte
	lineStart  int // offset in buffer of the current line
	readOffset int // offset in buffer where the next bytes from Reader should be written
	scanOffset int // offset in buffer where we should resume looking for '\n'
}

// NewLineTailer builds a new LineTailer.
// Set initialBufSize to the size of the buffer to use initially, it will be grown if lines don't fit in it.
// The maximum size of an I/O read is the size of that buffer, so make it large enough to avoid many small reads when tailing files.
func NewLineTailer(reader io.Reader, initialBufSize int) *LineTailer {
	return &LineTailer{Reader: reader, buffer: make([]byte, initialBufSize)}
}

// ReadLine returns the next line read (or already buffered) from the io.Reader , with its '\n' stripped.
// CAVEAT: the returned line is a reference to the LineTailer's internal buffer,
// later calls to ReadLine can corrupt it.  If you need to use it after the next
// call to ReadLine, make a copy of it.
// If an error (including io.EOF) occurs before a '\n' is found, it returns nil,
// and the error itself.
// It's expected that it will happen, especially with io.EOF, and the LineTailer
// keeps working just fine even after that, assuming the reader itself keeps
// providing its byte stream on further calls to Read: further calls to ReadLine
// keep returning next lines, without skipping anything, rewinding, or other
// similar blunders.
func (t *LineTailer) ReadLine() ([]byte, error) {
	for {
		if n := t.readOffset - t.scanOffset; n > 0 {
			if line := t.scan(); line != nil {
				return line, nil
			}
		}
		n, err := t.Reader.Read(t.buffer[t.readOffset:])
		t.readOffset += n // yes, even if err isn't nil
		line := t.scan()  // yes, even if err isn't nil
		if line != nil {
			return line, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func (t *LineTailer) scan() []byte {
	k := bytes.IndexByte(t.buffer[t.scanOffset:t.readOffset], '\n')
	if k < 0 {
		t.scanOffset = t.readOffset
		if t.readOffset >= len(t.buffer) {
			if t.lineStart > len(t.buffer)/2 {
				t.scanOffset = copy(t.buffer, t.buffer[t.lineStart:t.readOffset])
				t.readOffset = t.scanOffset
				t.lineStart = 0
			} else { // double the buffer size
				t.buffer = append(t.buffer, t.buffer...)
			}
		}
		return nil
	}
	lineEnd := t.scanOffset + k
	line := append([]byte{}, t.buffer[t.lineStart:lineEnd]...) // makes a copy
	t.scanOffset = lineEnd + 1
	t.lineStart = t.scanOffset
	return line
}
