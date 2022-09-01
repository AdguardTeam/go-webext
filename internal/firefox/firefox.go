// Package firefox package contains methods for working with AMO store.
package firefox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/log"
	"github.com/adguardteam/go-webext/internal/fileutil"
	"github.com/golang-jwt/jwt/v4"
)

// AMO main url is https://addons.mozilla.org/
// Please use AMO dev environment at https://addons-dev.allizom.org/ or the AMO stage
// environment at https://addons.allizom.org/ for testing.
// credential keys can't be sent via email, so you need to ask them in the chat https://matrix.to/#/#amo:mozilla.org
// last time I've asked them from mat https://matrix.to/#/@mat:mozilla.org
// signed xpi build from dev environments is corrupted, so you need to build it from the production environment

// Client describes client structure.
type Client struct {
	clientID     string
	clientSecret string
	now          func() int64
}

// ClientConfig config describes client config structure.
type ClientConfig struct {
	ClientID     string
	ClientSecret string
	Now          func() int64
}

// TODO(maximtop): consider to make this constant an option.
const requestTimeout = 20 * time.Minute

// maxReadLimit limits response size returned from the store api.
const maxReadLimit = 10 * fileutil.MB

// NewClient creates instance of the Client.
func NewClient(config ClientConfig) Client {
	c := config

	if c.Now == nil {
		c.Now = func() int64 {
			return time.Now().Unix()
		}
	}

	return Client{
		clientID:     c.ClientID,
		clientSecret: c.ClientSecret,
		now:          c.Now,
	}
}

// GenAuthHeader generates header used for authorization.
func (c Client) GenAuthHeader() (result string, err error) {
	const expirationSec = 5 * 60

	currentTimeSec := c.now()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": c.clientID,
		"iat": currentTimeSec,
		"exp": currentTimeSec + expirationSec,
	})

	signedToken, err := token.SignedString([]byte(c.clientSecret))
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return "JWT " + signedToken, nil
}

// Store type describes store structure.
type Store struct {
	Client *Client
	URL    *url.URL
}

// Manifest describes required fields parsed from the manifest.
type Manifest struct {
	Version      string `json:"version"`
	Applications struct {
		Gecko struct {
			ID string `json:"id"`
		} `json:"gecko"`
	} `json:"applications"`
}

// parseManifest reads zip archive, and extracts manifest.json out of it.
func parseManifest(zipFilepath string) (result Manifest, err error) {
	fileContent, err := fileutil.ReadFileFromZip(zipFilepath, "manifest.json")
	if err != nil {
		return Manifest{}, fmt.Errorf("can't read manifest.json from zip file %q due to: %w", zipFilepath, err)
	}

	err = json.Unmarshal(fileContent, &result)
	if err != nil {
		return Manifest{}, fmt.Errorf("can't unmarshal manifest.json %q due to: %w", zipFilepath, err)
	}

	return result, nil
}

// Status returns status of the extension by appID.
func (s *Store) Status(appID string) (result []byte, err error) {
	apiPath := "api/v5/addons/addon/"

	apiURL := s.URL.JoinPath(apiPath, appID).String()

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	authHeader, err := s.Client.GenAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("generating auth header: %w", err)
	}

	req.Header.Add("Authorization", authHeader)

	client := &http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got code %d, body: %q", res.StatusCode, body)
	}

	// TODO (maximtop): make identical responses for all browsers
	return body, nil
}

type version struct {
	ID      int    `json:"id"`
	Version string `json:"version"`
}

type versionResponse struct {
	PageSize  int         `json:"page_size"`
	PageCount int         `json:"page_count"`
	Count     int         `json:"count"`
	Next      interface{} `json:"next"`
	Previous  interface{} `json:"previous"`
	Results   []version   `json:"results"`
}

// VersionID retrieves version ID by version number.
func (s *Store) VersionID(appID, version string) (result string, err error) {
	log.Debug("getting version ID for appID: %s, version: %s", appID, version)

	const apiPath = "api/v5/addons/addon/"

	queryString := url.Values{}
	queryString.Add("filter", "all_with_unlisted")
	apiURL := s.URL.JoinPath(apiPath, appID, "versions").String() + "?" + queryString.Encode()

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	authHeader, err := s.Client.GenAuthHeader()
	if err != nil {
		return "", fmt.Errorf("generating auth header: %w", err)
	}

	req.Header.Add("Authorization", authHeader)

	client := &http.Client{Timeout: requestTimeout}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("got code %d, body: %q", res.StatusCode, body)
	}

	var versions versionResponse

	err = json.Unmarshal(body, &versions)
	if err != nil {
		return "", fmt.Errorf("unmarshalling response body: %s, error: %w", body, err)
	}

	var versionID string

	for _, resultVersion := range versions.Results {
		if resultVersion.Version == version {
			versionID = strconv.Itoa(resultVersion.ID)
			break
		}
	}

	if versionID == "" {
		return "", fmt.Errorf("version %s not found", version)
	}

	log.Debug("Version ID: %s", versionID)

	return versionID, nil
}

