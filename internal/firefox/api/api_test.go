package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/testutil"
	"github.com/adguardteam/go-webext/internal/firefox"
	"github.com/adguardteam/go-webext/internal/firefox/api"
	"github.com/adguardteam/go-webext/internal/urlutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO test maxReadLimit
// TODO test requestTimeout

const (
	appID        = "test_app_id"
	version      = "1.0.0"
	clientID     = "test_client_id"
	clientSecret = "test_client_secret"
	versionID    = "test_version_id"
	testTime     = 1234567890
	testContent  = "test content"
)

func TestStatus(t *testing.T) {
	expectedStatus := &firefox.StatusResponse{
		ID:             appID,
		CurrentVersion: version,
		Status:         "incomplete",
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}

		assert.Equal(t, http.MethodGet, r.Method)

		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePath, "addon", appID))

		// assert that has auth header
		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		w.WriteHeader(http.StatusOK)

		expectedResponse, err := json.Marshal(api.AmoStatusResponse{
			GUID:           expectedStatus.ID,
			Status:         expectedStatus.Status,
			CurrentVersion: expectedStatus.CurrentVersion,
		})
		require.NoError(pt, err)

		_, err = w.Write(expectedResponse)
		require.NoError(pt, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxApi := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL: storeURL,
	})

	response, err := firefoxApi.Status(appID)
	require.NoError(t, err)

	assert.Equal(t, expectedStatus, response)
}

// TODO (maximtop): test errors if status is wrong or on invalid json
func TestUploadStatus(t *testing.T) {
	expectedUploadStatus := firefox.UploadStatus{
		GUID:             "",
		Active:           false,
		AutomatedSigning: false,
		Files:            nil,
		PassedReview:     false,
		Pk:               "",
		Processed:        false,
		Reviewed:         false,
		URL:              "",
		Valid:            false,
		ValidationURL:    "",
		Version:          "",
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodGet, r.Method)

		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePath, appID, "versions", version))

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		w.WriteHeader(http.StatusOK)

		expectedResponse, err := json.Marshal(expectedUploadStatus)
		require.NoError(pt, err)

		_, err = w.Write(expectedResponse)
		require.NoError(pt, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxApi := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now:          func() int64 { return testTime },
		URL:          storeURL,
	})

	response, err := firefoxApi.UploadStatus(appID, version)
	require.NoError(t, err)

	assert.Equal(t, expectedUploadStatus, *response)
}

func TestUploadSource(t *testing.T) {
	expectedResponse, err := json.Marshal("test")
	require.NoError(t, err)

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePath, "addon", appID, "versions", versionID, "/"))
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)
		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		file, header, err := r.FormFile("source")
		require.NoError(pt, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()
		assert.Equal(t, "source.zip", header.Filename)

		fileContent, err := io.ReadAll(file)
		require.NoError(pt, err)
		assert.Equal(t, testContent, string(fileContent))

		_, err = w.Write(expectedResponse)
		require.NoError(pt, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxApi := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now:          func() int64 { return testTime },
		URL:          storeURL,
	})

	fileData := strings.NewReader(testContent)

	err = firefoxApi.UploadSource(appID, versionID, fileData)
	require.NoError(t, err)
}

func TestVersionID(t *testing.T) {
	const versionID = 123456
	expectedVersionResponse := api.VersionsResponse{
		PageSize:  0,
		PageCount: 0,
		Count:     0,
		Next:      nil,
		Previous:  nil,
		Results: []api.ResultVersion{
			{
				ID:      versionID,
				Version: version,
			},
		},
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodGet, r.Method)

		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePath, "addon", appID, "versions"))
		query, err := url.ParseQuery(r.URL.RawQuery)
		require.NoError(pt, err)

		assert.Equal(t, query.Get("filter"), "all_with_unlisted")

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		w.WriteHeader(http.StatusOK)

		expectedResponse, err := json.Marshal(expectedVersionResponse)
		require.NoError(pt, err)

		_, err = w.Write(expectedResponse)
		require.NoError(pt, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxApi := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now:          func() int64 { return testTime },
		URL:          storeURL,
	})

	response, err := firefoxApi.VersionID(appID, version)
	require.NoError(t, err)

	assert.Equal(t, strconv.Itoa(versionID), response)
}

// TODO (maximtop): add test to check that 201 is also accepted
func TestUploadNew(t *testing.T) {
	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodPost, r.Method)

		// trailing slash is required for this request
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePath, "/"))

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		// assert that has file in request body
		file, header, err := r.FormFile("upload")
		require.NoError(pt, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()
		assert.Equal(t, api.DefaultExtensionFilename, header.Filename)

		// assert the content of the received file
		receivedContent, err := io.ReadAll(file)
		require.NoError(pt, err)
		assert.Equal(t, testContent, string(receivedContent))

		w.WriteHeader(http.StatusAccepted)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxApi := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL: storeURL,
	})

	fileData := strings.NewReader(testContent)

	err = firefoxApi.UploadNew(fileData)
	require.NoError(t, err)
}

func TestUploadUpdate(t *testing.T) {
	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodPut, r.Method)

		// A trailing slash is required for this request.
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePath, appID, "versions", version, "/"))

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		// assert that has file in request body
		file, header, err := r.FormFile("upload")
		require.NoError(pt, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()
		assert.Equal(t, api.DefaultExtensionFilename, header.Filename)

		// assert the content of the received file
		receivedContent, err := io.ReadAll(file)
		require.NoError(pt, err)
		assert.Equal(t, testContent, string(receivedContent))

		w.WriteHeader(http.StatusAccepted)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxApi := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL: storeURL,
	})

	fileData := strings.NewReader(testContent)

	err = firefoxApi.UploadUpdate(appID, version, fileData)
	require.NoError(t, err)
}

func TestDownloadSignedByURL(t *testing.T) {
	expectedResponse := []byte("test")
	expectedURLPath := "/addons/signed.xpi"

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodGet, r.Method)

		assert.Equal(t, r.URL.Path, expectedURLPath)

		// assert that has auth header
		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		w.WriteHeader(http.StatusOK)

		_, err = w.Write(expectedResponse)
		require.NoError(pt, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxApi := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now:          func() int64 { return testTime },
		URL:          storeURL,
	})

	response, err := firefoxApi.DownloadSignedByURL(storeURL.JoinPath(expectedURLPath).String())
	require.NoError(t, err)

	assert.Equal(t, expectedResponse, response)
}
