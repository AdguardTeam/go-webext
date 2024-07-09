package firefox_test

import (
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/adguardteam/go-webext/internal/firefox"
	"github.com/stretchr/testify/require"
)

const (
	testAppID      = "test_app_id"
	testVersion    = "0.0.3"
	testFilepath   = "testdata/extension.zip"
	testSourcepath = "testdata/source.zip"
	testVersionID  = 123456
	testUUID       = "test_uuid"
	testChannel    = firefox.ChannelListed
	testURL        = "https://example.org/firefox.xpi"
)

type MockAPI struct {
	firefox.API
	onStatus                func(appID string) (*firefox.StatusResponse, error)
	onCreateUpload          func(fileData io.Reader, channel firefox.Channel) (*firefox.UploadDetail, error)
	onUploadDetail          func(UUID string) (*firefox.UploadDetail, error)
	onCreateAddon           func(UUID string) (*firefox.AddonInfo, error)
	onAttachSourceToVersion func(appID, versionID string, sourceData io.Reader) error
	onCreateVersion         func(appID, UUID string) (*firefox.VersionInfo, error)
	onVersionDetail         func(appID, versionID string) (*firefox.VersionInfo, error)
	onDownloadSignedByURL   func(url string) ([]byte, error)
	onVersionsList          func(appID string) ([]*firefox.VersionInfo, error)
}

func (m *MockAPI) Status(appID string) (*firefox.StatusResponse, error) {
	return m.onStatus(appID)
}

func (m *MockAPI) CreateUpload(fileData io.Reader, channel firefox.Channel) (*firefox.UploadDetail, error) {
	return m.onCreateUpload(fileData, channel)
}

func (m *MockAPI) UploadDetail(UUID string) (*firefox.UploadDetail, error) {
	return m.onUploadDetail(UUID)
}

func (m *MockAPI) CreateAddon(UUID string) (*firefox.AddonInfo, error) {
	return m.onCreateAddon(UUID)
}

func (m *MockAPI) AttachSourceToVersion(appID, versionID string, sourceData io.Reader) error {
	return m.onAttachSourceToVersion(appID, versionID, sourceData)
}

func (m *MockAPI) CreateVersion(appID, UUID string) (*firefox.VersionInfo, error) {
	return m.onCreateVersion(appID, UUID)
}

func (m *MockAPI) VersionDetail(appID, versionID string) (*firefox.VersionInfo, error) {
	return m.onVersionDetail(appID, versionID)
}

func (m *MockAPI) DownloadSignedByURL(url string) ([]byte, error) {
	return m.onDownloadSignedByURL(url)
}

func (m *MockAPI) VersionsList(appID string) ([]*firefox.VersionInfo, error) {
	return m.onVersionsList(appID)
}

func TestStatus(t *testing.T) {
	expectedStatus := &firefox.StatusResponse{
		ID:             testAppID,
		CurrentVersion: testVersion,
	}

	mockAPI := &MockAPI{
		onStatus: func(appID string) (*firefox.StatusResponse, error) {
			require.Equal(t, testAppID, appID)
			return expectedStatus, nil
		},
	}
	store := firefox.Store{API: mockAPI}

	actualStatus, err := store.Status(testAppID)
	require.NoError(t, err)
	require.Equal(t, expectedStatus, actualStatus)
}

func TestInsert(t *testing.T) {
	mockAPI := &MockAPI{
		onCreateUpload: func(fileData io.Reader, c firefox.Channel) (*firefox.UploadDetail, error) {
			require.Equal(t, firefox.ChannelUnlisted, c)

			file, ok := fileData.(*os.File)
			require.True(t, ok, "fileData should be a file")

			actualPath := file.Name()

			require.Equal(t, testFilepath, actualPath)

			return &firefox.UploadDetail{
				UUID: testUUID,
			}, nil
		},
		onUploadDetail: func(UUID string) (*firefox.UploadDetail, error) {
			require.Equal(t, testUUID, UUID)

			return &firefox.UploadDetail{
				UUID:      testUUID,
				Processed: true,
				Valid:     true,
			}, nil
		},
		onCreateAddon: func(UUID string) (*firefox.AddonInfo, error) {
			require.Equal(t, testUUID, UUID)

			return &firefox.AddonInfo{
				ID: 0,
				Version: &firefox.VersionInfo{
					ID: testVersionID,
				},
			}, nil
		},
		onAttachSourceToVersion: func(appID, versionID string, sourceData io.Reader) error {
			require.Equal(t, testAppID, appID)
			require.Equal(t, strconv.Itoa(testVersionID), versionID)

			sourceFile, ok := sourceData.(*os.File)
			require.True(t, ok, "sourceData should be a file")
			actualPath := sourceFile.Name()
			require.Equal(t, testSourcepath, actualPath)

			return nil
		},
	}

	store := firefox.Store{API: mockAPI}

	err := store.Insert(testFilepath, testSourcepath)
	require.NoError(t, err)
}

