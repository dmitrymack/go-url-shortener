// Command reset scans the module for structs annotated with a
// "// generate:reset" comment and generates a Reset() method for each one,
// following the rules from the assignment: primitives go back to their zero
// value, slices are truncated (s[:0]) rather than nilled, maps are cleared
// with the builtin clear(), and non-nil pointers and nested structs that
// have their own Reset() method reset their pointee/value in place.
//
// All generated methods for a package are written to that package's
// reset.gen.go file, overwriting any previous version.
//
// Usage: go run ./cmd/reset [patterns...]
// With no patterns, it scans "./..." relative to the current directory,
// i.e. every package in the module from its root down.
package main

import (
	"fmt"
	"go/types"
	"log"
	"os"

	"golang.org/x/tools/go/packages"
)

func main() {
	patterns := os.Args[1:]
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	if err := generate(".", patterns); err != nil {
		log.Fatal(err)
	}
}

// generate scans the packages matched by patterns (resolved relative to
// dir) for structs marked with the generate:reset directive and (re)writes
// each affected package's reset.gen.go file.
func generate(dir string, patterns []string) error {
	cfg := &packages.Config{
		Dir: dir,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return fmt.Errorf("loading packages: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("packages contain errors")
	}

	byPkg := findResettableStructs(pkgs)
	resettable := resettableSet(byPkg)

	for pkg, names := range byPkg {
		methods := make([]string, 0, len(names))
		for _, name := range names {
			obj := pkg.Types.Scope().Lookup(name)
			structType := obj.Type().(*types.Named).Underlying().(*types.Struct)
			methods = append(methods, generateReset(name, structType, resettable))
		}

		if err := writeResetFile(pkg, methods); err != nil {
			return fmt.Errorf("package %s: %w", pkg.PkgPath, err)
		}
	}

	return nil
}
