package htt9

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bcogs/golibs/oil"
	"github.com/stretchr/testify/require"
)

type server struct {
	t          *testing.T
	listener   net.Listener
	httpServer *http.Server

	// modify these to change what the server replies
	replyStatus func() int // provides the return code (200 if nil)
	replyBody   []byte     // default: nil

	req     *http.Request // latest request received by the server
	reqBody []byte
}

func newServer(t *testing.T) *server {
	s := &server{t: t}
	s.httpServer = &http.Server{Addr: "localhost:0",
		Handler:     s,
		ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Second}
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	s.listener = listener
	go func() {
		err := s.httpServer.Serve(listener)
		require.ErrorIs(s.t, err, http.ErrServerClosed)
	}()
	return s
}

func (s *server) Close() {
	s.httpServer.Close()
}

func (s *server) URL() string {
	return "http://" + s.listener.Addr().String()
}

// ServeHTTP implements the http.Handler interface.
func (s *server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	s.req = req
	b, err := io.ReadAll(req.Body)
	require.NoError(s.t, err)
	s.reqBody = b
	rw.Header().Set("x-htt9", "grut")
	if s.replyStatus == nil {
		rw.WriteHeader(200)
	} else {
		rw.WriteHeader(s.replyStatus())
	}
	if s.replyBody != nil {
		require.NoError(s.t, oil.Second(io.Copy(rw, bytes.NewReader(s.replyBody))))
	}
}

func TestBasicQuery(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	defer s.Close()
	url := s.URL() + "/testBasicQuery"
	c := NewClient()

	q := &Query{URL: url}
	r := q.Do(c, 0)
	require.Equal(t, "GET", r.Req.Method)
	require.Equal(t, r.Query, q)
	require.NoError(t, r.Err)
	require.Equal(t, []byte{}, r.Body)
	require.Equalf(t, []string{"grut"}, r.Resp.Header["X-Htt9"], "%#v", r.Resp.Header)
	require.Equal(t, "/testBasicQuery", r.Req.URL.Path)
	require.Equal(t, "GET", s.req.Method)
	require.Equal(t, "/testBasicQuery", s.req.URL.Path)
	require.Equal(t, "", s.req.URL.Fragment)
	require.Equal(t, []byte{}, s.reqBody)

	q.Verb, q.Body, q.ExtraHeaders = "POST", []byte("foo=bar"), map[string]string{"x-foo": "x-bar"}
	r = q.Do(c, 0)
	require.Equal(t, "POST", r.Req.Method)
	require.Equal(t, r.Query, q)
	require.NoError(t, r.Err)
	require.Equal(t, []byte{}, r.Body)
	require.Equalf(t, []string{"x-bar"}, s.req.Header["X-Foo"], "%#v", s.req.Header)
	require.Equal(t, "/testBasicQuery", r.Req.URL.Path)
	require.Equal(t, "POST", s.req.Method)
	require.Equal(t, "/testBasicQuery", s.req.URL.Path)
	require.Equal(t, "", s.req.URL.Fragment)
	require.Equal(t, "foo=bar", string(s.reqBody))
}

func TestNilClient(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	defer s.Close()
	url := s.URL() + "/testNilClient"
	q := &Query{URL: url}
	r := q.Do(nil, 0)
	require.Equal(t, "GET", r.Req.Method)
	require.Equal(t, r.Query, q)
	require.NoError(t, r.Err)
	require.Equal(t, []byte{}, r.Body)
	require.Equalf(t, []string{"grut"}, r.Resp.Header["X-Htt9"], "%#v", r.Resp.Header)
	require.Equal(t, "/testNilClient", r.Req.URL.Path)
	require.Equal(t, "GET", s.req.Method)
	require.Equal(t, "/testNilClient", s.req.URL.Path)
	require.Equal(t, "", s.req.URL.Fragment)
	require.Equal(t, []byte{}, s.reqBody)
}

