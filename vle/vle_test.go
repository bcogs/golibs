package vle

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"golang.org/x/exp/constraints"

	"github.com/bcogs/golibs/oil"
	"github.com/stretchr/testify/require"
)

func TestEncodeUnsigned(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{n uint64; expected []byte}{
		{0x0, []byte{0}},
		{0x1, []byte{1}},
		{0x7f, []byte{0x7f}},
		{0x80, []byte{0x81, 0x00}},
		{0x7fff, []byte{0x81, 0xff, 0x7f}},
	}{
		b := EncodeUnsigned(tc.n)
		require.Equalf(t, tc.expected, b, "%x -> %x, expected %x", tc.n, b, tc.expected)
	}
}

func TestEncodeSigned(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{n int64; expected []byte}{
		{-0x1, []byte{0x40}},
		{0x0, []byte{0}},
		{0x1, []byte{1}},
		{0x3f, []byte{0x3f}},
		{0x40, []byte{0x80, 0x40}},
		{0x7f, []byte{0x80, 0x7f}},
		{0x80, []byte{0x81, 0}},
		{0x81, []byte{0x81, 1}},
		{0x1fff, []byte{0xbf, 0x7f}},
		{0x3fff, []byte{0x80, 0xff, 0x7f}},
		{0x4000, []byte{0x81, 0x80, 0x00}},
		{0xfffff, []byte{0xbf, 0xff, 0x7f}},
		{0x100000, []byte{0x80, 0xc0, 0x80, 0x00}},
		{0x7ffffff, []byte{0xbf, 0xff, 0xff, 0x7f}},
	}{
		b := EncodeSigned(tc.n)
		require.Equalf(t, tc.expected, b, "%#x -> %x, expected %x", tc.n, b, tc.expected)
		if tc.n > 1 {
			tc.expected[0] |= 0x40
			n := -(tc.n + 1)
			b = EncodeSigned(n)
			require.Equalf(t, tc.expected, b, "0x#%x -> %x, expected %x", n, b, tc.expected)
		}
	}
}

type mockReaderCall struct {
		n int // expected argument of the call
		b []byte  // if non-nil, expect a Peek(n) and return (b, err)
		discarded int // if b is nil, expect a Discard(n) and return (discarded, err)
		err error
}

type mockReader struct {  // mock implementation of BufioReader
	t *testing.T
	calls chan mockReaderCall
}

func newMockReader(t *testing.T) *mockReader { return &mockReader{t: t, calls: make(chan mockReaderCall, 10)} }

func (m *mockReader) Discard(n int) (discarded int, err error) {
	e := <-m.calls
	require.Nil(m.t, e.b, "Discard was called, but a call to Peek() was expected")
	require.Equal(m.t, e.n, n)
	return e.discarded, e.err
}

func (m *mockReader) Peek(n int) ([]byte, error) {
	e := <-m.calls
	require.NotNil(m.t, e.b, "Peek was called, but a call to Discard() was expected")
	require.Equal(m.t, e.n, n)
	return e.b, e.err
}

func TestReadSignedNoErr(t *testing.T) {
	t.Parallel()
	testReadIntNoError[int16](t, ReadSigned[int16], EncodeSigned[int16], -0x8000, 0x7fff)
}

func TestReadUnsignedNoErr(t *testing.T) {
	t.Parallel()
	testReadIntNoError[uint16](t, ReadUnsigned[uint16], EncodeUnsigned[uint16], 0, 0xffff)
}

func testReadIntNoError[N constraints.Integer](
t *testing.T,
read func(BufioReader)(N, int, error),
encode func(N) []byte,
from, to int) {
	for i := from; i <= to; i++ {
		n := N(i)
		marshaled := encode(n)
		more := (n % 91) == 0
		r := bytes.NewReader(oil.If(more, append(marshaled, []byte("more")...), marshaled))
		br := bufio.NewReader(r)
		got, l, err := read(br)
		if err != io.EOF { require.NoErrorf(t, err, "%#x %x", marshaled, n) }
		require.Equalf(t, n, got, "%x -> %#x, expected %#x", marshaled, got, n)
		require.Equalf(t, len(marshaled), l, "%#x %x", marshaled, n)
		if more {
			b, err := br.Peek(4)
			if err != io.EOF { require.NoErrorf(t, err, "%#x %x", marshaled, n) }
			require.Equalf(t, "more", string(b), "%#x %x", marshaled, n)
		} else {
			require.Equal(t, io.EOF, oil.Second(br.Peek(1)))
		}
	}
}

func TestSignedMaxLength(t *testing.T) {
	t.Parallel()
	testSignedMaxLength[int8, uint8](t)
	testSignedMaxLength[int16, uint16](t)
	testSignedMaxLength[int32, uint32](t)
	testSignedMaxLength[int64, uint64](t)
}

func testSignedMaxLength[S constraints.Signed, U constraints.Unsigned](t *testing.T) {
	maxInt := S(^U(0) >> 1)
	b := EncodeSigned(maxInt)
	got, l, err := ReadSigned[S](bufio.NewReader(bytes.NewReader(b)))
	if err != io.EOF { require.NoError(t, err) }
	require.Equal(t, maxInt, got)
	require.Equal(t, len(b), l)

	minInt := -maxInt -1
	b = EncodeSigned(minInt)
	got, l, err = ReadSigned[S](bufio.NewReader(bytes.NewReader(b)))
	if err != io.EOF { require.NoError(t, err) }
	require.Equal(t, minInt, got)
	require.Equal(t, len(b), l)
}

func TestUnsignedMaxLength(t *testing.T) {
	t.Parallel()
	testUnsignedMaxLength[uint8](t)
	testUnsignedMaxLength[uint16](t)
	testUnsignedMaxLength[uint32](t)
	testUnsignedMaxLength[uint64](t)
}

