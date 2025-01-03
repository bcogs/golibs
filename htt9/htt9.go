// Package htt9 simplifies most client http queries to one liners: sending a query, checking the return code, optional json (un-)marshalling, and setting of good defaults (timeouts, retries etc) is a one line business.
// Example 1: fetch a page and print it
//
//	if r := (&htt9.Query{URL: "https://example.com"}).Do(nil, 3 /* max retries */); r.Err != nil { fmt.Println(r.Err) }
//	else { fmt.Println(r.Body) }
//
// Example 2: serialize a Bar object to json, POST it, deserialize the result as a *Foo
//
//	if foo, r := DeJSON[Foo]((&Query{URL: "...", Verb: "POST"}).DoWithJSON(nil, &Bar{...})); r.Err != nil { fmt.Println(r.Err) }
//	else { /* do something cool with foo, which is a *Foo */ }
package htt9

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bcogs/golibs/oil"
)

// DefaultTimeout is the default client timeout for requests (each retry can use a full timeout).
const DefaultTimeout = time.Minute

// ResponseInterpreter is a function called to interpret an HTTP response and decide if it's a success or error, and whether that error is retryable.
// If it returns a nil error or an error and false, the response is final,
// otherwise, the request will be retried if the retry max wasn't reached yet.
// Check the code of DefaultInterpretResponse for an example of how to create your own function.
type ResponseInterpreter func(r *Result /* r.Err is nil */, retriesLeft uint) (err error, retryable bool)

// Result conveys a htt9.Query and its http.Request and http.Response.  It's the return type of the Query.Do* functions.
// Some fields are nil if no http.Request was actually sent or no http.Response was received.
// Even when one of Do* is called, it's possbile that no http query will be sent, for example if the marshaling fails when calling DoWithJSON.
type Result struct {
	Query *Query         // can be nil in case of very early failure
	Body  []byte         // body of the reply (or nil if there was no reply)
	Resp  *http.Response // nil if there wasn't a reply, Body field is Close()d
	Req   *http.Request  // nil if there was no attempt to send a request, Body field is Close()d
	Err   error
}

// Query provides simple one line HTTP operations with sane defaults, and allows customizations for advanced needs.
type Query struct {
	URL          string
	Body         []byte            // optional
	ExtraHeaders map[string]string // headers to Add() to the http.Request (note net/http sends a few headers by default)

	Verb string // if nil, will use GET
	// optional function that interprets the http response and crafts an error if needed
	// the default is DefaultInterpretResponse: it checks the response is a 2xx, and otherwise generates a detailed error
	InterpretResponse ResponseInterpreter

	defaultContentType string
}

// Do sends the query and returns the result.
// maxRetries is a number of retries, so the first attempt doesn't count, e.g. if maxRetries is 2, up to 3 attempts can be made.
func (q *Query) Do(c *Client /* optional */, maxRetries uint) *Result {
	if c == nil {
		c = NewClient()
	}
	r, verb := &Result{Query: q}, q.verb()
	req, err := http.NewRequest(verb, q.URL, nil)
	if err != nil {
		r.Err = fmt.Errorf("error while crafting %s query to %s - %w", verb, q.URL, err)
		return r
	}
	r.Req = req
	defaultContentType := q.defaultContentType
	for k, v := range q.ExtraHeaders {
		req.Header.Add(k, v)
		if defaultContentType != "" && lowerStrEqual(k, "content-type") {
			defaultContentType = ""
		}
	}
	if defaultContentType != "" {
		req.Header.Add("Content-Type", defaultContentType)
	}
	interpretResponse := oil.If(q.InterpretResponse == nil, DefaultInterpretResponse, q.InterpretResponse)
	for {
		req.Body = io.NopCloser(bytes.NewReader(q.Body))
		if r.Body, r.Resp, err = q.do(c.HttpClient, req); err == nil {
			var retry bool
			if err, retry = interpretResponse(r, maxRetries); err == nil || !retry {
				return r
			}
		}
		if maxRetries == 0 {
			r.Err = err
			return r
		}
		maxRetries--
	}
}

