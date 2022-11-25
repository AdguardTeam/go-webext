package firefox_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/adguardteam/go-webext/internal/firefox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	clientID     = "test_client_id"
	clientSecret = "test_client_secret"
	appID        = "test_app_id"
	version      = "0.0.3"
	status       = "test_status"
	response     = "test_response"
)

func TestStatus(t *testing.T) {
	now := func() int64 {
		return 1
	}

	client := firefox.NewClient(firefox.ClientConfig{ClientID: clientID, ClientSecret: clientSecret, Now: now})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, http.MethodGet)
		assert.Contains(t, r.URL.Path, appID)
		authHeader, err := client.GenAuthHeader()
		require.NoError(t, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		_, err = w.Write([]byte(status))
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := firefox.Store{
		Client: &client,
		URL:    storeURL,
	}

	actualStatus, err := store.Status(appID)

	require.NoError(t, err)

	assert.Equal(t, status, string(actualStatus))
}

func TestUploadNew(t *testing.T) {
	currentTimeSec := time.Now().Unix()
	now := func() int64 {
		return currentTimeSec
	}

	client := firefox.NewClient(firefox.ClientConfig{ClientID: clientID, ClientSecret: clientSecret, Now: now})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, http.MethodPost)
		authHeader, err := client.GenAuthHeader()
		require.NoError(t, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)
		assert.Contains(t, r.URL.Path, "/api/v5/addons")
		file, _, err := r.FormFile("upload")
		require.NoError(t, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()

		body, err := io.ReadAll(file)
		require.NoError(t, err)

		assert.Contains(t, string(body), "test content")

		w.WriteHeader(http.StatusCreated)
		_, err = w.Write([]byte(status))
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := firefox.Store{
		Client: &client,
		URL:    storeURL,
	}

	result, err := store.UploadNew("testdata/test.txt")
	require.NoError(t, err)

	assert.Equal(t, status, string(result))
}

func TestUploadUpdate(t *testing.T) {
	currentTimeSec := time.Now().Unix()
	now := func() int64 {
		return currentTimeSec
	}

	client := firefox.NewClient(firefox.ClientConfig{ClientID: clientID, ClientSecret: clientSecret, Now: now})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.URL.Path, path.Join("api/v5/addons", appID, "versions", version))
		authHeader, err := client.GenAuthHeader()
		require.NoError(t, err)

		assert.Equal(t, r.Header.Get("Authorization"), authHeader)
		file, header, err := r.FormFile("upload")
		require.NoError(t, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()

		assert.Equal(t, header.Filename, "extension.zip")

		w.WriteHeader(http.StatusCreated)
		_, err = w.Write([]byte(response))
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := firefox.Store{
		Client: &client,
		URL:    storeURL,
	}

	actualResponse, err := store.UploadUpdate(appID, version, "testdata/extension.zip")
	require.NoError(t, err)

	assert.Equal(t, response, string(actualResponse))
}

func TestUploadSource(t *testing.T) {
	testFile := "testdata/source.zip"
	versionID := "test_version_id"

	client := firefox.NewClient(firefox.ClientConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return 1
		},
	})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Contains(t, r.URL.Path, "/api/v5/addons/addon/"+appID+"/versions/"+versionID+"/")
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		file, header, err := r.FormFile("source")
		require.NoError(t, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()

		assert.Equal(t, header.Filename, "source.zip")
		_, err = w.Write([]byte(response))
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := firefox.Store{
		Client: &client,
		URL:    storeURL,
	}

	uploadResponse, err := store.UploadSource(appID, versionID, testFile)
	require.NoError(t, err)

	assert.Equal(t, response, string(uploadResponse))
}

func TestVersionID(t *testing.T) {
	expectedVersionID := 100

	client := firefox.NewClient(firefox.ClientConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return time.Now().Unix()
		},
	})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)

		assert.Contains(t, r.URL.Path, "/api/v5/addons/addon/"+appID+"/versions")

		query, err := url.ParseQuery(r.URL.RawQuery)
		require.NoError(t, err)
		assert.Equal(t, query.Get("filter"), "all_with_unlisted")

		authHeader, err := client.GenAuthHeader()
		require.NoError(t, err)
		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		w.WriteHeader(http.StatusOK)

		resultVersion := firefox.ResultVersion{
			ID:      expectedVersionID,
			Version: version,
		}
		var results []firefox.ResultVersion
		results = append(results, resultVersion)

		versionResponse := firefox.VersionResponse{
			PageSize:  0,
			PageCount: 0,
			Count:     0,
			Next:      nil,
			Previous:  nil,
			Results:   results,
		}

		versionResponseBytes, err := json.Marshal(versionResponse)
		require.NoError(t, err)
		_, err = w.Write(versionResponseBytes)
		require.NoError(t, err)
	}))
	defer storeServer.Close()
	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := firefox.Store{
		Client: &client,
		URL:    storeURL,
	}

	actualVersionID, err := store.VersionID(appID, version)
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(expectedVersionID), actualVersionID)
}

func TestUploadStatus(t *testing.T) {
	expectedStatus := firefox.UploadStatus{
		GUID:             "test",
		Active:           false,
		AutomatedSigning: false,
		Files:            nil,
		PassedReview:     true,
		Pk:               "",
		Processed:        true,
		Reviewed:         true,
		URL:              "",
		Valid:            false,
		ValidationURL:    "",
		Version:          "",
	}

	client := firefox.NewClient(firefox.ClientConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Now: func() int64 {
			return time.Now().Unix()
		},
	})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)

		assert.Contains(t, r.URL.Path, "/api/v5/addons/"+appID+"/versions/"+version)

		authHeader, err := client.GenAuthHeader()
		require.NoError(t, err)
		assert.Equal(t, r.Header.Get("Authorization"), authHeader)

		w.WriteHeader(http.StatusOK)

		response, err := json.Marshal(expectedStatus)
		_, err = w.Write(response)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := firefox.Store{
		Client: &client,
		URL:    storeURL,
	}

	actualStatus, err := store.UploadStatus(appID, version)
	require.NoError(t, err)

	assert.Equal(t, expectedStatus, *actualStatus)
}