func testUnsignedMaxLength[U constraints.Unsigned](t *testing.T) {
	maxInt := ^U(0)
	b := EncodeUnsigned(maxInt)
	got, l, err := ReadUnsigned[U](bufio.NewReader(bytes.NewReader(b)))
	if err != io.EOF { require.NoError(t, err) }
	require.Equal(t, maxInt, got)
	require.Equal(t, len(b), l)
}

func TestReadIntTooManyBits(t *testing.T) {
	const maxs16 = 0x7fff
	require.NoError(t, oil.Third(ReadSigned[int16](bufio.NewReader(bytes.NewReader(EncodeSigned(maxs16))))))
	_, l, err := ReadSigned[int16](bufio.NewReader(bytes.NewReader(EncodeSigned(int32(maxs16 + 1)))))
	require.ErrorContains(t, err, "parse")
	require.Equal(t, 0, l)

	const mins8 = -128
	require.NoError(t, oil.Third(ReadSigned[int8](bufio.NewReader(bytes.NewReader(EncodeSigned(mins8))))))
	_, l, err = ReadSigned[int8](bufio.NewReader(bytes.NewReader(EncodeSigned(int16(mins8 - 1)))))
	require.ErrorContains(t, err, "parse")
	require.Equal(t, 0, l)

	const maxu32 = 0xffffffff
	require.NoError(t, oil.Third(ReadUnsigned[uint32](bufio.NewReader(bytes.NewReader(EncodeSigned(maxu32))))))
	_, l, err = ReadUnsigned[uint32](bufio.NewReader(bytes.NewReader(EncodeUnsigned(uint64(maxu32 + 1)))))
	require.ErrorContains(t, err, "parse")
	require.Equal(t, 0, l)
}

func TestReadIntIOError(t *testing.T) {
	t.Parallel()
	testReadIntIOError[int16](t, ReadSigned[int16])
	testReadIntIOError[uint16](t, ReadUnsigned[uint16])
}

func testReadIntIOError[N constraints.Integer](t *testing.T, read func(BufioReader) (N, int, error)) {
	errz, mr := fmt.Errorf("fake error"), newMockReader(t)
	// return too short a slice
	for _, call := range []mockReaderCall{
		{n: 3, b: []byte{}, err: errz},
		{n: 3, b: []byte{}, err: nil},
		{n: 3, b: []byte{}, err: io.EOF},
		{n: 3, b: []byte{0x81}, err: errz},
		{n: 3, b: []byte{0x81}, err: nil},
		{n: 3, b: []byte{0x81, 0x80}, err: errz},
		{n: 3, b: []byte{0x81, 0x80}, err: nil},
	}{
		mr.calls<- call
		_, l, err := read(mr)
		require.Equal(t, l, 0)
		require.Equal(t, call.err, err)
	}
	// too short a slice again, but non-0 length and EOF
	for _, call := range []mockReaderCall{
		{n: 3, b: []byte{0x81}, err: io.EOF},
		{n: 3, b: []byte{0x81, 0x80}, err: io.EOF},
	}{
		mr.calls<- call
		_, l, err := read(mr)
		require.Equal(t, l, 0)
		require.ErrorContains(t, err, "parse")
	}
	// long enough slice
	for _, call := range []mockReaderCall{
		{n: 3, b: []byte{0x40}, err: errz},
		{n: 3, b: []byte{0xc1, 0x00}, err: errz},
		{n: 3, b: []byte{0x81, 0x8f, 0x00}, err: errz},
	}{
		mr.calls<- call
		mr.calls<- mockReaderCall{n: len(call.b), err: nil}
		_, l, err := read(mr)
		require.Equalf(t, len(call.b), l, "%x", call.b)
		require.Equal(t, call.err, err)
	}
}

func TestReadIntParseError(t *testing.T) {
	t.Parallel()
	b1 := []byte{0x81, 0x82, 0x83, 0x84, 0x85}
	b2 := []byte{0xff, 0xff, 0x7f}

	r := bufio.NewReader(bytes.NewReader(b1))
	_, l, err := ReadSigned[int16](r)
	require.ErrorContains(t, err, "parse")
	require.LessOrEqual(t, l, 0)
	require.Equal(t, []byte{0x81}, oil.First(r.Peek(1)))

	r = bufio.NewReader(bytes.NewReader(b1))
	_, l, err = ReadSigned[int8](r)
	require.ErrorContains(t, err, "parse")
	require.LessOrEqual(t, l, 0)
	require.Equal(t, []byte{0x81}, oil.First(r.Peek(1)))

	r = bufio.NewReader(bytes.NewReader(b2))
	_, l, err = ReadSigned[int16](r)
	require.ErrorContains(t, err, "parse")
	require.LessOrEqual(t, l, 0)
	require.Equal(t, []byte{0xff}, oil.First(r.Peek(1)))

	r = bufio.NewReader(bytes.NewReader(b1))
	_, l, err = ReadUnsigned[uint16](r)
	require.ErrorContains(t, err, "parse")
	require.LessOrEqual(t, l, 0)
	require.Equal(t, []byte{0x81}, oil.First(r.Peek(1)))

	r = bufio.NewReader(bytes.NewReader(b1))
	_, l, err = ReadUnsigned[uint8](r)
	require.ErrorContains(t, err, "parse")
	require.LessOrEqual(t, l, 0)
	require.Equal(t, []byte{0x81}, oil.First(r.Peek(1)))

	r = bufio.NewReader(bytes.NewReader(b2))
	_, l, err = ReadUnsigned[uint16](r)
	require.ErrorContains(t, err, "parse")
	require.LessOrEqual(t, l, 0)
	require.Equal(t, []byte{0xff}, oil.First(r.Peek(1)))
}
