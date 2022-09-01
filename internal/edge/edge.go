// Package edge helps user to interact with the Microsoft Edge store.
package edge

import (
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

// Client represent the edge client.
type Client struct {
	ClientID       string
	ClientSecret   string
	AccessTokenURL *url.URL
}

// NewClient creates a new edge Client instance.
func NewClient(clientID, clientSecret, rawAccessTokenURL string) (client Client, err error) {
	accessTokenURL, err := url.Parse(rawAccessTokenURL)
	if err != nil {
		return Client{}, fmt.Errorf("parsing access token from URL: %s, error: %w", rawAccessTokenURL, err)
	}

	return Client{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		AccessTokenURL: accessTokenURL,
	}, nil
}

// AuthorizeResponse describes the response received from the Edge Store
// authorization request.
type AuthorizeResponse struct {
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	AccessToken string `json:"access_token"`
}

// Authorize returns the access token.
func (c *Client) Authorize() (accessToken string, err error) {
	form := url.Values{
		"client_id":     {c.ClientID},
		"scope":         {"https://api.addons.microsoftedge.microsoft.com/.default"},
		"client_secret": {c.ClientSecret},
		"grant_type":    {"client_credentials"},
	}

	req, err := http.NewRequest(http.MethodPost, c.AccessTokenURL.String(), strings.NewReader(form.Encode()))
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

// Store represents the edge store instance
type Store struct {
	Client *Client
	URL    *url.URL
}

// Status represents the status of the update or publish.
type Status int64

const (
	// InProgress status is returned when update or publish are in progress yet.
	InProgress Status = iota
	// Succeeded status is returned when update or publish were successful.
	Succeeded
	// Failed status is returned when update or publish were failed.
	Failed
)

// String returns the string representation of the status.
func (u Status) String() string {
	switch u {
	case InProgress:
		return "InProgress"
	case Succeeded:
		return "Succeeded"
	case Failed:
		return "Failed"
	}

	return "unknown"
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
	Status          string        `json:"status"`
	Message         string        `json:"message"`
	ErrorCode       string        `json:"errorCode"`
	Errors          []StatusError `json:"errors"`
}

// UpdateOptions represents the options for the update.
type UpdateOptions struct {
	RetryTimeout      time.Duration
	WaitStatusTimeout time.Duration
}

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

	operationID, err := s.UploadUpdate(appID, filepath)
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

		if status.Status == InProgress.String() {
			log.Debug("update is in progress, retry in: %s", updateOptions.RetryTimeout)
			time.Sleep(updateOptions.RetryTimeout)

			continue
		}

		if status.Status == Succeeded.String() {
			return status, nil
		}

		if status.Status == Failed.String() {
			return nil, fmt.Errorf("update failed due to %s, full error %+v", status.Message, status)
		}
	}
}

// UploadUpdate uploads the update to the store.
func (s Store) UploadUpdate(appID, filePath string) (result string, err error) {
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

	req, err := http.NewRequest(http.MethodPost, apiURL, file)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	accessToken, err := s.Client.Authorize()
	if err != nil {
		return "", fmt.Errorf("authorizing: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+accessToken)
	req.Header.Add("Content-Type", "application/zip")

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

// UploadStatus returns the status of the upload.
func (s Store) UploadStatus(appID, operationID string) (response *UploadStatusResponse, err error) {
	apiPath := "v1/products"
	apiURL := s.URL.JoinPath(apiPath, appID, "submissions/draft/package/operations", operationID).String()

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	accessToken, err := s.Client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("authorizing: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

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

	accessToken, err := s.Client.Authorize()
	if err != nil {
		return "", fmt.Errorf("authorizing: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

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

	accessToken, err := s.Client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("authorizing: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

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

	if response.Status == Failed.String() {
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
