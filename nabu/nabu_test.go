package nabu

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/bcogs/golibs/iotest"
	"github.com/stretchr/testify/require"
)

func TestUTF8(t *testing.T) {
	t.Parallel()
	require.Equal(t, "utf-8", UTF8.Name)
}

func TestNewNamedEncoding(t *testing.T) {
	t.Parallel()
	ne, err := NewNamedEncoding("ascii")
	require.NoError(t, err)
	require.Equal(t, "ascii", ne.Name)
	ne, err = NewNamedEncoding("noexists")
	require.Error(t, err)
}

var (
	nonUTF8Once     sync.Once
	nonUTF8Encoding NamedEncoding
	nonUTF8Err      error
)

// nonUTF8 returns a supported non-utf-8 encoding.  It's safe to call it
// concurrently, and it only looks for the encoding once.
func nonUTF8(t *testing.T) NamedEncoding {
	t.Helper()
	nonUTF8Once.Do(func() {
		for _, name := range []string{"iso-8859-1", "cp850", "cp1250", "latin9"} {
			if nonUTF8Encoding, nonUTF8Err = NewNamedEncoding(name); nonUTF8Err == nil {
				return
			}
		}
	})
	require.NoError(t, nonUTF8Err) // we found at least one supported non-utf-8 encoding
	return nonUTF8Encoding
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	ne := nonUTF8(t)
	var err error
	const str = "éàî¿¡"
	var buf bytes.Buffer
	w := NewWriter(&buf, ne.Encoding)
	_, err = io.Copy(w, bytes.NewReader([]byte(str)))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	r := NewReader(&buf, ne.Encoding)
	var buf2 bytes.Buffer
	_, err = io.Copy(&buf2, r)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%x", []byte(str)), fmt.Sprintf("%x", buf2.Bytes()))
}

// randomSeed seeds the generation of the random test strings.  It's hardcoded
// so that a failing test can be rerun to investigate the failure.
const randomSeed = 0x5115ede7

// transformBufSize is the size of the buffers golang.org/x/text transforms data
// through.  Buffering bugs are most likely at multiples of it, so the test
// strings must be several times longer than that.
const transformBufSize = 4096

// splitTestEncodings are the encodings the split point tests run against.
// iso-8859-1 keeps the encoded form single byte, utf-16be makes it 2 bytes wide
// plus surrogate pairs, and gb18030 makes it vary between 1, 2 and 4 bytes, so
// that split points fall inside multibyte sequences on both sides.  iso-2022-jp
// is stateful: it switches charsets with escape sequences and must return to
// ASCII at the end, which only happens when the Writer is closed.
var splitTestEncodings = []string{"iso-8859-1", "utf-16be", "gb18030", "iso-2022-jp"}

// candidateRuneRanges are the half open rune ranges the test strings are made
// of.  They cover all the possible lengths of an UTF-8 sequence, from the 1
// byte ASCII range to the 4 bytes musical symbols and emoji ones.
var candidateRuneRanges = [][2]rune{
	{0x20, 0x80},       // printable ASCII, 1 byte
	{0xa0, 0x100},      // latin-1 supplement, 2 bytes
	{0x390, 0x3d0},     // greek, 2 bytes
	{0x4e00, 0x4e40},   // CJK, 3 bytes
	{0x1d11e, 0x1d130}, // musical symbols, 4 bytes
	{0x1f600, 0x1f620}, // emoji, 4 bytes
}

// encodableRunes returns the runes of candidateRuneRanges that survive a round
// trip through ne, so that the test strings only fail on split point bugs, and
// not because the encoding can't represent one of their runes.
func encodableRunes(t *testing.T, ne NamedEncoding) []rune {
	t.Helper()
	var runes []rune
	for _, rr := range candidateRuneRanges {
		for r := rr[0]; r < rr[1]; r++ {
			encoded, err := ne.Encoding.NewEncoder().String(string(r))
			if err != nil {
				continue
			}
			if decoded, err := ne.Encoding.NewDecoder().String(encoded); err == nil && decoded == string(r) {
				runes = append(runes, r)
			}
		}
	}
	require.NotEmpty(t, runes, "%s can't encode any of the candidate runes", ne.Name)
	return runes
}

