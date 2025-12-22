// Package chrome helps to interact with the Chrome Web Store.
package chrome

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/httphdr"
	"github.com/adguardteam/go-webext/internal/fileutil"
)

const (
	// requestTimeout is the timeout for standard API requests.
	requestTimeout = 30 * time.Second
	// extendedRequestTimeout is the timeout for long-running operations like publish.
	extendedRequestTimeout = 120 * time.Second
	// TODO: make configurable
	// maxReadLimit limits response size returned from the store.
	maxReadLimit = 100 * fileutil.MB
)

// Client describes structure of a Chrome Store API client.
type Client struct {
	url          string
	clientID     string
	clientSecret string
	refreshToken string
	logger       *slog.Logger
}

// ClientConfig contains configuration parameters for creating a Chrome extension store instance
type ClientConfig struct {
	URL          string
	ClientID     string
	ClientSecret string
	RefreshToken string
	Logger       *slog.Logger
}

// NewClient creates a new Chrome extension store instance
func NewClient(config ClientConfig) *Client {
	return &Client{
		url:          config.URL,
		clientID:     config.ClientID,
		clientSecret: config.ClientSecret,
		refreshToken: config.RefreshToken,
		logger:       config.Logger,
	}
}

// RequestOptions contains optional parameters for HTTP requests.
type RequestOptions struct {
	Body        io.Reader
	ContentType string
}

// makeRequest is a base helper function for HTTP requests with JSON responses.
// It handles request execution, response reading, and JSON unmarshaling.
func makeRequest(
	method,
	url string,
	accessToken string,
	timeout time.Duration,
	result interface{},
	opts *RequestOptions,
) error {
	client := &http.Client{Timeout: timeout}

	var body io.Reader
	if opts != nil {
		body = opts.Body
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if accessToken != "" {
		req.Header.Add(httphdr.Authorization, "Bearer "+accessToken)
	}

	if opts != nil && opts.ContentType != "" {
		req.Header.Add(httphdr.ContentType, opts.ContentType)
	}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("got code %d, body: %q", res.StatusCode, responseBody)
	}

	if result != nil {
		err = json.Unmarshal(responseBody, result)
		if err != nil {
			return fmt.Errorf("unmarshaling response body: %w", err)
		}
	}

	return nil
}

// makeJSONRequest sends a request with JSON body and expects JSON response.
func makeJSONRequest(
	method,
	url string,
	body io.Reader,
	accessToken string,
	timeout time.Duration,
	result interface{},
) error {
	var opts *RequestOptions
	if body != nil {
		opts = &RequestOptions{
			Body:        body,
			ContentType: "application/json",
		}
	}
	return makeRequest(method, url, accessToken, timeout, result, opts)
}

// makeZipRequest sends a request with ZIP file body and expects JSON response.
func makeZipRequest(
	method,
	url string,
	body io.Reader,
	accessToken string,
	timeout time.Duration,
	result interface{},
) error {
	return makeRequest(method, url, accessToken, timeout, result, &RequestOptions{
		Body:        body,
		ContentType: "application/zip",
	})
}

// AuthorizeResponse describes the response received from the Chrome Store
// authorization request.
type AuthorizeResponse struct {
	AccessToken string `json:"access_token"`
}

// Authorize retrieves access token.
func (c *Client) Authorize() (accessToken string, err error) {
	l := c.logger.With("action", "Authorize")
	l.Debug("initiating authorization")

	data := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"refresh_token": {c.refreshToken},
		"grant_type":    {"refresh_token"},
		"redirect_uri":  {"urn:ietf:wg:oauth:2.0:oob"},
	}

	result := &AuthorizeResponse{}
	err = makeRequest(
		http.MethodPost,
		c.url,
		"", // no access token
		requestTimeout,
		result,
		&RequestOptions{
			Body:        strings.NewReader(data.Encode()),
			ContentType: "application/x-www-form-urlencoded",
		},
	)
	if err != nil {
		return "", err
	}

	l.Debug(
		"authorization completed",
		"status", "success",
	)

	return result.AccessToken, nil
}
