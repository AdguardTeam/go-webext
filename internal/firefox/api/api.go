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

// API represents an instance of a remote API that the client can interact with.
type API struct {
	ClientID     string       // ClientID is the ID used for authentication.
	ClientSecret string       // ClientSecret is the secret used for authentication.
	now          func() int64 // Now is a function that returns the current Unix time in seconds.
	URL          *url.URL     // URL is the base URL for the remote API.
}

// Config represents configuration options for creating a new API instance.
type Config struct {
	ClientID     string       // ClientID is the ID used for authentication.
	ClientSecret string       // ClientSecret is the secret used for authentication.
	Now          func() int64 // Now is a function that returns the current Unix time in seconds.
	URL          *url.URL     // URL is the base URL for the remote API.
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

// AddonsBasePath is a base path for addons api.
const AddonsBasePath = "api/v5/addons"

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

	req.Header.Add("Authorization", authHeader)

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

// AmoStatusResponse represents a response from the status request.
type AmoStatusResponse struct {
	GUID           string `json:"guid"`
	Status         string `json:"status"`
	CurrentVersion string `json:"current_version"`
}

// Status returns status of the extension by appID.
func (a *API) Status(appID string) (response *firefox.StatusResponse, err error) {
	apiURL := a.URL.JoinPath(AddonsBasePath, "addon", appID).String()

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

	var amoResponse AmoStatusResponse
	err = json.Unmarshal(responseBody, &amoResponse)
	if err != nil {
		return nil, fmt.Errorf("decoding response body: %s, error: %w", responseBody, err)
	}

	response = &firefox.StatusResponse{
		ID:             amoResponse.GUID,
		Status:         amoResponse.Status,
		CurrentVersion: amoResponse.CurrentVersion,
	}

	return response, nil
}

// UploadStatus retrieves upload status of the extension.
//
// CURL example:
//
//	curl "https://addons.mozilla.org/api/v5/addons/@my-addon/versions/1.0/" \
//		-g -H "Authorization: JWT <jwt-token>"
func (a *API) UploadStatus(appID, version string) (response *firefox.UploadStatus, err error) {
	apiURL := a.URL.JoinPath(AddonsBasePath, appID, "versions", version).String()

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

	var uploadStatus firefox.UploadStatus

	err = json.Unmarshal(body, &uploadStatus)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response body: %s, due to: %w", body, err)
	}

	log.Debug("received upload status: %+v", uploadStatus)

	return &uploadStatus, nil
}

// UploadSource uploads source code of the extension to the store.
// Source can be uploaded only after the extension is validated.
func (a *API) UploadSource(appID, versionID string, fileData io.Reader) (err error) {
	apiURL := a.URL.JoinPath(AddonsBasePath, "addon", appID, "versions", versionID, "/").String()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("source", DefaultSourceFilename)
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}

	_, err = io.Copy(part, fileData)
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

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: requestTimeout}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	// ignore response body as it is not required
	_, err = readBody(res, []int{http.StatusOK})
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	return nil
}

// ResultVersion describes response version structure.
type ResultVersion struct {
	ID      int    `json:"id"`
	Version string `json:"version"`
}

// VersionsResponse describes response from the store api.
type VersionsResponse struct {
	PageSize  int             `json:"page_size"`
	PageCount int             `json:"page_count"`
	Count     int             `json:"count"`
	Next      any             `json:"next"`
	Previous  any             `json:"previous"`
	Results   []ResultVersion `json:"results"`
}

// VersionID retrieves apps versions by appID.
func (a *API) VersionID(appID, version string) (response string, err error) {
	queryString := url.Values{}
	queryString.Add("filter", "all_with_unlisted")
	apiURL := a.URL.JoinPath(AddonsBasePath, "addon", appID, "versions").String() + "?" + queryString.Encode()

	req, err := a.prepareRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("preparing request: %w", err)
	}

	client := &http.Client{Timeout: requestTimeout}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	responseBody, err := readBody(res, []int{http.StatusOK})
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	var versionResponse VersionsResponse

	err = json.Unmarshal(responseBody, &versionResponse)
	if err != nil {
		return "", fmt.Errorf("unmarshalling response body: %s, error: %w", responseBody, err)
	}

	var versionID string

	for _, resultVersion := range versionResponse.Results {
		if resultVersion.Version == version {
			versionID = strconv.Itoa(resultVersion.ID)
			break
		}
	}

	if versionID == "" {
		return "", fmt.Errorf("version %s not found", version)
	}

	return versionID, nil
}

// UploadNew uploads extension file to the store for the first time.
//
// CURL example:
//
//	curl -v -XPOST \
//	 -H "Authorization: JWT ${ACCESS_TOKEN}" \
//	 -F "upload=@tmp/extension.zip" \
//	 "https://addons.mozilla.org/api/v5/addons/"
//
// More details find in [docs].
//
// [docs]: https://addons-server.readthedocs.io/en/latest/topics/api/signing.html?highlight=%2Faddons%2F#post--api-v5-addons-
func (a *API) UploadNew(fileData io.Reader) (err error) {
	// trailing slash is required for this request
	apiURL := a.URL.JoinPath(AddonsBasePath, "/").String()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("upload", DefaultExtensionFilename)
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}

	_, err = io.Copy(part, fileData)
	if err != nil {
		return fmt.Errorf("copying: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("closing writer: %w", err)
	}

	req, err := a.prepareRequest(http.MethodPost, apiURL, body)
	if err != nil {
		return fmt.Errorf("preparing request: %w", err)
	}

	req.Header.Add("Content-Type", writer.FormDataContentType())

	client := http.Client{Timeout: requestTimeout}

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	// ignore response body, as it is not required
	_, err = readBody(res, []int{http.StatusCreated, http.StatusAccepted})
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	return nil
}

// UploadUpdate uploads the extension update.
func (a *API) UploadUpdate(appID, version string, fileData io.Reader) (err error) {
	// A trailing slash is required for this request.
	apiURL := a.URL.JoinPath(AddonsBasePath, appID, "versions", version, "/").String()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("upload", DefaultExtensionFilename)
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}

	_, err = io.Copy(part, fileData)
	if err != nil {
		return fmt.Errorf("copying file to the form file: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return fmt.Errorf("closing writer descriptor: %w", err)
	}

	client := http.Client{Timeout: requestTimeout}

	req, err := a.prepareRequest(http.MethodPut, apiURL, body)
	if err != nil {
		return fmt.Errorf("preparing request: %w", err)
	}

	req.Header.Add("Content-Type", writer.FormDataContentType())

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func() { err = errors.WithDeferred(err, res.Body.Close()) }()

	// ignore response body, as it is not required
	_, err = readBody(res, []int{http.StatusCreated, http.StatusAccepted})
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

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
