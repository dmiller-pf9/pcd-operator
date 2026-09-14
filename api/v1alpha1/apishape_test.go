package v1alpha1

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// parseAPIPackage parses every non-generated, non-test source file in this
// package. parser.ParseFile is used rather than parser.ParseDir because the
// latter is deprecated and `make lint` rejects deprecated calls.
func parseAPIPackage(t *testing.T) (*token.FileSet, []*ast.File) {
	t.Helper()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		base := filepath.Base(p)
		if strings.HasSuffix(base, "_test.go") || strings.HasPrefix(base, "zz_generated.") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("parsed no source files")
	}
	return fset, files
}

// declaredTypes returns every type declared in the package, by name.
func declaredTypes(files []*ast.File) map[string]*ast.TypeSpec {
	out := map[string]*ast.TypeSpec{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					out[ts.Name.Name] = ts
				}
			}
		}
	}
	return out
}

// localRefs collects the names of package-local types referenced by expr.
// Struct *field names* are deliberately not visited — only field types — so a
// field named the same as a type cannot create a false edge.
func localRefs(expr ast.Expr, declared map[string]*ast.TypeSpec, out map[string]bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		if _, ok := declared[t.Name]; ok {
			out[t.Name] = true
		}
	case *ast.StarExpr:
		localRefs(t.X, declared, out)
	case *ast.ArrayType:
		localRefs(t.Elt, declared, out)
	case *ast.MapType:
		localRefs(t.Key, declared, out)
		localRefs(t.Value, declared, out)
	case *ast.StructType:
		for _, field := range t.Fields.List {
			localRefs(field.Type, declared, out)
		}
	case *ast.SelectorExpr:
		// A type from another package. Nothing local to record.
	}
}

// API-005: a type declared in api/ and reachable from no root Kind is dead API
// surface. `go vet` and golangci-lint will not find it, because exported types
// in a library package are never "unused".
func TestEveryDeclaredTypeIsReachableFromARootKind(t *testing.T) {
	_, files := parseAPIPackage(t)
	declared := declaredTypes(files)

	edges := map[string]map[string]bool{}
	for name, ts := range declared {
		refs := map[string]bool{}
		localRefs(ts.Type, declared, refs)
		edges[name] = refs
	}

	roots := []string{
		"PCDUnderlay", "PCDUnderlayList",
		"PCDInstallation", "PCDInstallationList",
		"PCDRegion", "PCDRegionList",
	}
	for _, r := range roots {
		if _, ok := declared[r]; !ok {
			t.Fatalf("root kind %s is not declared", r)
		}
	}

	reached := map[string]bool{}
	queue := append([]string{}, roots...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if reached[name] {
			continue
		}
		reached[name] = true
		for ref := range edges[name] {
			if !reached[ref] {
				queue = append(queue, ref)
			}
		}
	}

	for name := range declared {
		if !reached[name] {
			t.Errorf("type %s is declared in api/v1alpha1 but not reachable from any root Kind", name)
		}
	}
}

// A field without a json tag becomes a CRD property named after the Go field,
// which is capitalised and wrong, and nothing else catches it.
func TestEveryFieldHasALowerCamelJSONTag(t *testing.T) {
	fset, files := parseAPIPackage(t)

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range st.Fields.List {
				pos := fset.Position(field.Pos())
				if field.Tag == nil {
					t.Errorf("%s: %s has a field with no struct tag", pos, ts.Name.Name)
					continue
				}
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					t.Errorf("%s: unparsable struct tag %s", pos, field.Tag.Value)
					continue
				}
				jsonTag := reflect.StructTag(raw).Get("json")
				if jsonTag == "" {
					t.Errorf("%s: %s has a field with no json tag", pos, ts.Name.Name)
					continue
				}
				name := strings.Split(jsonTag, ",")[0]
				if name == "" {
					// Embedded/inline field, e.g. `json:",inline"`.
					continue
				}
				if c := name[0]; c >= 'A' && c <= 'Z' {
					t.Errorf("%s: %s json tag %q is not lowerCamelCase", pos, ts.Name.Name, name)
				}
			}
			return true
		})
	}
}