func TestUpdate(t *testing.T) {
	mockAPI := &MockAPI{
		onCreateUpload: func(fileData io.Reader, c firefox.Channel) (*firefox.UploadDetail, error) {
			require.Equal(t, firefox.ChannelListed, c)

			file, ok := fileData.(*os.File)
			require.True(t, ok, "fileData should be a file")

			actualPath := file.Name()

			require.Equal(t, testFilepath, actualPath)

			return &firefox.UploadDetail{
				UUID: testUUID,
			}, nil
		},
		onUploadDetail: func(UUID string) (*firefox.UploadDetail, error) {
			require.Equal(t, testUUID, UUID)

			return &firefox.UploadDetail{
				UUID:      testUUID,
				Processed: true,
				Valid:     true,
			}, nil
		},
		onCreateVersion: func(appID, UUID string) (*firefox.VersionInfo, error) {
			require.Equal(t, testAppID, appID)
			require.Equal(t, testUUID, UUID)

			return &firefox.VersionInfo{
				ID: testVersionID,
			}, nil
		},
		onAttachSourceToVersion: func(appID, versionID string, sourceData io.Reader) error {
			require.Equal(t, testAppID, appID)
			require.Equal(t, strconv.Itoa(testVersionID), versionID)

			sourceFile, ok := sourceData.(*os.File)
			require.True(t, ok, "sourceData should be a file")
			actualPath := sourceFile.Name()
			require.Equal(t, testSourcepath, actualPath)

			return nil
		},
	}
	store := firefox.Store{API: mockAPI}

	err := store.Update(testFilepath, testSourcepath, testChannel)
	require.NoError(t, err)
}

func TestSign(t *testing.T) {
	expectedFilename := "firefox.xpi"

	mockAPI := &MockAPI{
		onCreateUpload: func(fileData io.Reader, channel firefox.Channel) (*firefox.UploadDetail, error) {
			file, ok := fileData.(*os.File)
			require.True(t, ok, "fileData should be a file")

			actualPath := file.Name()

			require.Equal(t, testFilepath, actualPath)
			// by default, the channel should be unlisted
			require.Equal(t, firefox.ChannelUnlisted, channel)

			return &firefox.UploadDetail{
				UUID: testUUID,
			}, nil
		},
		onUploadDetail: func(UUID string) (*firefox.UploadDetail, error) {
			require.Equal(t, testUUID, UUID)

			return &firefox.UploadDetail{
				UUID:      testUUID,
				Processed: true,
				Valid:     true,
			}, nil
		},
		onCreateVersion: func(appID, UUID string) (*firefox.VersionInfo, error) {
			require.Equal(t, testAppID, appID)
			require.Equal(t, testUUID, UUID)

			return &firefox.VersionInfo{
				ID: testVersionID,
			}, nil
		},
		onAttachSourceToVersion: func(appID, versionID string, sourceData io.Reader) error {
			require.Equal(t, testAppID, appID)
			require.Equal(t, strconv.Itoa(testVersionID), versionID)

			sourceFile, ok := sourceData.(*os.File)
			require.True(t, ok, "sourceData should be a file")
			actualPath := sourceFile.Name()
			require.Equal(t, testSourcepath, actualPath)

			return nil
		},
		onVersionDetail: func(appID, versionID string) (*firefox.VersionInfo, error) {
			require.Equal(t, testAppID, appID)
			require.Equal(t, strconv.Itoa(testVersionID), versionID)

			return &firefox.VersionInfo{
				File: firefox.FileInfo{
					Status: "public",
					URL:    testURL,
				},
			}, nil
		},
		onDownloadSignedByURL: func(url string) ([]byte, error) {
			require.Equal(t, testURL, url)

			return []byte(""), nil
		},
		onVersionsList: func(appID string) ([]*firefox.VersionInfo, error) {
			return []*firefox.VersionInfo{}, nil
		},
	}
	store := firefox.Store{API: mockAPI}

	err := store.Sign(testFilepath, testSourcepath, expectedFilename)
	require.NoError(t, err)

	// Check if the sourcefile exists.
	_, err = os.Stat(expectedFilename)
	require.NoError(t, err)

	// Remove the sourcefile after the test run.
	t.Cleanup(func() {
		err = os.Remove(expectedFilename)
		if err != nil {
			t.Error("Failed to remove sourcefile:", err)
		}
	})
}
