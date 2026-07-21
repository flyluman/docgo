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
			n.Decs.NodeDecs.Start.Clear()

		case *dst.FuncDecl:
			if c.Parent() == file && n.Name.IsExported() && len(n.Decs.NodeDecs.Start) == 0 {
				doc := fmt.Sprintf("// %s [TODO:description]", n.Name.Name)
				n.Decs.NodeDecs.Start.Prepend(doc)
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
							if len(s.Decs.NodeDecs.Start) == 0 {
								doc := fmt.Sprintf("// %s [TODO:description]", s.Name.Name)
								s.Decs.NodeDecs.Start.Prepend(doc)
							}
						} else {
							if len(n.Decs.NodeDecs.Start) == 0 && len(s.Decs.NodeDecs.Start) == 0 {
								doc := fmt.Sprintf("// %s [TODO:description]", s.Name.Name)
								n.Decs.NodeDecs.Start.Prepend(doc)
							}
						}
					}
				case *dst.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() {
							if isGroup {
								if len(s.Decs.NodeDecs.Start) == 0 {
									doc := fmt.Sprintf("// %s [TODO:description]", name.Name)
									s.Decs.NodeDecs.Start.Prepend(doc)
								}
							} else {
								if len(n.Decs.NodeDecs.Start) == 0 && len(s.Decs.NodeDecs.Start) == 0 {
									doc := fmt.Sprintf("// %s [TODO:description]", name.Name)
									n.Decs.NodeDecs.Start.Prepend(doc)
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

func processFile(filePath string) error {
	src, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	f, err := decorator.Parse(src)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	addMissingDocComments(f)

	fset, astFile, err := decorator.RestoreFile(f)
	if err != nil {
		return fmt.Errorf("failed to restore DST to AST for file %s: %w", filePath, err)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, astFile); err != nil {
		return fmt.Errorf("failed to format modified AST for %s: %w", filePath, err)
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("failed to write updated file %s: %w", filePath, err)
	}

	return nil
}

func processDirectory(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
			fmt.Printf("Updating %s...\n", path)
			if err := processFile(path); err != nil {
				return fmt.Errorf("error processing %s: %w", path, err)
			}
		}

		return nil
	})
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
