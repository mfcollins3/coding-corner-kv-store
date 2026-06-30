package kvstore

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSTablesFailsIfManifestCannotBeOpened(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := path.Join(tempDir, "MANIFEST")
	manifest, err := newManifest(manifestPath)
	assert.NoError(t, err)

	orig := openRead
	t.Cleanup(func() { openRead = orig })
	openRead = func(name string) (*os.File, error) {
		return nil, fmt.Errorf("open error")
	}
	_, err = manifest.sstables()
	assert.Error(t, err)
}

func TestSSTablesReturnsSSTableFiles(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := path.Join(tempDir, "MANIFEST")
	{
		file, err := os.Create(manifestPath)
		assert.NoError(t, err)
		defer func() {
			_ = file.Close()
		}()

		_, _ = fmt.Fprintln(file, "sst-1.json")
		_, _ = fmt.Fprintln(file, "sst-2.json")
		_, _ = fmt.Fprintln(file, "sst-3.json")
	}

	manifest, err := openManifest(manifestPath)
	assert.NoError(t, err)
	sstables, err := manifest.sstables()

	assert.NoError(t, err)
	assert.Equal(t, 3, len(sstables))
	expected := []string{"sst-3.json", "sst-2.json", "sst-1.json"}
	assert.Equal(t, expected, sstables)
}

func TestNewManifestFailsIfManifestCannotBeCreated(t *testing.T) {
	orig := openFile
	t.Cleanup(func() { openFile = orig })
	openFile = func(
		name string,
		flag int,
		perm os.FileMode,
	) (*os.File, error) {
		return nil, fmt.Errorf("create error")
	}
	tempDir := t.TempDir()
	manifestPath := path.Join(tempDir, "MANIFEST")

	_, err := newManifest(manifestPath)

	assert.Error(t, err)
}

func TestOpenManifestFailsIfSSTableDataIsInvalid(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := path.Join(tempDir, "MANIFEST")
	{
		file, err := os.Create(manifestPath)
		assert.NoError(t, err)
		defer func() {
			_ = file.Close()
		}()

		_, _ = fmt.Fprintln(file, "sst-1.json")
		_, _ = fmt.Fprintln(file, "sst-fail.json")
		_, _ = fmt.Fprintln(file, "sst-3.json")
	}

	_, err := openManifest(manifestPath)
	assert.Error(t, err)
}
