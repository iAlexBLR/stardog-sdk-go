package stardog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

var errNonNilContext = errors.New("context must be non-nil")

const (
	mediaTypeV3    = "application/json"
	defaultBaseURL = "http://localhost:5820/"
)

type BasicAuth struct {
	username string
	password string
}

// A Client manages communication with the Stardog API.
type Client struct {
	clientMu sync.Mutex
	client   *http.Client

	// Basic URL used when communicating with the Stardog API.
	BaseURL *url.URL

	// User agent used when communicating with the Stardog API.
	UserAgent string

	common service // Reuse a single struct instead of allocating one for each service on the heap.

	// Services used for talking to different parts of the Stardog API.
	Users *UsersService

	// Basic auth used for setting authentification
	BasicAuth *BasicAuth
}

type service struct {
	client *Client
}

// Client returns the http.Client used by this Stardog client.
func (c *Client) Client() *http.Client {
	c.clientMu.Lock()
	defer c.clientMu.Unlock()
	clientCopy := *c.client
	return &clientCopy
}

func NewClient(httpClient *http.Client, baseURL string) *Client {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	if httpClient == nil {
		httpClient = &http.Client{}
		logger.Info("HTTP client not provided, using default client.")
	}

	if baseURL == "" {
		baseURL = defaultBaseURL
		logger.Info("Base URL not provided, using default URL: %s", zap.String("url", baseURL))
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		logger.Fatal("Error parsing base URL", zap.Error(err))
	}

	c := &Client{client: httpClient, BaseURL: parsedBaseURL}
	c.common.client = c
	c.Users = (*UsersService)(&c.common)

	logger.Info("New client created successfully.")
	return c
}

type Response struct {
	*http.Response

	// These fields provide the page values for paginating through a set of
	// results. Any or all of these may be set to the zero value for
	// responses that are not part of a paginated set, or for which there
	// are no additional pages.
	//
	// These fields support what is called "offset pagination" and should
	// be used with the ListOptions struct.
	NextPage  int
	PrevPage  int
	FirstPage int
	LastPage  int

	// Additionally, some APIs support "cursor pagination" instead of offset.
	// This means that a token points directly to the next record which
	// can lead to O(1) performance compared to O(n) performance provided
	// by offset pagination.
	//
	// For APIs that support cursor pagination (such as
	// TeamsService.ListIDPGroupsInOrganization), the following field
	// will be populated to point to the next page.
	//
	// To use this token, set ListCursorOptions.Page to this value before
	// calling the endpoint again.
	NextPageToken string

	// For APIs that support cursor pagination, such as RepositoriesService.ListHookDeliveries,
	// the following field will be populated to point to the next page.
	// Set ListCursorOptions.Cursor to this value when calling the endpoint again.
	Cursor string

	// For APIs that support before/after pagination, such as OrganizationsService.AuditLog.
	Before string
	After  string

	// Explicitly specify the Rate type so Rate's String() receiver doesn't
	// propagate to Response.
	Rate Rate

	// token's expiration date
	TokenExpiration Timestamp
}

// RequestOption represents an option that can modify an http.Request.
type RequestOption func(req *http.Request)

func (c *Client) NewRequest(method, urlStr string, body interface{}, opts ...RequestOption) (*http.Request, error) {
	if !strings.HasSuffix(c.BaseURL.Path, "/") {
		return nil, fmt.Errorf("BaseURL must have a trailing slash, but %q does not", c.BaseURL)
	}

	u, err := c.BaseURL.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	var buf io.ReadWriter
	if body != nil {
		buf = &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		err := enc.Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", mediaTypeV3)
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	if c.BasicAuth != nil {
		req.SetBasicAuth(c.BasicAuth.username, c.BasicAuth.password)
	}

	for _, opt := range opts {
		opt(req)
	}

	return req, nil
}

type Rate struct {
	// The number of requests per hour the client is currently limited to.
	Limit int `json:"limit"`

	// The number of remaining requests the client can make this hour.
	Remaining int `json:"remaining"`

	// The time at which the current rate limit will reset.
	Reset Timestamp `json:"reset"`
}

type Timestamp struct {
	time.Time
}

// sanitizeURL redacts the client_secret parameter from the URL which may be
// exposed to the user.
func sanitizeURL(uri *url.URL) *url.URL {
	if uri == nil {
		return nil
	}
	params := uri.Query()
	if len(params.Get("client_secret")) > 0 {
		params.Set("client_secret", "REDACTED")
		uri.RawQuery = params.Encode()
	}
	return uri
}

func withContext(ctx context.Context, req *http.Request) *http.Request {
	return req.WithContext(ctx)
}

func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	response.populatePageValues()
	return response
}