func TestHTTPError(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	defer s.Close()
	url := s.URL() + "/testHTTPError"
	c := NewClient()

	replyStatus := make(chan int, 10)
	s.replyStatus = func() int { return <-replyStatus }
	replyStatus <- 403
	replyStatus <- 200
	r := (&Query{URL: url}).Do(c, 0)
	require.Equalf(t, []string{"grut"}, r.Resp.Header["X-Htt9"], "%#v", r.Resp.Header)
	require.Equal(t, []byte{}, r.Body)
	require.Error(t, r.Err)
	require.NoError(t, (&Query{URL: url}).Do(c, 0).Err)

	replyStatus <- 500
	replyStatus <- 404
	replyStatus <- 200
	r = (&Query{URL: url, Body: []byte("body")}).Do(c, 2)
	require.NoError(t, r.Err)
	require.Equal(t, "body", string(s.reqBody))
	require.Equal(t, []byte{}, r.Body)

	s.replyBody = []byte("abcd")
	replyStatus <- 403
	r = (&Query{URL: url, Verb: "GET"}).Do(c, 0)
	require.Equal(t, "abcd", string(r.Body))
	require.Error(t, r.Err)
}

func TestTimeout(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	defer s.Close()
	url := s.URL() + "/testTimeout"
	c := NewClient().WithTimeout(time.Second / 10)
	delay := time.Second
	s.replyStatus = func() int { d := delay; delay = 0; time.Sleep(d); return 200 }
	require.NoError(t, (&Query{URL: url}).Do(c, uint(time.Minute/delay)).Err)
	s.replyStatus = func() int { time.Sleep(time.Minute); return 200 }
	r := (&Query{URL: url, Verb: "POST"}).Do(c, 3)
	require.Error(t, r.Err)
}

type mockRoundTripper struct{ errs <-chan error }

func (m *mockRoundTripper) Close() error { return nil } // Close implements the io.ReadCloser interface.

func (m *mockRoundTripper) Read(p []byte) (int, error) { // Read implements the io.ReadCloser interface.
	if len(p) < 1 {
		return 0, nil
	}
	err := <-m.errs
	if err != nil {
		return 0, err
	}
	p[0] = 'A'
	return 1, nil
}

// RoundTrip implements the net.http.RoundTripper interface.
func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Body.Close()
	req.Body = nil
	resp := &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{},
		Body:          m,
		ContentLength: -1,
		Close:         false,
		Uncompressed:  true,
		Trailer:       http.Header{},
		Request:       req,
	}
	return resp, nil
}

func TestBodyReadError(t *testing.T) {
	t.Parallel()
	const url = "http://localhost:1337/testBodyReadError"
	errs := make(chan error, 10)
	rt := &mockRoundTripper{errs: errs}
	c := NewClient()
	c.HttpClient.Transport = rt

	errs <- nil
	errs <- io.EOF
	r := (&Query{URL: url}).Do(c, 0)
	require.NoError(t, r.Err)
	require.Equal(t, "A", string(r.Body))

	errs <- errors.New("error, shouldn't be retried")
	r = (&Query{URL: url}).Do(c, 0)
	require.Error(t, r.Err)
	require.Nil(t, r.Body)

	errs <- nil
	errs <- errors.New("error, shouldn't be retried")
	r = (&Query{URL: url}).Do(c, 0)
	require.Error(t, r.Err)
	require.Nil(t, r.Body)

	errs <- errors.New("error, should be retried")
	errs <- nil
	errs <- io.EOF
	r = (&Query{URL: url}).Do(c, 1)
	require.NoError(t, r.Err)
	require.Equal(t, "A", string(r.Body))

	errs <- nil
	errs <- errors.New("error, should be retried")
	errs <- nil
	errs <- nil
	errs <- io.EOF
	r = (&Query{URL: url}).Do(c, 1)
	require.NoError(t, r.Err)
	require.Equal(t, "AA", string(r.Body))

	errs <- nil
	errs <- nil
	errs <- errors.New("first error, should be retried")
	errs <- nil
	errs <- errors.New("second error, should be retried")
	errs <- nil
	errs <- nil
	errs <- nil
	errs <- io.EOF
	r = (&Query{URL: url}).Do(c, 2)
	require.NoError(t, r.Err)
	require.Equal(t, "AAA", string(r.Body))
}

func TestInputBody(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	defer s.Close()
	url := s.URL() + "/testInputBody"
	c := NewClient()
	const contentType = "Content-Type"

	require.NoError(t, (&Query{URL: url}).Do(c, 0).Err)
	require.Equal(t, []byte{}, s.reqBody)
	require.NotContains(t, s.req.Header, contentType)

	require.NoError(t, (&Query{URL: url, Body: []byte("[]byte body")}).Do(c, 0).Err)
	require.Equal(t, "[]byte body", string(s.reqBody))
	require.NotContains(t, s.req.Header, contentType)
}

