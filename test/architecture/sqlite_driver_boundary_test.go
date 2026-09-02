package architecture_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConcreteSQLiteDriverStaysInCLICompositionRoot(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	allowed := filepath.Clean(filepath.Join(root, "cmd", "dbprobe", "sqlite_history.go"))
	needle := []byte("modernc.org/" + "sqlite")

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if filepath.Ext(path) != ".go" || filepath.Clean(path) == allowed {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(content, needle) {
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			t.Errorf("concrete SQLite driver import escaped CLI composition root: %s", relative)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
