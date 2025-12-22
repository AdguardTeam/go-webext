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
)

// StoreV2 implements Chrome Web Store API v2.
type StoreV2 struct {
	client      *Client
	url         *url.URL
	publisherID string
	logger      *slog.Logger
}

// StoreV2Config contains configuration parameters for creating a Chrome extension store v2 instance.
type StoreV2Config struct {
	Client      *Client
	URL         *url.URL
	PublisherID string
	Logger      *slog.Logger
}

// NewStoreV2 creates a new Chrome extension store v2 instance.
func NewStoreV2(config StoreV2Config) *StoreV2 {
	return &StoreV2{
		client:      config.Client,
		url:         config.URL,
		publisherID: config.PublisherID,
		logger:      config.Logger,
	}
}

// DistributionChannel describes deployment information for a specific release channel.
type DistributionChannel struct {
	DeployPercentage int    `json:"deployPercentage"`
	CrxVersion       string `json:"crxVersion"`
}

// ItemRevisionStatus describes the status of an item revision.
type ItemRevisionStatus struct {
	State                ItemState             `json:"state"`
	DistributionChannels []DistributionChannel `json:"distributionChannels"`
}

// StatusResponse describes status response from v2 API.
type StatusResponse struct {
	// Name is the resource name in format "publishers/{publisherId}/items/{itemId}".
	Name string `json:"name"`
	// ItemID is the unique identifier for the Chrome extension item.
	ItemID string `json:"itemId"`
	// PublicKey is the base64-encoded public key for the extension.
	PublicKey string `json:"publicKey"`
	// PublishedItemRevisionStatus contains information about the currently published version.
	PublishedItemRevisionStatus *ItemRevisionStatus `json:"publishedItemRevisionStatus,omitempty"`
	// SubmittedItemRevisionStatus contains information about the version currently under review.
	SubmittedItemRevisionStatus *ItemRevisionStatus `json:"submittedItemRevisionStatus,omitempty"`
	// LastAsyncUploadState indicates the status of the most recent upload operation.
	LastAsyncUploadState UploadStateV2 `json:"lastAsyncUploadState,omitempty"`
	// TakenDown indicates whether the item has been taken down due to policy violations.
	TakenDown bool `json:"takenDown"`
	// Warned indicates whether the item has received a warning from Chrome Web Store.
	Warned bool `json:"warned"`
}

// Status retrieves status of the extension in the store using v2 API.
func (s *StoreV2) Status(itemID string) (result *StatusResponse, err error) {
	l := s.logger.With(
		"action", "Status",
		"item_id", itemID,
		"publisher_id", s.publisherID,
		"api_version", "v2",
	)
	l.Debug("retrieving extension status")

	// v2 API: /v2/publishers/{publisherId}/items/{itemId}:fetchStatus
	apiURL := s.url.JoinPath(
		"v2",
		"publishers",
		s.publisherID,
		"items",
		itemID+":fetchStatus",
	)

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	result = &StatusResponse{}
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
		"item_id", result.ItemID,
	)

	return result, nil
}

// UploadStateV2 describes the state of upload operation in v2 API.
type UploadStateV2 uint8

const (
	// UploadStateInvalidV2 default invalid state.
	UploadStateInvalidV2 UploadStateV2 = iota
	// UploadStateSucceededV2 means that the upload was successful.
	UploadStateSucceededV2
	// UploadStateFailedV2 means that the upload failed.
	UploadStateFailedV2
	// UploadStateInProgressV2 means that the upload is in progress.
	UploadStateInProgressV2
	// UploadStateNotFoundV2 means that the item was not found.
	UploadStateNotFoundV2
)

// String returns the string representation of UploadStateV2.
func (u UploadStateV2) String() string {
	switch u {
	case UploadStateInvalidV2:
		return "UPLOAD_STATE_UNSPECIFIED"
	case UploadStateSucceededV2:
		return "SUCCEEDED"
	case UploadStateFailedV2:
		return "FAILED"
	case UploadStateInProgressV2:
		return "IN_PROGRESS"
	case UploadStateNotFoundV2:
		return "NOT_FOUND"
	}
	return fmt.Sprintf("!bad_status_%d", u)
}

// UnmarshalJSON deserializes UploadStateV2 from JSON.
func (u *UploadStateV2) UnmarshalJSON(data []byte) error {
	var state string
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	switch state {
	case "UPLOAD_STATE_UNSPECIFIED":
		*u = UploadStateInvalidV2
	case "SUCCEEDED":
		*u = UploadStateSucceededV2
	case "FAILED":
		*u = UploadStateFailedV2
	case "IN_PROGRESS":
		*u = UploadStateInProgressV2
	case "NOT_FOUND":
		*u = UploadStateNotFoundV2
	default:
		return fmt.Errorf("unknown upload state: %q", state)
	}
	return nil
}

// MarshalJSON serializes UploadStateV2 to JSON.
func (u UploadStateV2) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

// ItemState describes the state of an item in v2 API.
// Used by Status and Publish operations.
type ItemState uint8

const (
	// ItemStateUnspecified default unspecified state.
	ItemStateUnspecified ItemState = iota
	// ItemStatePendingReview means the item is pending review.
	ItemStatePendingReview
	// ItemStateStaged means the item is staged for publishing.
	ItemStateStaged
	// ItemStatePublished means the item is published.
	ItemStatePublished
	// ItemStatePublishedToTesters means the item is published to testers.
	ItemStatePublishedToTesters
	// ItemStateRejected means the item was rejected.
	ItemStateRejected
	// ItemStateCancelled means the item submission was cancelled.
	ItemStateCancelled
)

