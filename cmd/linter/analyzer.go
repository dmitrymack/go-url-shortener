// Package main implements a project-specific static analyzer, runnable via
// golang.org/x/tools/go/analysis/singlechecker, that enforces two rules:
//
//  1. The built-in panic function must not be used anywhere in the analyzed
//     code. Panicking unwinds the stack without giving callers a chance to
//     handle the failure; this project reports errors instead.
//
//  2. In package main, log.Fatal and os.Exit must only be called directly
//     from func main. Both terminate the process immediately, skipping any
//     deferred cleanup — main is the one place in the call graph meant to
//     make that decision; helper functions should return an error instead
//     and let main decide whether to exit.
package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer reports uses of panic anywhere, and uses of log.Fatal/os.Exit
// outside func main in package main.
var Analyzer = &analysis.Analyzer{
	Name:     "noexitpanic",
	Doc:      "reports panic calls, and log.Fatal/os.Exit calls outside func main in package main",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}

		call := n.(*ast.CallExpr)
		checkPanic(pass, call)
		if pass.Pkg.Name() == "main" && !inMain(stack) {
			checkExit(pass, call)
		}
		return true
	})

	return nil, nil
}

// inMain reports whether the innermost enclosing named function on stack
// is func main with no receiver — i.e. the main function of package main.
// A func literal (closure) defined inside main does not change this: only
// the nearest ast.FuncDecl on the stack is consulted.
func inMain(stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		if fd, ok := stack[i].(*ast.FuncDecl); ok {
			return fd.Recv == nil && fd.Name.Name == "main"
		}
	}
	return false
}

// checkPanic reports call if it invokes the built-in panic function.
func checkPanic(pass *analysis.Pass, call *ast.CallExpr) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return
	}
	b, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok || b.Name() != "panic" {
		return
	}
	pass.Reportf(call.Pos(), "use of panic is forbidden, return an error instead")
}

// checkExit reports call if it invokes log.Fatal or os.Exit.
func checkExit(pass *analysis.Pass, call *ast.CallExpr) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil {
		return
	}

	switch {
	case fn.Pkg().Path() == "os" && fn.Name() == "Exit":
		pass.Reportf(call.Pos(), "os.Exit must only be called from func main")
	case fn.Pkg().Path() == "log" && fn.Name() == "Fatal":
		pass.Reportf(call.Pos(), "log.Fatal must only be called from func main")
	}
}
