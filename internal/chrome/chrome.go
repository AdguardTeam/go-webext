// Package chrome helps to interact with the Chrome Web Store.
package chrome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/httphdr"
	"github.com/adguardteam/go-webext/internal/fileutil"
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

// TODO make configurable
// maxReadLimit limits response size returned from the store.
const maxReadLimit = 100 * fileutil.MB

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

	res, err := http.PostForm(c.url, data)
	if err != nil {
		return "", fmt.Errorf("posting a form: %w", err)
	}

	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	result := &AuthorizeResponse{}

	err = json.Unmarshal(body, result)
	if err != nil {
		return "", fmt.Errorf("unmarshaling response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("got code %d, body: %q", res.StatusCode, body)
	}

	l.Debug(
		"authorization completed",
		"status", "success",
	)

	return result.AccessToken, nil
}

// Store describes structure of the store.
type Store struct {
	client *Client
	url    *url.URL
	logger *slog.Logger
}

// StoreConfig contains configuration parameters for creating a Chrome extension store instance
type StoreConfig struct {
	Client *Client
	URL    *url.URL
	Logger *slog.Logger
}

// NewStore creates a new Chrome extension store instance
func NewStore(config StoreConfig) *Store {
	return &Store{
		client: config.Client,
		url:    config.URL,
		logger: config.Logger,
	}
}

// StatusResponse describes status response fields.
type StatusResponse struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	PublicKey   string `json:"publicKey"`
	UploadState string `json:"uploadState"`
	CrxVersion  string `json:"crxVersion"`
}

const requestTimeout = 30 * time.Second

// Extended request timeout for waiting skip review check during publish.
const extendedRequestTimeout = 120 * time.Second

// Status retrieves status of the extension in the store.
func (s *Store) Status(appID string) (result []byte, err error) {
	l := s.logger.With("action", "Status", "app_id", appID)
	l.Debug("retrieving extension status")

	const apiPath = "chromewebstore/v1.1/items"
	apiURL := s.url.JoinPath(apiPath, appID).String()

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	client := &http.Client{Timeout: requestTimeout}

	var req *http.Request

	req, err = http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add(httphdr.Authorization, "Bearer "+accessToken)
	q := req.URL.Query()
	q.Add("projection", "DRAFT")
	req.URL.RawQuery = q.Encode()

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got code %d, body: %q", res.StatusCode, body)
	}

	l.Debug(
		"status retrieval completed",
		"status", "success",
	)

	return body, nil
}

// UploadState describes the state of request.
type UploadState uint8

const (
	// UploadStateInvalid default invalid state.
	UploadStateInvalid UploadState = iota
	// UploadStateSuccess means that the request was successful.
	UploadStateSuccess
	// UploadStateFailure means that the request failed.
	UploadStateFailure
	// UploadStateInProgress means that the request is in progress.
	UploadStateInProgress
	// UploadStateNotFound means that the item was not found.
	UploadStateNotFound
)

// String returns the string representation of the .
func (u UploadState) String() string {
	switch u {
	case UploadStateSuccess:
		return "SUCCESS"
	case UploadStateFailure:
		return "FAILURE"
	case UploadStateInProgress:
		return "IN_PROGRESS"
	case UploadStateNotFound:
		return "NOT_FOUND"
	}

	return fmt.Sprintf("!bad_status_%d", u)
}

// UnmarshalJSON unmarshals the string representation of the UploadState type
func (u *UploadState) UnmarshalJSON(data []byte) error {
	var state string

	err := json.Unmarshal(data, &state)
	if err != nil {
		return err
	}

	switch state {
	case "SUCCESS":
		*u = UploadStateSuccess
	case "FAILURE":
		*u = UploadStateFailure
	case "IN_PROGRESS":
		*u = UploadStateInProgress
	case "NOT_FOUND":
		*u = UploadStateNotFound
	default:
		return fmt.Errorf("unknown upload state: %q", state)
	}

	return nil
}

// MarshalJSON marshals the UploadState type to string.
func (u UploadState) MarshalJSON() ([]byte, error) {
	fmt.Printf("marshaling %s", u.String())
	return json.Marshal(u.String())
}

// ItemError error provides details of the error
type ItemError struct {
	ErrorCode   string `json:"error_code"`
	ErrorDetail string `json:"error_detail"`
}

// ItemResource describes structure returned on the update and insert requests.
// Error example:
//
//	{
//		"kind": "chromewebstore#item",
//		"id": "null",
//		"uploadState": "FAILURE",
//		"itemError": [
//			{
//				"error_code": "PKG_MANIFEST_PARSE_ERROR",
//				"error_detail": "The minimum Chrome version of 79.0 does not meet the minimum requirement to be published. To be published, a manifest V3 item must require at least Chrome version 88."
//			}
//		]
//	}
type ItemResource struct {
	Kind        string      `json:"kind"`
	ID          string      `json:"id"`
	UploadState UploadState `json:"uploadState"`
	ItemError   []ItemError `json:"itemError"`
}

