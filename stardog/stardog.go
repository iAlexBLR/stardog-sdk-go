package stardog

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

var errNonNilContext = errors.New("context must be non-nil")

// A Client manages communication with the GitHub API.
type Client struct {
	clientMu sync.Mutex   // clientMu protects the client during calls that modify the CheckRedirect func.
	client   *http.Client // HTTP client used to communicate with the API.

	// Base URL for API requests. Defaults to the public GitHub API, but can be
	// set to a domain endpoint to use with GitHub Enterprise. BaseURL should
	// always be specified with a trailing slash.
	BaseURL *url.URL

	// Base URL for uploading files.
	UploadURL *url.URL

	// User agent used when communicating with the GitHub API.
	UserAgent string

	rateMu sync.Mutex
	// rateLimits [categories]Rate // Rate limits for the client as determined by the most recent API calls.

	common service // Reuse a single struct instead of allocating one for each service on the heap.

	// Services used for talking to different parts of the GitHub API.
	Users *UsersService
}

type service struct {
	client *Client
}

// Client returns the http.Client used by this GitHub client.
func (c *Client) Client() *http.Client {
	c.clientMu.Lock()
	defer c.clientMu.Unlock()
	clientCopy := *c.client
	return &clientCopy
}

func HelloWorld() {
	fmt.Printf("hello")
}