// tests whether two string are equal in a case insensitive way
func lowerStrEqual(sa, sb string) bool {
	// the code's a bit hard to read, but check the unit test to gain confidence: it tries all sorts of combinations
	a, b := []byte(sa), []byte(sb)
	if len(a) != len(b) {
		return false
	}
	for i, c := range a {
		x := c ^ b[i]
		if x != 0 && !(x == 0x20 && (c|0x20)-'a' <= 'z'-'a') {
			return false
		}
	}
	return true
}

// DoWithJSON marshals an object in json, and on success sends the query by calling Do(), setting the json as the Query Body field.
// If the Query's ExtraHeaders doesn't have a Content-Type key, an application/json content-type header is inserted.
func (q *Query) DoWithJSON(c *Client /* optional */, maxRetries uint, body any) *Result {
	var err error
	q.Body, err = json.Marshal(body)
	if err != nil {
		return &Result{Query: q, Err: fmt.Errorf("unable to send %s query to %q - marshaling the body to JSON failed - %w", q.verb(), q.URL, err)}
	}
	q.defaultContentType = "application/json"
	r := q.Do(c, maxRetries)
	q.defaultContentType = "" // in case of future call to r.Query.Do
	return r
}

func (q *Query) verb() string { return oil.If(q.Verb == "", "GET", q.Verb) }

func (q *Query) do(httpClient *http.Client, req *http.Request) ([]byte /* body */, *http.Response, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%s query to %s failed - %w", req.Method, q.URL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, fmt.Errorf("error while reading response body to %s query to %s (reply status %q) - %w", req.Method, q.URL, resp.Status, err)
	}
	return body, resp, nil
}

// Client contains the resources used across multiple queries.
type Client struct {
	HttpClient *http.Client
}

// NewClient creates a new Client.
func NewClient() *Client {
	return &Client{
		HttpClient: &http.Client{Timeout: DefaultTimeout},
	}
}

// WithTimeout sets a Timeout other than DefaultTimeout and returns the Client itself.
func (c *Client) WithTimeout(t time.Duration) *Client {
	c.HttpClient.Timeout = t
	return c
}

// DeJSON unmarshals the json body after an http request.
// It's meant to wrap Do* Query method calls, and correctly handles the situation if the query fails.
// Example use:
//	if foo, r := DeJSON[Foo]((&Query{URL: "...", Verb: "POST"}).DoWithJSON(nil, &Bar{...})); r.Err != nil { fmt.Println(r.Err) }
//	else { /* do something cool with foo, which is a *Foo */ }
func DeJSON[T any](r *Result) ( /* unmarshaled reply body */ *T, *Result) {
	if r.Err != nil {
		return nil, r
	}
	x := new(T)
	if err := json.Unmarshal(r.Body, x); err != nil {
		r.Err = fmt.Errorf("JSON unmarshaling failed when reading the reply to the %s query to %q - %w", r.Req.Method, r.Query.URL, err)
		return nil, r
	}
	return x, r
}

// DefaultInterpretResponse is the default function used to interpret http
// responses after a query that succeeded at the http layer.
// It succeeds if the status code is 2xx, and otherwise returns an error.
// If the retry count is down to 0, the returned error contains the http response body, truncated if it's too long.
func DefaultInterpretResponse(r *Result, retriesLeft uint) (error /* retryable */, bool) {
	if r.Resp.StatusCode/100 == 2 {
		return nil, false
	}
	s := fmt.Sprintf("%s query to %s failed with HTTP status %q", r.Req.Method, r.Query.URL, r.Resp.Status)
	if retriesLeft > 0 { // no need to craft the full error message
		return errors.New(s), true
	}
	// truncate the body and append it to the error message, escaping it if it's multi-line or has special chars
	body := bytes.TrimSpace(r.Body)
	if len(body) == 0 {
		return errors.New(s), true
	}
	const maxLen = 3000
	if len(body) > maxLen {
		body = body[:maxLen]
		body = bytes.TrimSpace(r.Body)
		s += "; reply body (truncated): "
	} else {
		s += "; reply body: "
	}
	for _, c := range body {
		if c < 32 {
			return errors.New(s + strconv.Quote(string(body))), true
		}
	}
	return errors.New(s + string(body)), true
}
