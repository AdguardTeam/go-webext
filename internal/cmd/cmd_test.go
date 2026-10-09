package cmd

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/adguardteam/go-webext/internal/edge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// fakeEdgeUpdateStore is a stub Edge store for the update command tests.
type fakeEdgeUpdateStore struct {
	result *edge.UploadStatusResponse
	err    error
}

// Update returns the configured result and error.
func (s *fakeEdgeUpdateStore) Update(
	appID, filepath string,
	options edge.UpdateOptions,
) (*edge.UploadStatusResponse, error) {
	return s.result, s.err
}

// fakeEdgePublishStore is a stub Edge store for the publish command tests.
type fakeEdgePublishStore struct {
	result *edge.PublishStatusResponse
	err    error
}

// Publish returns the configured result and error.
func (s *fakeEdgePublishStore) Publish(
	appID string,
	options edge.PublishOptions,
) (*edge.PublishStatusResponse, error) {
	return s.result, s.err
}

// newEdgeSkipFlags returns the skip-related flags read by the Edge update and
// publish commands.
func newEdgeSkipFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{Name: skipIfInProgressFlagName},
		&cli.StringFlag{Name: skipMarkerFlagName},
	}
}

// newEdgeTestContext builds a CLI context with the given flag values.
func newEdgeTestContext(t *testing.T, flags []cli.Flag, values map[string]string) *cli.Context {
	t.Helper()

	set := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range flags {
		require.NoError(t, f.Apply(set))
	}

	for name, value := range values {
		require.NoError(t, set.Set(name, value))
	}

	return cli.NewContext(cli.NewApp(), set, nil)
}

func TestRunEdgeUpdate(t *testing.T) {
	inProgressErr := &edge.InProgressSubmissionError{
		ID:        "operation-id",
		Message:   "Can't update extension as your extension submission is in progress.",
		ErrorCode: edge.ErrorCodeInProgressSubmission,
	}

	t.Run("skips and writes the marker with the flag", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
			skipMarkerFlagName:       markerPath,
		})

		err := runEdgeUpdate(ctx, &fakeEdgeUpdateStore{err: inProgressErr})
		require.NoError(t, err)

		content, err := os.ReadFile(markerPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "edge update skipped")
	})

	t.Run("skips without a marker when no path is given", func(t *testing.T) {
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
		})

		err := runEdgeUpdate(ctx, &fakeEdgeUpdateStore{err: inProgressErr})
		require.NoError(t, err)
	})

	t.Run("rejects a marker without the skip flag", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipMarkerFlagName: markerPath,
		})

		err := runEdgeUpdate(ctx, &fakeEdgeUpdateStore{})
		require.Error(t, err)
		assert.ErrorContains(t, err, skipIfInProgressFlagName)
	})

	t.Run("does not write the marker on success", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
			skipMarkerFlagName:       markerPath,
		})

		err := runEdgeUpdate(ctx, &fakeEdgeUpdateStore{result: &edge.UploadStatusResponse{}})
		require.NoError(t, err)
		assert.NoFileExists(t, markerPath)
	})

	t.Run("fails without the flag", func(t *testing.T) {
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), nil)

		err := runEdgeUpdate(ctx, &fakeEdgeUpdateStore{err: inProgressErr})
		require.Error(t, err)
		assert.ErrorContains(t, err, "updating extension")
	})

	t.Run("fails on other errors and does not write the marker", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
			skipMarkerFlagName:       markerPath,
		})

		err := runEdgeUpdate(ctx, &fakeEdgeUpdateStore{err: assert.AnError})
		require.Error(t, err)
		assert.ErrorContains(t, err, "updating extension")
		assert.NoFileExists(t, markerPath)
	})

	t.Run("fails when the marker cannot be written", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "missing-dir", "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
			skipMarkerFlagName:       markerPath,
		})

		err := runEdgeUpdate(ctx, &fakeEdgeUpdateStore{err: inProgressErr})
		require.Error(t, err)
		assert.ErrorContains(t, err, "writing skip marker")
	})
}

func TestRunEdgePublish(t *testing.T) {
	inProgressErr := &edge.InProgressSubmissionError{
		ID:        "operation-id",
		Message:   "Can't publish extension as your extension submission is in progress.",
		ErrorCode: edge.ErrorCodeInProgressSubmission,
	}

	t.Run("skips and writes the marker with the flag", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
			skipMarkerFlagName:       markerPath,
		})

		err := runEdgePublish(ctx, &fakeEdgePublishStore{err: inProgressErr})
		require.NoError(t, err)

		content, err := os.ReadFile(markerPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "edge publish skipped")
	})

	t.Run("skips without a marker when no path is given", func(t *testing.T) {
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
		})

		err := runEdgePublish(ctx, &fakeEdgePublishStore{err: inProgressErr})
		require.NoError(t, err)
	})

	t.Run("rejects a marker without the skip flag", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipMarkerFlagName: markerPath,
		})

		err := runEdgePublish(ctx, &fakeEdgePublishStore{})
		require.Error(t, err)
		assert.ErrorContains(t, err, skipIfInProgressFlagName)
	})

	t.Run("does not write the marker on success", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
			skipMarkerFlagName:       markerPath,
		})

		err := runEdgePublish(ctx, &fakeEdgePublishStore{result: &edge.PublishStatusResponse{}})
		require.NoError(t, err)
		assert.NoFileExists(t, markerPath)
	})

	t.Run("fails without the flag", func(t *testing.T) {
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), nil)

		err := runEdgePublish(ctx, &fakeEdgePublishStore{err: inProgressErr})
		require.Error(t, err)
		assert.ErrorContains(t, err, "publishing extension")
	})

	t.Run("fails on other errors and does not write the marker", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
			skipMarkerFlagName:       markerPath,
		})

		err := runEdgePublish(ctx, &fakeEdgePublishStore{err: assert.AnError})
		require.Error(t, err)
		assert.ErrorContains(t, err, "publishing extension")
		assert.NoFileExists(t, markerPath)
	})

	t.Run("fails when the marker cannot be written", func(t *testing.T) {
		markerPath := filepath.Join(t.TempDir(), "missing-dir", "skip-marker")
		ctx := newEdgeTestContext(t, newEdgeSkipFlags(), map[string]string{
			skipIfInProgressFlagName: "true",
			skipMarkerFlagName:       markerPath,
		})

		err := runEdgePublish(ctx, &fakeEdgePublishStore{err: inProgressErr})
		require.Error(t, err)
		assert.ErrorContains(t, err, "writing skip marker")
	})
}
