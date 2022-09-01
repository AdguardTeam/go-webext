package edge_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/adguardteam/go-webext/internal/edge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAuthServer(t *testing.T, accessToken string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := edge.AuthorizeResponse{
			TokenType:   "",
			ExpiresIn:   0,
			AccessToken: accessToken,
		}

		responseData, err := json.Marshal(response)
		require.NoError(t, err)

		_, err = w.Write(responseData)
		require.NoError(t, err)
	}))
}

const (
	clientID     = "test_client_id"
	clientSecret = "test_client_secret"
	accessToken  = "test_access_token"
	appID        = "test_app_id"
	operationID  = "test_operation_id"
)

func TestAuthorize(t *testing.T) {
	assert := assert.New(t)

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(req.Method, http.MethodPost)
		assert.Equal(req.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
		assert.Equal(req.FormValue("client_id"), clientID)
		assert.Equal(req.FormValue("scope"), "https://api.addons.microsoftedge.microsoft.com/.default")
		assert.Equal(req.FormValue("client_secret"), clientSecret)
		assert.Equal(req.FormValue("grant_type"), "client_credentials")

		response, err := json.Marshal(edge.AuthorizeResponse{
			TokenType:   "",
			ExpiresIn:   0,
			AccessToken: accessToken,
		})
		require.NoError(t, err)

		_, err = w.Write(response)
		require.NoError(t, err)
	}))

	client, err := edge.NewClient(clientID, clientSecret, authServer.URL)
	require.NoError(t, err)

	actualAccessToken, err := client.Authorize()
	require.NoError(t, err)

	assert.Equal(accessToken, actualAccessToken)
}

func TestUploadUpdate(t *testing.T) {
	assert := assert.New(t)

	operationID := "test_operation_id"

	authServer := newAuthServer(t, accessToken)
	defer authServer.Close()

	client, err := edge.NewClient(clientID, clientSecret, authServer.URL)
	require.NoError(t, err)

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(http.MethodPost, r.Method)
		assert.Equal("Bearer "+accessToken, r.Header.Get("Authorization"))
		assert.Equal("application/zip", r.Header.Get("Content-Type"))
		assert.Equal(path.Join("/v1/products", appID, "submissions/draft/package"), r.URL.Path)

		responseBody, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		assert.Equal("test_file_content", string(responseBody))

		w.Header().Set("Location", operationID)
		w.WriteHeader(http.StatusAccepted)

		_, err = w.Write(nil)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := edge.Store{
		Client: &client,
		URL:    storeURL,
	}

	actualUpdateResponse, err := store.UploadUpdate(appID, "./testdata/test.txt")
	require.NoError(t, err)

	assert.Equal(operationID, actualUpdateResponse)
}

func TestUploadStatus(t *testing.T) {
	assert := assert.New(t)

	response := edge.UploadStatusResponse{
		ID:              "{operationID}",
		CreatedTime:     "Date Time",
		LastUpdatedTime: "Date Time",
		Status:          "Failed",
		Message:         "Error Message.",
		ErrorCode:       "Error Code",
		Errors:          []edge.StatusError{{Message: "test error"}},
	}

	authServer := newAuthServer(t, accessToken)
	defer authServer.Close()

	client, err := edge.NewClient(clientID, clientSecret, authServer.URL)
	require.NoError(t, err)

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(r.Header.Get("Authorization"), "Bearer "+accessToken)
		assert.Equal(r.URL.Path, "/v1/products/"+appID+"/submissions/draft/package/operations/"+operationID)

		response, err := json.Marshal(response)
		require.NoError(t, err)

		_, err = w.Write(response)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := edge.Store{
		Client: &client,
		URL:    storeURL,
	}

	uploadStatus, err := store.UploadStatus(appID, operationID)
	require.NoError(t, err)

	assert.Equal(response, *uploadStatus)
}

