package lintleak

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// isLeakyConstructor reports whether a call is one whose string arguments reach
// a user. Matching is by callee name rather than by resolved type: the linter
// runs on source that may not compile yet, so it cannot depend on type
// information, and the name set below is small enough that a false match is
// cheaper than a miss.
func isLeakyConstructor(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	name := sel.Sel.Name

	// Event recorder methods, on any receiver.
	switch name {
	case "Event", "Eventf", "AnnotatedEventf":
		return true
	}

	// Error constructors, qualified by package name.
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	switch {
	case pkg.Name == "fmt" && (name == "Errorf" || name == "Sprintf"):
		return true
	case pkg.Name == "errors" && name == "New":
		return true
	}
	return false
}

// ScanGoSource reports banned terms in string literals passed to error and event
// constructors in src. It takes source text rather than a path so tests can use
// fixture strings — a fixture file containing banned words would otherwise be
// caught by the linter's own tree scan.
func ScanGoSource(path, src string) ([]Finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var found []Finding
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isLeakyConstructor(call) {
			return true
		}
		for _, arg := range call.Args {
			lit, ok := arg.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			text, err := strconv.Unquote(lit.Value)
			if err != nil {
				text = lit.Value
			}
			line := fset.Position(lit.Pos()).Line
			for _, term := range terms {
				if term.Pattern.MatchString(text) {
					found = append(found, Finding{
						Path: path,
						Line: line,
						Term: term.Name,
						Why:  term.Why,
						Text: text,
					})
				}
			}
		}
		return true
	})
	return found, nil
}

// ScanGoTree runs ScanGoSource over every non-test Go file under root.
func ScanGoTree(root string) ([]Finding, error) {
	var found []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || skipFile(d.Name()) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := ScanGoSource(path, string(b))
		if err != nil {
			// A file that does not parse is a compile error, not a leak. Let
			// `go build` report it rather than failing here with a worse message.
			return nil
		}
		found = append(found, f...)
		return nil
	})
	return found, err
}
