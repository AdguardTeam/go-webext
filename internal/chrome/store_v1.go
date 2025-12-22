// Package chrome helps to interact with the Chrome Web Store.
package chrome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

// StoreV1 implements Chrome Web Store API v1.1.
type StoreV1 struct {
	client *Client
	url    *url.URL
	logger *slog.Logger
}

// StoreV1Config contains configuration parameters for creating a Chrome extension store v1 instance.
type StoreV1Config struct {
	Client *Client
	URL    *url.URL
	Logger *slog.Logger
}

// NewStoreV1 creates a new Chrome extension store v1 instance.
func NewStoreV1(config StoreV1Config) *StoreV1 {
	return &StoreV1{
		client: config.Client,
		url:    config.URL,
		logger: config.Logger,
	}
}

// ItemError describes an error in v1 API response.
// Used by both Status and Upload operations.
type ItemError struct {
	ErrorCode   string `json:"error_code"`
	ErrorDetail string `json:"error_detail"`
}

// StatusResponseV1 describes v1 status response fields.
type StatusResponseV1 struct {
	Kind          string      `json:"kind"`
	ID            string      `json:"id"`
	PublicKey     string      `json:"publicKey"`
	UploadStateV1 string      `json:"uploadState"`
	CrxVersion    string      `json:"crxVersion"`
	ItemError     []ItemError `json:"itemError,omitempty"`
}

// Status retrieves status of the extension using v1.1 API.
func (s *StoreV1) Status(itemID string) (*StatusResponseV1, error) {
	l := s.logger.With(
		"action", "Status",
		"item_id", itemID,
		"api_version", "v1.1",
	)
	l.Debug("retrieving extension status")

	// v1.1 API: /chromewebstore/v1.1/items/{itemId}?projection=DRAFT
	const apiPath = "chromewebstore/v1.1/items"
	apiURL := s.url.JoinPath(apiPath, itemID)
	q := apiURL.Query()
	q.Add("projection", "DRAFT")
	apiURL.RawQuery = q.Encode()

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	result := &StatusResponseV1{}
	err = makeRequest(
		http.MethodGet,
		apiURL.String(),
		accessToken,
		requestTimeout,
		result,
		nil,
	)
	if err != nil {
		return nil, err
	}

	l.Debug(
		"status retrieval completed",
		"status", "success",
		"item_id", result.ID,
		"upload_state", result.UploadStateV1,
	)

	return result, nil
}

// UploadStateV1 describes the state of request.
type UploadStateV1 uint8

const (
	// UploadStateInvalidV1 default invalid state.
	UploadStateInvalidV1 UploadStateV1 = iota
	// UploadStateSuccessV1 means that the request was successful.
	UploadStateSuccessV1
	// UploadStateFailureV1 means that the request failed.
	UploadStateFailureV1
	// UploadStateInProgressV1 means that the request is in progress.
	UploadStateInProgressV1
	// UploadStateNotFoundV1 means that the item was not found.
	UploadStateNotFoundV1
)

// String returns the string representation of the UploadStateV1.
func (u UploadStateV1) String() string {
	switch u {
	case UploadStateSuccessV1:
		return "SUCCESS"
	case UploadStateFailureV1:
		return "FAILURE"
	case UploadStateInProgressV1:
		return "IN_PROGRESS"
	case UploadStateNotFoundV1:
		return "NOT_FOUND"
	}

	return fmt.Sprintf("!bad_status_%d", u)
}

// UnmarshalJSON unmarshals the string representation of the UploadStateV1 type
func (u *UploadStateV1) UnmarshalJSON(data []byte) error {
	var state string

	err := json.Unmarshal(data, &state)
	if err != nil {
		return err
	}

	switch state {
	case "SUCCESS":
		*u = UploadStateSuccessV1
	case "FAILURE":
		*u = UploadStateFailureV1
	case "IN_PROGRESS":
		*u = UploadStateInProgressV1
	case "NOT_FOUND":
		*u = UploadStateNotFoundV1
	default:
		return fmt.Errorf("unknown upload state: %q", state)
	}

	return nil
}

// MarshalJSON marshals the UploadStateV1 type to string.
func (u UploadStateV1) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// ItemResourceV1 describes an item resource in v1 API.
type ItemResourceV1 struct {
	Kind          string        `json:"kind"`
	ID            string        `json:"id"`
	UploadStateV1 UploadStateV1 `json:"uploadState"`
	ItemError     []ItemError   `json:"itemError"`
}

