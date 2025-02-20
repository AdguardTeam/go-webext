// Package firefox package contains methods for working with AMO store.
package firefox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/adguardteam/go-webext/internal/fileutil"
)

// Store type describes store structure.
type Store struct {
	api    API
	logger *slog.Logger
}

// StoreConfig contains configuration parameters for creating a Firefox extension store instance
type StoreConfig struct {
	API    API
	Logger *slog.Logger
}

// NewStore creates a new Firefox extension store instance
func NewStore(config StoreConfig) *Store {
	return &Store{
		api:    config.API,
		logger: config.Logger,
	}
}

type gecko struct {
	ID string `json:"id"`
}

type applications struct {
	Gecko gecko `json:"gecko"`
}

type browserSpecificSettings struct {
	Gecko gecko `json:"gecko"`
}

// Channel represents channel type of extension either listed or unlisted.
type Channel string

const (
	// ChannelUnlisted represents unlisted channel.
	ChannelUnlisted Channel = "unlisted"
	// ChannelListed represents listed channel.
	ChannelListed Channel = "listed"
)

// NewChannel creates new channel from string.
func NewChannel(channel string) (Channel, error) {
	switch channel {
	case "listed":
		return ChannelListed, nil
	case "unlisted":
		return ChannelUnlisted, nil
	default:
		return "", fmt.Errorf("unknown channel: %q", channel)
	}
}

// manifest describes required fields parsed from the manifest.
// extensions might have either "applications" or "browser_specific_settings"
type manifest struct {
	Version                 string                  `json:"Version"`
	Applications            applications            `json:"applications"`
	BrowserSpecificSettings browserSpecificSettings `json:"browser_specific_settings"`
}

// parseManifest reads zip archive, and extracts manifest.json out of it.
func parseManifest(zipFilepath string) (result manifest, err error) {
	fileContent, err := fileutil.ReadFileFromZip(zipFilepath, "manifest.json")
	if err != nil {
		return manifest{}, fmt.Errorf("can't read manifest.json from zip file %q due to: %w", zipFilepath, err)
	}

	err = json.Unmarshal(fileContent, &result)
	if err != nil {
		return manifest{}, fmt.Errorf("can't unmarshal manifest.json %q due to: %w", zipFilepath, err)
	}

	return result, nil
}

// extensionData various form of different extension data extracted from manifest.
type extensionData struct {
	appID   string
	version string
}

// extDataFromFile retrieves extensionData from manifest and validates it.
func extDataFromFile(zipFilepath string) (*extensionData, error) {
	manifest, err := parseManifest(zipFilepath)
	if err != nil {
		return nil, fmt.Errorf("can't parse manifest: %w", err)
	}

	resultData := &extensionData{}

	if manifest.Applications.Gecko.ID != "" {
		resultData.appID = manifest.Applications.Gecko.ID
	} else if manifest.BrowserSpecificSettings.Gecko.ID != "" {
		resultData.appID = manifest.BrowserSpecificSettings.Gecko.ID
	} else {
		return nil, fmt.Errorf("can't get appID from manifest: %q", zipFilepath)
	}

	if manifest.Version == "" {
		return nil, fmt.Errorf("can't get Version from manifest: %q", zipFilepath)
	}

	resultData.version = manifest.Version

	return resultData, nil
}

// StatusResponse represents a generic response from the status request.
type StatusResponse struct {
	ID             string
	Status         string
	CurrentVersion string
}

// FileInfo represents file info structure.
type FileInfo struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

// CompatibilityInfo represents firefox compatibility info structure.
type CompatibilityInfo struct {
	Min string `json:"min"`
	Max string `json:"max"`
}

// Compatibility represents compatibility info structure.
type Compatibility struct {
	Firefox CompatibilityInfo `json:"firefox"`
}

// VersionInfo describes response with Version structure.
// Actually info structure is bigger, but we don't need it.
// https://addons-server.readthedocs.io/en/latest/topics/api/addons.html#get--api-v5-addons-addon-(int-addon_id|string-addon_slug|string-addon_guid)-versions-(int-id|string-version_number)-
type VersionInfo struct {
	ID                           int           `json:"id"`
	ApprovalNotes                string        `json:"approval_notes"`
	Channel                      string        `json:"channel"`
	Compatibility                Compatibility `json:"compatibility"`
	EditURL                      string        `json:"edit_url"`
	File                         FileInfo      `json:"file"`
	IsDisabled                   bool          `json:"is_disabled"`
	IsStrictCompatibilityEnabled bool          `json:"is_strict_compatibility_enabled"`
	License                      interface{}   `json:"license"`
	ReleaseNotes                 interface{}   `json:"release_notes"`
	Reviewed                     interface{}   `json:"reviewed"`
	Source                       interface{}   `json:"source"`
	Version                      string        `json:"version"`
}

