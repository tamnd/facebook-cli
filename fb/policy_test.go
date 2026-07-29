package fb

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// policy_test.go reads the package as source rather than running it, because
// the thing being checked is what the code is allowed to contain.
//
// fb is a read-only tool, and "read-only" here is not a promise in the README.
// It is two properties anybody can check: every request to facebook.com is a
// GET except the one form post that replays a query the page itself shipped,
// and every operation the tool exposes is a read. Both are easy to break by
// accident and neither breaks a test that only checks what a parser returns.

// packageFiles parses every non-test file in the package once.
func packageFiles(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if f.Name.Name != "fb" {
			t.Fatalf("%s is package %s, which means this test is reading the wrong directory", name, f.Name.Name)
		}
		files[name] = f
	}
	if len(files) == 0 {
		t.Fatal("no fb package here, which means this test is reading the wrong directory")
	}
	return fset, files
}

// methodOf pulls the request method out of a call to http.NewRequest or
// http.NewRequestWithContext, which are the only two ways this package makes a
// request. It returns the method as written, and false for a call it does not
// recognise as one of those two.
func methodOf(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "http" {
		return "", false
	}
	var arg ast.Expr
	switch sel.Sel.Name {
	case "NewRequest":
		if len(call.Args) < 1 {
			return "", false
		}
		arg = call.Args[0]
	case "NewRequestWithContext":
		if len(call.Args) < 2 {
			return "", false
		}
		arg = call.Args[1]
	default:
		return "", false
	}
	switch v := arg.(type) {
	case *ast.SelectorExpr:
		// http.MethodGet and friends.
		return strings.TrimPrefix(v.Sel.Name, "Method"), true
	case *ast.BasicLit:
		if s, err := strconv.Unquote(v.Value); err == nil {
			return s, true
		}
	}
	// A method computed at runtime is worse than a wrong one, because nothing
	// reading the source can tell what it will be.
	return "computed", true
}

// TestEveryRequestIsAGETExceptTheReplay. There is exactly one non-GET in this
// tool: the form post to /api/graphql/ that replays an operation the page just
// shipped, with the doc id and the variables the page itself carried. Every
// other read is a plain GET of a URL a browser would navigate to.
//
// A second POST appearing anywhere is how a read-only tool stops being one,
// and it does not have to be a deliberate write to do damage: Facebook counts
// a POST differently from a GET, and a session that starts posting from a
// command line is a session that gets challenged.
func TestEveryRequestIsAGETExceptTheReplay(t *testing.T) {
	fset, files := packageFiles(t)
	for path, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, ok := methodOf(call)
			if !ok {
				return true
			}
			if method == "Get" || method == "GET" {
				return true
			}
			pos := fset.Position(call.Pos())
			if strings.HasSuffix(path, "graphql.go") {
				// The one exception, and only for the one endpoint.
				return true
			}
			t.Errorf("%s:%d issues a %s. Only the GraphQL replay in graphql.go may be anything but a GET",
				path, pos.Line, strings.ToUpper(method))
			return true
		})
	}
}

// TestTheOnlyPostGoesToTheGraphQLEndpoint keeps the exception above honest.
// The test above allows a non-GET anywhere in graphql.go, so on its own it
// would let a second post to some other URL move into that file and pass. This
// one takes the endpoint the post is actually given, follows it back to where
// it was built, and requires /api/graphql/ to be in it.
func TestTheOnlyPostGoesToTheGraphQLEndpoint(t *testing.T) {
	fset, files := packageFiles(t)
	var f *ast.File
	for path, file := range files {
		if strings.HasSuffix(path, "graphql.go") {
			f = file
		}
	}
	if f == nil {
		t.Fatal("no graphql.go, so this test is checking nothing")
	}
	// Every string literal assigned to a name, so an endpoint built a line
	// before the request can be followed to what it was built from.
	built := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		var name string
		var rhs []ast.Expr
		switch v := n.(type) {
		case *ast.AssignStmt:
			if len(v.Lhs) != 1 || len(v.Rhs) != 1 {
				return true
			}
			id, ok := v.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			name, rhs = id.Name, v.Rhs
		default:
			return true
		}
		ast.Inspect(rhs[0], func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil {
					built[name] += s
				}
			}
			return true
		})
		return true
	})

	var found int
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := methodOf(call)
		if !ok || method == "Get" || method == "GET" {
			return true
		}
		found++
		line := fset.Position(call.Pos()).Line
		if len(call.Args) < 3 {
			t.Errorf("graphql.go:%d builds a request with no URL", line)
			return true
		}
		id, ok := call.Args[2].(*ast.Ident)
		if !ok {
			t.Errorf("graphql.go:%d takes its URL from something this test cannot follow, so read it by hand", line)
			return true
		}
		if !strings.Contains(built[id.Name], "/api/graphql") {
			t.Errorf("graphql.go:%d posts to %q, and the replay endpoint is the only thing fb may post to", line, built[id.Name])
		}
		return true
	})
	if found != 1 {
		t.Errorf("graphql.go makes %d non-GET requests, and there should be exactly one", found)
	}
}

// TestNoOperationIsAMutation. Every doc id fb replays is harvested from a page
// it just fetched, and the friendly name that goes with it is written here so
// the right preload can be picked out. Facebook names its write operations
// Mutation, so a name ending in Mutation in this package means the tool has
// learned to change something.
func TestNoOperationIsAMutation(t *testing.T) {
	_, files := packageFiles(t)
	for path, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if strings.HasSuffix(s, "Mutation") {
				t.Errorf("%s names the operation %q, and fb does not send mutations", path, s)
			}
			return true
		})
	}
}

// TestNoDocIDIsHardcoded. A doc id is a long run of digits, it rotates every
// few weeks, and a hardcoded one is a read that works until it silently does
// not. The rule from doc 01 section 3 is that they only ever come off a page
// fb just fetched.
//
// reactions.go is exempt, and it has to be by name because a reaction id and a
// doc id are both sixteen digits and nothing in the string tells them apart.
// The seven ids in that table are platform constants: Like has been
// 1635855486666999 since reactions shipped, they arrive in the payload with no
// name attached on a comment, and there is nowhere to harvest them from. The
// test that keeps them honest is TestReactionTableMatchesFixtures, which checks
// each one against the localized_name Facebook sent beside it.
func TestNoDocIDIsHardcoded(t *testing.T) {
	_, files := packageFiles(t)
	for path, f := range files {
		if strings.HasSuffix(path, "reactions.go") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err != nil || len(s) < 12 {
				return true
			}
			for _, r := range s {
				if r < '0' || r > '9' {
					return true
				}
			}
			t.Errorf("%s carries the literal %q, which looks like a doc id. They rotate: harvest it from the page instead", path, s)
			return true
		})
	}
}

// TestEveryOperationIsARead is the other half of the policy, checked against
// the registrations rather than the source. Every operation belongs to the read
// group and none is marked Write, so `fb serve` and `fb mcp` have nothing on
// them that changes anything, on Facebook or on this machine.
func TestEveryOperationIsARead(t *testing.T) {
	app := kit.New(kit.Identity{Binary: "fb", Short: "test app"})
	RegisterOps(app, OpOptions{NoCLI: true})
	ops := app.Ops()
	if len(ops) == 0 {
		t.Fatal("no operations registered, so this test is checking nothing")
	}
	for _, op := range ops {
		m := op.Meta()
		if m.Write {
			t.Errorf("operation %q is marked Write", m.Name)
		}
		if m.Group != "read" {
			t.Errorf("operation %q is in group %q, and read is the only group this tool has", m.Name, m.Group)
		}
	}
}