// UploadSource uploads source code of the extension to the store.
// Source can be uploaded only after the extension is validated.
func (s *Store) UploadSource(appID, versionID, sourcePath string) (result []byte, err error) {
	log.Debug("uploading source for appID: %s, versionID: %s", appID, versionID)

	const apiPath = "api/v5/addons/addon/"

	apiURL := s.URL.JoinPath(apiPath, appID, "versions", versionID, "/").String()

	file, err := os.Open(filepath.Clean(sourcePath))
	if err != nil {
		return nil, fmt.Errorf("opening file %s, error: %w", sourcePath, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("source", file.Name())
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("can't copy file: %s, error: %w", sourcePath, err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("closing writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPatch, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	authHeader, err := s.Client.GenAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("generating header: %w", err)
	}

	req.Header.Add("Authorization", authHeader)

	client := &http.Client{Timeout: requestTimeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d, body: %q", res.StatusCode, body)
	}

	log.Debug("successfully uploaded source")

	return responseBody, nil
}

// UploadStatusFiles represents upload status files structure
type UploadStatusFiles struct {
	DownloadURL string `json:"download_url"`
	Hash        string `json:"hash"`
	Signed      bool   `json:"signed"`
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

// UploadStatus retrieves upload status of the extension.
//
// curl "https://addons.mozilla.org/api/v5/addons/@my-addon/versions/1.0/"
//
//	-g -H "Authorization: JWT <jwt-token>"
func (s *Store) UploadStatus(appID, version string) (status *UploadStatus, err error) {
	log.Debug("getting upload status for appID: %s, version: %s", appID, version)

	const apiPath = "api/v5/addons"
	apiURL := s.URL.JoinPath(apiPath, appID, "versions", version).String()

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	authHeader, err := s.Client.GenAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("generating header: %w", err)
	}

	req.Header.Add("Authorization", authHeader)

	client := &http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	body, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected code %d, body: %q", res.StatusCode, body)
	}

	var uploadStatus UploadStatus

	err = json.Unmarshal(body, &uploadStatus)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response body: %s, due to: %w", body, err)
	}

	log.Debug("received upload status: %+v", uploadStatus)

	return &uploadStatus, nil
}

// AwaitValidation awaits validation of the extension.
func (s *Store) AwaitValidation(appID, version string) (err error) {
	// TODO(maximtop): move constants to config
	const retryInterval = time.Second
	const maxAwaitTime = time.Minute * 20

	startTime := time.Now()

	for {
		if time.Since(startTime) > maxAwaitTime {
			return fmt.Errorf("await validation timeout")
		}

		uploadStatus, err := s.UploadStatus(appID, version)
		if err != nil {
			return fmt.Errorf("getting upload status: %w", err)
		}
		if uploadStatus.Processed {
			log.Debug("extension upload processed successfully")
			break
		} else {
			log.Debug("upload not processed yet, retrying in: %s", retryInterval)
			time.Sleep(retryInterval)
		}
	}

	return nil
}

// UploadNew uploads the extension to the store for the first time
// https://addons-server.readthedocs.io/en/latest/topics/api/signing.html?highlight=%2Faddons%2F#post--api-v5-addons-
// CURL example:
//
//	curl -v -XPOST \
//	 -H "Authorization: JWT ${ACCESS_TOKEN}" \
//	 -F "upload=@tmp/extension.zip" \
//	 "https://addons.mozilla.org/api/v5/addons/"
func (s *Store) UploadNew(filePath string) (result []byte, err error) {
	log.Debug("uploading new extension: %q", filePath)

	const apiPath = "api/v5/addons"

	// trailing slash is required for this request
	// in go 1.19 would be possible u.JoinPath("users", "/")
	apiURL := s.URL.JoinPath(apiPath, "/").String()

	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, fmt.Errorf("can't open file: %q, due to: %w", filePath, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("upload", file.Name())
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("copying: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("[UploadNew] wasn't able to close form file due to: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("[UploadNew] wasn't able to create request due to: %w", err)
	}

	authHeader, err := s.Client.GenAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("[UploadNew] wasn't able to generate auth header due to: %w", err)
	}

	req.Header.Add("Authorization", authHeader)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	client := http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[UploadNew] wasn't able to send request due to: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	respBody, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return nil, fmt.Errorf("[UploadNew] wasn't able to read response body: %s due to: %w", respBody, err)
	}

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("got code %d, body: %q", res.StatusCode, respBody)
	}

	log.Debug("uploaded new extension: %q, response: %s", filePath, respBody)

	return respBody, nil
}

