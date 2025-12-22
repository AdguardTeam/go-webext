package chrome_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/AdguardTeam/golibs/httphdr"
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
	publisherID  = "test-publisher"
	itemID       = "test-item-id"
	publicKey    = "test-public-key"
	crxVersion   = "1.0.0"
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

func TestStatusV1(t *testing.T) {
	statusV1 := chrome.StatusResponseV1{
		Kind:          "chromewebstore#item",
		ID:            itemID,
		PublicKey:     publicKey,
		UploadStateV1: chrome.UploadStateSuccessV1.String(),
		CrxVersion:    crxVersion,
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
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/chromewebstore/v1.1/items/"+itemID)
		assert.Equal(t, "Bearer "+accessToken, r.Header.Get(httphdr.Authorization))
		assert.Equal(t, "DRAFT", r.URL.Query().Get("projection"))

		expectedJSON, err := json.Marshal(statusV1)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStoreV1(chrome.StoreV1Config{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	actualStatus, err := store.Status(itemID)
	require.NoError(t, err)

	assert.Equal(t, &statusV1, actualStatus)
}

func TestUpdateV1(t *testing.T) {
	updateResponse := chrome.ItemResourceV1{
		Kind:          "chromewebstore#item",
		ID:            itemID,
		UploadStateV1: chrome.UploadStateSuccessV1,
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
		assert.Contains(t, r.URL.Path, "/upload/chromewebstore/v1.1/items/"+itemID)
		assert.Equal(t, "Bearer "+accessToken, r.Header.Get(httphdr.Authorization))
		assert.Equal(t, "application/zip", r.Header.Get(httphdr.ContentType))

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

	store := chrome.NewStoreV1(chrome.StoreV1Config{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	result, err := store.Update(itemID, "./testdata/test.txt")
	require.NoError(t, err)

	assert.Equal(t, updateResponse, *result)
}

func TestInsertV1(t *testing.T) {
	insertResponse := chrome.ItemResourceV1{
		Kind:          "chromewebstore#item",
		ID:            itemID,
		UploadStateV1: chrome.UploadStateSuccessV1,
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
		assert.Contains(t, r.URL.Path, "/upload/chromewebstore/v1.1/items")
		assert.NotContains(t, r.URL.Path, itemID) // Insert doesn't have itemID in path
		assert.Equal(t, "Bearer "+accessToken, r.Header.Get(httphdr.Authorization))
		assert.Equal(t, "application/zip", r.Header.Get(httphdr.ContentType))

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

	store := chrome.NewStoreV1(chrome.StoreV1Config{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	result, err := store.Insert("./testdata/test.txt")
	require.NoError(t, err)

	assert.Equal(t, insertResponse, *result)
}

func TestPublishV1(t *testing.T) {
	publishResponse := chrome.PublishResponseV1{
		Kind:   "chromewebstore#item",
		ItemID: itemID,
		Status: []string{"OK"},
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
		assert.Contains(t, r.URL.Path, "/chromewebstore/v1.1/items/"+itemID+"/publish")
		assert.Equal(t, "Bearer "+accessToken, r.Header.Get(httphdr.Authorization))

		// Verify that parameters are in request body, not in query string
		assert.Empty(t, r.URL.Query(), "query parameters should be empty for POST request")

		// Check Content-Type if body is present
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		if len(body) > 0 {
			assert.Equal(t, "application/json", r.Header.Get(httphdr.ContentType))
			var opts chrome.PublishOptionsV1
			err = json.Unmarshal(body, &opts)
			require.NoError(t, err)
		}

		expectedJSON, err := json.Marshal(publishResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStoreV1(chrome.StoreV1Config{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	// Test without options
	result, err := store.Publish(itemID, nil)
	require.NoError(t, err)
	assert.Equal(t, publishResponse, *result)

	// Test with options
	percentage := 50
	opts := &chrome.PublishOptionsV1{
		Target:           "trustedTesters",
		DeployPercentage: &percentage,
	}
	result, err = store.Publish(itemID, opts)
	require.NoError(t, err)
	assert.Equal(t, publishResponse, *result)

	// Test with invalid deployPercentage (too high)
	invalidPercentage := 101
	invalidOpts := &chrome.PublishOptionsV1{
		DeployPercentage: &invalidPercentage,
	}
	_, err = store.Publish(itemID, invalidOpts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deploy percentage must be between 0 and 100")

	// Test with invalid deployPercentage (negative)
	negativePercentage := -1
	invalidOpts = &chrome.PublishOptionsV1{
		DeployPercentage: &negativePercentage,
	}
	_, err = store.Publish(itemID, invalidOpts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deploy percentage must be between 0 and 100")
}

func TestAuthorizeV2(t *testing.T) {
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

func TestStatusV2(t *testing.T) {
	status := chrome.StatusResponse{
		Name:      "publishers/" + publisherID + "/items/" + itemID,
		ItemID:    itemID,
		PublicKey: publicKey,
		PublishedItemRevisionStatus: &chrome.ItemRevisionStatus{
			State: chrome.ItemStatePublished,
			DistributionChannels: []chrome.DistributionChannel{{
				CrxVersion:       crxVersion,
				DeployPercentage: 100,
			}},
		},
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
		assert.Contains(t, r.URL.Path, "/v2/publishers/"+publisherID+"/items/"+itemID+":fetchStatus")
		assert.Equal(t, r.Header.Get(httphdr.Authorization), "Bearer "+accessToken)

		expectedJSON, err := json.Marshal(status)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStoreV2(chrome.StoreV2Config{
		Client:      client,
		URL:         storeURL,
		PublisherID: publisherID,
		Logger:      slogutil.NewDiscardLogger(),
	})

	actualStatus, err := store.Status(itemID)
	require.NoError(t, err)

	assert.Equal(t, &status, actualStatus)
}

func TestUploadV2(t *testing.T) {
	uploadResponse := chrome.UploadResponse{
		Name:          "publishers/" + publisherID + "/items/" + itemID,
		ItemID:        itemID,
		CrxVersion:    crxVersion,
		UploadStateV2: chrome.UploadStateSucceededV2,
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
		assert.Contains(t, r.URL.Path, "/upload/v2/publishers/"+publisherID+"/items/"+itemID+":upload")
		assert.Equal(t, r.Header.Get(httphdr.Authorization), "Bearer "+accessToken)
		assert.Equal(t, "application/zip", r.Header.Get(httphdr.ContentType))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.Equal(t, "test file", string(body))

		expectedJSON, err := json.Marshal(uploadResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStoreV2(chrome.StoreV2Config{
		Client:      client,
		URL:         storeURL,
		PublisherID: publisherID,
		Logger:      slogutil.NewDiscardLogger(),
	})

	result, err := store.Upload(itemID, "./testdata/test.txt")
	require.NoError(t, err)

	assert.Equal(t, uploadResponse, *result)
}

func TestPublishV2(t *testing.T) {
	publishResponse := chrome.PublishResponse{
		Name:   "publishers/" + publisherID + "/items/" + itemID,
		ItemID: itemID,
		State:  chrome.ItemStatePublished,
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
		assert.Contains(t, r.URL.Path, "/v2/publishers/"+publisherID+"/items/"+itemID+":publish")
		assert.Equal(t, r.Header.Get(httphdr.Authorization), "Bearer "+accessToken)

		expectedJSON, err := json.Marshal(publishResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStoreV2(chrome.StoreV2Config{
		Client:      client,
		URL:         storeURL,
		PublisherID: publisherID,
		Logger:      slogutil.NewDiscardLogger(),
	})

	// Test without options
	result, err := store.Publish(itemID, nil)
	require.NoError(t, err)
	assert.Equal(t, publishResponse, *result)

	// Test with options
	opts := &chrome.PublishOptions{
		PublishType: chrome.PublishTypeDefault,
		DeployInfos: []chrome.DeployInfo{{DeployPercentage: 50}},
		SkipReview:  true,
	}
	result, err = store.Publish(itemID, opts)
	require.NoError(t, err)
	assert.Equal(t, publishResponse, *result)
}

// TestUploadV2FailedState tests that Upload method properly handles FAILED upload state
func TestUploadV2FailedState(t *testing.T) {
	uploadResponse := chrome.UploadResponse{
		Name:          "publishers/" + publisherID + "/items/" + itemID,
		ItemID:        itemID,
		CrxVersion:    crxVersion,
		UploadStateV2: chrome.UploadStateFailedV2,
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
		expectedJSON, err := json.Marshal(uploadResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStoreV2(chrome.StoreV2Config{
		Client:      client,
		URL:         storeURL,
		PublisherID: publisherID,
		Logger:      slogutil.NewDiscardLogger(),
	})

	result, err := store.Upload(itemID, "./testdata/test.txt")

	// Should return error for failed upload state
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "upload failed")
}

// TestInsertV1FailureState tests error handling for failed insert
func TestInsertV1FailureState(t *testing.T) {
	insertResponse := chrome.ItemResourceV1{
		Kind:          "chromewebstore#item",
		ID:            itemID,
		UploadStateV1: chrome.UploadStateFailureV1,
		ItemError: []chrome.ItemError{
			{
				ErrorCode:   "PKG_INVALID",
				ErrorDetail: "Package is invalid",
			},
		},
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
		expectedJSON, err := json.Marshal(insertResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStoreV1(chrome.StoreV1Config{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	result, err := store.Insert("./testdata/test.txt")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "non success upload state")
}

// TestUpdateV1FailureState tests error handling for failed update
func TestUpdateV1FailureState(t *testing.T) {
	updateResponse := chrome.ItemResourceV1{
		Kind:          "chromewebstore#item",
		ID:            itemID,
		UploadStateV1: chrome.UploadStateFailureV1,
		ItemError: []chrome.ItemError{
			{
				ErrorCode:   "PKG_INVALID",
				ErrorDetail: "Package is invalid",
			},
		},
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
		expectedJSON, err := json.Marshal(updateResponse)
		require.NoError(t, err)

		_, err = w.Write(expectedJSON)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := chrome.NewStoreV1(chrome.StoreV1Config{
		Client: client,
		URL:    storeURL,
		Logger: slogutil.NewDiscardLogger(),
	})

	result, err := store.Update(itemID, "./testdata/test.txt")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failure in response")
}
