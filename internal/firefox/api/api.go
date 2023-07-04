// Package api provides an API for making requests to the [Addons.Mozilla.Org]
// (aka AMO) store.
//
// Please use an [AMO dev environment] or the [AMO stage environment] for
// testing.  Credential keys cannot be sent via email, so you will need to
// request them in the [chat].  Last time I asked for them from the guy with the
// nick [mat].
//
// The signed xpi build from the development environment is corrupted, so you
// need to build it from the production environment.
//
// [AMO dev environment]: https://addons-dev.allizom.org/
// [AMO stage environment]: https://addons.allizom.org/
// [Addons.Mozilla.Org]: https://addons.mozilla.org/
// [chat]: https://matrix.to/#/#amo:mozilla.org
// [mat]: https://matrix.to/#/@mat:mozilla.org
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/httphdr"
	"github.com/AdguardTeam/golibs/log"
	"github.com/adguardteam/go-webext/internal/fileutil"
	"github.com/adguardteam/go-webext/internal/firefox"
	"github.com/golang-jwt/jwt/v4"
)

// TODO (maximtop): consider to make this constant an option.
const requestTimeout = 20 * time.Minute

// maxReadLimit limits response size returned from the store api.
const maxReadLimit = 10 * fileutil.MB

// DefaultExtensionFilename is the default filename for the extension zip file to be
// uploaded.
const DefaultExtensionFilename = "extension.zip"

// DefaultSourceFilename is the default filename for the source zip file to be
// uploaded.
const DefaultSourceFilename = "source.zip"

// AddonsBasePathV5 is a base path for addons api v5.
const AddonsBasePathV5 = "api/v5/addons"

// API represents an instance of a remote API that the client can interact with.
type API struct {
	ClientID     string       // ClientID is the ID used for authentication.
	ClientSecret string       // ClientSecret is the secret used for authentication.
	now          func() int64 // Now is a function that returns the current Unix time in seconds.
	URL          *url.URL     // URL is the base URL for the remote API.
}

// JoinPath joins the provided path parts with the base URL of the API.
func (a *API) JoinPath(pathParts ...string) string {
	paths := append([]string{AddonsBasePathV5}, pathParts...)
	return a.URL.JoinPath(paths...).String()
}

// Config represents configuration options for creating a new API instance.
type Config struct {
	ClientID     string       // ClientID is the ID used for authentication.
	ClientSecret string       // ClientSecret is the secret used for authentication.
	Now          func() int64 // Now is a function that returns the current Unix time in seconds.
	URL          *url.URL     // URL is the base URL for the remote API.
}

// VersionCreateRequest describes version json structure for request to the store api.
type VersionCreateRequest struct {
	Upload string `json:"upload"`
}

// AddonCreateRequest describes addon json structure to the store api.
type AddonCreateRequest struct {
	Version VersionCreateRequest `json:"version"`
}

// NewAPI creates a new instance of the API with the provided configuration
// options.  If the Now function is not provided, it defaults to
// time.Now().Unix().
func NewAPI(config Config) *API {
	c := config

	if c.Now == nil {
		c.Now = func() int64 {
			return time.Now().Unix()
		}
	}

	return &API{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		now:          c.Now,
		URL:          c.URL,
	}
}

// AuthHeader generates an authorization header that can be used in API
// requests.  The header contains a JWT token signed with the client's secret.
func AuthHeader(clientID, clientSecret string, currentTimeSec int64) (result string, err error) {
	const expirationSec = 5 * 60

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss": clientID,
		"iat": currentTimeSec,
		"exp": currentTimeSec + expirationSec,
	})

	signedToken, err := token.SignedString([]byte(clientSecret))
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return "JWT " + signedToken, nil
}

// prepareRequest creates a new HTTP request object.  The function adds an
// authorization header using the client's credentials.
func (a *API) prepareRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	authHeader, err := AuthHeader(a.ClientID, a.ClientSecret, a.now())
	if err != nil {
		return nil, fmt.Errorf("generating auth header: %w", err)
	}

	req.Header.Add(httphdr.Authorization, authHeader)

	return req, nil
}

// readBody reads the response body up to a specified limit (maxReadLimit) and
// verifies if the response status code is one of the allowed status codes.
func readBody(res *http.Response, allowedStatusCodes []int) (body []byte, err error) {
	body, err = io.ReadAll(io.LimitReader(res.Body, maxReadLimit))
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	isStatusCodeAllowed := false
	for _, status := range allowedStatusCodes {
		if res.StatusCode == status {
			isStatusCodeAllowed = true
			break
		}
	}

	codes := make([]string, len(allowedStatusCodes))
	for i, code := range allowedStatusCodes {
		codes[i] = strconv.Itoa(code)
	}

	if !isStatusCodeAllowed {
		return nil, fmt.Errorf(
			"expected codes: %s, but got: %d, body: %s",
			strings.Join(codes, ", "),
			res.StatusCode,
			body)
	}

	return body, nil
}

// Status returns status of the extension by appID.
func (a *API) Status(appID string) (response *firefox.StatusResponse, err error) {
	apiURL := a.JoinPath("addon", appID)

	req, err := a.prepareRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	client := &http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := readBody(res, []int{http.StatusOK})
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var addonDetail firefox.AddonInfo
	err = json.Unmarshal(responseBody, &addonDetail)
	if err != nil {
		return nil, fmt.Errorf("decoding response body: %s, error: %w", responseBody, err)
	}

	var currentVersion string
	if addonDetail.Version != nil {
		currentVersion = addonDetail.Version.Version
	} else if addonDetail.CurrentVersion != nil {
		currentVersion = addonDetail.CurrentVersion.Version
	} else if addonDetail.LatestUnlistedVersion != nil {
		currentVersion = addonDetail.LatestUnlistedVersion.Version
	} else {
		return nil, fmt.Errorf("addon doesn't have any versions")
	}

	response = &firefox.StatusResponse{
		ID:             addonDetail.GUID,
		Status:         addonDetail.Status,
		CurrentVersion: currentVersion,
	}

	return response, nil
}