// VersionsListResponse represents list of versions.
type VersionsListResponse struct {
	Count    int           `json:"count"`
	Next     string        `json:"next"`
	Previous string        `json:"previous"`
	Results  []VersionInfo `json:"results"`
}

// AddonInfo describes request body for addon creation.
// Actually info structure is bigger, but we don't need it.
// https://addons-server.readthedocs.io/en/latest/topics/api/addons.html#get--api-v5-addons-addon-(int-id|string-slug|string-guid)-
type AddonInfo struct {
	ID      int `json:"id"`
	Authors []struct {
		ID         int         `json:"id"`
		Name       string      `json:"name"`
		URL        string      `json:"url"`
		Username   string      `json:"username"`
		PictureURL interface{} `json:"picture_url"`
	} `json:"authors"`
	AverageDailyUsers int          `json:"average_daily_users"`
	Categories        struct{}     `json:"categories"`
	ContributionsURL  string       `json:"contributions_url"`
	Created           time.Time    `json:"created"`
	CurrentVersion    *VersionInfo `json:"current_version"`
	DefaultLocale     string       `json:"default_locale"`
	Description       interface{}  `json:"description"`
	DeveloperComments interface{}  `json:"developer_comments"`
	EditURL           string       `json:"edit_url"`
	GUID              string       `json:"guid"`
	HasEula           bool         `json:"has_eula"`
	HasPrivacyPolicy  bool         `json:"has_privacy_policy"`
	Homepage          interface{}  `json:"homepage"`
	IconURL           string       `json:"icon_url"`
	Icons             struct {
		Field1 string `json:"32"`
		Field2 string `json:"64"`
		Field3 string `json:"128"`
	} `json:"icons"`
	IsDisabled     bool      `json:"is_disabled"`
	IsExperimental bool      `json:"is_experimental"`
	LastUpdated    time.Time `json:"last_updated"`
	Name           struct {
		EnUS string `json:"en-US"`
	} `json:"name"`
	Previews []interface{} `json:"previews"`
	Promoted interface{}   `json:"promoted"`
	Ratings  struct {
		Average         float64 `json:"average"`
		BayesianAverage float64 `json:"bayesian_average"`
		Count           int     `json:"count"`
		TextCount       int     `json:"text_count"`
	} `json:"ratings"`
	RatingsURL      string `json:"ratings_url"`
	RequiresPayment bool   `json:"requires_payment"`
	ReviewURL       string `json:"review_url"`
	Slug            string `json:"slug"`
	Status          string `json:"status"`
	Summary         struct {
		EnUS string `json:"en-US"`
	} `json:"summary"`
	SupportEmail          interface{}   `json:"support_email"`
	SupportURL            interface{}   `json:"support_url"`
	Tags                  []interface{} `json:"tags"`
	Type                  string        `json:"type"`
	URL                   string        `json:"url"`
	VersionsURL           string        `json:"versions_url"`
	WeeklyDownloads       int           `json:"weekly_downloads"`
	LatestUnlistedVersion *VersionInfo  `json:"latest_unlisted_version"`
	Version               *VersionInfo  `json:"version"`
}

// UploadDetail is a status of the upload .
type UploadDetail struct {
	UUID       string      `json:"uuid"`
	Channel    string      `json:"channel"`
	Processed  bool        `json:"processed"`
	Submitted  bool        `json:"submitted"`
	URL        string      `json:"url"`
	Valid      bool        `json:"valid"`
	Validation interface{} `json:"validation"`
	Version    string      `json:"Version"`
}

// API is an interface for the store client.
type API interface {
	DownloadSignedByURL(url string) ([]byte, error)
	Status(appID string) (*StatusResponse, error)
	CreateUpload(fileData io.Reader, c Channel) (*UploadDetail, error)
	UploadDetail(UUID string) (*UploadDetail, error)
	CreateVersion(appID, UUID string) (*VersionInfo, error)
	VersionDetail(appID, versionID string) (versionInfo *VersionInfo, err error)
	CreateAddon(UUID string) (*AddonInfo, error)
	AttachSourceToVersion(appID, versionID string, sourceData io.Reader) (err error)
	VersionsList(appID string) ([]*VersionInfo, error)
}

