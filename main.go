// Package main ...
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"github.com/dave/dst/dstutil"
)

func addMissingDocComments(file *dst.File) {
	dstutil.Apply(file, func(c *dstutil.Cursor) bool {
		node := c.Node()
		if node == nil {
			return true
		}

		switch n := node.(type) {
		case *dst.Field:
			n.Decs.Start.Clear()

		case *dst.FuncDecl:
			if c.Parent() == file && n.Name.IsExported() && len(n.Decs.Start) == 0 {
				doc := fmt.Sprintf("// %s [TODO:description]", n.Name.Name)
				n.Decs.Start.Prepend(doc)
			}

		case *dst.GenDecl:
			if c.Parent() != file {
				return true
			}
			isGroup := n.Lparen
			for _, spec := range n.Specs {
				switch s := spec.(type) {
				case *dst.TypeSpec:
					if s.Name.IsExported() {
						if isGroup {
							if len(s.Decs.Start) == 0 {
								doc := fmt.Sprintf("// %s [TODO:description]", s.Name.Name)
								s.Decs.Start.Prepend(doc)
							}
						} else {
							if len(n.Decs.Start) == 0 && len(s.Decs.Start) == 0 {
								doc := fmt.Sprintf("// %s [TODO:description]", s.Name.Name)
								n.Decs.Start.Prepend(doc)
							}
						}
					}
				case *dst.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() {
							if isGroup {
								if len(s.Decs.Start) == 0 {
									doc := fmt.Sprintf("// %s [TODO:description]", name.Name)
									s.Decs.Start.Prepend(doc)
								}
							} else {
								if len(n.Decs.Start) == 0 && len(s.Decs.Start) == 0 {
									doc := fmt.Sprintf("// %s [TODO:description]", name.Name)
									n.Decs.Start.Prepend(doc)
								}
							}
							break
						}
					}
				}
			}
		}

		return true
	}, nil)
}

func processPackage(filePaths []string) error {
	type parsedFile struct {
		path string
		file *dst.File
	}

	var files []parsedFile
	hasPkgDoc := false

	for _, path := range filePaths {
		src, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		f, err := decorator.Parse(src)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		if len(f.Decs.Start) > 0 || len(f.Decs.Package) > 0 {
			hasPkgDoc = true
		}

		files = append(files, parsedFile{path: path, file: f})
	}

	if len(files) == 0 {
		return nil
	}

	if !hasPkgDoc {
		first := files[0].file
		doc := fmt.Sprintf("// Package %s [TODO:description]", first.Name.Name)
		first.Decs.Start.Prepend(doc)
	}

	for _, pf := range files {
		addMissingDocComments(pf.file)

		fset, astFile, err := decorator.RestoreFile(pf.file)
		if err != nil {
			return fmt.Errorf("failed to restore DST to AST for file %s: %w", pf.path, err)
		}

		var buf bytes.Buffer
		if err := format.Node(&buf, fset, astFile); err != nil {
			return fmt.Errorf("failed to format modified AST for %s: %w", pf.path, err)
		}

		if err := os.WriteFile(pf.path, buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("failed to write updated file %s: %w", pf.path, err)
		}
	}

	return nil
}

func processDirectory(root string) error {
	pkgFiles := make(map[string][]string)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go") {
			dir := filepath.Dir(path)
			pkgFiles[dir] = append(pkgFiles[dir], path)
		}

		return nil
	})

	if err != nil {
		return err
	}

	for dir, files := range pkgFiles {
		fmt.Printf("Processing package directory %s...\n", dir)
		if err := processPackage(files); err != nil {
			return fmt.Errorf("error processing package in %s: %w", dir, err)
		}
	}

	return nil
}

func main() {
	targetDir := "."
	if len(os.Args) > 1 {
		targetDir = os.Args[1]
	}

	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Printf("Invalid directory path: %v\n", err)
		os.Exit(1)
	}

	if err := processDirectory(absPath); err != nil {
		fmt.Printf("Execution stopped: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully saved missing doc comments to disk.")
}