func TestUpdate(t *testing.T) {
	filepath := "testdata/test.txt"

	t.Run("waits for successful response", func(t *testing.T) {
		succeededResponse := edge.UploadStatusResponse{
			ID:              "",
			CreatedTime:     "",
			LastUpdatedTime: "",
			Status:          edge.Succeeded.String(),
			Message:         "",
			ErrorCode:       "",
			Errors:          nil,
		}

		authServer := newAuthServer(t, accessToken)
		defer authServer.Close()

		client, err := edge.NewClient(clientID, clientSecret, authServer.URL)
		require.NoError(t, err)

		counter := 0
		storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "submissions/draft/package/operations") {
				if counter == 0 {
					inProgressResponse, err := json.Marshal(edge.UploadStatusResponse{
						ID:              "",
						CreatedTime:     "",
						LastUpdatedTime: "",
						Status:          edge.InProgress.String(),
						Message:         "",
						ErrorCode:       "",
						Errors:          nil,
					})
					require.NoError(t, err)

					_, err = w.Write(inProgressResponse)
					require.NoError(t, err)
				}
				if counter == 1 {
					marshaledSucceededResponse, err := json.Marshal(succeededResponse)
					require.NoError(t, err)

					_, err = w.Write(marshaledSucceededResponse)
					require.NoError(t, err)
				}
				counter++

				return
			}

			w.Header().Set("Location", operationID)
			w.WriteHeader(http.StatusAccepted)

			_, err := w.Write(nil)
			require.NoError(t, err)
		}))
		defer storeServer.Close()

		storeURL, err := url.Parse(storeServer.URL)
		require.NoError(t, err)

		store := edge.Store{
			Client: &client,
			URL:    storeURL,
		}

		response, err := store.Update(
			appID,
			filepath,
			edge.UpdateOptions{
				RetryTimeout: time.Nanosecond,
			})
		require.NoError(t, err)

		assert.Equal(t, succeededResponse, *response)
	})

	t.Run("throws error on timeout", func(t *testing.T) {
		updateOptions := edge.UpdateOptions{
			RetryTimeout:      time.Millisecond,
			WaitStatusTimeout: 2 * time.Millisecond,
		}

		authServer := newAuthServer(t, accessToken)
		defer authServer.Close()

		client, err := edge.NewClient(clientID, clientSecret, authServer.URL)
		require.NoError(t, err)

		storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "submissions/draft/package/operations") {
				inProgressResponse, err := json.Marshal(edge.UploadStatusResponse{
					ID:              "",
					CreatedTime:     "",
					LastUpdatedTime: "",
					Status:          edge.InProgress.String(),
					Message:         "",
					ErrorCode:       "",
					Errors:          nil,
				})
				require.NoError(t, err)

				_, err = w.Write(inProgressResponse)
				require.NoError(t, err)
				return
			}

			w.Header().Set("Location", operationID)
			w.WriteHeader(http.StatusAccepted)

			_, err := w.Write(nil)
			require.NoError(t, err)
		}))
		defer storeServer.Close()

		storeURL, err := url.Parse(storeServer.URL)
		require.NoError(t, err)

		store := edge.Store{
			Client: &client,
			URL:    storeURL,
		}

		_, err = store.Update(appID, filepath, updateOptions)
		assert.ErrorContains(t, err, "update failed due to timeout")
	})
}

func TestPublishExtension(t *testing.T) {
	authServer := newAuthServer(t, accessToken)
	defer authServer.Close()

	client, err := edge.NewClient(clientID, clientSecret, authServer.URL)
	require.NoError(t, err)

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/products/"+appID+"/submissions", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, r.Header.Get("Authorization"), "Bearer "+accessToken)

		w.Header().Set("Location", operationID)
		w.WriteHeader(http.StatusAccepted)
		_, err := w.Write([]byte(nil))
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := edge.Store{
		Client: &client,
		URL:    storeURL,
	}

	response, err := store.PublishExtension(appID)
	require.NoError(t, err)

	assert.Equal(t, operationID, response)
}

func TestPublishStatus(t *testing.T) {
	statusResponse := edge.PublishStatusResponse{
		ID:              "",
		CreatedTime:     "",
		LastUpdatedTime: "",
		Status:          "Succeeded",
		Message:         "",
		ErrorCode:       "",
		Errors:          nil,
	}

	authServer := newAuthServer(t, accessToken)
	defer authServer.Close()

	client, err := edge.NewClient(clientID, clientSecret, authServer.URL)
	require.NoError(t, err)

	storeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/products/"+appID+"/submissions/operations/"+operationID, r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "Bearer "+accessToken, r.Header.Get("Authorization"))

		response, err := json.Marshal(statusResponse)
		require.NoError(t, err)

		_, err = w.Write(response)
		require.NoError(t, err)
	}))
	defer storeServer.Close()

	storeURL, err := url.Parse(storeServer.URL)
	require.NoError(t, err)

	store := edge.Store{
		Client: &client,
		URL:    storeURL,
	}

	response, err := store.PublishStatus(appID, operationID)
	require.NoError(t, err)

	assert.Equal(t, statusResponse, *response)
}
