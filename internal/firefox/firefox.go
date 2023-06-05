// Package firefox package contains methods for working with AMO store.
package firefox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/log"
	"github.com/adguardteam/go-webext/internal/fileutil"
	"github.com/adguardteam/go-webext/internal/spinner"
)

// Store type describes store structure.
type Store struct {
	API API
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

// manifest describes required fields parsed from the manifest.
// extensions might have either "applications" or "browser_specific_settings"
type manifest struct {
	Version                 string                  `json:"version"`
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
		return nil, fmt.Errorf("can't get version from manifest: %q", zipFilepath)
	}

	resultData.version = manifest.Version

	return resultData, nil
}

// UploadStatusFiles represents upload status files structure
type UploadStatusFiles struct {
	DownloadURL string `json:"download_url"`
	Hash        string `json:"hash"`
	Signed      bool   `json:"signed"`
}

// ReviewedStatus represents reviewed status structure
type ReviewedStatus bool

// UnmarshalJSON parses ReviewedStatus.
// used because review status may be boolean or string
func (w *ReviewedStatus) UnmarshalJSON(b []byte) error {
	rawString := string(b)

	stringVal, err := strconv.Unquote(rawString)
	if err != nil {
		stringVal = rawString
	}

	boolVal, err := strconv.ParseBool(stringVal)
	if err == nil {
		*w = ReviewedStatus(boolVal)
		return nil
	}
	if len(stringVal) > 0 {
		*w = true
	} else {
		*w = false
	}

	return nil
}

// UploadStatus represents upload status structure
type UploadStatus struct {
	GUID             string              `json:"guid"`
	Active           bool                `json:"active"`
	AutomatedSigning bool                `json:"automated_signing"`
	Files            []UploadStatusFiles `json:"files"`
	PassedReview     bool                `json:"passed_review"`
	Pk               string              `json:"pk"`
	Processed        bool                `json:"processed"`
	Reviewed         ReviewedStatus      `json:"reviewed"`
	URL              string              `json:"url"`
	Valid            bool                `json:"valid"`
	ValidationURL    string              `json:"validation_url"`
	Version          string              `json:"version"`
}

// TODO (maximtop): consider returning the same response for the chrome store.

// StatusResponse represents a generic response from the status request.
type StatusResponse struct {
	ID             string
	Status         string
	CurrentVersion string
}

// API is an interface for the store client.
type API interface {
	UploadStatus(appID, version string) (*UploadStatus, error)
	UploadSource(appID, versionID string, fileData io.Reader) error
	VersionID(appID, version string) (string, error)
	UploadNew(fileData io.Reader) error
	UploadUpdate(appID, version string, fileData io.Reader) error
	DownloadSignedByURL(url string) ([]byte, error)
	Status(appID string) (*StatusResponse, error)
}

// awaitValidation awaits validation of the extension.
func (s *Store) awaitValidation(appID, version string) (err error) {
	// TODO (maximtop): move constants to config
	// with one 1 second timeout request may be throttled, so we use 5 seconds
	const retryInterval = time.Second * 5
	const maxAwaitTime = time.Minute * 20

	startTime := time.Now()

	for {
		if time.Since(startTime) > maxAwaitTime {
			return fmt.Errorf("await validation timeout")
		}

		uploadStatus, err := s.API.UploadStatus(appID, version)
		if err != nil {
			return fmt.Errorf("getting upload status: %w", err)
		}

		if uploadStatus.Processed {
			if uploadStatus.Valid {
				log.Debug("[awaitValidation] extension with id: %s, version: %s is valid", appID, version)
			} else {
				return fmt.Errorf("not valid, validation url: %s", uploadStatus.ValidationURL)
			}
			break
		}
		log.Debug("[awaitValidation] upload not processed yet, retrying in: %s", retryInterval)
		time.Sleep(retryInterval)
	}

	return nil
}