// nameSeed derives a seed from an encoding name, so that adding an encoding to
// splitTestEncodings doesn't change the strings generated for the others.
func nameSeed(name string) uint64 {
	seed := uint64(14695981039346656037) // FNV-1a
	for _, b := range []byte(name) {
		seed = (seed ^ uint64(b)) * 1099511628211
	}
	return seed
}

// randomStrings returns count reproducible pseudo-random strings of at least
// size bytes each, made of runes ne can encode.  Their runes have various
// lengths, so each string aligns differently with the transform buffers - a
// coverage no single handcrafted string can provide, since sweeping the split
// points only varies where the input is cut, not where the buffers end.
func randomStrings(t *testing.T, ne NamedEncoding, count, size int) []string {
	t.Helper()
	runes := encodableRunes(t, ne)
	rng := rand.New(rand.NewPCG(randomSeed, nameSeed(ne.Name)))
	strs := make([]string, count)
	for i := range strs {
		var sb strings.Builder
		for sb.Len() < size {
			sb.WriteRune(runes[rng.IntN(len(runes))])
		}
		strs[i] = sb.String()
	}
	return strs
}

// splitTestCase is a test string, in its utf-8 and encoded forms.
type splitTestCase struct {
	name    string
	ne      NamedEncoding
	utf8    string
	encoded []byte
}

// splitTestCases returns the strings to sweep the split points of, for each
// encoding of splitTestEncodings.  In short mode, it returns fewer and shorter
// strings, since the sweeps take a time quadratic in the length of the strings.
func splitTestCases(t *testing.T) []splitTestCase {
	t.Helper()
	count, size := 8, 3*transformBufSize
	if testing.Short() {
		count, size = 1, transformBufSize+7
	}
	var cases []splitTestCase
	for _, name := range splitTestEncodings {
		ne, err := NewNamedEncoding(name)
		require.NoError(t, err)
		add := func(str string, mustEncode bool) {
			// The reference encoding comes from Bytes, which transforms with
			// atEOF set, so it includes whatever terminator the encoding needs.
			encoded, err := ne.Encoding.NewEncoder().Bytes([]byte(str))
			if err != nil {
				// Some of the handwritten strings are out of the repertoire of
				// some of the encodings, which is no reason to fail.
				require.False(t, mustEncode, "%s can't encode %q: %v", ne.Name, str, err)
				return
			}
			cases = append(cases, splitTestCase{
				name:    name + "/" + strconv.Itoa(len(cases)),
				ne:      ne,
				utf8:    str,
				encoded: encoded,
			})
		}
		for _, str := range []string{"", "a", "é", "hello", "éàî¿¡", "日本語", "abc日本語"} {
			add(str, false)
		}
		for _, str := range randomStrings(t, ne, count, size) {
			add(str, true) // randomStrings only uses runes ne can encode
		}
	}
	return cases
}

// sweepSplitOffsets calls check for every offset from 0 to n included, from
// parallel subtests each covering a chunk of consecutive offsets.  Chunking
// keeps the number of subtests reasonable while still using every core, and
// check runs on the goroutine of a subtest, as require needs.
func sweepSplitOffsets(t *testing.T, n int, check func(t *testing.T, offset int)) {
	const chunk = 256
	for start := 0; start <= n; start += chunk {
		t.Run(strconv.Itoa(start), func(t *testing.T) {
			t.Parallel()
			for offset := start; offset <= min(start+chunk-1, n); offset++ {
				check(t, offset)
			}
		})
	}
}