// populatePageValues parses the HTTP Link response headers and populates the
// various pagination link values in the Response.
func (r *Response) populatePageValues() {
	if links, ok := r.Response.Header["Link"]; ok && len(links) > 0 {
		for _, link := range strings.Split(links[0], ",") {
			segments := strings.Split(strings.TrimSpace(link), ";")

			// link must at least have href and rel
			if len(segments) < 2 {
				continue
			}

			// ensure href is properly formatted
			if !strings.HasPrefix(segments[0], "<") || !strings.HasSuffix(segments[0], ">") {
				continue
			}

			// try to pull out page parameter
			url, err := url.Parse(segments[0][1 : len(segments[0])-1])
			if err != nil {
				continue
			}

			q := url.Query()

			if cursor := q.Get("cursor"); cursor != "" {
				for _, segment := range segments[1:] {
					switch strings.TrimSpace(segment) {
					case `rel="next"`:
						r.Cursor = cursor
					}
				}

				continue
			}

			page := q.Get("page")
			since := q.Get("since")
			before := q.Get("before")
			after := q.Get("after")

			if page == "" && before == "" && after == "" && since == "" {
				continue
			}

			if since != "" && page == "" {
				page = since
			}

			for _, segment := range segments[1:] {
				switch strings.TrimSpace(segment) {
				case `rel="next"`:
					if r.NextPage, err = strconv.Atoi(page); err != nil {
						r.NextPageToken = page
					}
					r.After = after
				case `rel="prev"`:
					r.PrevPage, _ = strconv.Atoi(page)
					r.Before = before
				case `rel="first"`:
					r.FirstPage, _ = strconv.Atoi(page)
				case `rel="last"`:
					r.LastPage, _ = strconv.Atoi(page)
				}
			}
		}
	}
}

func (c *Client) BareDo(ctx context.Context, req *http.Request) (*Response, error) {
	if ctx == nil {
		return nil, errNonNilContext
	}

	req = withContext(ctx, req)

	resp, err := c.client.Do(req)
	if err != nil {
		// If we got an error, and the context has been canceled,
		// the context's error is probably more useful.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// If the error type is *url.Error, sanitize its URL before returning.
		if e, ok := err.(*url.Error); ok {
			if url, err := url.Parse(e.URL); err == nil {
				e.URL = sanitizeURL(url).String()
				return nil, e
			}
		}

		return nil, err
	}

	response := newResponse(resp)

	return response, err
}

func (c *Client) Do(ctx context.Context, req *http.Request, v interface{}) (*Response, error) {
	resp, err := c.BareDo(ctx, req)
	if err != nil {
		return resp, err
	}

	defer resp.Body.Close()

	switch v := v.(type) {
	case nil:
	case io.Writer:
		_, err = io.Copy(v, resp.Body)
	default:
		decErr := json.NewDecoder(resp.Body).Decode(v)
		if decErr == io.EOF {
			decErr = nil // ignore EOF errors caused by empty response body
		}
		if decErr != nil {
			err = decErr
		}
	}
	return resp, err
}

func (c *Client) SetBasicAuth(username string, password string) {
	basicAuth := BasicAuth{username: username, password: password}
	c.BasicAuth = &basicAuth
}
