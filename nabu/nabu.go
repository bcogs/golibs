// Package nabu converts text streams between character encodings and UTF-8.
//
// It looks encodings up by name, as they appear in HTTP headers, XML
// declarations or meta tags, and wraps a Reader or a Writer so that the code
// using it only ever deals with UTF-8:
//
//	ne, err := nabu.NewNamedEncoding("iso-8859-1")
//	if err != nil { ... }
//	r := nabu.NewReader(input, ne.Encoding) // decodes iso-8859-1 to UTF-8
//	w := nabu.NewWriter(output, ne.Encoding) // encodes UTF-8 to iso-8859-1
//	defer w.Close()
//
// Whatever the encoding, what nabu produces is well formed: input that isn't
// valid in the encoding it's converted from is replaced with U+FFFD rather than
// let through.  utf-8 is no exception - converting it is a real conversion, not
// a pass through - so no encoding is more trusted than the others.
//
// Reading never fails on malformed input, as UTF-8 can always represent U+FFFD.
// Writing usually does, as most encodings have no character for it.
//
// It's named after the Mesopotamian god of scribes and cuneiform.
package nabu

import (
	"io"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

// UTF8 is the utf-8 encoding.  Converting from UTF-8 to itself isn't a no-op:
// it sanitizes malformed input like any other encoding does.
var UTF8 = func() NamedEncoding {
	ne, err := NewNamedEncoding("utf-8")
	if err != nil {
		panic(err)
	}
	return ne
}()

// NamedEncoding is an encoding together with the name it was looked up by,
// which is worth keeping around to report what a stream was decoded from.
type NamedEncoding struct {
	Name     string
	Encoding encoding.Encoding
}

// NewNamedEncoding returns the encoding called name, which can be any label of
// the WHATWG encoding standard, such as "utf-8", "iso-8859-1" or "shift_jis".
// It fails if no encoding goes by that name.
func NewNamedEncoding(name string) (NamedEncoding, error) {
	encoding, err := htmlindex.Get(name)
	if err != nil {
		return NamedEncoding{}, err
	}
	return NamedEncoding{Name: name, Encoding: encoding}, nil
}

// String implements fmt.Stringer.
func (ne NamedEncoding) String() string {
	return ne.Name
}

// NewReader returns a Reader that reads r and decodes it from e to UTF-8.
//
// Bytes that don't form a valid character in e are replaced with U+FFFD, so
// what's read is always well formed UTF-8, and reading never fails on malformed
// input.
//
// Unlike NewWriter, it needs no closing: a decoder holds back an incomplete
// multibyte sequence until the bytes completing it are read, and there's
// nothing to flush once r is exhausted.
func NewReader(r io.Reader, e encoding.Encoding) io.Reader {
	return e.NewDecoder().Reader(r)
}

// NewWriter returns a WriteCloser that encodes to e what's written to it, and
// writes the result to output.
//
// What's written to it must be UTF-8.  Malformed bytes are replaced with
// U+FFFD, which most encodings have no character for, so writing them usually
// fails - as does writing a rune e can't represent.  Only utf-8 never fails,
// every rune being representable in it.
//
// Its Close must be called once everything has been written, as some bytes are
// only emitted at that point - the terminator of a stateful encoding, or the
// error of a trailing incomplete rune.  Close never closes output, which needn't
// even be closeable: it only writes the last bytes to it.
func NewWriter(output io.Writer, e encoding.Encoding) io.WriteCloser {
	return transform.NewWriter(output, e.NewEncoder())
}
