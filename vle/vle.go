// Package vle provides variable length encoding (un-)marshaling.
package vle

import (
	"errors"
	"fmt"
	"io"
	"math/bits"
	"unsafe"

	"golang.org/x/exp/constraints"
)

func encodePositiveExceptFirstByte[N constraints.Integer](n, nostop N) ([]byte, byte) {
	buf, b := make([]byte, (unsafe.Sizeof(n) * 8 + 6) / 7), byte(n & 0x7f)
	i := len(buf) - 1
	for n >= nostop {
		buf[i], n = b, n >> 7
		b = byte(n & 0x7f) | 0x80
		i--
	}
	return buf[i:], b
}

// EncodeSigned marshals a signed integer.
func EncodeSigned[N constraints.Signed](n N) []byte {
	signBit := byte(0)
	if n < 0 {
	  n, signBit = -1-n, 0x40
	}
	buf, b := encodePositiveExceptFirstByte(n, 0x40)
	buf[0] = b | signBit
	return buf
}

// EncodeUnsigned marshals an unsigned integer.
func EncodeUnsigned[N constraints.Unsigned](n N) []byte {
	buf, b := encodePositiveExceptFirstByte(n, 0x80)
	buf[0] = b
	return buf
}

func parsePositive[N constraints.Integer](b []byte) (N, int) {
	n := N(0)
	for pos, val := range b {
		n = (n << 7) | (N(val) & 0x7f)
		if val & 0x80 == 0 {
			return n, pos + 1
		}
	}
	return 0, -1
}

// BufioReader is an interface used to read in a buffered manner.
// In practice, it should often be a bufio.Reader.
type BufioReader interface {
	Discard(n int) (discarded int, err error)
	Peek(n int) ([]byte, error)
}

// ReadSigned reads and parses a signed integer.
// It returns the integer, the number of bytes Discard()ed from the reader, and an error.
// Note the error can be non-nil even if an integer was successfully read and parsed.  The real test to know if an integer was parsed is to check that the number of bytes discarded (second returned item) is >0.
func ReadSigned[N constraints.Signed](r BufioReader) (N, int, error) {
	nBits := uint(unsafe.Sizeof(N(0)) * 8)
	maxBytes := int((nBits + 6) / 7)
	buf, err := r.Peek(int((nBits + 6) / 7))
	if len(buf) <= 0 { return 0, 0, err }
	b0 := buf[0]
	sign := /* -1 or 1 */ 1 - N(b0 & 0x40) / 0x20
	if b0 & 0x80 != 0 {
		n, l := parsePositive[N](buf[1:])
		if l < 0 {
			if len(buf) < maxBytes && !errors.Is(err, io.EOF) {
				return 0, 0, err
			}
			return 0, 0, fmt.Errorf("vle parse error: marshaled %T is longer than the expected %d bytes", n, len(buf))
		}
		if p := uint(bits.Len(uint(b0 & 0x3f))) + 7 * uint(l); p >= nBits {
			return 0, 0, fmt.Errorf("vle parse error: %T unmarshals to %d bits", n, p)
		}
		n = (N(b0 & 0x3f) << (7 * l)) | n
		l++
		r.Discard(l)
		return sign * (n + max(-sign, 0)), l, err
	} else {
		r.Discard(1)
		return sign * ((N(b0) & 0x3f) + (1 - sign) / 2), 1, err
	}
}

// ReadUnsigned reads and parses an unsigned integer.
// It returns the integer, the number of bytes Discard()ed from the reader, and an error.
// Note the error can be non-nil even if an integer was successfully read and parsed.  The real test to know if an integer was parsed is to check that the number of bytes discarded (second returned item) is >0.
func ReadUnsigned[N constraints.Unsigned](r BufioReader) (N, int, error) {
	nBits := uint(unsafe.Sizeof(N(0)) * 8)
	maxBytes := int((nBits + 6) / 7)
	buf, err := r.Peek(maxBytes)
	if len(buf) <= 0 { return 0, 0, err }
	n, l := parsePositive[N](buf)
	if l < 0 {
		if len(buf) < maxBytes && !errors.Is(err, io.EOF) {
			return 0, 0, err
		}
		return 0, 0, fmt.Errorf("vle parse error: marshaled %T is longer than the expected %d bytes", n, len(buf))
	}
	if p := uint(bits.Len(uint(buf[0] & 0x7f))) + 7 * uint(max(l-1,0)); p > nBits {
		return 0, 0, fmt.Errorf("vle parse error: %T unmarshals to %d bits", n, p)
	}
	r.Discard(l)
	return n, l, err
}
