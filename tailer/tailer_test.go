package tail

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeAll(t *testing.T, writer io.Writer, data []byte) int {
	n, err := io.Copy(writer, bytes.NewReader(data))
	require.NoError(t, err)
	return int(n)
}

func testWithAtMostOneLinePerRead(t *testing.T, initialBufSize int, writeBlocks ...[]byte) {
	var buf bytes.Buffer
	expectedLine := ""
	tailer := NewLineTailer(&buf, initialBufSize)

	for _, wb := range writeBlocks {
		writeAll(t, &buf, wb)
		line, err := tailer.ReadLine()
		switch {
		case len(wb) <= 0:
			require.Equal(t, io.EOF, err, "%d %q", initialBufSize, writeBlocks)

			require.Nil(t, line, "%d %q", initialBufSize, writeBlocks)
		case wb[len(wb)-1] != '\n':
			require.Equal(t, io.EOF, err, "%d %q", initialBufSize, writeBlocks)

			require.Nil(t, line, "%d %q", initialBufSize, writeBlocks)
			expectedLine += string(wb)
		default:
			require.NoError(t, err, "%d %q", initialBufSize, writeBlocks)

			require.Equal(t, expectedLine+string(wb[:len(wb)-1]), string(line), "%d %q", initialBufSize, writeBlocks)
			expectedLine = ""
		}
	}
}

func TestReadlineWithAtMostOneLinePerRead(t *testing.T) {
	t.Parallel()
	makeBuf := func(b byte, repeats int, newLine bool) []byte {
		buf := bytes.Repeat([]byte{b}, repeats)
		if newLine {
			buf = append(buf, '\n')
		}
		return buf
	}

	for _, initialBufSize := range []int{1, 2, 3, 5, 10} {
		max := 4*initialBufSize + 4
		for i1 := 0; i1 < max; i1++ {
			b1 := makeBuf('1', i1/2, i1%2 == 1)
			for i2 := 0; i2 < max; i2++ {
				b2 := makeBuf('2', i2/2, i2%2 == 1)
				for l3 := 0; l3 < max; l3++ {
					b3 := makeBuf('3', l3/3, l3%2 == 1)
					testWithAtMostOneLinePerRead(t, initialBufSize, b1, b2, b3, []byte{'\n'})
				}
			}
		}
	}
}

func testWithMultipleLinesPerRead(t *testing.T, initialBufSize int, writeBlocks ...[]byte) {
	var buf bytes.Buffer
	tailer := NewLineTailer(&buf, initialBufSize)

	unread := []byte{}
	for _, wb := range writeBlocks {
		if len(wb) <= 0 {
			line, err := tailer.ReadLine()
			require.Equal(t, io.EOF, err, "%d %q", initialBufSize, writeBlocks)
			require.Nil(t, line, "%d %q", initialBufSize, writeBlocks)
			continue
		}
		writeAll(t, &buf, wb)
		unread = append(unread, wb...)
		for {
			i := bytes.IndexByte(unread, '\n')
			if i < 0 {
				break
			}
			line, err := tailer.ReadLine()
			require.NoError(t, err, "%d %q", initialBufSize, writeBlocks)
			require.Equal(t, string(unread[:i]), string(line), "%d %q", initialBufSize, writeBlocks)
			unread = unread[i+1:]
		}
	}
}

func TestReadlineWithMultipleLinesPerRead(t *testing.T) {
	t.Parallel()
	const s0 = "abcdefghi" // HEY! complexity of the test is O(N*2**N) where N is len(s0)
	for n := 0; n <= 1<<len(s0); n++ {
		// input will be set to all possible values that can be formed by transforming any number of characters of s0 into '\n'
		input := []byte(s0)
		for i, c := range s0 {
			if n&(1<<i) != 0 {
				input[i] = '\n'
			} else {
				input[i] = byte(c)
			}
			for i1 := 0; i1 < len(input); i1++ {
				b1 := input[:i1]
				for i2 := i1; i2 < len(input); i2++ {
					b2, b3 := input[i1:i2], input[i2:]
					for _, initialBufSize := range []int{1, 2, 3, 5, 10, 20} {
						testWithMultipleLinesPerRead(t, initialBufSize, b1, b2, b3, []byte{'\n'})
					}
				}
			}
		}
	}
}

