package chrome_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/AdguardTeam/golibs/logutil/slogutil"
	"github.com/adguardteam/go-webext/internal/chrome"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	clientID     = "test_client_id"
	clientSecret = "test_client_secret"
	accessToken  = "test_access_token"
	refreshToken = "test_refresh_token"
	appID        = "test_app_id"
)

func createAuthServer(t *testing.T, accessToken string) *httptest.Server {
	t.Helper()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedJSON, err := json.Marshal(map[string]string{
			"access_token": accessToken,
		})
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))

	return authServer
}

func TestAuthorize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, clientID, r.FormValue("client_id"))
		assert.Equal(t, clientSecret, r.FormValue("client_secret"))
		assert.Equal(t, refreshToken, r.FormValue("refresh_token"))
		assert.Equal(t, "refresh_token", r.FormValue("grant_type"))
		assert.Equal(t, "urn:ietf:wg:oauth:2.0:oob", r.FormValue("redirect_uri"))

		expectedJSON, err := json.Marshal(map[string]string{
			"access_token": accessToken,
		})
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))

	defer server.Close()

	client := chrome.NewClient(chrome.ClientConfig{
		URL:          server.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
		Logger:       slogutil.NewDiscardLogger(),
	})

	result, err := client.Authorize()
	if err != nil {
		assert.NoError(t, err, "Should be no errors")
	}

	assert.Equal(t, accessToken, result, "Tokens should be equal")
}

func TestStatus(t *testing.T) {
	status := chrome.StatusResponse{
		Kind:        "test kind",
		ID:          appID,
		PublicKey:   "test public key",
		UploadState: "test upload state",
		CrxVersion:  "test version",
	}

	authServer := createAuthServer(t, accessToken)
	defer authServer.Close()

	client := chrome.NewClient(chrome.ClientConfig{
		URL:          authServer.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
		Logger:       slogutil.NewDiscardLogger(),
	})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Method, http.MethodGet)
		assert.Contains(t, r.URL.Path, "chromewebstore/v1.1/items/"+appID)
		assert.Equal(t, r.URL.Query().Get("projection"), "DRAFT")
		assert.Equal(t, r.Header.Get("Authorization"), "Bearer "+accessToken)

		expectedJSON, err := json.Marshal(map[string]string{
			"kind":        status.Kind,
			"id":          appID,
			"publicKey":   status.PublicKey,
			"uploadState": status.UploadState,
			"crxVersion":  status.CrxVersion,
		})
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStore(chrome.StoreConfig{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	actualStatusBytes, err := store.Status(appID)
	require.NoError(t, err)

	var actualStatus chrome.StatusResponse
	err = json.Unmarshal(actualStatusBytes, &actualStatus)
	require.NoError(t, err)

	assert.Equal(t, status, actualStatus)
}

func TestInsert(t *testing.T) {
	insertResponse := chrome.ItemResource{
		Kind:        "chromewebstore#item",
		ID:          "lcfmdcpihnaincdpgibhlncnekofobkc",
		UploadState: chrome.UploadStateSuccess,
	}

	authServer := createAuthServer(t, accessToken)
	defer authServer.Close()

	client := chrome.NewClient(chrome.ClientConfig{
		URL:          authServer.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
		Logger:       slogutil.NewDiscardLogger(),
	})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "upload/chromewebstore/v1.1/items")
		assert.Equal(t, r.Header.Get("Authorization"), "Bearer "+accessToken)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.Equal(t, "test file", string(body))

		expectedJSON, err := json.Marshal(insertResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStore(chrome.StoreConfig{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	result, err := store.Insert("./testdata/test.txt")
	require.NoError(t, err)

	assert.Equal(t, insertResponse, *result)
}

func TestUpdate(t *testing.T) {
	updateResponse := chrome.ItemResource{
		Kind:        "test kind",
		ID:          appID,
		UploadState: chrome.UploadStateSuccess,
	}

	authServer := createAuthServer(t, accessToken)
	defer authServer.Close()

	client := chrome.NewClient(chrome.ClientConfig{
		URL:          authServer.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
		Logger:       slogutil.NewDiscardLogger(),
	})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.URL.Path, "upload/chromewebstore/v1.1/items/"+appID)
		assert.Equal(t, r.Header.Get("Authorization"), "Bearer "+accessToken)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.Equal(t, "test file", string(body))

		expectedJSON, err := json.Marshal(updateResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStore(chrome.StoreConfig{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	result, err := store.Update(appID, "testdata/test.txt")
	require.NoError(t, err)

	assert.Equal(t, updateResponse, *result)
}

func TestPublish(t *testing.T) {
	publishResponse := chrome.PublishResponse{
		Kind:         "test_kind",
		ItemID:       appID,
		Status:       []string{"ok"},
		StatusDetail: []string{"ok"},
	}

	authServer := createAuthServer(t, accessToken)
	defer authServer.Close()

	client := chrome.NewClient(chrome.ClientConfig{
		URL:          authServer.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
		Logger:       slogutil.NewDiscardLogger(),
	})

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "chromewebstore/v1.1/items/"+appID+"/publish")
		assert.Equal(t, r.Header.Get("Authorization"), "Bearer "+accessToken)

		// Store query parameters in a variable
		query := r.URL.Query()

		// Test with options
		if query.Get("deployPercentage") == "50" {
			assert.Equal(t, "50", query.Get("deployPercentage"))
			assert.Equal(t, "true", query.Get("reviewExemption"))
			assert.Equal(t, "trustedTesters", query.Get("publishTarget"))
		}

		expectedJSON, err := json.Marshal(publishResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStore(chrome.StoreConfig{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	// Test without options
	result, err := store.Publish(appID, nil)
	require.NoError(t, err)
	assert.Equal(t, publishResponse, *result)

	// Test with options
	p := 50 // Create a variable to get its address
	opts := &chrome.PublishOptions{
		Target:           "trustedTesters",
		DeployPercentage: &p,
		ReviewExemption:  true,
	}
	result, err = store.Publish(appID, opts)
	require.NoError(t, err)
	assert.Equal(t, publishResponse, *result)
}