// String returns the string representation of the ItemState.
func (i ItemState) String() string {
	switch i {
	case ItemStateUnspecified:
		return "ITEM_STATE_UNSPECIFIED"
	case ItemStatePendingReview:
		return "PENDING_REVIEW"
	case ItemStateStaged:
		return "STAGED"
	case ItemStatePublished:
		return "PUBLISHED"
	case ItemStatePublishedToTesters:
		return "PUBLISHED_TO_TESTERS"
	case ItemStateRejected:
		return "REJECTED"
	case ItemStateCancelled:
		return "CANCELLED"
	}
	return fmt.Sprintf("!bad_item_state_%d", i)
}

// UnmarshalJSON deserializes ItemState from JSON.
func (i *ItemState) UnmarshalJSON(data []byte) error {
	var state string
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	switch state {
	case "ITEM_STATE_UNSPECIFIED":
		*i = ItemStateUnspecified
	case "PENDING_REVIEW":
		*i = ItemStatePendingReview
	case "STAGED":
		*i = ItemStateStaged
	case "PUBLISHED":
		*i = ItemStatePublished
	case "PUBLISHED_TO_TESTERS":
		*i = ItemStatePublishedToTesters
	case "REJECTED":
		*i = ItemStateRejected
	case "CANCELLED":
		*i = ItemStateCancelled
	default:
		return fmt.Errorf("unknown item state: %q", state)
	}
	return nil
}

// MarshalJSON serializes ItemState to JSON.
func (i ItemState) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

// Upload submits an extension package to the store using v2 API.
func (s *StoreV2) Upload(itemID, filePath string) (result *UploadResponse, err error) {
	l := s.logger.With(
		"action", "Upload",
		"item_id", itemID,
		"publisher_id", s.publisherID,
		"file_path", filePath,
		"api_version", "v2",
	)
	l.Debug("initiating extension upload")

	// v2 API: /upload/v2/publishers/{publisherId}/items/{itemId}:upload
	apiURL := s.url.JoinPath(
		"upload",
		"v2",
		"publishers",
		s.publisherID,
		"items",
		itemID+":upload",
	)

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	body, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer body.Close()

	result = &UploadResponse{}
	err = makeZipRequest(
		http.MethodPost,
		apiURL.String(),
		body,
		accessToken,
		requestTimeout,
		result,
	)
	if err != nil {
		return nil, err
	}

	// Check for explicitly failed upload state
	if result.UploadStateV2 == UploadStateFailedV2 {
		return nil, fmt.Errorf("upload failed with state: %s", result.UploadStateV2)
	}

	l.Debug(
		"extension upload completed",
		"status", "success",
		"item_id", result.ItemID,
		"upload_state", result.UploadStateV2,
	)

	return result, nil
}

// UploadResponse describes the response from upload operation in v2 API.
type UploadResponse struct {
	Name          string        `json:"name"`
	ItemID        string        `json:"itemId"`
	CrxVersion    string        `json:"crxVersion"`
	UploadStateV2 UploadStateV2 `json:"uploadState"`
}

// PublishType describes the type of publishing.
type PublishType string

const (
	// PublishTypeUnspecified default unspecified type.
	PublishTypeUnspecified PublishType = "PUBLISH_TYPE_UNSPECIFIED"
	// PublishTypeDefault publishes immediately on approval.
	PublishTypeDefault PublishType = "DEFAULT_PUBLISH"
	// PublishTypeStaged stages for publishing in the future.
	PublishTypeStaged PublishType = "STAGED_PUBLISH"
)

// DeployInfo describes deployment configuration for rollout.
type DeployInfo struct {
	DeployPercentage int `json:"deployPercentage"`
}

// PublishOptions describes optional parameters for the publish method in v2 API.
type PublishOptions struct {
	// PublishType specifies when to publish the extension (immediately or staged).
	PublishType PublishType `json:"publishType,omitempty"`
	// DeployInfos configures gradual rollout with deployment percentage.
	DeployInfos []DeployInfo `json:"deployInfos,omitempty"`
	// SkipReview attempts to skip review process if the extension qualifies.
	SkipReview bool `json:"skipReview,omitempty"`
}

// Publish publishes an extension to the store using v2 API.
func (s *StoreV2) Publish(itemID string, opts *PublishOptions) (result *PublishResponse, err error) {
	l := s.logger.With(
		"action", "Publish",
		"item_id", itemID,
		"publisher_id", s.publisherID,
		"options", opts,
		"api_version", "v2",
	)
	l.Debug("initiating extension publish")

	// v2 API: /v2/publishers/{publisherId}/items/{itemId}:publish
	apiURL := s.url.JoinPath(
		"v2",
		"publishers",
		s.publisherID,
		"items",
		itemID+":publish",
	)

	accessToken, err := s.client.Authorize()
	if err != nil {
		return nil, fmt.Errorf("getting access token: %w", err)
	}

	var body io.Reader
	if opts != nil {
		jsonData, err := json.Marshal(opts)
		if err != nil {
			return nil, fmt.Errorf("marshaling publish options: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}

	result = &PublishResponse{}
	err = makeJSONRequest(
		http.MethodPost,
		apiURL.String(),
		body,
		accessToken,
		extendedRequestTimeout,
		result,
	)
	if err != nil {
		return nil, err
	}

	l.Debug(
		"extension publish completed",
		"status", "success",
		"item_id", result.ItemID,
		"state", result.State.String(),
	)

	return result, nil
}

// PublishResponse describes response from publish operation in v2 API.
type PublishResponse struct {
	Name   string    `json:"name"`
	ItemID string    `json:"itemId"`
	State  ItemState `json:"state"`
}