// CreateUpload creates new upload for the extension. Upload is a file with extension uploaded to amo servers.
// After it is uploaded, it can be used to create new version of the extension.
// https://addons-server.readthedocs.io/en/latest/topics/api/addons.html#upload-create
func (a *API) CreateUpload(fileData io.Reader, channel firefox.Channel) (result *firefox.UploadDetail, err error) {
	log.Debug("creating upload (uploading extension file for further processing)")

	// trailing slash is required
	apiURL := a.JoinPath("upload", "/")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	err = writer.WriteField("channel", string(channel))
	if err != nil {
		return nil, fmt.Errorf("writing field: %w", err)
	}

	part, err := writer.CreateFormFile("upload", DefaultExtensionFilename)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}

	_, err = io.Copy(part, fileData)
	if err != nil {
		return nil, fmt.Errorf("copying file error: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("closing writer: %w", err)
	}

	req, err := a.prepareRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	req.Header.Set(httphdr.ContentType, writer.FormDataContentType())
	client := &http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := readBody(res, []int{http.StatusCreated, http.StatusAccepted})
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response body: %s, error: %w", responseBody, err)
	}

	log.Debug("upload created successfully")

	return result, nil
}

// UploadDetail retrieves upload status for the upload by id.
func (a *API) UploadDetail(uuid string) (response *firefox.UploadDetail, err error) {
	apiURL := a.JoinPath("upload", uuid)

	req, err := a.prepareRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	client := &http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	body, err := readBody(res, []int{http.StatusOK})
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response body: %s, error: %w", body, err)
	}

	return response, nil
}

// CreateAddon creates new addon in the store.
func (a *API) CreateAddon(UUID string) (addonInfo *firefox.AddonInfo, err error) {
	apiURL := a.JoinPath("addon", "/")

	addonCreateRequest := AddonCreateRequest{
		Version: VersionCreateRequest{
			Upload: UUID,
		},
	}

	jsonBody, err := json.Marshal(addonCreateRequest)
	if err != nil {
		return nil, fmt.Errorf("marshalling request body: %w", err)
	}

	req, err := a.prepareRequest(http.MethodPost, apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	req.Header.Set(httphdr.ContentType, "application/json")

	client := &http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	body, err := readBody(res, []int{http.StatusCreated})
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	err = json.Unmarshal(body, &addonInfo)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response body: %s, error: %w", body, err)
	}

	return addonInfo, nil
}

// CreateVersion creates new version for the extension with sourceData
// https://addons-server.readthedocs.io/en/latest/topics/api/addons.html#version-create
func (a *API) CreateVersion(appID, UUID string) (versionInfo *firefox.VersionInfo, err error) {
	apiURL := a.JoinPath("addon", appID, "versions", "/")

	versionCreateRequest := VersionCreateRequest{
		Upload: UUID,
	}

	jsonBody, err := json.Marshal(versionCreateRequest)
	if err != nil {
		return nil, fmt.Errorf("marshalling request body: %w", err)
	}

	req, err := a.prepareRequest(http.MethodPost, apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	req.Header.Set(httphdr.ContentType, "application/json")

	client := &http.Client{Timeout: requestTimeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	body, err := readBody(res, []int{http.StatusCreated})
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	err = json.Unmarshal(body, &versionInfo)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response body: %s, error: %w", body, err)
	}

	return versionInfo, nil
}

// VersionDetail returns current version details of the extension.
func (a *API) VersionDetail(appID, versionID string) (versionInfo *firefox.VersionInfo, err error) {
	log.Debug("api: VersionDetail: Getting version details appID: %s, versionID: %s", appID, versionID)

	apiURL := a.JoinPath("addon", appID, "versions", versionID, "/")

	req, err := a.prepareRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	req.Header.Set(httphdr.ContentType, "application/json")
	client := &http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	body, err := readBody(res, []int{http.StatusOK})
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	err = json.Unmarshal(body, &versionInfo)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response body: %s, error: %w", body, err)
	}

	log.Debug("api: VersionDetail: version details successfully retrieved: %s", body)
	return versionInfo, nil
}

// AttachSourceToVersion uploads source code to the specified version.
// https://addons-server.readthedocs.io/en/latest/topics/api/addons.html#version-sources
func (a *API) AttachSourceToVersion(appID, versionID string, sourceData io.Reader) (err error) {
	log.Debug("attaching source to appID: %s and versionID: %s", appID, versionID)

	apiURL := a.JoinPath("addon", appID, "versions", versionID, "/")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("source", DefaultSourceFilename)
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}

	_, err = io.Copy(part, sourceData)
	if err != nil {
		return fmt.Errorf("copying file error: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("closing writer: %w", err)
	}

	req, err := a.prepareRequest(http.MethodPatch, apiURL, body)
	if err != nil {
		return fmt.Errorf("preparing request: %w", err)
	}

	req.Header.Set(httphdr.ContentType, writer.FormDataContentType())
	client := &http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	resBody, err := readBody(res, []int{http.StatusOK})
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	log.Debug("successfully attached, response: %s", resBody)

	return nil
}

// DownloadSignedByURL downloads extension by url.
func (a *API) DownloadSignedByURL(url string) (response []byte, err error) {
	client := http.Client{Timeout: requestTimeout}

	req, err := a.prepareRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("preparing request: %w", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := readBody(res, []int{http.StatusOK})

	return responseBody, nil
}
