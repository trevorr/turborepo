package globwatcher

import (
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/hashicorp/go-hclog"
	"github.com/vercel/turborepo/cli/internal/fs"
	"gotest.tools/v3/assert"
)

func setup(t *testing.T, repoRoot fs.AbsolutePath) {
	// Directory layout:
	// <repoRoot>/
	//   my-pkg/
	//     irrelevant
	//     dist/
	//       dist-file
	//       distChild/
	//         child-file
	//     .next/
	//       next-file
	distPath := repoRoot.Join("my-pkg", "dist")
	childFilePath := distPath.Join("distChild", "child-file")
	err := childFilePath.EnsureDir()
	assert.NilError(t, err, "EnsureDir")
	f, err := childFilePath.Create()
	assert.NilError(t, err, "Create")
	err = f.Close()
	assert.NilError(t, err, "Close")
	distFilePath := repoRoot.Join("my-pkg", "dist", "dist-file")
	f, err = distFilePath.Create()
	assert.NilError(t, err, "Create")
	err = f.Close()
	assert.NilError(t, err, "Close")
	nextFilePath := repoRoot.Join("my-pkg", ".next", "next-file")
	err = nextFilePath.EnsureDir()
	assert.NilError(t, err, "EnsureDir")
	f, err = nextFilePath.Create()
	assert.NilError(t, err, "Create")
	err = f.Close()
	assert.NilError(t, err, "Close")
	irrelevantPath := repoRoot.Join("my-pkg", "irrelevant")
	f, err = irrelevantPath.Create()
	assert.NilError(t, err, "Create")
	err = f.Close()
	assert.NilError(t, err, "Close")
}

func TestTrackOutputs(t *testing.T) {
	logger := hclog.Default()

	repoRootRaw := t.TempDir()
	repoRoot := fs.AbsolutePathFromUpstream(repoRootRaw)

	setup(t, repoRoot)

	globWatcher := New(logger, repoRoot)

	globs := []string{
		"my-pkg/dist/**",
		"my-pkg/.next/**",
	}
	hash := "the-hash"
	err := globWatcher.WatchGlobs(hash, globs)
	assert.NilError(t, err, "WatchGlobs")

	changed, err := globWatcher.GetChangedGlobs(hash, globs)
	assert.NilError(t, err, "GetChangedGlobs")
	assert.Equal(t, 0, len(changed), "Expected no changed paths")

	// Make an irrelevant change
	globWatcher.OnFileWatchEvent(fsnotify.Event{
		Op:   fsnotify.Create,
		Name: repoRoot.Join("my-pkg", "irrelevant").ToString(),
	})

	changed, err = globWatcher.GetChangedGlobs(hash, globs)
	assert.NilError(t, err, "GetChangedGlobs")
	assert.Equal(t, 0, len(changed), "Expected no changed paths")

	// Make a relevant change
	globWatcher.OnFileWatchEvent(fsnotify.Event{
		Op:   fsnotify.Create,
		Name: repoRoot.Join("my-pkg", "dist", "foo").ToString(),
	})

	changed, err = globWatcher.GetChangedGlobs(hash, globs)
	assert.NilError(t, err, "GetChangedGlobs")
	assert.Equal(t, 1, len(changed), "Expected one changed path remaining")
	expected := "my-pkg/dist/**"
	assert.Equal(t, expected, changed[0], "Expected dist glob to have changed")

	// Change a file matching the other glob
	globWatcher.OnFileWatchEvent(fsnotify.Event{
		Op:   fsnotify.Create,
		Name: repoRoot.Join("my-pkg", ".next", "foo").ToString(),
	})
	// We should no longer be watching anything, since both globs have
	// registered changes
	if len(globWatcher.hashGlobs) != 0 {
		t.Errorf("expected to not track any hashes, found %v", globWatcher.hashGlobs)
	}

	// Both globs have changed, we should have stopped tracking
	// this hash
	changed, err = globWatcher.GetChangedGlobs(hash, globs)
	assert.NilError(t, err, "GetChangedGlobs")
	assert.DeepEqual(t, globs, changed)
}

func TestWatchSingleFile(t *testing.T) {
	logger := hclog.Default()

	repoRoot := fs.AbsolutePathFromUpstream(t.TempDir())

	setup(t, repoRoot)

	//watcher := newTestWatcher()
	globWatcher := New(logger, repoRoot)
	globs := []string{
		"my-pkg/.next/next-file",
	}
	hash := "the-hash"
	err := globWatcher.WatchGlobs(hash, globs)
	assert.NilError(t, err, "WatchGlobs")

	assert.Equal(t, 1, len(globWatcher.hashGlobs))

	// A change to an irrelevant file
	globWatcher.OnFileWatchEvent(fsnotify.Event{
		Op:   fsnotify.Create,
		Name: repoRoot.Join("my-pkg", ".next", "foo").ToString(),
	})
	assert.Equal(t, 1, len(globWatcher.hashGlobs))

	// Change the watched file
	globWatcher.OnFileWatchEvent(fsnotify.Event{
		Op:   fsnotify.Write,
		Name: repoRoot.Join("my-pkg", ".next", "next-file").ToString(),
	})
	assert.Equal(t, 0, len(globWatcher.hashGlobs))
}
