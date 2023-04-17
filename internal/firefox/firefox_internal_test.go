package firefox

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/adguardteam/go-webext/internal/urlutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAppID      = "test_app_id"
	testVersion    = "0.0.3"
	testSourcepath = "testdata/source.zip"
	testVersionID  = "123456"
	testFilepath   = "testdata/extension.zip"
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
	onUploadStatus        func(appID, version string) (*UploadStatus, error)
	onUploadSource        func(appID, versionID string, file io.Reader) error
	onUploadUpdate        func(appID, version string, file io.Reader) error
	onDownloadSignedByURL func(url string) ([]byte, error)
}

func (a *testAPI) UploadStatus(appID, version string) (status *UploadStatus, err error) {
	return a.onUploadStatus(appID, version)
}

func (a *testAPI) UploadSource(appID, versionID string, file io.Reader) error {
	return a.onUploadSource(appID, versionID, file)
}

func (a *testAPI) UploadUpdate(appID, version string, file io.Reader) error {
	return a.onUploadUpdate(appID, version, file)
}

func (a *testAPI) DownloadSignedByURL(url string) ([]byte, error) {
	return a.onDownloadSignedByURL(url)
}

// TODO (maximtop): test timeout case during awaiting validation.
func TestAwaitValidation(t *testing.T) {
	expectedUploadStatus := &UploadStatus{
		GUID:             "",
		Active:           false,
		AutomatedSigning: false,
		Files:            nil,
		PassedReview:     false,
		Pk:               "",
		Processed:        true,
		Reviewed:         false,
		URL:              "",
		Valid:            true,
		ValidationURL:    "",
		Version:          "",
	}

	mockAPI := &testAPI{
		onUploadStatus: func(appID, version string) (*UploadStatus, error) {
			require.Equal(t, testAppID, appID)
			require.Equal(t, testVersion, version)
			return expectedUploadStatus, nil
		},
	}

	store := Store{API: mockAPI}
	err := store.awaitValidation(testAppID, testVersion)
	require.NoError(t, err)
}

func TestAwaitSigning(t *testing.T) {
	testURL := "https://addons.mozilla.org/firefox/downloads/file/1234567/test.xpi"
	expectedUploadStatus := &UploadStatus{
		GUID:             "test",
		Active:           true,
		AutomatedSigning: true,
		Files: []UploadStatusFiles{
			{
				DownloadURL: testURL,
				Hash:        "",
				Signed:      true,
			},
		},
		PassedReview:  true,
		Pk:            "",
		Processed:     true,
		Reviewed:      true,
		URL:           "",
		Valid:         true,
		ValidationURL: "",
		Version:       "",
	}
	mockAPI := &testAPI{
		onUploadStatus: func(appID, version string) (*UploadStatus, error) {
			require.Equal(t, testAppID, appID)
			require.Equal(t, testVersion, version)
			return expectedUploadStatus, nil
		},
	}
	store := Store{API: mockAPI}

	err := store.awaitSigning(testAppID, testVersion)
	require.NoError(t, err)
}

func TestUploadSource(t *testing.T) {
	// Read the contents of the test file
	expectedContents, err := os.ReadFile(filepath.Clean(testSourcepath))
	require.NoError(t, err)

	mockAPI := &testAPI{
		onUploadSource: func(appID, versionID string, file io.Reader) error {
			require.Equal(t, testAppID, appID)
			require.Equal(t, testVersionID, versionID)

			fileContents, err := io.ReadAll(file)
			require.NoError(t, err)

			require.Equal(t, expectedContents, fileContents)

			return nil
		},
	}

	store := Store{API: mockAPI}

	err = store.uploadSource(testAppID, testVersionID, testSourcepath)
	require.NoError(t, err)
}

func TestUploadUpdate(t *testing.T) {
	// Read the contents of the test file
	expectedContents, err := os.ReadFile(filepath.Clean(testFilepath))
	require.NoError(t, err)

	mockAPI := &testAPI{
		onUploadUpdate: func(appID, version string, file io.Reader) error {
			require.Equal(t, testAppID, appID)
			require.Equal(t, testVersion, version)

			fileContents, err := io.ReadAll(file)
			require.NoError(t, err)

			require.Equal(t, expectedContents, fileContents)

			return nil
		},
	}

	store := Store{API: mockAPI}
	err = store.uploadUpdate(testAppID, testVersion, testFilepath)
	require.NoError(t, err)
}

func TestDownloadSigned(t *testing.T) {
	expectedFilename := "signed.xpi"
	expectedURL := urlutil.JoinPath("https://addons.mozilla.org/files", expectedFilename)

	expectedUploadStatus := UploadStatus{
		GUID:             "",
		Active:           false,
		AutomatedSigning: false,
		Files: []UploadStatusFiles{
			{
				DownloadURL: expectedURL,
				Hash:        "",
				Signed:      true,
			},
		},
		PassedReview:  true,
		Pk:            "",
		Processed:     true,
		Reviewed:      true,
		URL:           "",
		Valid:         true,
		ValidationURL: "",
		Version:       "",
	}

	mockAPI := &testAPI{
		onUploadStatus: func(appID, version string) (*UploadStatus, error) {
			require.Equal(t, testAppID, appID)
			require.Equal(t, testVersion, version)
			return &expectedUploadStatus, nil
		},
		onDownloadSignedByURL: func(url string) ([]byte, error) {
			require.Equal(t, expectedURL, url)
			return []byte("test"), nil
		},
	}

	store := Store{API: mockAPI}
	actualFilename, err := store.downloadSigned(testAppID, testVersion)
	require.NoError(t, err)

	assert.Equal(t, expectedFilename, actualFilename)

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