// requireSameBytes fails the test if want and got differ, reporting where they
// start to differ rather than dumping the whole buffers, which are huge.
func requireSameBytes(t *testing.T, want, got []byte, splitOffset int) {
	t.Helper()
	for i := 0; i < min(len(want), len(got)); i++ {
		if want[i] != got[i] {
			require.Failf(t, "unexpected output", "with the input split at %d, the output differs from byte %d on: want % x, got % x",
				splitOffset, i, want[i:min(i+8, len(want))], got[i:min(i+8, len(got))])
		}
	}
	require.Equalf(t, len(want), len(got), "with the input split at %d, the output has the wrong length", splitOffset)
}

// TestWriterSplitPoints checks that a Writer encodes correctly no matter where
// the data it's given is split, in particular in the middle of a rune.  The
// RInjector is what forces the split: without it, io.Copy would hand the whole
// string to the Writer in a single Write, as its buffer is far larger.
func TestWriterSplitPoints(t *testing.T) {
	t.Parallel()
	for _, tc := range splitTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sweepSplitOffsets(t, len(tc.utf8), func(t *testing.T, offset int) {
				var buf bytes.Buffer
				w := NewWriter(&buf, tc.ne.Encoding)
				src := &iotest.RInjector{Reader: strings.NewReader(tc.utf8), SplitOffset: offset}
				_, err := io.Copy(w, src)
				require.NoErrorf(t, err, "with the input split at %d", offset)
				require.NoErrorf(t, w.Close(), "closing the writer, with the input split at %d", offset)
				requireSameBytes(t, tc.encoded, buf.Bytes(), offset)
			})
		})
	}
}

// TestReaderSplitPoints checks that a Reader decodes correctly no matter where
// the reads of the encoded data are split, in particular in the middle of a
// multibyte sequence.
func TestReaderSplitPoints(t *testing.T) {
	t.Parallel()
	for _, tc := range splitTestCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sweepSplitOffsets(t, len(tc.encoded), func(t *testing.T, offset int) {
				var buf bytes.Buffer
				src := &iotest.RInjector{Reader: bytes.NewReader(tc.encoded), SplitOffset: offset}
				_, err := io.Copy(&buf, NewReader(src, tc.ne.Encoding))
				require.NoErrorf(t, err, "with the input split at %d", offset)
				requireSameBytes(t, []byte(tc.utf8), buf.Bytes(), offset)
			})
		})
	}
}

var errInjected = errors.New("injected error")

// TestWriterErrors checks that a Writer reports the errors of the Writer it
// wraps, whatever the offset at which they occur.
func TestWriterErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range splitTestCases(t) {
		if len(tc.encoded) == 0 {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Only a few offsets: unlike the split point sweeps, this exercises
			// one error path, not the handling of every rune boundary.
			for _, offset := range []int{0, 1, len(tc.encoded) / 2, len(tc.encoded) - 1} {
				wi := &iotest.WInjector{Writer: io.Discard, SplitOffset: offset, Err: errInjected}
				w := NewWriter(wi, tc.ne.Encoding)
				_, err := w.Write([]byte(tc.utf8))
				if err == nil {
					// The last bytes are only emitted on Close, so that's where
					// an error injected in them surfaces.
					err = w.Close()
				}
				require.ErrorIsf(t, err, errInjected, "with the error injected at %d", offset)
			}
		})
	}
}

// closeCounter counts how many times it's closed, to check that the WriteCloser
// NewWriter returns doesn't close the Writer it was given.
type closeCounter struct {
	io.Writer
	closes int
}

func (cc *closeCounter) Close() error { cc.closes++; return nil }

// TestUTF8Passthrough checks that utf-8 goes through a Reader and a Writer
// unchanged, and that closing the Writer doesn't close the output.
func TestUTF8Passthrough(t *testing.T) {
	t.Parallel()
	const str = "é日"
	var buf bytes.Buffer
	cc := &closeCounter{Writer: &buf}
	w := NewWriter(cc, UTF8.Encoding)
	_, err := w.Write([]byte(str))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.Equal(t, str, buf.String())
	require.Zero(t, cc.closes) // Close only flushes, it doesn't close the output
	read, err := io.ReadAll(NewReader(&buf, UTF8.Encoding))
	require.NoError(t, err)
	require.Equal(t, str, string(read))
}

