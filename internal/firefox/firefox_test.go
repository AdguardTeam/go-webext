package firefox_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
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
	assert := assert.New(t)

	now := func() int64 {
		return 1
	}

	client := firefox.NewClient(firefox.ClientConfig{ClientID: clientID, ClientSecret: clientSecret, Now: now})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(r.Method, http.MethodGet)
		assert.Contains(r.URL.Path, appID)
		authHeader, err := client.GenAuthHeader()
		require.NoError(t, err)

		assert.Equal(r.Header.Get("Authorization"), authHeader)

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

	assert.Equal(status, string(actualStatus))
}

func TestUploadNew(t *testing.T) {
	assert := assert.New(t)

	currentTimeSec := time.Now().Unix()
	now := func() int64 {
		return currentTimeSec
	}

	client := firefox.NewClient(firefox.ClientConfig{ClientID: clientID, ClientSecret: clientSecret, Now: now})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(r.Method, http.MethodPost)
		authHeader, err := client.GenAuthHeader()
		require.NoError(t, err)

		assert.Equal(r.Header.Get("Authorization"), authHeader)
		assert.Contains(r.URL.Path, "/api/v5/addons")
		file, _, err := r.FormFile("upload")
		require.NoError(t, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()

		body, err := io.ReadAll(file)
		require.NoError(t, err)

		assert.Contains(string(body), "test content")

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

	assert.Equal(status, string(result))
}

func TestUploadUpdate(t *testing.T) {
	assert := assert.New(t)

	currentTimeSec := time.Now().Unix()
	now := func() int64 {
		return currentTimeSec
	}

	client := firefox.NewClient(firefox.ClientConfig{ClientID: clientID, ClientSecret: clientSecret, Now: now})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(http.MethodPut, r.Method)
		assert.Contains(r.URL.Path, path.Join("api/v5/addons", appID, "versions", version))
		authHeader, err := client.GenAuthHeader()
		require.NoError(t, err)

		assert.Equal(r.Header.Get("Authorization"), authHeader)
		file, header, err := r.FormFile("upload")
		require.NoError(t, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()

		assert.Equal(header.Filename, "extension.zip")

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

	assert.Equal(response, string(actualResponse))
}

func TestUploadSource(t *testing.T) {
	assert := assert.New(t)

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
		assert.Equal(http.MethodPatch, r.Method)
		assert.Contains(r.URL.Path, "/api/v5/addons/addon/"+appID+"/versions/"+versionID+"/")
		assert.Contains(r.Header.Get("Content-Type"), "multipart/form-data")

		file, header, err := r.FormFile("source")
		require.NoError(t, err)
		defer func() { err = errors.WithDeferred(err, file.Close()) }()

		assert.Equal(header.Filename, "source.zip")
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

	assert.Equal(response, string(uploadResponse))
}
