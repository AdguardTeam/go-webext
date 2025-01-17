package edge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorize(t *testing.T) {
	const (
		clientID     = "test_client_id"
		clientSecret = "test_client_secret"
		accessToken  = "test_access_token"
	)

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, req.Method, http.MethodPost)
		assert.Equal(t, req.Header.Get("Content-Type"), "application/x-www-form-urlencoded")
		assert.Equal(t, req.FormValue("client_id"), clientID)
		assert.Equal(t, req.FormValue("scope"), "https://api.addons.microsoftedge.microsoft.com/.default")
		assert.Equal(t, req.FormValue("client_secret"), clientSecret)
		assert.Equal(t, req.FormValue("grant_type"), "client_credentials")

		response, err := json.Marshal(AuthorizeResponse{
			TokenType:   "",
			ExpiresIn:   0,
			AccessToken: accessToken,
		})
		require.NoError(t, err)

		_, err = w.Write(response)
		require.NoError(t, err)
	}))
	defer authServer.Close()

	accessTokenURL, err := url.Parse(authServer.URL)
	require.NoError(t, err)

	clientConfig := NewV1Config(clientID, clientSecret, accessTokenURL)
	client := NewClient(clientConfig)

	req, err := http.NewRequest(http.MethodGet, "http://test.com", nil)
	require.NoError(t, err)

	err = client.setRequestHeaders(req)
	require.NoError(t, err)

	assert.Equal(t, "Bearer "+accessToken, req.Header.Get("Authorization"))
}