// malformedReadCases are inputs that aren't valid in the encoding they're read
// as.  Each needs an encoding with genuinely invalid byte sequences, which rules
// out single byte charsets: in iso-8859-1, every byte but a handful is a valid
// character, so a test reading one would replace nothing and prove nothing.
var malformedReadCases = []struct{ encoding, input string }{
	{"utf-8", "a\xffb"},         // \xff can't appear in UTF-8 at all
	{"shift_jis", "a\x81\x20b"}, // \x81 is a lead byte, \x20 an invalid trail
	{"utf-16be", "a\x00b"},      // odd length, so the last code unit is truncated
}

// TestMalformedInput checks that malformed input is sanitized rather than let
// through, for utf-8 just like for any other encoding.
func TestMalformedInput(t *testing.T) {
	t.Parallel()
	// reading bytes that aren't valid in the source encoding yields U+FFFD
	for _, tc := range malformedReadCases {
		t.Run("read/"+tc.encoding, func(t *testing.T) {
			ne, err := NewNamedEncoding(tc.encoding)
			require.NoError(t, err)
			read, err := io.ReadAll(NewReader(strings.NewReader(tc.input), ne.Encoding))
			require.NoError(t, err)
			require.True(t, utf8.Valid(read), "%q isn't valid UTF-8", read)
			// the point of the test: the bad bytes were replaced, not let through
			require.Contains(t, string(read), "�")
		})
	}
	// writing malformed utf-8 doesn't let it through to the output
	for _, ne := range []NamedEncoding{UTF8, nonUTF8(t)} {
		t.Run("write/"+ne.Name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf, ne.Encoding)
			_, err1 := w.Write([]byte("a\xffb"))
			err2 := w.Close()
			require.NotContains(t, buf.String(), "\xff")
			if ne.Encoding == UTF8.Encoding {
				// utf-8 can encode U+FFFD, so it replaces rather than fails
				require.NoError(t, err1)
				require.NoError(t, err2)
				require.Equal(t, "a�b", buf.String())
			} else {
				// other encodings have no U+FFFD, so they bail out
				require.Error(t, errors.Join(err1, err2))
			}
		})
	}
	// writing a rune the encoding has no character for fails, except in utf-8,
	// which can represent every rune
	for _, ne := range []NamedEncoding{UTF8, nonUTF8(t)} {
		t.Run("unmappable/"+ne.Name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf, ne.Encoding)
			_, err1 := w.Write([]byte("a日b"))
			err2 := w.Close()
			if ne.Encoding == UTF8.Encoding {
				require.NoError(t, errors.Join(err1, err2))
				require.Equal(t, "a日b", buf.String())
			} else {
				require.Error(t, errors.Join(err1, err2))
			}
		})
	}
	// a trailing incomplete rune is reported by Close, whatever the encoding
	for _, ne := range []NamedEncoding{UTF8, nonUTF8(t)} {
		t.Run("truncated/"+ne.Name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf, ne.Encoding)
			_, err := w.Write([]byte("ab\xc3"))
			require.NoError(t, err)
			require.NotContains(t, buf.String(), "\xc3")
			if ne.Encoding == UTF8.Encoding {
				require.NoError(t, w.Close())
				require.Equal(t, "ab�", buf.String())
			} else {
				require.Error(t, w.Close())
			}
		})
	}
}

// TestWriterDoesntCloseOutput checks that closing the WriteCloser NewWriter
// returns doesn't close the Writer it was given, which may not even be
// closeable, and may still be needed by the caller.
func TestWriterDoesntCloseOutput(t *testing.T) {
	t.Parallel()
	for _, name := range splitTestEncodings {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ne, err := NewNamedEncoding(name)
			require.NoError(t, err)
			cc := &closeCounter{Writer: &bytes.Buffer{}}
			w := NewWriter(cc, ne.Encoding)
			_, err = w.Write([]byte("abc"))
			require.NoError(t, err)
			require.NoError(t, w.Close())
			require.Zero(t, cc.closes)
		})
	}
}
