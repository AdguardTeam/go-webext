package firefox_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/log"
	"github.com/adguardteam/go-webext/internal/firefox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	appID          = "test_app_id"
	version        = "0.0.3"
	testFilepath   = "testdata/extension.zip"
	testSourcepath = "testdata/source.zip"
	testVersionID  = "123456"
)

type MockAPI struct {
	mock.Mock
}

func (m *MockAPI) UploadStatus(appID, version string) (*firefox.UploadStatus, error) {
	args := m.Called(appID, version)
	return args.Get(0).(*firefox.UploadStatus), args.Error(1)
}

func (m *MockAPI) UploadSource(appID, versionID string, fileData io.Reader) error {
	args := m.Called(appID, versionID, fileData)
	return args.Error(0)
}

func (m *MockAPI) VersionID(appID, versionID string) (string, error) {
	args := m.Called(appID, versionID)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockAPI) UploadNew(fileData io.Reader) error {
	args := m.Called(fileData)
	return args.Error(0)
}

func (m *MockAPI) UploadUpdate(appID, versionID string, fileData io.Reader) error {
	args := m.Called(appID, versionID, fileData)
	return args.Error(0)
}

func (m *MockAPI) DownloadSignedByURL(url string) ([]byte, error) {
	args := m.Called(url)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockAPI) Status(appID string) (*firefox.StatusResponse, error) {
	args := m.Called(appID)
	return args.Get(0).(*firefox.StatusResponse), args.Error(1)
}

func TestInsert(t *testing.T) {
	mockAPI := &MockAPI{}
	store := firefox.Store{API: mockAPI}

	file, err := os.Open(filepath.Clean(testFilepath))
	require.NoError(t, err)
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	mockAPI.On("UploadNew", mock.MatchedBy(func(fileArg *os.File) bool {
		return fileArg.Name() == file.Name()
	})).Return(nil)

	expectedUploadStatus := &firefox.UploadStatus{
		GUID:             "test",
		Active:           false,
		AutomatedSigning: false,
		Files:            nil,
		PassedReview:     true,
		Pk:               "",
		Processed:        true,
		Reviewed:         true,
		URL:              "",
		Valid:            true,
		ValidationURL:    "",
		Version:          "",
	}

	mockAPI.On("UploadStatus", appID, version).Return(expectedUploadStatus, nil)
	mockAPI.On("VersionID", appID, version).Return(testVersionID, nil)

	sourceFile, err := os.Open(filepath.Clean(testSourcepath))
	defer func() { err = errors.WithDeferred(err, sourceFile.Close()) }()

	mockAPI.On("UploadSource", appID, testVersionID, mock.MatchedBy(func(fileArg *os.File) bool {
		return fileArg.Name() == sourceFile.Name()
	})).Return(nil)

	err = store.Insert(testFilepath, testSourcepath)
	require.NoError(t, err)
}

func TestUpdate(t *testing.T) {
	mockAPI := &MockAPI{}
	store := firefox.Store{API: mockAPI}

	file, err := os.Open(filepath.Clean(testFilepath))
	require.NoError(t, err)
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	mockAPI.On("UploadUpdate", appID, version, mock.MatchedBy(func(fileArg *os.File) bool {
		return fileArg.Name() == file.Name()
	})).Return(nil)

	expectedUploadStatus := &firefox.UploadStatus{
		GUID:             "test",
		Active:           false,
		AutomatedSigning: false,
		Files:            nil,
		PassedReview:     true,
		Pk:               "",
		Processed:        true,
		Reviewed:         true,
		URL:              "",
		Valid:            true,
		ValidationURL:    "",
		Version:          "",
	}

	mockAPI.On("UploadStatus", appID, version).Return(expectedUploadStatus, nil)

	mockAPI.On("VersionID", appID, version).Return(testVersionID, nil)

	mockAPI.On("UploadSource", appID, testVersionID, mock.MatchedBy(func(file *os.File) bool {
		return file.Name() == testSourcepath
	})).Return(nil)

	err = store.Update(testFilepath, testSourcepath)
	require.NoError(t, err)
}

func TestSign(t *testing.T) {
	testFilename := "test.xpi"
	testURL := "https://addons.mozilla.org/firefox/downloads/file/1234567/" + testFilename

	mockAPI := &MockAPI{}
	file, err := os.Open(filepath.Clean(testFilepath))
	defer func() { err = errors.WithDeferred(err, file.Close()) }()

	mockAPI.On("UploadUpdate", appID, version, mock.MatchedBy(func(fileArg *os.File) bool {
		log.Debug(fileArg.Name())
		log.Debug(file.Name())
		return fileArg.Name() == file.Name()
	})).Return(nil)

	expectedUploadStatus := &firefox.UploadStatus{
		GUID:             "test",
		Active:           true,
		AutomatedSigning: true,
		Files: []firefox.UploadStatusFiles{
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

	mockAPI.On("UploadStatus", appID, version).Return(expectedUploadStatus, nil)

	mockAPI.On("VersionID", appID, version).Return(testVersionID, nil)

	sourcefile, err := os.Open(filepath.Clean(testSourcepath))
	require.NoError(t, err)
	defer func() { err = errors.WithDeferred(err, sourcefile.Close()) }()

	mockAPI.On("UploadSource", appID, testVersionID, mock.MatchedBy(func(file *os.File) bool {
		return file.Name() == testSourcepath
	})).Return(nil)

	mockAPI.On("DownloadSignedByURL", testURL).Return([]byte(""), nil)

	store := firefox.Store{API: mockAPI}
	actualFilename, err := store.Sign(testFilepath, testSourcepath)
	require.NoError(t, err)

	assert.Equal(t, testFilename, actualFilename)

	// Check if the sourcefile exists.
	_, err = os.Stat(testFilename)
	require.NoError(t, err)

	// Remove the sourcefile after the test run.
	t.Cleanup(func() {
		err = os.Remove(testFilename)
		if err != nil {
			t.Error("Failed to remove sourcefile:", err)
		}
	})
}

func TestStatus(t *testing.T) {
	expectedStatus := &firefox.StatusResponse{
		ID:             appID,
		Status:         "status",
		CurrentVersion: version,
	}

	mockAPI := &MockAPI{}
	store := firefox.Store{API: mockAPI}

	mockAPI.On("Status", appID).Return(expectedStatus, nil)

	actualStatus, err := store.Status(appID)
	require.NoError(t, err)
	require.Equal(t, expectedStatus, actualStatus)
}
