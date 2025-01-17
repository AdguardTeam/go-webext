// Package edge helps user to interact with the Microsoft Edge store.
package edge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/log"
)

const requestTimeout = 30 * time.Second

// API versions
const (
	// APIVersionV1 represents the v1 API version (default)
	APIVersionV1 = "v1"
	// APIVersionV1_1 represents the v1.1 API version
	APIVersionV1_1 = "v1.1"
)

// ClientConfig defines the behavior for different client configurations.
type ClientConfig interface {
	SetRequestHeaders(req *http.Request) error
}

// V1Config is the configuration for the v1 API.
type V1Config struct {
	clientID       string
	clientSecret   string
	accessTokenURL *url.URL
}

// NewV1Config creates a new V1Config with the specified parameters.
func NewV1Config(clientID, clientSecret string, accessTokenURL *url.URL) *V1Config {
	return &V1Config{
		clientID:       clientID,
		clientSecret:   clientSecret,
		accessTokenURL: accessTokenURL,
	}
}

// V1_1Config is the configuration for the v1.1 API.
type V1_1Config struct {
	clientID string
	apiKey   string
}

// NewV1_1Config creates a new V1_1Config with the specified parameters.
func NewV1_1Config(clientID, apiKey string) *V1_1Config {
	return &V1_1Config{
		clientID: clientID,
		apiKey:   apiKey,
	}
}

