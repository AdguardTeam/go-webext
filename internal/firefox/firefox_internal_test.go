package firefox

import (
	"os"
	"testing"

	"github.com/AdguardTeam/golibs/logutil/slogutil"
	"github.com/adguardteam/go-webext/internal/urlutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAppID   = "test_app_id"
	testVersion = "0.0.3"
)

func TestExtDataFromFile(t *testing.T) {
	t.Run("application manifest", func(t *testing.T) {
		extData, err := extDataFromFile("testdata/extension.zip")
		require.NoError(t, err)

		assert.Equal(t, testVersion, extData.version)
		assert.Equal(t, testAppID, extData.appID)
	})

	t.Run("browser specific manifest", func(t *testing.T) {
		extData, err := extDataFromFile("testdata/extension-browser-specific.zip")
		require.NoError(t, err)

		assert.Equal(t, testVersion, extData.version)
		assert.Equal(t, testAppID, extData.appID)
	})
}

type testAPI struct {
	API
	onDownloadSignedByURL func(url string) ([]byte, error)
	onVersionDetail       func(appID, versionID string) (*VersionInfo, error)
}

func (a *testAPI) DownloadSignedByURL(url string) ([]byte, error) {
	return a.onDownloadSignedByURL(url)
}

func (a *testAPI) VersionDetail(appID, version string) (*VersionInfo, error) {
	return a.onVersionDetail(appID, version)
}

func TestDownloadSigned(t *testing.T) {
	expectedFilename := "firefox.xpi"
	expectedURL := urlutil.JoinPath("https://addons.mozilla.org/files", expectedFilename)

	versionInfo := &VersionInfo{
		File: FileInfo{
			URL: expectedURL,
		},
	}

	mockAPI := &testAPI{
		onVersionDetail: func(appID, version string) (*VersionInfo, error) {
			require.Equal(t, testAppID, appID)
			require.Equal(t, testVersion, version)
			return versionInfo, nil
		},
		onDownloadSignedByURL: func(url string) ([]byte, error) {
			require.Equal(t, expectedURL, url)
			return []byte("test"), nil
		},
	}

	store := NewStore(StoreConfig{
		API:    mockAPI,
		Logger: slogutil.NewDiscardLogger(),
	})

	err := store.downloadSigned(testAppID, testVersion, expectedFilename)
	require.NoError(t, err)

	// Check if the file exists.
	_, err = os.Stat(expectedFilename)
	require.NoError(t, err)

	// Remove the file after the test run.
	t.Cleanup(func() {
		err = os.Remove(expectedFilename)
		if err != nil {
			t.Error("Failed to remove file:", err)
		}
	})
}