// awaitSigning waits for the extension to be signed.
func (s *Store) awaitSigning(appID, version string) (err error) {
	log.Debug("start waiting for signing of extension: %s", appID)

	// TODO (maximtop): move constants to config
	// with one 1 second timeout request may be throttled, so we use 5 seconds
	const retryInterval = time.Second * 5
	const maxAwaitTime = time.Minute * 20

	startTime := time.Now()

	for {
		if time.Since(startTime) > maxAwaitTime {
			return fmt.Errorf("await signing timeout")
		}

		uploadStatus, err := s.API.UploadStatus(appID, version)
		if err != nil {
			return fmt.Errorf("getting upload status for appID: %s, version: %s, due to: %w", appID, version, err)
		}

		var signedAndReady bool
		var requiresManualReview bool
		if uploadStatus.Valid {
			signedAndReady = uploadStatus.Active &&
				bool(uploadStatus.Reviewed) &&
				len(uploadStatus.Files) > 0
			requiresManualReview = !uploadStatus.AutomatedSigning
		}

		if signedAndReady {
			log.Debug("[awaitSigning] extension is signed and ready: %s", appID)
			return nil
		} else if requiresManualReview {
			return fmt.Errorf("extension won't be signed automatically, status: %+v", uploadStatus)
		} else {
			log.Debug("[awaitSigning] extension is not processed yet, retry in %s", retryInterval)
			time.Sleep(retryInterval)
		}
	}
}