// awaitUploadValidation awaits validation of the upload.
func (s *Store) awaitUploadValidation(UUID string) (err error) {
	l := s.logger.With("action", "awaitUploadValidation", "uuid", UUID)
	l.Debug("awaiting upload validation")

	// TODO (maximtop): move constants to config
	// with one 1 second timeout request may be throttled, so we use 5 seconds
	const retryInterval = time.Second * 5
	const maxAwaitTime = time.Minute * 20

	startTime := time.Now()

	for {
		if elapsed := time.Since(startTime); elapsed > maxAwaitTime {
			return fmt.Errorf("await validation timeout after %v, maximum allowed time is %v", elapsed, maxAwaitTime)
		}

		uploadDetail, err := s.api.UploadDetail(UUID)
		if err != nil {
			return fmt.Errorf("getting upload status: %w", err)
		}

		if uploadDetail.Processed {
			if uploadDetail.Valid {
				l.Debug("extension validation successful")
			} else {
				l.Debug("extension validation failed")

				// Pretty print upload details
				prettyJSON, err := json.MarshalIndent(uploadDetail, "", "    ")
				if err != nil {
					return fmt.Errorf("failed to generate pretty JSON: %w", err)
				}

				// Log pretty printed JSON
				l.Debug(
					"upload validation details",
					"stage", "awaitUploadValidation",
					"raw_details", prettyJSON,
				)

				return fmt.Errorf("not valid, validation url: %s", uploadDetail.URL)
			}
			break
		}
		l.Debug(
			"upload processing pending",
			"retry_interval", retryInterval,
			"status", "pending",
		)
		time.Sleep(retryInterval)
	}

	return nil
}

// awaitSigning waits for the extension to be signed.
func (s *Store) awaitVersionSigning(appID, versionID string) (err error) {
	l := s.logger.With("action", "awaitVersionSigning", "appID", appID, "versionID", versionID)
	l.Debug("start waiting for signing of extension")

	// TODO (maximtop): move constants to config
	// with one 1 second timeout request may be throttled, so we use 5 seconds
	const retryInterval = time.Second * 5
	const maxAwaitTime = time.Minute * 20

	startTime := time.Now()

	for {
		if time.Since(startTime) > maxAwaitTime {
			return fmt.Errorf("await signing timeout")
		}

		versionDetail, err := s.api.VersionDetail(appID, versionID)
		if err != nil {
			return fmt.Errorf("getting upload status for appID: %s, versionID: %s, due to: %w", appID, versionID, err)
		}

		if versionDetail.File.Status == "public" {
			l.Debug("extension is signed and ready")
			return nil
		}
		if versionDetail.File.Status == "disabled" {
			return fmt.Errorf("extension won't be signed automatically, version detail: %+v", versionDetail)
		}

		l.Debug(
			"extension signing pending",
			"retry_interval", retryInterval,
			"status", "pending",
		)

		time.Sleep(retryInterval)
	}
}

