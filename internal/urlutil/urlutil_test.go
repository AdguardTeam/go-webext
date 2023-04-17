package urlutil_test

import (
	"testing"

	"github.com/adguardteam/go-webext/internal/urlutil"
	"github.com/stretchr/testify/assert"
)

func TestAuthorize(t *testing.T) {
	assert.Equal(t, "/app/version/", urlutil.JoinPath("/", "app", "version", "/"))
	assert.Equal(t, "/app/version", urlutil.JoinPath("app", "version"))
}
