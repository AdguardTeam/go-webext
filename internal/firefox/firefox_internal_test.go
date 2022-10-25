package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	appID   = "test_app_id"
	version = "0.0.3"
)

func TestExtDataFromFile(t *testing.T) {
	t.Run("application manifest", func(t *testing.T) {
		assert := assert.New(t)

		extData, err := extDataFromFile("testdata/extension.zip")
		require.NoError(t, err)

		assert.Equal(version, extData.version)
		assert.Equal(appID, extData.appID)
	})

	t.Run("browser specific manifest", func(t *testing.T) {
		assert := assert.New(t)

		extData, err := extDataFromFile("testdata/extension-browser-specific.zip")
		require.NoError(t, err)

		assert.Equal(version, extData.version)
		assert.Equal(appID, extData.appID)
	})
}
