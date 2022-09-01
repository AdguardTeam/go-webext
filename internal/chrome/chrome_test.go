package chrome_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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
	assert := assert.New(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(http.MethodPost, r.Method)
		assert.Equal(clientID, r.FormValue("client_id"))
		assert.Equal(clientSecret, r.FormValue("client_secret"))
		assert.Equal(refreshToken, r.FormValue("refresh_token"))
		assert.Equal("refresh_token", r.FormValue("grant_type"))
		assert.Equal("urn:ietf:wg:oauth:2.0:oob", r.FormValue("redirect_uri"))

		expectedJSON, err := json.Marshal(map[string]string{
			"access_token": accessToken,
		})
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))

	defer server.Close()

	client := chrome.Client{
		URL:          server.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
	}

	result, err := client.Authorize()
	if err != nil {
		assert.NoError(err, "Should be no errors")
	}

	assert.Equal(accessToken, result, "Tokens should be equal")
}

func TestStatus(t *testing.T) {
	assert := assert.New(t)

	status := chrome.StatusResponse{
		Kind:        "test kind",
		ID:          appID,
		PublicKey:   "test public key",
		UploadState: "test upload state",
		CrxVersion:  "test version",
	}

	authServer := createAuthServer(t, accessToken)
	defer authServer.Close()

	client := chrome.Client{
		URL:          authServer.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(r.Method, http.MethodGet)
		assert.Contains(r.URL.Path, "chromewebstore/v1.1/items/"+appID)
		assert.Equal(r.URL.Query().Get("projection"), "DRAFT")
		assert.Equal(r.Header.Get("Authorization"), "Bearer "+accessToken)

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

	store := chrome.Store{
		Client: &client,
		URL:    storeURL,
	}

	actualStatusBytes, err := store.Status(appID)
	require.NoError(t, err)

	var actualStatus chrome.StatusResponse
	err = json.Unmarshal(actualStatusBytes, &actualStatus)
	require.NoError(t, err)

	assert.Equal(status, actualStatus)
}

func TestInsert(t *testing.T) {
	assert := assert.New(t)

	insertResponse := chrome.InsertResponse{
		Kind:        "chromewebstore#item",
		ID:          "lcfmdcpihnaincdpgibhlncnekofobkc",
		UploadState: "SUCCESS",
	}

	authServer := createAuthServer(t, accessToken)
	defer authServer.Close()

	client := chrome.Client{
		URL:          authServer.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(http.MethodPost, r.Method)
		assert.Contains(r.URL.Path, "upload/chromewebstore/v1.1/items")
		assert.Equal(r.Header.Get("Authorization"), "Bearer "+accessToken)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.Equal("test file", string(body))

		expectedJSON, err := json.Marshal(map[string]string{
			"kind":        insertResponse.Kind,
			"id":          insertResponse.ID,
			"uploadState": insertResponse.UploadState,
		})
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.Store{
		Client: &client,
		URL:    storeURL,
	}

	result, err := store.Insert("./testdata/test.txt")
	require.NoError(t, err)

	assert.Equal(insertResponse, *result)
}

func TestUpdate(t *testing.T) {
	assert := assert.New(t)

	updateResponse := chrome.UpdateResponse{
		Kind:        "test kind",
		ID:          appID,
		UploadState: "test success",
	}

	authServer := createAuthServer(t, accessToken)
	defer authServer.Close()

	client := chrome.Client{
		URL:          authServer.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(http.MethodPut, r.Method)
		assert.Contains(r.URL.Path, "upload/chromewebstore/v1.1/items/"+appID)
		assert.Equal(r.Header.Get("Authorization"), "Bearer "+accessToken)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.Equal("test file", string(body))

		expectedJSON, err := json.Marshal(updateResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.Store{
		Client: &client,
		URL:    storeURL,
	}

	result, err := store.Update(appID, "testdata/test.txt")
	require.NoError(t, err)
	assert.Equal(updateResponse, *result)
}

func TestPublish(t *testing.T) {
	assert := assert.New(t)

	publishResponse := chrome.PublishResponse{
		Kind:         "test_kind",
		ItemID:       appID,
		Status:       []string{"ok"},
		StatusDetail: []string{"ok"},
	}

	authServer := createAuthServer(t, accessToken)
	defer authServer.Close()

	client := chrome.Client{
		URL:          authServer.URL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken,
	}

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(http.MethodPost, r.Method)
		assert.Contains(r.URL.Path, "chromewebstore/v1.1/items/"+appID+"/publish")
		assert.Equal(r.Header.Get("Authorization"), "Bearer "+accessToken)
		assert.Equal(r.Header.Get("Content-Length"), "0")

		expectedJSON, err := json.Marshal(publishResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.Store{
		Client: &client,
		URL:    storeURL,
	}

	result, err := store.Publish(appID)
	require.NoError(t, err)
	assert.Equal(publishResponse, *result)
}
