// Package bunch simplifies the management of large number of files in a directory, by making the sharding less tedious.
//
// The key concepts are the Bunch, which represents the directory and all its content, and relative paths, which are slices of strings representing paths relative to the directory, sharded in subdirectories.  The user of the package remains in control of the sharding.
//
// Example use, to create a million files in subdirectories:
//
//	b, err := bunch.NewBunch("/path/of/the/root", &bunch.Options{})
//	if err != nil { panic(err) }
//	for i := 0; i < 1000 * 1000; i++ {
//	  // create /path/of/the/root/XX/YY/ZZ.txt
//	  relativePath := []string{
//	    fmt.Sprintf("%02d", i / 10000),
//	    fmt.Sprintf("%02d", (i / 100) % 100),
//	    fmt.Sprintf("%02d.txt", i % 100),
//	  }
//	  if err = b.Write(relativePath, strings.NewReader("hello")); err != nil { panic(err) }
//	}
//	// read /path/of/the/root/03/04/05.txt
//	os.ReadFile(b.Path([]string{"03", "04", "05.txt"}))
package bunch

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Bunch represents a directory and the bunch of files it contains.
type Bunch struct {
	Root string // root directory of the Bunch
}

// Options contains possible options when instantiating a Bunch.
type Options struct{}

// NewBunch creates a new Bunch.  The root directory must exist.
func NewBunch(root string, o *Options) (*Bunch, error) {
	fi, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%q isn't a directory", fi.Name())
	}
	return &Bunch{Root: root}, nil
}

// Path gives a usable file path, given its relative path.
func (b *Bunch) Path(relPath []string) string {
	return b.Root + string(os.PathSeparator) + filepath.Join(relPath...)
}

// Walk enumerates all the files of the Bunch.  It has the same semantics as filepath.WalkDir, except it skips all directories and returns only files.
// The callback is called with a path that starts with the bunch root.
// All temporary or garbage files, whose name start with a dot, are skipped.
func (b *Bunch) Walk(fn fs.WalkDirFunc) error {
	return filepath.WalkDir(b.Root, func(path string, de fs.DirEntry, err error) error {
		if de.IsDir() {
			return nil
		}
		if k := bytes.LastIndexByte([]byte(path), filepath.Separator); k >= 0 && []byte(path)[k+1] == '.' {
			return nil
		}
		return fn(path, de, err)
	})
}

// Write creates (or overwrites) a file with the content of a reader, creating all needed subdirectories.
// The write is done atomically by writing a temporary file and renaming it.
// The relative path must be valid (see ValidateRelPath).
func (b *Bunch) Write(relPath []string, reader io.Reader) error {
	var err error
	if err = ValidateRelPath(relPath); err != nil {
		return fmt.Errorf("invalid relative path to %s - %w", b.Root, err)
	}
	var f *os.File
	createDir := true
	tmpFileDir, tmpFileBase := b.tmpFilePath(relPath)
	for {
		f, err = os.CreateTemp(tmpFileDir, tmpFileBase)
		if err != nil {
			if createDir && errors.Is(err, os.ErrNotExist) {
				createDir = false
				if err = os.MkdirAll(tmpFileDir, 0777); err != nil {
					return fmt.Errorf("creating directory failed - %w", err)
				}
				continue
			}
			return fmt.Errorf("creating temporary file failed - %w", err)
		}
		break
	}
	defer f.Close()
	if _, err = io.Copy(f, reader); err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("writing to temporary file %s failed - %w", f.Name(), err)
	}
	err = os.Rename(f.Name(), b.Path(relPath))
	if err != nil {
		os.Remove(f.Name())
		return fmt.Errorf("renaming temporary file failed - %w", err)
	}
	return nil
}

func (b *Bunch) tmpFilePath(relPath []string) (string, string) {
	dir := b.Path(relPath[:len(relPath)-1])
	return dir, ".tmp" + relPath[len(relPath)-1]
}

// ValidateRelPath verifies that a relative path is valide for use in a Bunch.
// The path components mustn't result in escaping the root, mustn't contain too exotic characters, and mustn't start with a dot (except "." and "..").
func ValidateRelPath(rp []string) error {
	depth := 0
	for _, s := range rp {
		switch s {
		case ".":
			break
		case "..":
			depth--
			if depth < 0 {
				return fmt.Errorf("%q has too many \"..\" and escapes the root", rp)
			}
		default:
			if len(s) > 0 && s[0] == '.' {
				return fmt.Errorf("%q has a subdirectory that starts with a dot", rp)
			}
			depth++
		}
		for _, c := range []byte(s) {
			if c < 0x2b || c == '/' || c == '\\' || c > 0x7e {
				return fmt.Errorf("%q contains invalid character %q", rp, string(c))
			}
		}
	}
	if depth <= 0 {
		return fmt.Errorf("%q is a relative path with 0 depth", rp)
	}
	return nil
}
