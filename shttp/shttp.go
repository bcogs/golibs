// Package shttp provides a simple to use HTTP client, making most common operations one liners.
// For instance, it can in one single line fetch a page and check that the status code is 2XX, and retry on failure.
// Example:
//   body, headers, err := shttp.NewClient().Query("GET", "https://www.example.com", 0, nil, nil)
//   fmt.Println(string(body), headers, err)
package shttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"
)

// DefaultTimeout is the default client timeout for requests (each retry can use a full timeout).
const DefaultTimeout = time.Minute

// Client provides simple one line HTTP operations with sane defaults, and allows customizations for advanced needs.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}
}

// Query sends an HTTP query and provides the reply, all in one line.
// If the returned status code isn't a 2XX, it considers this as an error.
// maxRetries is a number of retries after the first query: e.g. if maxRetries is 3, up to 4 queries can be sent.
// The body can be nil, a []byte, a string, or anything that can be serialized with json.Marshal.
// If the latter and there's no content-type extra header, a application/json content-type will be added.
// The returned header keys are in net.http.CanonicalHeaderKey form (first letter and any letter following a hyphen in uppercase, the rest lowercase).
func (c *Client) Query(verb, url string, maxRetries uint, body any, extraHeaders map[string]string) ([]byte /* body */, http.Header /* map[string][]string */, error) {
	req, err := http.NewRequest(verb, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("error while crafting %s query to %s - %w", verb, url, err)
	}
	var b []byte
	if body != nil {
		switch v := body.(type) {
		case []byte:
			b = v
		case string:
			b = []byte(v)
		default:
			var err error
			if b, err = json.Marshal(v); err != nil {
				return nil, nil, fmt.Errorf("bad body parameter in call to shttp.Client.Query: not nil, not []byte, not string, and marshaling it to JSON failed - %w", err)
			}
			setDefaultHeader(req.Header, "Content-Type", "application/json")
		}
	}
	for k, v := range extraHeaders {
		req.Header.Add(k, v)
	}
	for {
		req.Body = io.NopCloser(bytes.NewReader(b))
		if replyBody, replyHeaders, err := c.do(verb, url, req); err == nil || maxRetries == 0 {
			return replyBody, replyHeaders, err
		}
		maxRetries--
	}
}

func (c *Client) do(verb, url string, req *http.Request) ([]byte /* body */, http.Header, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("error while sending %s query to %s - %w", verb, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, nil, fmt.Errorf("%s query to %s failed with status %s", verb, url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error while reading response body to %s query to %s - %w", verb, url, err)
	}
	return body, map[string][]string(resp.Header), nil
}

// Sets a timeout other than DefaultTimeout and returns the Client itself.
func (c *Client) WithTimeout(t time.Duration) *Client {
	c.httpClient.Timeout = t
	return c
}

func setDefaultHeader(header http.Header, name, value string) {
	canonicalName := http.CanonicalHeaderKey(name)
	if _, ok := header[canonicalName]; !ok {
		header[canonicalName] = []string{value}
	}
}

// DeJSON is meant to wrap calls to Query to unmarshal a JSON reply body, while correctly handling the case where that body is nil due to an error.
func DeJSON[T any](body []byte, headers http.Header, err error) (*T, http.Header, error) {
	if err != nil {
		return nil, headers, err
	}
	x := new(T)
	if err = json.Unmarshal(body, x); err != nil {
		return nil, headers, fmt.Errorf("JSON unmarshaling of a %s failed - %w", reflect.TypeOf(x).String()[1:], err)
	}
	return x, headers, err
}
