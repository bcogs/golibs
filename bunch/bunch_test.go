package bunch

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bcogs/golibs/oil"
	"github.com/stretchr/testify/require"
)

func TestNewBunch(t *testing.T) {
	t.Parallel()
	require.Error(t, oil.Second(NewBunch("/noexist/noexist/noexist", &Options{})))
	// no need to test the happy path, because other tests use it
}

type failingReader struct{}

func (fr failingReader) Read(p []byte) (int, error) { return 0, fmt.Errorf("injected error") }

func TestWriteThenRead(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	b, err := NewBunch(tmp, &Options{})
	require.NoError(t, err)
	// happy path
	for _, dir := range []string{"", "dir", "dir", "dir,subdir"} {
		relDir := strings.Split(dir, ",")
		for _, f := range []string{"file1", "file2"} {
			relFile := append(relDir, f)
			p := b.Path(relFile)
			require.Equal(t, filepath.Join(append([]string{tmp}, relFile...)...), p, relFile)
			require.NoError(t, b.Write(relFile, strings.NewReader("blah")), relFile)
			content, err := os.ReadFile(p)
			require.NoError(t, err, relFile)
			require.Equal(t, "blah", string(content), relFile)
		}
	}
	// error path
	for _, tc := range []struct {
		path        string
		reader      io.Reader
		errContains string
	}{
		{"..,invalid,path", strings.NewReader("baz"), ".."},
		{"dir,fail", failingReader{}, filepath.Join(tmp, "dir", ".tmpfail")},
		{"dir,subdir", strings.NewReader("blah"), filepath.Join(tmp, "dir", ".tmpsubdir")},
	} {
		relPath := strings.Split(tc.path, ",")
		require.ErrorContains(t, b.Write(relPath, tc.reader), tc.errContains)
	}
	// check there's no leftover garbage
	m := map[string]bool{
		tmp:                                          true,
		filepath.Join(tmp, "dir"):                    true,
		filepath.Join(tmp, "file1"):                  true,
		filepath.Join(tmp, "file2"):                  true,
		filepath.Join(tmp, "dir", "file1"):           true,
		filepath.Join(tmp, "dir", "file2"):           true,
		filepath.Join(tmp, "dir", "subdir"):          true,
		filepath.Join(tmp, "dir", "subdir", "file1"): true,
		filepath.Join(tmp, "dir", "subdir", "file2"): true,
	}
	require.NoError(t, filepath.Walk(tmp, func(path string, _ fs.FileInfo, err error) error {
		require.NoError(t, err)
		require.True(t, m[path], path)
		return nil
	}))
}

func TestTmpFilePath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	for _, tc := range []struct{ relPath, expectedDir string }{
		{"foo", tmp},
		{"foo,bar", filepath.Join(tmp, "foo")},
		{"foo,bar,baz", filepath.Join(tmp, "foo", "bar")},
	} {
		b, err := NewBunch(tmp, &Options{})
		require.NoError(t, err, tc)
		relPath := strings.Split(tc.relPath, ",")
		dir, base := b.tmpFilePath(relPath)
		require.Equal(t, tc.expectedDir, strings.TrimRight(dir, string(filepath.Separator)), tc)
		require.Equal(t, ".tmp"+relPath[len(relPath)-1], base, tc)
	}
}

func TestValidateRelPath(t *testing.T) {
	t.Parallel()
	require.Error(t, ValidateRelPath([]string{}))
	for _, tc := range []struct {
		relPath string
		valid   bool
	}{
		{"foo", true},
		{".", false},
		{"..", false},
		{"..,foo", false},
		{"foo,..", false},
		{"foo,bar", true},
		{"foo,..,bar", true},
		{"foo,.,bar", true},
		{".,bar", true},
		{".,..,bar", false},
		{"fo/o", false},
		{"fo\\o", false},
		{"fo\x01o", false},
		{".foo", false},
		{"foo,.bar", false},
	} {
		oil.If(tc.valid, require.NoError, require.Error)(t, ValidateRelPath(strings.Split(tc.relPath, ",")), tc)
	}
}
