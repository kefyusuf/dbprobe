package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type rule struct {
	root      string
	forbidden []string
}

func TestDependencyBoundaries(t *testing.T) {
	engineDependencies := []string{
		"/adapters/",
		"go-sql-driver/mysql",
		"jackc/pgx",
		"mongo-driver",
		"gocql",
	}

	rules := []rule{
		{"../../internal/core", append(append([]string{}, engineDependencies...), "database/sql")},
		{"../../internal/app", append(append([]string{}, engineDependencies...), "database/sql")},
		{"../../internal/platform", engineDependencies},
		{"../../internal/surfaces", append(append([]string{}, engineDependencies...), "database/sql")},
		{"../../sdk", append(append(append([]string{}, engineDependencies...), "database/sql"), "/internal/")},
	}

	for _, r := range rules {
		if _, err := os.Stat(r.root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(r.root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range file.Imports {
				importPath, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				for _, forbidden := range r.forbidden {
					if strings.Contains(importPath, forbidden) {
						t.Errorf("%s imports forbidden dependency %q", path, importPath)
					}
				}
				if r.root == "../../internal/platform" && importPath == "database/sql" {
					relative, err := filepath.Rel(r.root, path)
					if err != nil {
						return err
					}
					if !strings.HasPrefix(filepath.ToSlash(relative), "sqlite/") {
						t.Errorf("%s imports database/sql outside internal/platform/sqlite", path)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
