package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/gopher-lua/ast"
	"github.com/yuin/gopher-lua/parse"
)

// allowedPluginGlobals is a second, independent line of defense (issue
// #284) — it mirrors what newSandboxedState leaves reachable at runtime,
// but is its own explicit list rather than derived from
// dangerousBaseGlobals, so it still catches something the runtime denylist
// ever misses.
//
// Trimmed to exactly what the four real shipped plugins use today (error,
// pcall, print, tostring, http, files) rather than preemptively allowing
// pairs/type/string/table just because they're safe — expand this only
// when a real plugin demonstrates an actual need.
var allowedPluginGlobals = map[string]bool{
	"error":    true,
	"pcall":    true,
	"print":    true,
	"tostring": true,
	"http":     true,
	"files":    true,
}

// pluginValidator walks a plugin's parsed AST. Global *reads* (calling a
// function, referencing a bare name) are checked against
// allowedPluginGlobals. Global *writes* (a plugin defining Salience,
// WhoAmI, OnTick, ...) are tracked in definedGlobals and always permitted —
// that's how a plugin attaches to a hook. Locals (function params, `local
// x = `) are tracked per lexical scope and always permitted.
type pluginValidator struct {
	definedGlobals map[string]bool
	locals         []map[string]bool // scope stack, innermost last
	errs           []string
}

func newPluginValidator() *pluginValidator {
	return &pluginValidator{
		definedGlobals: map[string]bool{},
		locals:         []map[string]bool{{}},
	}
}

func (v *pluginValidator) pushScope() { v.locals = append(v.locals, map[string]bool{}) }
func (v *pluginValidator) popScope()  { v.locals = v.locals[:len(v.locals)-1] }

func (v *pluginValidator) declareLocal(name string) {
	v.locals[len(v.locals)-1][name] = true
}

func (v *pluginValidator) isLocal(name string) bool {
	for i := len(v.locals) - 1; i >= 0; i-- {
		if v.locals[i][name] {
			return true
		}
	}
	return false
}

// checkRead runs on every IdentExpr used as a value — never on an
// assignment/declaration target, which go through declareLocal or
// definedGlobals instead.
func (v *pluginValidator) checkRead(id *ast.IdentExpr) {
	name := id.Value
	if v.isLocal(name) || v.definedGlobals[name] || allowedPluginGlobals[name] {
		return
	}
	v.errs = append(v.errs, fmt.Sprintf("line %d: reference to disallowed global %q", id.Line(), name))
}

// validatePluginSource parses src and walks it, without executing anything.
// Called from loadPlugins before DoFile — a malicious plugin's top-level
// code runs immediately on DoFile, before any hook function is ever
// called, so this is the only thing that catches it pre-execution.
func validatePluginSource(src []byte, name string) error {
	chunk, err := parse.Parse(bytes.NewReader(src), name)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	v := newPluginValidator()
	v.walkStmts(chunk)

	if len(v.errs) > 0 {
		return fmt.Errorf("plugin %s failed static validation:\n%s", name, strings.Join(v.errs, "\n"))
	}
	return nil
}

func (v *pluginValidator) walkStmts(stmts []ast.Stmt) {
	for _, s := range stmts {
		v.walkStmt(s)
	}
}

