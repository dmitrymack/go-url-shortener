package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// marker is the comment directive that flags a struct for Reset() generation.
const marker = "generate:reset"

// findResettableStructs scans every package's syntax trees for struct types
// preceded by the marker comment and returns, per package, the (sorted)
// names of the structs found.
func findResettableStructs(pkgs []*packages.Package) map[*packages.Package][]string {
	byPkg := make(map[*packages.Package][]string)

	for _, pkg := range pkgs {
		var names []string

		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}

				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if _, ok := typeSpec.Type.(*ast.StructType); !ok {
						continue
					}

					doc := typeSpec.Doc
					if doc == nil && len(genDecl.Specs) == 1 {
						doc = genDecl.Doc
					}
					if !hasMarker(doc) {
						continue
					}

					names = append(names, typeSpec.Name.Name)
				}
			}
		}

		if len(names) > 0 {
			sort.Strings(names)
			byPkg[pkg] = names
		}
	}

	return byPkg
}

// hasMarker reports whether doc contains a line equal to marker, ignoring
// the leading "// " that go/ast keeps as part of each line's text.
func hasMarker(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, line := range strings.Split(doc.Text(), "\n") {
		if strings.TrimSpace(line) == marker {
			return true
		}
	}
	return false
}

// resettableSet collects the type objects of every struct found by
// findResettableStructs, across all packages, so that resetStmts can
// recognize a field whose type will get a generated Reset() method even
// before that method actually exists in pkg.Types (it's being generated in
// this same run).
func resettableSet(byPkg map[*packages.Package][]string) map[types.Object]bool {
	set := make(map[types.Object]bool)
	for pkg, names := range byPkg {
		for _, name := range names {
			if obj := pkg.Types.Scope().Lookup(name); obj != nil {
				set[obj] = true
			}
		}
	}
	return set
}

// generateReset renders the Reset() method body for a struct named
// typeName with fields structType. resettable is the set of type objects
// that either already have a Reset() method or will get one generated in
// this run.
func generateReset(typeName string, structType *types.Struct, resettable map[types.Object]bool) string {
	recv := strings.ToLower(typeName[:1])

	var stmts []string
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		expr := recv + "." + field.Name()
		stmts = append(stmts, resetStmts(expr, field.Type(), resettable)...)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "func (%s *%s) Reset() {\n", recv, typeName)
	fmt.Fprintf(&b, "\tif %s == nil {\n\t\treturn\n\t}\n", recv)
	if len(stmts) > 0 {
		b.WriteString("\n")
		for _, s := range stmts {
			b.WriteString(indent(s, "\t"))
			b.WriteString("\n")
		}
	}
	b.WriteString("}\n")

	return b.String()
}

// resetStmts returns the statements that reset expr (an addressable
// expression of type t) to its zero state, per the assignment's rules. It
// recurses through pointers so a pointer to a slice/map/resettable struct
// is handled too. A type it doesn't know how to reset (a struct with no
// Reset() method, a channel, an interface, ...) yields no statements.
func resetStmts(expr string, t types.Type, resettable map[types.Object]bool) []string {
	if ptr, ok := t.(*types.Pointer); ok {
		var inner []string
		if named, ok := ptr.Elem().(*types.Named); ok && isResettable(named, resettable) {
			inner = []string{expr + ".Reset()"}
		} else {
			inner = resetStmts("*"+expr, ptr.Elem(), resettable)
		}
		if len(inner) == 0 {
			return nil
		}

		stmts := []string{"if " + expr + " != nil {"}
		for _, s := range inner {
			stmts = append(stmts, indent(s, "\t"))
		}
		return append(stmts, "}")
	}

	if named, ok := t.(*types.Named); ok && isResettable(named, resettable) {
		return []string{expr + ".Reset()"}
	}

	switch u := t.Underlying().(type) {
	case *types.Basic:
		return []string{expr + " = " + zeroLiteral(u)}
	case *types.Slice:
		return []string{expr + " = " + expr + "[:0]"}
	case *types.Map:
		return []string{"clear(" + expr + ")"}
	default:
		// Struct without Reset(), interface, channel, func, array: left
		// untouched, there's no generic way to zero it in place.
		return nil
	}
}

// isResettable reports whether named already has a Reset() method, or is
// itself marked generate:reset and so will get one generated in this run.
func isResettable(named *types.Named, resettable map[types.Object]bool) bool {
	if resettable[named.Obj()] {
		return true
	}
	return hasResetMethod(named)
}

// hasResetMethod reports whether *named has a method "Reset()" with no
// parameters and no results — the shape the generated code relies on to
// reset nested values in place.
func hasResetMethod(named *types.Named) bool {
	obj, _, _ := types.LookupFieldOrMethod(types.NewPointer(named), true, nil, "Reset")
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}

	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}
	return sig.Params().Len() == 0 && sig.Results().Len() == 0
}

// zeroLiteral returns the Go source for the zero value of a basic type.
func zeroLiteral(b *types.Basic) string {
	switch {
	case b.Info()&types.IsBoolean != 0:
		return "false"
	case b.Info()&types.IsString != 0:
		return `""`
	case b.Kind() == types.UnsafePointer:
		return "nil"
	default:
		return "0"
	}
}

// indent prefixes every line of s with prefix.
func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// writeResetFile writes the generated Reset() methods for pkg to its
// reset.gen.go file, overwriting any previous version.
func writeResetFile(pkg *packages.Package, methods []string) error {
	if len(pkg.GoFiles) == 0 {
		return fmt.Errorf("no files to determine package directory")
	}
	dir := filepath.Dir(pkg.GoFiles[0])

	var b strings.Builder
	b.WriteString("// Code generated by cmd/reset; DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "package %s\n\n", pkg.Name)
	for i, m := range methods {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(m)
	}

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return fmt.Errorf("formatting generated source: %w", err)
	}

	return os.WriteFile(filepath.Join(dir, "reset.gen.go"), src, 0666)
}