// SetRequestHeaders sets the authorization headers for the request using v1 API configuration.
func (c *V1Config) SetRequestHeaders(req *http.Request) error {
	accessToken, err := c.authorize()
	if err != nil {
		return fmt.Errorf("authorizing: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+accessToken)
	return nil
}

// Authorize performs the authorization for v1 API and returns an access token.
func (c *V1Config) authorize() (string, error) {
	form := url.Values{
		"client_id":     {c.clientID},
		"scope":         {"https://api.addons.microsoftedge.microsoft.com/.default"},
		"client_secret": {c.clientSecret},
		"grant_type":    {"client_credentials"},
	}

	req, err := http.NewRequest(http.MethodPost, c.accessTokenURL.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	var authorizeResponse AuthorizeResponse

	err = json.Unmarshal(responseBody, &authorizeResponse)
	if err != nil {
		return "", fmt.Errorf("can't unmarshal response: %s, error: %w", responseBody, err)
	}

	return authorizeResponse.AccessToken, nil
}

// SetRequestHeaders sets the authorization headers for the request using v1.1 API configuration.
func (c *V1_1Config) SetRequestHeaders(req *http.Request) error {
	req.Header.Add("Authorization", "ApiKey "+c.apiKey)
	req.Header.Add("X-ClientID", c.clientID)
	return nil
}

// Client represents the edge client.
type Client struct {
	config ClientConfig
}

// NewClient creates a new Client with the specified configuration.
func NewClient(config ClientConfig) *Client {
	return &Client{config: config}
}

// setRequestHeaders sets the authorization headers for the request using the client's configuration.
func (c *Client) setRequestHeaders(req *http.Request) error {
	return c.config.SetRequestHeaders(req)
}

// Store represents the edge store instance
type Store struct {
	Client *Client
	URL    *url.URL
}

// Status represents the status of the update or publish.
type Status int64

const (
	// StatusInvalid status for default value
	StatusInvalid Status = iota
	// StatusInProgress status is returned when update or publish are in progress yet.
	StatusInProgress
	// StatusSucceeded status is returned when update or publish were successful.
	StatusSucceeded
	// StatusFailed status is returned when update or publish were failed.
	StatusFailed
)

// String returns the string representation of the status.
func (s Status) String() string {
	switch s {
	case StatusInProgress:
		return "InProgress"
	case StatusSucceeded:
		return "Succeeded"
	case StatusFailed:
		return "Failed"
	}

	return fmt.Sprintf("!bad_status_%d", s)
}

// UnmarshalJSON implements the json.Unmarshaler interface for status
func (s *Status) UnmarshalJSON(data []byte) error {
	var status string

	err := json.Unmarshal(data, &status)
	if err != nil {
		return fmt.Errorf("can't unmarshal status: %w", err)
	}

	switch status {
	case "InProgress":
		*s = StatusInProgress
	case "Succeeded":
		*s = StatusSucceeded
	case "Failed":
		*s = StatusFailed
	default:
		return fmt.Errorf("unknown status: %s", status)
	}

	return nil
}

// MarshalJSON implements the json.Marshaler interface for status
func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// StatusError represents the error returned by the edge store.
type StatusError struct {
	Message string `json:"message"`
}

// UploadStatusResponse represents the response from the upload status endpoint.
type UploadStatusResponse struct {
	ID              string        `json:"id"`
	CreatedTime     string        `json:"createdTime"`
	LastUpdatedTime string        `json:"lastUpdatedTime"`
	Status          Status        `json:"status"`
	Message         string        `json:"message"`
	ErrorCode       string        `json:"errorCode"`
	Errors          []StatusError `json:"errors"`
}

// UpdateOptions represents the options for the update.
type UpdateOptions struct {
	RetryTimeout      time.Duration
	WaitStatusTimeout time.Duration
	UploadTimeout     time.Duration
}

// Insert returns error, because edge store doesn't support insert.
func (s Store) Insert() (result []byte, err error) {
	return nil, errors.Error("there is no API for creating a new store item. you must complete these tasks manually in Microsoft Partner Center")
}

// DefaultUploadTimeout is the default timeout for the upload.
const DefaultUploadTimeout = 1 * time.Minute

// Update uploads the update to the store and waits for the update to be processed.
func (s Store) Update(appID, filepath string, updateOptions UpdateOptions) (result *UploadStatusResponse, err error) {
	const defaultRetryTimeout = 5 * time.Second
	const defaultWaitStatusTimeout = 1 * time.Minute

	if updateOptions.RetryTimeout == 0 {
		updateOptions.RetryTimeout = defaultRetryTimeout
	}

	if updateOptions.WaitStatusTimeout == 0 {
		updateOptions.WaitStatusTimeout = defaultWaitStatusTimeout
	}

	if updateOptions.UploadTimeout == 0 {
		updateOptions.UploadTimeout = DefaultUploadTimeout
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(updateOptions.UploadTimeout))
	defer cancel()

	operationID, err := s.UploadUpdate(ctx, appID, filepath)
	if err != nil {
		return nil, fmt.Errorf(
			"[Update] failed to upload update for appID: %s, with filepath: %q, due to error: %w", appID, filepath, err,
		)
	}

	startTime := time.Now()

	for {
		if time.Now().After(startTime.Add(updateOptions.WaitStatusTimeout)) {
			return nil, fmt.Errorf("update failed due to timeout")
		}

		log.Debug("getting upload status...")

		status, err := s.UploadStatus(appID, operationID)
		if err != nil {
			return nil, fmt.Errorf(
				"[Update] failed to get upload status for appID: %s, with operationID: %s, due to error: %w", appID, operationID, err,
			)
		}

		if status.Status == StatusInProgress {
			log.Debug("update is in progress, retry in: %s", updateOptions.RetryTimeout)
			time.Sleep(updateOptions.RetryTimeout)

			continue
		}

		if status.Status == StatusSucceeded {
			return status, nil
		}

		if status.Status == StatusFailed {
			return nil, fmt.Errorf("update failed due to %s, full error %+v", status.Message, status)
		}
	}
}

// UploadUpdate uploads the update to the store.
func (s Store) UploadUpdate(ctx context.Context, appID, filePath string) (result string, err error) {
	const apiPath = "/v1/products"

	apiURL := s.URL.JoinPath(apiPath, appID, "submissions/draft/package").String()

	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return "", fmt.Errorf("can't open file: %q, error: %w", filePath, err)
	}
	defer func() {
		err := errors.WithDeferred(err, file.Close())
		if err != nil {
			log.Debug("[UploadUpdate] failed to close file: %q due to error: %s", filePath, err)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, file)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	err = s.Client.setRequestHeaders(req)
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/zip")

	client := http.Client{}

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	if res.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("unexpected status code %s", res.Status)
	}

	operationID := res.Header.Get("Location")

	if operationID == "" {
		return "", fmt.Errorf("empty operation ID")
	}

	return operationID, nil
}

// UploadStatus returns the status of the upload.
func (s Store) UploadStatus(appID, operationID string) (response *UploadStatusResponse, err error) {
	apiPath := "v1/products"
	apiURL := s.URL.JoinPath(apiPath, appID, "submissions/draft/package/operations", operationID).String()

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	err = s.Client.setRequestHeaders(req)
	if err != nil {
		return nil, err
	}

	client := http.Client{
		Timeout: requestTimeout,
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal response body: %s, error: %w", responseBody, err)
	}

	return response, nil
}

// PublishExtension publishes the extension to the store and returns operationID.
func (s Store) PublishExtension(appID string) (result string, err error) {
	apiPath := "/v1/products/"
	apiURL := s.URL.JoinPath(apiPath, appID, "submissions").String()

	// TODO (maximtop): consider adding body to the request with notes for reviewers.
	req, err := http.NewRequest(http.MethodPost, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	err = s.Client.setRequestHeaders(req)
	if err != nil {
		return "", err
	}

	client := http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	if res.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("unexpected status code %s", res.Status)
	}

	operationID := res.Header.Get("Location")

	if operationID == "" {
		return "", fmt.Errorf("empty operation ID")
	}

	return operationID, nil
}

// PublishStatusResponse is the response of the PublishStatus API.
type PublishStatusResponse struct {
	ID              string        `json:"id"`
	CreatedTime     string        `json:"createdTime"`
	LastUpdatedTime string        `json:"lastUpdatedTime"`
	Status          string        `json:"status"`
	Message         string        `json:"message"`
	ErrorCode       string        `json:"errorCode"`
	Errors          []StatusError `json:"errors"`
}

// PublishStatus returns the status of the extension publish.
func (s Store) PublishStatus(appID, operationID string) (response *PublishStatusResponse, err error) {
	apiPath := "v1/products/"
	apiURL := s.URL.JoinPath(apiPath, appID, "submissions/operations", operationID).String()

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	err = s.Client.setRequestHeaders(req)
	if err != nil {
		return nil, err
	}

	client := http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response %s", res.Status)
	}

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	response = &PublishStatusResponse{}
	err = json.Unmarshal(responseBody, response)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response body: %s, error: %w", responseBody, err)
	}

	if response.Status == StatusFailed.String() {
		return nil, fmt.Errorf("publish failed due to: \"%s\", full error: %+v", response.Message, response)
	}

	return response, nil
}

// Publish publishes the extension to the store.
func (s Store) Publish(appID string) (response *PublishStatusResponse, err error) {
	operationID, err := s.PublishExtension(appID)
	if err != nil {
		return nil, fmt.Errorf("publishing extension with appID: %s, error: %w", appID, err)
	}

	return s.PublishStatus(appID, operationID)
}

// AuthorizeResponse describes the response received from the Edge Store
// authorization request.
type AuthorizeResponse struct {
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	AccessToken string `json:"access_token"`
}