// downloadSigned downloads signed extension.
// If output is empty, then it will be set to "firefox.xpi".
func (s *Store) downloadSigned(appID, versionID, output string) error {
	l := s.logger.With("action", "downloadSigned", "appID", appID)
	l.Debug("initiating signed extension download")

	if output == "" {
		output = "firefox.xpi"
	}

	versionDetail, err := s.api.VersionDetail(appID, versionID)
	if err != nil {
		return fmt.Errorf("getting version detail for appID: %s, versionID: %s, due to: %w", appID, versionID, err)
	}

	downloadURL := versionDetail.File.URL

	response, err := s.api.DownloadSignedByURL(downloadURL)
	if err != nil {
		return fmt.Errorf("downloading signed extension: %s, due to: %w", downloadURL, err)
	}

	file, err := os.Create(filepath.Clean(output))
	if err != nil {
		return fmt.Errorf("creating file: %s due to: %w", output, err)
	}

	_, err = io.Copy(file, bytes.NewReader(response))
	if err != nil {
		return fmt.Errorf("writing response to file: %s due to: %w", output, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	l.Debug("successfully downloaded signed extension")

	return nil
}

// Status returns status of the extension by appID.
func (s *Store) Status(appID string) (result *StatusResponse, err error) {
	l := s.logger.With("action", "Status", "appID", appID)
	l.Debug("retrieving extension status")

	response, err := s.api.Status(appID)
	if err != nil {
		return nil, err
	}

	// TODO (maximtop): make identical responses for all browsers
	l.Debug(
		"full status response details",
		"raw_response", response,
	)

	return response, nil
}

// Insert uploads extension to the amo for the first time.
func (s *Store) Insert(filePath, sourcepath string) (err error) {
	l := s.logger.With("action", "Insert", "filePath", filePath, "sourcePath", sourcepath)
	l.Debug("initiating new extension upload")

	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("opening file: %q, due to: %w", filePath, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	// we do not support uploading of the first extension to the listed channel
	uploadDetail, err := s.api.CreateUpload(file, ChannelUnlisted)
	if err != nil {
		return fmt.Errorf("uploading new extension: %w", err)
	}

	// log upload details
	l.Debug(
		"upload details",
		"upload", uploadDetail,
	)

	err = s.awaitUploadValidation(uploadDetail.UUID)
	if err != nil {
		return fmt.Errorf("awaiting validation: %w", err)
	}

	addonInfo, err := s.api.CreateAddon(uploadDetail.UUID)
	if err != nil {
		return fmt.Errorf("creating addon: %w", err)
	}

	// We can't append the source before the addon is created.
	if sourcepath != "" {
		sourceReader, err := os.Open(filepath.Clean(sourcepath))
		if err != nil {
			return fmt.Errorf("opening file: %q, due to: %w", sourcepath, err)
		}
		defer func() { err = errors.WithDeferred(err, sourceReader.Close()) }()

		extData, err := extDataFromFile(filePath)
		if err != nil {
			return fmt.Errorf("parsing manifest: %q, error: %w", filePath, err)
		}

		err = s.api.AttachSourceToVersion(extData.appID, strconv.Itoa(addonInfo.Version.ID), sourceReader)
		if err != nil {
			return fmt.Errorf("uploading source: %w", err)
		}
	}

	return nil
}

// Update uploads new Version of extension to the store
// Before uploading it reads manifest.json for getting extension Version and uuid.
func (s *Store) Update(extpath, sourcepath string, channel Channel) (err error) {
	l := s.logger.With("action", "Update", "extpath", extpath, "sourcepath", sourcepath, "channel", channel)
	l.Debug("initiating extension update")

	cleanExtPath := filepath.Clean(extpath)
	extData, err := extDataFromFile(cleanExtPath)
	if err != nil {
		return fmt.Errorf("getting extension data: %q due to: %w", extpath, err)
	}

	appID := extData.appID

	file, err := os.Open(filepath.Clean(extpath))
	if err != nil {
		return fmt.Errorf("opening file: %q, due to: %w", extpath, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	uploadDetail, err := s.api.CreateUpload(file, channel)
	if err != nil {
		return fmt.Errorf("creating upload: %w", err)
	}

	err = s.awaitUploadValidation(uploadDetail.UUID)
	if err != nil {
		return fmt.Errorf("awaiting validation: %w", err)
	}

	versionInfo, err := s.api.CreateVersion(appID, uploadDetail.UUID)
	if err != nil {
		return fmt.Errorf("creating version: %w", err)
	}

	if sourcepath != "" {
		cleanSourcePath := filepath.Clean(sourcepath)
		sourceReader, err := os.Open(cleanSourcePath)
		if err != nil {
			return fmt.Errorf("opening file: %q, due to: %w", cleanSourcePath, err)
		}
		err = s.api.AttachSourceToVersion(appID, strconv.Itoa(versionInfo.ID), sourceReader)
		if err != nil {
			return fmt.Errorf("attaching source to version: %w", err)
		}
	}

	l.Debug("extension update completed")

	return nil
}

// // LogValue implements the slog.LogValue interface for VersionInfo.
// // Also is used in the api component.
// func (v VersionInfo) LogValue() slog.Value {
// 	return slog.GroupValue(
// 		slog.Int("id", v.ID),
// 		slog.String("version", v.Version),
// 		slog.String("channel", v.Channel),
// 		slog.Any("compatibility", v.Compatibility),
// 		slog.String("file_status", v.File.Status),
// 		slog.Bool("disabled", v.IsDisabled),
// 		slog.Bool("strict_compatibility", v.IsStrictCompatibilityEnabled),
// 		slog.String("approval_notes", v.ApprovalNotes),
// 		slog.Any("license", v.License),
// 		slog.Any("release_notes", v.ReleaseNotes),
// 	)
// }

// getVersionID returns versionID for the appID and version.
func (s *Store) getVersionID(appID, version string) (versionID string, err error) {
	l := s.logger.With("action", "getVersionID", "appID", appID, "version", version)
	l.Debug("getting version ID")

	versionsList, err := s.api.VersionsList(appID)
	if err != nil {
		return "", fmt.Errorf("getting versions list for appID: %s, due to: %w", appID, err)
	}

	for _, v := range versionsList {
		if v.Version == version {
			l.Debug(
				"version details",
				"version", v,
			)

			return strconv.Itoa(v.ID), nil
		}
	}

	return "", nil
}

// hasVersion checks if a specific version of the app is already uploaded and is in a valid state.
func (s *Store) hasVersion(appID, version string) (versionID string, err error) {
	versionID, err = s.getVersionID(appID, version)
	if err != nil {
		return "", err
	}
	return versionID, err
}

// isSigned checks if the extension is already uploaded and signed.
func (s *Store) isSigned(appID, versionID string) (bool, error) {
	l := s.logger.With("action", "isSigned", "appID", appID, "versionID", versionID)
	l.Debug("checking if extension is signed")

	versionDetail, err := s.api.VersionDetail(appID, versionID)
	if err != nil {
		return false, fmt.Errorf("failed to get upload status for appID: %s, versionID: %s, error: %w", appID, versionID, err)
	}

	if versionDetail.File.Status == "public" {
		l.Debug(
			"full version details",
			"version", versionDetail,
		)

		return true, nil
	} else if versionDetail.File.Status == "disabled" {
		return false, fmt.Errorf("extension will not be signed automatically, version detail: %+v", versionDetail)
	}

	return false, fmt.Errorf("extension is pending signature, version detail: %+v", versionDetail)
}

// Sign uploads the extension to the store, waits for the signing process to complete, then downloads and saves the signed
// extension in the specified directory. The unlisted channel is always used for signing.
// If the extension is already uploaded, it will be downloaded and saved in the specified directory.
func (s *Store) Sign(extpath, sourcepath, output string) (err error) {
	l := s.logger.With("action", "Sign", "extpath", extpath, "sourcepath", sourcepath)
	l.Debug("initiating extension signing")

	cleanExtPath := filepath.Clean(extpath)
	extData, err := extDataFromFile(cleanExtPath)
	if err != nil {
		return fmt.Errorf("getting extension data: %q due to: %w", extpath, err)
	}

	appID := extData.appID
	version := extData.version

	// if the extension is already uploaded and signed, download it
	versionID, err := s.hasVersion(appID, version)
	if err != nil {
		return fmt.Errorf("checking version: %w", err)
	}
	if versionID != "" {
		isSigned, err := s.isSigned(appID, versionID)
		if err != nil {
			return fmt.Errorf("checking if extension is signed: %w", err)
		}
		if isSigned {
			err = s.downloadSigned(appID, versionID, output)
			if err != nil {
				return fmt.Errorf("error downloading already existing and signed extension '%s' with versionID '%s': %w", appID, versionID, err)
			}
			return nil
		}
		l.Info(
			"extension uploaded but not signed",
			"app_id", appID,
			"version", version,
			"status", "pending_signature",
		)
		return nil
	}

	file, err := os.Open(filepath.Clean(extpath))
	if err != nil {
		return fmt.Errorf("error opening file %q: %w", extpath, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	uploadDetail, err := s.api.CreateUpload(file, ChannelUnlisted)
	if err != nil {
		return fmt.Errorf("error creating upload: %w", err)
	}

	err = s.awaitUploadValidation(uploadDetail.UUID)
	if err != nil {
		return fmt.Errorf("error waiting for validation: %w", err)
	}

	versionInfo, err := s.api.CreateVersion(appID, uploadDetail.UUID)
	if err != nil {
		return fmt.Errorf("error creating version: %w", err)
	}

	versionID = strconv.Itoa(versionInfo.ID)

	if sourcepath != "" {
		cleanSourcePath := filepath.Clean(sourcepath)
		sourceReader, err := os.Open(cleanSourcePath)
		if err != nil {
			return fmt.Errorf("opening file: %q, due to: %w", cleanSourcePath, err)
		}
		err = s.api.AttachSourceToVersion(appID, versionID, sourceReader)
		if err != nil {
			return fmt.Errorf("error attaching source to version: %w", err)
		}
	}

	err = s.awaitVersionSigning(appID, versionID)
	if err != nil {
		return fmt.Errorf("error waiting for signing of extension '%s' with versionID '%s': %w", appID, versionID, err)
	}

	err = s.downloadSigned(appID, versionID, output)
	if err != nil {
		return fmt.Errorf("error downloading signed extension '%s' with versionID '%s': %w", appID, versionID, err)
	}

	return nil
}