func TestDoWithJSON(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	defer s.Close()
	url := s.URL() + "/testDoWithJSON"
	c := NewClient()
	x := map[string]string{"foo": "bar"}
	for _, extraHeader := range [][]byte{nil, []byte{}, []byte("application/json"), []byte("foo")} {
		const contentType = "Content-Type"
		q := &Query{URL: url}
		if extraHeader != nil {
			q.ExtraHeaders = map[string]string{contentType: string(extraHeader)}
		}
		r := q.DoWithJSON(c, 3, &x)
		require.NoError(t, r.Err)
		var y map[string]string
		require.NoError(t, json.Unmarshal(s.reqBody, &y))
		require.Equal(t, x, y)
		if extraHeader != nil {
			require.Equal(t, []string{string(extraHeader)}, s.req.Header[contentType])
			require.Equal(t, []string{string(extraHeader)}, r.Req.Header[contentType])
			require.Equal(t, string(extraHeader), q.ExtraHeaders[contentType])
		} else {
			require.Equal(t, []string{"application/json"}, s.req.Header[contentType])
			require.Equal(t, []string{"application/json"}, r.Req.Header[contentType])
			require.NotContains(t, q.ExtraHeaders, contentType)
		}
		r = q.Do(c, 0)
		require.NoError(t, r.Err)
		if extraHeader != nil {
			require.Equal(t, []string{string(extraHeader)}, s.req.Header[contentType])
			require.Equal(t, []string{string(extraHeader)}, r.Req.Header[contentType])
			require.Equal(t, string(extraHeader), q.ExtraHeaders[contentType])
		} else {
			require.NotContains(t, s.req.Header, contentType)
			require.NotContains(t, r.Req.Header, contentType)
			require.NotContains(t, q.ExtraHeaders, contentType)
		}
	}
	// it would be good to test the behavior when the marshaling fails, but
	// I couldn't come up with something that makes it fail; the stuff
	// mentioned in the documentation that's supposed to make it fail
	// (circular references, types incompatible with json) is actually
	// silently ignored rather than causing failures
}

func TestDeJSON(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	defer s.Close()
	url := s.URL() + "/testDeJSON"
	c := NewClient()
	obj := map[string]string{"foo": "bar"}
	body, err := json.Marshal(obj)
	require.NoError(t, err)
	invalid := []byte("invalid")
	for _, err := range []error{nil, errors.New("fake error")} {
		for _, b := range [][]byte{[]byte{}, body, invalid} {
			s.replyBody = b
			expectedCode := 200
			if err != nil {
				expectedCode = 500
				s.replyStatus = func() int { return expectedCode }
			}
			m, r := DeJSON[map[string]string]((&Query{URL: url}).Do(c, 0))
			require.Equal(t, url, r.Query.URL)
			require.Equal(t, string(b), string(r.Body))
			require.Equal(t, expectedCode, r.Resp.StatusCode)
			if err != nil || string(b) != string(body) {
				require.Error(t, r.Err)
				require.Nil(t, m)
			} else {
				require.NoError(t, r.Err)
				require.Equal(t, obj, *m)
			}
		}
	}
}

func testLowerStrEqual(t *testing.T) {
	i := 0
	for c1 := byte('a'); c1 <= 'z'; c1++ {
		for c2 := byte('a'); c2 <= 'z'; c2++ {
			j := i % 3
			s1 := string(append([]byte{c1, c1 | 0x20, c1, c1 | 0x20}, make([]byte, c1, j)...))
			b := append(make([]byte, c2|0x20, j), []byte{c2, c2, c2 | 0x20, c2 | 0x20}...)
			s2 := string(b)
			require.Equal(t, c1 == c2, lowerStrEqual(s1, s2))
			require.Equal(t, c1 == c2, lowerStrEqual(s1, strings.ToUpper(s2)))
			require.Equal(t, c1 == c2, lowerStrEqual(strings.ToUpper(s1), s2))
			require.Equal(t, c1 == c2, lowerStrEqual(strings.ToUpper(s1), strings.ToUpper(s2)))
			require.Equal(t, c1 == c2, lowerStrEqual(strings.ToLower(s1), strings.ToLower(s2)))
			i++
			if c1 != c2 {
				continue
			}
			for c3 := 0; c3 < 0x100; c3++ {
				b[i%len(b)] = byte(c3)
				require.Equal(t, int(c1) == c3, lowerStrEqual(s1, string(b)))
			}
		}
	}
	require.True(t, lowerStrEqual("", ""))
	require.False(t, lowerStrEqual("ab", "abc"))
	require.False(t, lowerStrEqual("ab", "a"))
}
