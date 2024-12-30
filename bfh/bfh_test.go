package bfh

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
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
	replyStatus func() int // 200 always 200 if nil, otherwise the return value
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
	rw.Header().Set("x-bfh", "grut")
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

	body, headers, err := c.Query("GET", url, 0, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, body)
	require.Equal(t, "", string(body))
	require.Equalf(t, []string{"grut"}, headers["X-Bfh"], "%#v", headers)
	require.Equal(t, "GET", s.req.Method)
	require.Equal(t, "/testBasicQuery", s.req.URL.Path)
	require.Equal(t, "", s.req.URL.Fragment)
	require.Equal(t, []byte{}, s.reqBody)

	body, _, err = c.Query("POST", url, 0, []byte("foo=bar"), map[string]string{"x-foo": "x-bar"})
	require.NoError(t, err)
	require.Equal(t, "POST", s.req.Method)
	require.Equalf(t, []string{"x-bar"}, s.req.Header["X-Foo"], "%#v", s.req.Header)
	require.Equal(t, "/testBasicQuery", s.req.URL.Path)
	require.Equal(t, "", s.req.URL.Fragment)
	require.Equal(t, "foo=bar", string(s.reqBody))
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
	_, headers, err := c.Query("GET", url, 0, nil, nil)
	require.Equalf(t, []string{"grut"}, headers["X-Bfh"], "%#v", headers)
	require.Error(t, err)
	require.NoError(t, oil.Third(c.Query("GET", url, 0, nil, nil)))

	replyStatus <- 500
	replyStatus <- 404
	replyStatus <- 200
	require.NoError(t, oil.Third(c.Query("GET", url, 2, []byte("body"), nil)))
	require.Equal(t, "body", string(s.reqBody))

	s.replyBody = []byte("abcd")
	replyStatus <- 403
	body, _, err := c.Query("GET", url, 0, nil, nil)
	require.Equal(t, "abcd", string(body))
	require.Error(t, err)
}

func TestTimeout(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	defer s.Close()
	url := s.URL() + "/testTimeout"
	c := NewClient().WithTimeout(time.Second / 10)
	delay := time.Second
	s.replyStatus = func() int { d := delay; delay = 0; time.Sleep(d); return 200 }
	require.NoError(t, oil.Third(c.Query("GET", url, uint(time.Minute/delay), nil, nil)))
	s.replyStatus = func() int { time.Sleep(time.Minute); return 200 }
	require.Error(t, oil.Third(c.Query("GET", url, 3, nil, nil)))
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
	c.httpClient.Transport = rt

	errs <- nil
	errs <- io.EOF
	require.NoError(t, oil.Third(c.Query("GET", url, 0, nil, nil)))

	errs <- errors.New("error, shouldn't be retried")
	require.Error(t, oil.Third(c.Query("GET", url, 0, nil, nil)))

	errs <- nil
	errs <- errors.New("error, shouldn't be retried")
	require.Error(t, oil.Third(c.Query("GET", url, 0, nil, nil)))

	errs <- errors.New("error, should be retried")
	errs <- nil
	errs <- io.EOF
	require.NoError(t, oil.Third(c.Query("GET", url, 1, nil, nil)))

	errs <- nil
	errs <- errors.New("error, should be retried")
	errs <- nil
	errs <- io.EOF
	require.NoError(t, oil.Third(c.Query("GET", url, 1, nil, nil)))

	errs <- nil
	errs <- errors.New("first error, should be retried")
	errs <- nil
	errs <- errors.New("second error, should be retried")
	errs <- nil
	errs <- io.EOF
	require.NoError(t, oil.Third(c.Query("GET", url, 2, nil, nil)))
}

func TestInputBody(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	defer s.Close()
	url := s.URL() + "/testInputBody"
	c := NewClient()
	const contentType = "Content-Type"

	_, _, err := c.Query("GET", url, 0, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []byte{}, s.reqBody)
	require.NotContains(t, s.req.Header, "Content-Type")

	_, _, err = c.Query("GET", url, 0, []byte("[]byte body"), nil)
	require.NoError(t, err)
	require.Equal(t, "[]byte body", string(s.reqBody))
	require.NotContains(t, s.req.Header, "Content-Type")

	_, _, err = c.Query("GET", url, 0, "string body", nil)
	require.NoError(t, err)
	require.Equal(t, "string body", string(s.reqBody))
	require.NotContains(t, s.req.Header, "Content-Type")

	obj := map[string]string{"foo": "bar"}
	extraHeaders := make(map[string]string, 1)
	expectedContentType := "application/json"
	for i := 0; i < 2; i++ {
		if i < 0 {
			expectedContentType = "foo"
			extraHeaders["contEnt-tYpe"] = expectedContentType
		}
		_, _, err = c.Query("GET", url, 0, obj, extraHeaders)
		require.NoError(t, err)
		got := new(map[string]string)
		require.NoError(t, json.Unmarshal(s.reqBody, &got))
		require.Equal(t, obj, *got)
		require.Equal(t, []string{expectedContentType}, s.req.Header["Content-Type"])
	}
}

func TestDeJSON(t *testing.T) {
	t.Parallel()
	obj := map[string]string{"foo": "bar"}
	body, err := json.Marshal(obj)
	require.NoError(t, err)
	for _, headers := range []http.Header{nil, http.Header{"X-Foobar": []string{"Baz"}}} {
		m, h, err := DeJSON[map[string]string]([]byte("invalid"), headers, errors.New("some error"))
		require.Nil(t, m)
		require.Equal(t, h, headers)
		require.Error(t, err)

		m, h, err = DeJSON[map[string]string]([]byte("invalid"), headers, nil)
		require.Nil(t, m)
		require.Equal(t, h, headers)
		require.Error(t, err)

		m, h, err = DeJSON[map[string]string](body, headers, nil)
		require.Equal(t, obj, *m)
		require.Equal(t, h, headers)
		require.NoError(t, err)

		m, h, err = DeJSON[map[string]string](body, headers, errors.New("some error"))
		require.Equal(t, h, headers)
		require.Error(t, err)
	}
}