func (v *pluginValidator) walkStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {

	case *ast.LocalAssignStmt:
		// Rhs is evaluated before the names are declared — `local x = x`
		// reads the outer x, not itself.
		for _, e := range s.Exprs {
			v.walkExpr(e)
		}
		for _, n := range s.Names {
			v.declareLocal(n)
		}

	case *ast.AssignStmt:
		for _, e := range s.Rhs {
			v.walkExpr(e)
		}
		for _, e := range s.Lhs {
			// A bare global identifier target (Salience = 1) is a
			// definition, not a read. Anything more complex (a table
			// field, say) still needs its base object checked.
			if id, ok := e.(*ast.IdentExpr); ok && !v.isLocal(id.Value) {
				v.definedGlobals[id.Value] = true
				continue
			}
			v.walkExpr(e)
		}

	case *ast.FuncDefStmt:
		// `function OnTick(entry) ... end` defines a global.
		if id, ok := s.Name.Func.(*ast.IdentExpr); ok {
			v.definedGlobals[id.Value] = true
		}
		v.walkFunctionExpr(s.Func)

	case *ast.FuncCallStmt:
		v.walkExpr(s.Expr)

	case *ast.IfStmt:
		v.walkExpr(s.Condition)
		v.pushScope()
		v.walkStmts(s.Then)
		v.popScope()
		v.pushScope()
		v.walkStmts(s.Else)
		v.popScope()

	case *ast.ReturnStmt:
		for _, e := range s.Exprs {
			v.walkExpr(e)
		}

	case *ast.DoBlockStmt:
		v.pushScope()
		v.walkStmts(s.Stmts)
		v.popScope()

	case *ast.NumberForStmt:
		v.walkExpr(s.Init)
		v.walkExpr(s.Limit)
		if s.Step != nil {
			v.walkExpr(s.Step)
		}
		v.pushScope()
		v.declareLocal(s.Name)
		v.walkStmts(s.Stmts)
		v.popScope()

	case *ast.GenericForStmt:
		for _, e := range s.Exprs {
			v.walkExpr(e)
		}
		v.pushScope()
		for _, n := range s.Names {
			v.declareLocal(n)
		}
		v.walkStmts(s.Stmts)
		v.popScope()

	case *ast.WhileStmt:
		v.walkExpr(s.Condition)
		v.pushScope()
		v.walkStmts(s.Stmts)
		v.popScope()

	case *ast.RepeatStmt:
		v.pushScope()
		v.walkStmts(s.Stmts)
		v.walkExpr(s.Condition) // condition sees the loop body's locals in Lua
		v.popScope()

	case *ast.BreakStmt, *ast.LabelStmt, *ast.GotoStmt:
		// nothing to check

	default:
		v.errs = append(v.errs, fmt.Sprintf("unrecognized statement type %T", stmt))
	}
}

func (v *pluginValidator) walkFunctionExpr(fn *ast.FunctionExpr) {
	v.pushScope()
	for _, p := range fn.ParList.Names {
		v.declareLocal(p)
	}
	v.walkStmts(fn.Stmts)
	v.popScope()
}

func (v *pluginValidator) walkExpr(expr ast.Expr) {
	switch e := expr.(type) {

	case *ast.IdentExpr:
		v.checkRead(e)

	case *ast.AttrGetExpr:
		// obj.key / obj[key] — check the base object (the `http` in
		// `http.get(...)`), not a literal string key like .get.
		v.walkExpr(e.Object)
		if _, isString := e.Key.(*ast.StringExpr); !isString {
			v.walkExpr(e.Key)
		}

	case *ast.FuncCallExpr:
		// Func is nil for a method call (obj:method(...)); Receiver is
		// nil for a plain call (fn(...)) — never both set.
		if e.Func != nil {
			v.walkExpr(e.Func)
		}
		if e.Receiver != nil {
			v.walkExpr(e.Receiver)
		}
		for _, a := range e.Args {
			v.walkExpr(a)
		}

	case *ast.TableExpr:
		for _, f := range e.Fields {
			if f.Key != nil {
				v.walkExpr(f.Key)
			}
			v.walkExpr(f.Value)
		}

	case *ast.FunctionExpr:
		v.walkFunctionExpr(e)

	case *ast.StringConcatOpExpr:
		v.walkExpr(e.Lhs)
		v.walkExpr(e.Rhs)
	case *ast.ArithmeticOpExpr:
		v.walkExpr(e.Lhs)
		v.walkExpr(e.Rhs)
	case *ast.RelationalOpExpr:
		v.walkExpr(e.Lhs)
		v.walkExpr(e.Rhs)
	case *ast.LogicalOpExpr:
		v.walkExpr(e.Lhs)
		v.walkExpr(e.Rhs)
	case *ast.UnaryMinusOpExpr:
		v.walkExpr(e.Expr)
	case *ast.UnaryNotOpExpr:
		v.walkExpr(e.Expr)
	case *ast.UnaryLenOpExpr:
		v.walkExpr(e.Expr)

	case *ast.StringExpr, *ast.NumberExpr, *ast.TrueExpr, *ast.FalseExpr, *ast.NilExpr, *ast.Comma3Expr:
		// literals / varargs — nothing to check

	default:
		v.errs = append(v.errs, fmt.Sprintf("unrecognized expression type %T", expr))
	}
}
