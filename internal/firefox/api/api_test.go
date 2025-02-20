package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/logutil/slogutil"
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
	versionID    = "12345"
	testTime     = 1234567890
	testContent  = "test content"
	testUUID     = "test_uuid"
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

		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePathV5, "addon", appID))

		// assert that has auth header
		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		w.WriteHeader(http.StatusOK)

		expectedResponse, err := json.Marshal(firefox.AddonInfo{
			GUID:   expectedStatus.ID,
			Status: expectedStatus.Status,
			CurrentVersion: &firefox.VersionInfo{
				Version: version,
			},
		})
		require.NoError(pt, err)

		_, err = w.Write(expectedResponse)
		require.NoError(pt, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxAPI := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	response, err := firefoxAPI.Status(appID)
	require.NoError(t, err)

	assert.Equal(t, expectedStatus, response)
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

	firefoxAPI := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now:          func() int64 { return testTime },
		URL:          storeURL,
		Logger:       slogutil.NewDiscardLogger(),
	})

	response, err := firefoxAPI.DownloadSignedByURL(storeURL.JoinPath(expectedURLPath).String())
	require.NoError(t, err)

	assert.Equal(t, expectedResponse, response)
}

func TestCreateUpload(t *testing.T) {
	expectedUploadResponse := &firefox.UploadDetail{
		UUID:       "",
		Channel:    "listed",
		Processed:  true,
		Submitted:  false,
		URL:        "",
		Valid:      true,
		Validation: nil,
		Version:    "",
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePathV5, "upload", "/"))
		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		// assert that has field channel in request
		assert.Equal(t, r.FormValue("channel"), "listed")

		// assert that has file in request body
		file, header, err := r.FormFile("upload")
		require.NoError(pt, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()
		assert.Equal(t, api.DefaultExtensionFilename, header.Filename)

		// assert the content of the received file
		receivedContent, err := io.ReadAll(file)
		require.NoError(pt, err)
		assert.Equal(t, testContent, string(receivedContent))

		w.WriteHeader(http.StatusCreated)

		expectedResponse, err := json.Marshal(expectedUploadResponse)
		require.NoError(pt, err)

		_, err = w.Write(expectedResponse)
		require.NoError(pt, err)
	}))
	defer storeServer.Close()
	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxAPI := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	fileData := strings.NewReader(testContent)

	res, err := firefoxAPI.CreateUpload(fileData, "listed")
	require.NoError(t, err)

	assert.Equal(t, res, expectedUploadResponse)
}

func TestUploadDetail(t *testing.T) {
	expectedUUID := testUUID
	expectedUploadDetail := &firefox.UploadDetail{
		UUID:       expectedUUID,
		Channel:    "listed",
		Processed:  true,
		Submitted:  false,
		URL:        "",
		Valid:      true,
		Validation: nil,
		Version:    "",
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}

		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePathV5, "upload", expectedUUID))

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)
		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		expectedResponse, err := json.Marshal(expectedUploadDetail)
		require.NoError(pt, err)

		_, err = w.Write(expectedResponse)
		require.NoError(pt, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	firefoxAPI := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	res, err := firefoxAPI.UploadDetail(expectedUUID)
	require.NoError(t, err)

	assert.Equal(t, res, expectedUploadDetail)
}

func TestCreateAddon(t *testing.T) {
	expectedAddonInfo := &firefox.AddonInfo{
		Version: &firefox.VersionInfo{
			ID: 12345,
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}

		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePathV5, "addon", "/"))

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)
		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		// read body
		body, err := io.ReadAll(r.Body)
		require.NoError(pt, err)

		var actualAddonCreateRequest api.AddonCreateRequest
		err = json.Unmarshal(body, &actualAddonCreateRequest)
		require.NoError(pt, err)

		assert.Equal(t, actualAddonCreateRequest.Version.Upload, testUUID)

		expectedResponse, err := json.Marshal(expectedAddonInfo)
		require.NoError(pt, err)

		w.WriteHeader(http.StatusCreated)
		_, err = w.Write(expectedResponse)
		require.NoError(pt, err)
	}))
	defer server.Close()

	storeURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	firefoxAPI := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	res, err := firefoxAPI.CreateAddon(testUUID)
	require.NoError(t, err)

	assert.Equal(t, res, expectedAddonInfo)
}

func TestAttachSourceToVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePathV5, "addon", appID, "versions", testUUID, "/"))

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)
		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		// assert that has file in request body
		file, header, err := r.FormFile("source")
		require.NoError(pt, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()
		assert.Equal(t, api.DefaultSourceFilename, header.Filename)

		// assert the content of the received file
		receivedContent, err := io.ReadAll(file)
		require.NoError(pt, err)
		assert.Equal(t, testContent, string(receivedContent))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	storeURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	firefoxAPI := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	fileData := strings.NewReader(testContent)
	err = firefoxAPI.AttachSourceToVersion(appID, testUUID, fileData)
	require.NoError(t, err)
}

func TestCreateVersion(t *testing.T) {
	expectedVersionInfo := &firefox.VersionInfo{
		ID: 12345,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePathV5, "addon", appID, "versions", "/"))

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)
		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		body, err := io.ReadAll(r.Body)
		require.NoError(pt, err)

		var actualVersionCreateRequest api.VersionCreateRequest
		err = json.Unmarshal(body, &actualVersionCreateRequest)
		require.NoError(pt, err)

		assert.Equal(t, actualVersionCreateRequest.Upload, testUUID)

		response, err := json.Marshal(expectedVersionInfo)
		require.NoError(pt, err)

		w.WriteHeader(http.StatusCreated)
		_, err = w.Write(response)
		require.NoError(pt, err)
	}))

	defer server.Close()

	storeURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	firefoxAPI := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	versionInfo, err := firefoxAPI.CreateVersion(appID, testUUID)
	require.NoError(t, err)

	assert.Equal(t, versionInfo, expectedVersionInfo)
}

func TestVersionDetail(t *testing.T) {
	expectedVersionInfo := &firefox.VersionInfo{
		ID: 12345,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePathV5, "addon", appID, "versions", versionID, "/"))

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)
		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		response, err := json.Marshal(expectedVersionInfo)
		require.NoError(pt, err)

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(response)
		require.NoError(pt, err)
	}))

	defer server.Close()

	storeURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	firefoxAPI := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	versionInfo, err := firefoxAPI.VersionDetail(appID, versionID)
	require.NoError(t, err)

	assert.Equal(t, versionInfo, expectedVersionInfo)
}

func TestVersionsList(t *testing.T) {
	expectedVersionInfo := &firefox.VersionInfo{
		ID: 12345,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pt := testutil.PanicT{}
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, r.URL.Path, urlutil.JoinPath(api.AddonsBasePathV5, "addon", appID, "versions", "/"))

		authHeader, err := api.AuthHeader(clientID, clientSecret, testTime)
		require.NoError(pt, err)
		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		response, err := json.Marshal(firefox.VersionsListResponse{
			Count: 1,
			Results: []firefox.VersionInfo{
				*expectedVersionInfo,
			},
		})
		require.NoError(pt, err)

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(response)
		require.NoError(pt, err)
	}))

	defer server.Close()

	storeURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	firefoxAPI := api.NewAPI(api.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return testTime
		},
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	versionsList, err := firefoxAPI.VersionsList(appID)
	require.NoError(t, err)

	assert.Equal(t, []*firefox.VersionInfo{expectedVersionInfo}, versionsList)
}