// uploadSource uploads source code of the extension to the store.
// Source can be uploaded only after the extension is validated.
func (s *Store) uploadSource(appID, versionID, sourcePath string) (err error) {
	log.Debug("uploading source for appID: %s, versionID: %s", appID, versionID)

	file, err := os.Open(filepath.Clean(sourcePath))
	if err != nil {
		return fmt.Errorf("opening file %s, error: %w", sourcePath, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	err = s.API.UploadSource(appID, versionID, file)
	if err != nil {
		return fmt.Errorf("uploading source for app id: %s, version id: %s, error: %w", appID, versionID, err)
	}

	log.Debug("successfully uploaded source")

	return nil
}

// uploadUpdate uploads the extension update.
func (s *Store) uploadUpdate(appID, version, filePath string) (err error) {
	log.Debug("start uploading update for extension: %q", filePath)

	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("opening file: %q, due to: %w", filePath, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	err = s.API.UploadUpdate(appID, version, file)

	log.Debug("successfully uploaded update for extension: %q", filePath)

	return nil
}

// downloadSigned downloads signed extension.
func (s *Store) downloadSigned(appID, version string) (filename string, err error) {
	log.Debug("start downloading signed extension: %s", appID)

	uploadStatus, err := s.API.UploadStatus(appID, version)
	if err != nil {
		return "", fmt.Errorf("getting upload status: %s, version: %s, due to: %w", appID, version, err)
	}

	if len(uploadStatus.Files) == 0 {
		return "", fmt.Errorf("no files to download")
	}

	downloadURL := uploadStatus.Files[0].DownloadURL

	response, err := s.API.DownloadSignedByURL(downloadURL)
	if err != nil {
		return "", fmt.Errorf("downloading signed extension: %s, due to: %w", downloadURL, err)
	}

	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		return "", fmt.Errorf("parsing download URL: %s due to: %w", downloadURL, err)
	}

	filename = path.Base(parsedURL.Path)

	file, err := os.Create(filepath.Clean(filename))
	if err != nil {
		return "", fmt.Errorf("creating file: %s due to: %w", filename, err)
	}

	_, err = io.Copy(file, bytes.NewReader(response))
	if err != nil {
		return "", fmt.Errorf("writing response to file: %s due to: %w", filename, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	log.Debug("successfully downloaded signed extension: %s", appID)

	return filename, nil
}

// Insert uploads extension to the amo for the first time.
func (s *Store) Insert(filePath, sourcepath string) (err error) {
	log.Debug("start uploading new extension: %q, with source: %s", filePath, sourcepath)

	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return fmt.Errorf("opening file: %q, due to: %w", filePath, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	spin := spinner.New()
	spin.Start("uploading extension")

	err = s.API.UploadNew(file)
	if err != nil {
		return fmt.Errorf("uploading new extension: %w", err)
	}

	extData, err := extDataFromFile(filePath)
	if err != nil {
		return fmt.Errorf("parsing manifest: %q, error: %w", filePath, err)
	}

	appID := extData.appID
	version := extData.version

	spin.Message("awaiting validation")
	err = s.awaitValidation(appID, version)
	if err != nil {
		return fmt.Errorf("validating extension: %s, version: %s, error: %w", appID, version, err)
	}

	spin.Message("getting version id")
	versionID, err := s.API.VersionID(appID, version)
	if err != nil {
		return fmt.Errorf("getting version ID: %s, version: %s, error: %w", appID, version, err)
	}

	// TODO (maximtop): write tests for this case
	if sourcepath != "" {
		spin.Message("uploading source")
		// TODO (maximtop): create throttler between requests, if requests are sent to fast, there is a chance that
		//  the server will return 429 error. And test it properly. Make sure tests do not run too much time.
		// wait 5 sec before uploading source code
		time.Sleep(5 * time.Second)
		err = s.uploadSource(appID, versionID, sourcepath)
		if err != nil {
			return fmt.Errorf("uploading source: %s, version: %s, sourcepath: %s, error: %w", appID, version, sourcepath, err)
		}
	}
	spin.Stop("extension uploaded")

	log.Debug("successfully uploaded new extension: %q", filePath)

	return nil
}

// Update uploads new version of extension to the store
// Before uploading it reads manifest.json for getting extension version and uuid.
func (s *Store) Update(filepath, sourcepath string) (err error) {
	log.Debug("start uploading update for extension: %s, with source: %s", filepath, sourcepath)

	extData, err := extDataFromFile(filepath)
	if err != nil {
		return fmt.Errorf("getting extension data: %q due to: %w", filepath, err)
	}

	appID := extData.appID
	version := extData.version

	err = s.uploadUpdate(appID, version, filepath)
	if err != nil {
		return fmt.Errorf("uploading update for extension: %s, version: %s, due to: %w", appID, version, err)
	}

	// TODO (maximtop): create throttler between requests, if requests are sent to fast, there is a chance that
	//  the server will not found this version. And test it properly. Make sure tests do not run too much time.
	time.Sleep(5 * time.Second)
	err = s.awaitValidation(appID, version)
	if err != nil {
		return fmt.Errorf("awaiting validation of extension: %s, version: %s, due to: %w", appID, version, err)
	}

	versionID, err := s.API.VersionID(appID, version)
	if err != nil {
		return fmt.Errorf("getting version for appID: %s, version: %s, due to: %w", appID, version, err)
	}

	// TODO (maximtop): write tests for this case
	if sourcepath != "" {
		// TODO (maximtop): create throttler between requests, if requests are sent to fast, there is a chance that
		//  the server will return 429 error.
		// wait 5 sec before uploading source code
		time.Sleep(5 * time.Second)
		err = s.uploadSource(appID, versionID, sourcepath)
		if err != nil {
			return fmt.Errorf("uploading source for appID: %s, version: %s, sourcepath: %s, due to: %w", appID, version, sourcepath, err)
		}
	}

	log.Debug("successfully uploaded update for extension: %s", filepath)

	return nil
}

// Sign uploads the extension to the store, waits for signing, downloads and saves the signed
// extension in the directory
func (s *Store) Sign(filepath, sourcepath string) (filename string, err error) {
	log.Debug("start signing extension: %s", filepath)

	extData, err := extDataFromFile(filepath)
	if err != nil {
		return "", fmt.Errorf("getting extension data from file '%s': %w", filepath, err)
	}

	appID := extData.appID
	version := extData.version

	err = s.uploadUpdate(appID, version, filepath)
	if err != nil {
		return "", fmt.Errorf("uploading extension '%s' with version '%s': %w", appID, version, err)
	}

	// After uploading the extension, we upload the source code if sourcepath was provided.
	if sourcepath != "" {
		err = s.awaitValidation(appID, version)
		if err != nil {
			return "", fmt.Errorf("validating extension '%s' with version '%s': %w", appID, version, err)
		}

		versionID, err := s.API.VersionID(appID, version)
		if err != nil {
			return "", fmt.Errorf("getting versionID for extension '%s' with version '%s': %w", appID, version, err)
		}

		err = s.uploadSource(appID, versionID, sourcepath)
		if err != nil {
			return "", fmt.Errorf("uploading source for extension '%s' with version '%s' and source path '%s': %w", appID, version, sourcepath, err)
		}
	}

	err = s.awaitSigning(appID, version)
	if err != nil {
		return "", fmt.Errorf("waiting signing of extension '%s' with version '%s': %w", appID, version, err)
	}

	filename, err = s.downloadSigned(appID, version)
	if err != nil {
		return "", fmt.Errorf("downloading signed extension '%s' with version '%s': %w", appID, version, err)
	}

	return filename, nil
}

// Status returns status of the extension by appID.
func (s *Store) Status(appID string) (result *StatusResponse, err error) {
	log.Debug("getting status for extension: %s", appID)
	response, err := s.API.Status(appID)
	if err != nil {
		return nil, err
	}

	// TODO (maximtop): make identical responses for all browsers
	log.Debug("status response: %s", response)
	return response, nil
}
