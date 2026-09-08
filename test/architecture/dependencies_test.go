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
				if !sqliteDriverImportAllowed(path, importPath) {
					t.Errorf("%s imports modernc.org/sqlite outside internal/platform/sqlite/modernc", path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func sqliteDriverImportAllowed(path, importPath string) bool {
	if importPath != "modernc.org/sqlite" {
		return true
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	return strings.Contains(clean, "internal/platform/sqlite/modernc/")
}

func TestSQLiteDriverImportBoundary(t *testing.T) {
	cases := []struct {
		path       string
		importPath string
		allowed    bool
	}{
		{path: "internal/platform/sqlite/modernc/connector.go", importPath: "modernc.org/sqlite", allowed: true},
		{path: "internal/platform/sqlite/open.go", importPath: "modernc.org/sqlite", allowed: false},
		{path: "internal/app/diff/service.go", importPath: "modernc.org/sqlite", allowed: false},
		{path: "internal/core/temporal/store.go", importPath: "modernc.org/sqlite", allowed: false},
		{path: "internal/platform/sqlite/open.go", importPath: "database/sql", allowed: true},
	}
	for _, tc := range cases {
		if got := sqliteDriverImportAllowed(tc.path, tc.importPath); got != tc.allowed {
			t.Fatalf("path=%s import=%s got=%v want=%v", tc.path, tc.importPath, got, tc.allowed)
		}
	}
}