// Insert uploads a package to create a new store item.
func (s *Store) Insert(filePath string) (result *ItemResource, err error) {
	l := s.logger.With("action", "Insert", "file_path", filePath)
	l.Debug("initiating extension upload")

	const apiPath = "upload/chromewebstore/v1.1/items"
	apiURL := s.url.JoinPath(apiPath).String()

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	body, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	client := &http.Client{Timeout: requestTimeout}

	req, err := http.NewRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add(httphdr.Authorization, "Bearer "+accessToken)

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got code %d, body: %q", res.StatusCode, responseBody)
	}

	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling response body: %w", err)
	}

	if result.UploadState != UploadStateSuccess {
		return nil, fmt.Errorf("non success upload state received: %v, %v", result.UploadState, result.ItemError)
	}

	l.Debug(
		"extension upload completed",
		"status", "success",
		"upload_id", result.ID,
	)

	return result, nil
}

// Update uploads new version of the package to the store.
func (s *Store) Update(appID, filePath string) (result *ItemResource, err error) {
	l := s.logger.With("action", "Update", "app_id", appID, "file_path", filePath)
	l.Debug("initiating extension update")

	const apiPath = "upload/chromewebstore/v1.1/items/"
	apiURL := s.url.JoinPath(apiPath, appID).String()

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	client := &http.Client{Timeout: requestTimeout}

	body, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add(httphdr.Authorization, "Bearer "+accessToken)

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got code %d, body: %q", res.StatusCode, responseBody)
	}

	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling response body: %w", err)
	}

	if result.UploadState == UploadStateFailure {
		return nil, fmt.Errorf("failure in response: %v", result.ItemError)
	}

	l.Debug(
		"extension update completed",
		"status", "success",
		"upload_state", result.UploadState,
	)

	return result, nil
}

// PublishResponse describes response returned on publish request.
type PublishResponse struct {
	Kind         string   `json:"kind"`
	ItemID       string   `json:"item_id"`
	Status       []string `json:"status"`
	StatusDetail []string `json:"statusDetail"`
}

// PublishOptions describes optional parameters for publishing.
type PublishOptions struct {
	// Target specifies the publish target. Can be "trustedTesters" or "default".
	Target string `json:"target,omitempty"`
	// DeployPercentage specifies percentage of existing users who will receive an update.
	// Valid values are 0-100. New users always receive the latest version.
	// Once set, this value can only be increased.
	DeployPercentage *int `json:"deployPercentage,omitempty"`
	// ReviewExemption if true, attempts to use expedited review process.
	ReviewExemption bool `json:"reviewExemption,omitempty"`
}

// Publish publishes app to the store.
func (s *Store) Publish(appID string, opts *PublishOptions) (result *PublishResponse, err error) {
	l := s.logger.With("action", "Publish", "app_id", appID, "options", opts)
	l.Debug("initiating extension publish")

	const apiPath = "chromewebstore/v1.1/items"
	apiURL := s.url.JoinPath(apiPath, appID, "publish").String()

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	client := &http.Client{Timeout: extendedRequestTimeout}

	var body io.Reader
	if opts != nil {
		jsonData, err := json.Marshal(opts)
		if err != nil {
			return nil, fmt.Errorf("marshaling publish options: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add(httphdr.Authorization, "Bearer "+accessToken)
	if body != nil {
		req.Header.Add(httphdr.ContentType, "application/json")
	}

	// Add query parameters if specified
	if opts != nil {
		q := req.URL.Query()
		// Handle deployment percentage
		if opts.DeployPercentage != nil {
			if *opts.DeployPercentage < 0 || *opts.DeployPercentage > 100 {
				return nil, fmt.Errorf("deploy percentage must be between 0 and 100, got %d", *opts.DeployPercentage)
			}
			q.Add("deployPercentage", fmt.Sprintf("%d", *opts.DeployPercentage))
		}
		if opts.Target != "" {
			q.Add("publishTarget", opts.Target)
		}
		if opts.ReviewExemption {
			q.Add("reviewExemption", "true")
		}
		req.URL.RawQuery = q.Encode()
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	resultBody, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got code %d, body: %q", res.StatusCode, resultBody)
	}

	err = json.Unmarshal(resultBody, &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling response body: %w", err)
	}

	l.Debug(
		"extension publish completed",
		"status", "success",
		"item_id", result.ItemID,
	)

	return result, nil
}