// This test exercises the main use case of the LineTailer: tailing a real file rather than a bytes.Buffer.
func TestReadlineTailingAFile(t *testing.T) {
	t.Parallel()

	fileName := filepath.Join(t.TempDir(), "somefile")
	fileWriter, err := os.Create(fileName)
	require.NoError(t, err)
	defer fileWriter.Close()
	fileReader, err := os.Open(fileName)
	require.NoError(t, err)
	defer fileReader.Close()

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer listener.Close()
	var netReader net.Conn
	acceptErrChan := make(chan error)
	go func() {
		var err2 error
		netReader, err2 = listener.Accept()
		acceptErrChan <- err2
	}()
	netWriter, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer netWriter.Close()
	require.NoError(t, <-acceptErrChan)

	for _, tc := range []struct {
		name   string
		reader io.Reader
		writer io.Writer
	}{
		{"file", fileReader, fileWriter},
		{"tcp sockets", netReader, netWriter},
	} {
		tailer := NewLineTailer(tc.reader, 12)
		s := ""
		for _, x := range []struct{ write, expect string }{
			{"foo\nbar\nbaz\n", "foo,bar,baz,EOF"},
			{"foobarbazthisislong\n", "foobarbazthisislong"},
			{"\n\n\n\noof", ",,,,EOF,EOF,EOF"},
			{"\n", ",EOF,EOF"},
			{"rab\naaa", "rab"},
			{"bbb\n", "bbb,EOF"},
			{"\n\n\n", ",,,EOF,EOF,EOF,EOF"},
		} {
			writeAll(t, tc.writer, []byte(x.write))
			for _, expected := range bytes.Split([]byte(x.expect), []byte{','}) {
				if netConn, ok := tc.reader.(net.Conn); ok {
					netConn.SetReadDeadline(time.Now().Add(time.Second / 5))
				}
				line, err := tailer.ReadLine()
				if string(expected) == "EOF" {
					if assert.Error(t, err, "%s %q", tc.name, x.write) {
						assert.True(t, err == io.EOF || strings.Contains(err.Error(), "timeout"), "%s %q %s", tc.name, x.write, err)
					}
					assert.Nil(t, line, "%s %q", tc.name, x.write)
				} else {
					assert.NoError(t, err, "%s %q", tc.name, x.write)
					assert.Equal(t, s+string(expected), string(line), "%s %q", tc.name, x.write)
					s = ""
				}
			}
			if i := bytes.LastIndexByte([]byte(x.write), '\n'); i < len(x.write)-1 {
				s += string(x.write[i+1:])
			}
		}
	}
}

type mockReader struct {
	t           *testing.T
	readResults []string
	index       int
}

func (m *mockReader) read(to []byte) (int, error) {
	if m.index >= len(m.readResults) {
		return 0, io.EOF
	}
	i := m.index
	m.index++
	line := m.readResults[i]
	if len(line) > 0 && line[0] == 'R' {
		err, line := fmt.Errorf("%s", line), line[1:]
		return copy(to, []byte(line)), err
	} else {
		return copy(to, []byte(line)), nil
	}
}

func (m *mockReader) Read(to []byte) (int, error) {
	n, err := m.read(to)
	m.t.Logf("Read -> %d %q, %v", n, to[:n], err)
	return n, err
}

func TestReadlineWithPartialReadsInterruptedByTransientErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		readResults  string
		expectations string
	}{
		{"foo\n,Rbar\n,baz\n", "foo,bar,baz,E"},
		{"Rfoobar\nbarfoo\nbaz\n", "foobar,barfoo,baz,E"},
		{"Rfoo,Rbar\nbarfoo\nbaz", "R,foobar,barfoo,E"},
		{"Rfoo\n,Rbar,baz\n\n", "foo,R,barbaz,,E"},
		{"foo\n,bar,Rbaz\n,\n", "foo,barbaz,,E"},
	} {
		t.Logf("testcase %q", tc.readResults)
		tailer := NewLineTailer(&mockReader{t: t, readResults: strings.Split(tc.readResults, ",")}, 1000)
		for _, e := range strings.Split(tc.expectations, ",") {
			line, err := tailer.ReadLine()
			if len(e) > 0 {
				switch e[0] {
				case 'E':
					assert.Equal(t, io.EOF, err, "%q", e)
					e = e[1:]
				case 'R':
					assert.Error(t, err, "%q", e)
					e = e[1:]
				}
			} else {
				assert.NoError(t, err, "%q", e)
			}
			if e == "nil" {
				assert.Nil(t, line)
			} else {
				assert.Equal(t, e, string(line))
			}
		}
	}
}