// ItemResourceV1 describes structure returned on the update and insert requests.
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

// Insert creates a new extension using v1.1 insert API.
func (s *StoreV1) Insert(filePath string) (*ItemResourceV1, error) {
	l := s.logger.With(
		"action", "Insert",
		"file_path", filePath,
		"api_version", "v1.1",
	)
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
	defer body.Close()

	result := &ItemResourceV1{}
	err = makeZipRequest(
		http.MethodPost,
		apiURL,
		body,
		accessToken,
		requestTimeout,
		result,
	)
	if err != nil {
		return nil, err
	}

	if result.UploadStateV1 != UploadStateSuccessV1 {
		return nil, fmt.Errorf("non success upload state received: %v, %v", result.UploadStateV1, result.ItemError)
	}

	l.Debug(
		"extension insert completed",
		"status", "success",
		"item_id", result.ID,
		"upload_state", result.UploadStateV1,
	)

	return result, nil
}

// Update updates an existing extension using v1.1 update API.
func (s *StoreV1) Update(itemID, filePath string) (*ItemResourceV1, error) {
	l := s.logger.With(
		"action", "Update",
		"item_id", itemID,
		"file_path", filePath,
		"api_version", "v1.1",
	)
	l.Debug("initiating extension update")

	const apiPath = "upload/chromewebstore/v1.1/items"
	apiURL := s.url.JoinPath(apiPath, itemID).String()

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	body, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer body.Close()

	result := &ItemResourceV1{}
	err = makeZipRequest(
		http.MethodPut,
		apiURL,
		body,
		accessToken,
		requestTimeout,
		result,
	)
	if err != nil {
		return nil, err
	}

	if result.UploadStateV1 == UploadStateFailureV1 {
		return nil, fmt.Errorf("failure in response: %v", result.ItemError)
	}

	l.Debug(
		"extension update completed",
		"status", "success",
		"item_id", result.ID,
		"upload_state", result.UploadStateV1,
	)

	return result, nil
}

// PublishResponseV1 describes v1 publish response.
type PublishResponseV1 struct {
	Kind         string   `json:"kind"`
	ItemID       string   `json:"item_id"`
	Status       []string `json:"status"`
	StatusDetail []string `json:"statusDetail"`
}

// PublishOptionsV1 describes v1 publish options.
type PublishOptionsV1 struct {
	// Target specifies the publish target. Can be "trustedTesters" or "default".
	Target string `json:"target,omitempty"`
	// DeployPercentage specifies percentage of existing users who will receive an update.
	// Valid values are 0-100. New users always receive the latest version.
	// Once set, this value can only be increased.
	DeployPercentage *int `json:"deployPercentage,omitempty"`
	// ReviewExemption if true, attempts to use expedited review process.
	ReviewExemption bool `json:"reviewExemption,omitempty"`
}

// Publish publishes an extension to the store using v1.1 API.
func (s *StoreV1) Publish(itemID string, opts *PublishOptionsV1) (*PublishResponseV1, error) {
	l := s.logger.With(
		"action", "Publish",
		"item_id", itemID,
		"options", opts,
		"api_version", "v1.1",
	)
	l.Debug("initiating extension publish")

	const apiPath = "chromewebstore/v1.1/items"
	apiURL := s.url.JoinPath(apiPath, itemID, "publish")

	// Validate options
	if opts != nil && opts.DeployPercentage != nil {
		if *opts.DeployPercentage < 0 || *opts.DeployPercentage > 100 {
			return nil, fmt.Errorf("deploy percentage must be between 0 and 100, got %d", *opts.DeployPercentage)
		}
	}

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	// Prepare request body with options
	var reqOpts *RequestOptions
	if opts != nil {
		jsonData, err := json.Marshal(opts)
		if err != nil {
			return nil, fmt.Errorf("marshaling options: %w", err)
		}
		reqOpts = &RequestOptions{
			Body:        bytes.NewReader(jsonData),
			ContentType: "application/json",
		}
	}

	result := &PublishResponseV1{}
	err = makeRequest(
		http.MethodPost,
		apiURL.String(),
		accessToken,
		extendedRequestTimeout,
		result,
		reqOpts,
	)
	if err != nil {
		return nil, err
	}

	l.Debug(
		"extension publish completed",
		"status", "success",
		"item_id", result.ItemID,
		"statuses", result.Status,
	)

	return result, nil
}