// Insert uploads extension to the amo for the first time.
func (s *Store) Insert(filepath, sourcepath string) (err error) {
	log.Debug("start uploading new extension: %q, with source: %s", filepath, sourcepath)

	_, err = s.UploadNew(filepath)
	if err != nil {
		return fmt.Errorf("[Insert] wasn't able to upload new extension due to: %w", err)
	}

	manifest, err := parseManifest(filepath)
	if err != nil {
		return fmt.Errorf("[Insert] wasn't able to parse manifest: %q due to: %w", filepath, err)
	}

	appID := manifest.Applications.Gecko.ID
	version := manifest.Version

	err = s.AwaitValidation(appID, version)
	if err != nil {
		return fmt.Errorf("[Insert] wasn't able to validate extension: %s, version: %s, due to: %w", appID, version, err)
	}

	versionID, err := s.VersionID(appID, version)
	if err != nil {
		return fmt.Errorf("[Insert] wasn't able to get version ID: %s, version: %s, due to: %w", appID, version, err)
	}

	_, err = s.UploadSource(appID, versionID, sourcepath)
	if err != nil {
		return fmt.Errorf("[Insert] wasn't able to upload source: %s, version: %s, sourcepath: %s, due to: %w", appID, version, sourcepath, err)
	}

	return nil
}

// UploadUpdate uploads the extension update.
func (s *Store) UploadUpdate(appID, version, filePath string) (result []byte, err error) {
	log.Debug("start uploading update for extension: %q", filePath)

	const apiPath = "api/v5/addons"

	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return nil, fmt.Errorf("[UploadUpdate] wasn't able to open file: %q, due to: %w", filePath, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	// trailing slash is required for this request
	apiURL := s.URL.JoinPath(apiPath, appID, "versions", version, "/").String()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("upload", file.Name())
	if err != nil {
		return nil, fmt.Errorf("[UploadUpdate] wasn't able to create form file due to: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("[UploadUpdate] wasn't able to copy file to form file due to: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("[UploadUpdate] wasn't able to close form file due to: %w", err)
	}

	client := http.Client{Timeout: requestTimeout}

	req, err := http.NewRequest(http.MethodPut, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("[UploadUpdate] wasn't able to create request due to: %w", err)
	}

	authHeader, err := s.Client.GenAuthHeader()
	if err != nil {
		return nil, fmt.Errorf("[UploadUpdate] wasn't able to generate auth header due to: %w", err)
	}

	req.Header.Add("Authorization", authHeader)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[UploadUpdate] wasn't able to send request due to: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return nil, fmt.Errorf("[UploadUpdate] wasn't able to read response body: %s due to: %w", responseBody, err)
	}

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("got code %d, body: %q", res.StatusCode, responseBody)
	}

	log.Debug("Successfully uploaded update for extension: %q, response: %s", filePath, responseBody)

	return responseBody, nil
}

// Update uploads new version of extension to the store
// Before uploading it reads manifest.json for getting extension version and uuid.
func (s *Store) Update(filepath, sourcepath string) (err error) {
	log.Debug("start uploading update for extension: %s, with source: %s", filepath, sourcepath)

	manifest, err := parseManifest(filepath)
	if err != nil {
		return fmt.Errorf("[Update] wasn't able to parse manifest: %q due to: %w", filepath, err)
	}

	appID := manifest.Applications.Gecko.ID
	version := manifest.Version

	_, err = s.UploadUpdate(appID, version, filepath)
	if err != nil {
		return fmt.Errorf("[Update] wasn't able to upload update for extension: %s, version: %s, due to: %w", appID, version, err)
	}

	err = s.AwaitValidation(appID, version)
	if err != nil {
		return fmt.Errorf("[Update] wasn't able to validate extension: %s, version: %s, due to: %w", appID, version, err)
	}

	versionID, err := s.VersionID(appID, version)
	if err != nil {
		return fmt.Errorf("[Update] wasn't able to get version ID: %s, version: %s, due to: %w", appID, version, err)
	}

	_, err = s.UploadSource(appID, versionID, sourcepath)
	if err != nil {
		return fmt.Errorf("[Update] wasn't able to upload source: %s, version: %s, sourcepath: %s, due to: %w", appID, version, sourcepath, err)
	}

	return nil
}

// AwaitSigning waits for the extension to be signed.
func (s *Store) AwaitSigning(appID, version string) (err error) {
	log.Debug("start waiting for signing of extension: %s", appID)

	// TODO(maximtop): move constants to config
	const retryInterval = time.Second
	const maxAwaitTime = time.Minute * 20

	startTime := time.Now()

	for {
		if time.Since(startTime) > maxAwaitTime {
			return fmt.Errorf("await signing timeout")
		}

		uploadStatus, err := s.UploadStatus(appID, version)
		if err != nil {
			return fmt.Errorf("[AwaitSigning] wasn't able to get upload status: %s, version: %s, due to: %w", appID, version, err)
		}

		signedAndReady := uploadStatus.Valid && uploadStatus.Active && bool(uploadStatus.Reviewed) && len(uploadStatus.Files) > 0
		requiresManualReview := uploadStatus.Valid && !uploadStatus.AutomatedSigning

		if signedAndReady {
			log.Debug("[AwaitSigning] extension is signed and ready: %s", appID)
			return nil
		} else if requiresManualReview {
			return fmt.Errorf("[AwaitSigning] extension won't be signed automatically, status: %+v", uploadStatus)
		} else {
			log.Debug("[AwaitSigning] extension is not processed yet, retry in %s", retryInterval)
			time.Sleep(retryInterval)
		}
	}
}

// DownloadSigned downloads signed extension.
func (s *Store) DownloadSigned(appID, version string) (err error) {
	log.Debug("start downloading signed extension: %s", appID)

	uploadStatus, err := s.UploadStatus(appID, version)
	if err != nil {
		return fmt.Errorf("[DownloadSigned] wasn't able to get upload status: %s, version: %s, due to: %w", appID, version, err)
	}

	if len(uploadStatus.Files) == 0 {
		return fmt.Errorf("no files to download")
	}

	downloadURL := uploadStatus.Files[0].DownloadURL

	client := http.Client{Timeout: requestTimeout}

	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("[DownloadSigned] wasn't able to create request due to: %w", err)
	}

	authHeader, err := s.Client.GenAuthHeader()
	if err != nil {
		return fmt.Errorf("[DownloadSigned] wasn't able to generate auth header due to: %w", err)
	}

	req.Header.Add("Authorization", authHeader)

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("[DownloadSigned] wasn't able to send request due to: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return fmt.Errorf("[DownloadSigned] wasn't able to read response body: %s due to: %w", responseBody, err)
	}

	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		return fmt.Errorf("[DownloadSigned] wasn't able to parse download URL: %s due to: %w", downloadURL, err)
	}

	filename := path.Base(parsedURL.Path)

	// save response to file
	file, err := os.Create(filepath.Clean(filename))
	if err != nil {
		return fmt.Errorf("[DownloadSigned] wasn't able to create file: %s due to: %w", filename, err)
	}

	_, err = io.Copy(file, bytes.NewReader(responseBody))
	if err != nil {
		return fmt.Errorf("[DownloadSigned] wasn't able to write response body to file: %s due to: %w", filename, err)
	}
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	return nil
}

// Sign uploads the extension to the store, waits for signing, downloads and saves the signed
// extension in the directory
func (s *Store) Sign(filepath string) (err error) {
	log.Debug("start signing extension: %q", filepath)

	manifest, err := parseManifest(filepath)
	if err != nil {
		return fmt.Errorf("[Sign] wasn't able to parse manifest: %q, due to: %w", filepath, err)
	}

	appID := manifest.Applications.Gecko.ID
	version := manifest.Version

	_, err = s.UploadUpdate(appID, version, filepath)
	if err != nil {
		return fmt.Errorf("[Sign] wasn't able to upload extension: %s, version: %s, due to: %w", appID, version, err)
	}

	err = s.AwaitSigning(appID, version)
	if err != nil {
		return fmt.Errorf("[Sign] wasn't able to wait for signing of extension: %s, version: %s, due to: %w", appID, version, err)
	}

	err = s.DownloadSigned(appID, version)
	if err != nil {
		return fmt.Errorf("[Sign] wasn't able to download signed extension: %s, version: %s, due to: %w", appID, version, err)
	}

	return nil
}
