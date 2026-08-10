package k8s

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// bannedKubectlLookup is the exact call every production call site must
// route through KubectlPath instead — see kubectl_path.go. A literal
// substring match is enough here because the banned expression is a fixed
// call with a fixed string-literal argument, not a pattern that a method
// chain or a rename could vary.
const bannedKubectlLookup = `exec.LookPath("kubectl")`

// guardExemptFiles are the only files allowed to contain the banned call:
// the resolver's own implementation, its test's expected-value fixture, and
// this guard file, which necessarily quotes the banned literal to define it.
var guardExemptFiles = map[string]bool{
	"kubectl_path.go":            true,
	"kubectl_path_test.go":       true,
	"kubectl_path_guard_test.go": true,
}

// TestNoBareKubectlLookPath walks the module for the literal
// exec.LookPath("kubectl") call outside the shared resolver. Every other
// call site must go through KubectlPath so demo mode's KUBECTL_BIN override
// (which points kubeconfig precedence at /dev/null) cannot be bypassed by a
// path that falls back to the user's real default kubeconfig.
func TestNoBareKubectlLookPath(t *testing.T) {
	root := moduleRoot(t)

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if guardExemptFiles[filepath.Base(path)] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), bannedKubectlLookup) {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking module root: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("found bare %s outside the shared resolver; route these through k8s.KubectlPath() instead:\n%s",
			bannedKubectlLookup, strings.Join(offenders, "\n"))
	}
}

// execNameResolvers maps every os/exec function that resolves or spawns a
// binary/interpreter from a name argument to the index of that argument.
// Verified against `go doc -all os/exec`: LookPath and Command take the
// name first; CommandContext takes a context.Context first, so the name
// shifts to index 1. Those three are the entire set — no other exported
// os/exec function accepts a binary or interpreter name. Get the index
// wrong here and the guard silently stops checking that function's calls.
var execNameResolvers = map[string]int{
	"LookPath":       0,
	"Command":        0,
	"CommandContext": 1,
}

// execCallSite is one exec.LookPath, exec.Command, or exec.CommandContext
// call whose binary-name argument resolves to a constant string, found by
// walking the module's AST.
type execCallSite struct {
	file   string // path relative to the module root
	fn     string // "LookPath", "Command", or "CommandContext"
	binary string // the resolved argument value
}

// foldConstString evaluates expr as a compile-time string constant: a string
// literal, a same-file const identifier (resolved via consts), or a chain of
// string literals/consts joined with +. It does not use go/types, so it only
// sees what's visible in the current file: a const defined in another file
// or package, or a value built by anything other than literal concatenation
// (a function call, a const block using iota, etc.), is not resolved and the
// call is skipped rather than misreported.
func foldConstString(expr ast.Expr, consts map[string]string) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return v, true
	case *ast.Ident:
		v, ok := consts[e.Name]
		return v, ok
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		l, ok := foldConstString(e.X, consts)
		if !ok {
			return "", false
		}
		r, ok := foldConstString(e.Y, consts)
		if !ok {
			return "", false
		}
		return l + r, true
	default:
		return "", false
	}
}

// fileStringConsts collects every top-level `const` in astFile whose value
// folds to a string, keyed by name. Consts are resolved in declaration
// order, so a const defined in terms of an earlier const in the same file
// folds too; forward references within a file do not.
func fileStringConsts(astFile *ast.File) map[string]string {
	consts := make(map[string]string)
	for _, decl := range astFile.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			vspec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vspec.Names {
				if i >= len(vspec.Values) {
					continue
				}
				if v, ok := foldConstString(vspec.Values[i], consts); ok {
					consts[name.Name] = v
				}
			}
		}
	}
	return consts
}

// collectLiteralExecCalls walks every non-test .go file under root and
// returns each call to a function in execNameResolvers whose binary-name
// argument resolves to a constant string.
//
// An AST walk is used instead of a regex: a regex matching exec.LookPath("x")
// or exec.Command("x", ...) is easy to write, but it only matches that exact
// textual shape — it misses the call when it's spread across multiple lines,
// and can't reliably tell a literal argument from one reached through a
// method chain or a renamed import. Walking the parsed syntax tree instead
// means the shape is checked structurally: a *ast.CallExpr on a selector
// whose package identifier is exec and whose function name is in
// execNameResolvers, with the argument at that function's name-index folding
// to a constant string via foldConstString. Formatting never matters, and a
// variable argument (the pattern every guarded call site already uses) is
// never mistaken for a constant.
//
// A call whose name argument is a variable (i.e. every production
// exec.Command(kubectlPath, ...) call site) is not collected — by design,
// since routing the binary name through a variable resolved by a guarded
// function (KubectlPath, resolveHelmPath, ...) is exactly the pattern this
// guard exists to enforce.
func collectLiteralExecCalls(t *testing.T, root string) []execCallSite {
	t.Helper()

	var sites []execCallSite
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		astFile, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parsing %s: %w", path, parseErr)
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		consts := fileStringConsts(astFile)

		ast.Inspect(astFile, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "exec" {
				return true
			}
			argIdx, ok := execNameResolvers[sel.Sel.Name]
			if !ok {
				return true
			}
			if len(call.Args) <= argIdx {
				return true
			}
			binary, ok := foldConstString(call.Args[argIdx], consts)
			if !ok {
				return true
			}
			sites = append(sites, execCallSite{file: rel, fn: sel.Sel.Name, binary: binary})
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking module root: %v", err)
	}
	return sites
}

// externalBinaryLookupAllowlist is every constant-argument exec.LookPath,
// exec.Command, or exec.CommandContext call site the module may contain,
// keyed by its path relative to the module root and then by the resolved
// binary/interpreter name. Adding an entry here is a deliberate signal: the
// binary it resolves either can't run in demo mode, or is gated the way
// resolveHelmPath and vulnScanImage gate helm and trivy —
// k8s.DemoModeEnabled() checked before the binary is ever resolved. The "sh"
// entries are the shell interpreter invoked with an already-guarded command
// string (exec.Command("sh", "-c", cmd)), not an external tool being
// resolved, so they carry no bypass risk of their own.
//
// TASK-865 finding 2 found this guard only matched exec.LookPath, so a
// direct exec.Command("literal", ...) — which skips LookPath entirely —
// stayed invisible to it. Finding 3 found exec.CommandContext skipped for
// the same reason, plus two shapes collectLiteralExecCalls still can't see:
// a const or concatenation reached through another file or package (only
// same-file resolution is done — see foldConstString), and any value built
// by something other than literal concatenation (fmt.Sprintf, an iota-based
// const, a computed expression). Those slip through the same way a renamed
// import or a genuinely dynamic binary name always would; there is no
// go/types or x/tools/go/analysis dependency here to constant-fold them,
// since whole-module type-checking is a meaningfully heavier and slower
// addition than this guard warrants for shapes no current call site uses.
var externalBinaryLookupAllowlist = map[string]map[string]bool{
	filepath.Join("internal", "k8s", "kubectl_path.go"):       {"kubectl": true},
	filepath.Join("internal", "app", "commands_exec_helm.go"): {"helm": true, "sh": true},
	filepath.Join("internal", "app", "commands_exec_misc.go"): {"trivy": true, "sh": true},
	filepath.Join("internal", "app", "commandbar_execute.go"): {"sh": true},
	filepath.Join("internal", "app", "commands.go"):           {"sh": true},
}

// TestNoUnguardedExternalBinaryLookup walks the module for every
// constant-argument exec.LookPath, exec.Command, or exec.CommandContext call
// (see execNameResolvers) outside of production _test.go files and requires
// each one to be an already-reviewed entry in externalBinaryLookupAllowlist.
// Test files are skipped: they exist to exercise the guarded call sites
// (e.g. comparing resolveHelmPath's output against a raw
// exec.LookPath("helm")) and never run as part of the shipped binary.
func TestNoUnguardedExternalBinaryLookup(t *testing.T) {
	root := moduleRoot(t)

	var offenders []string
	for _, site := range collectLiteralExecCalls(t, root) {
		if externalBinaryLookupAllowlist[site.file][site.binary] {
			continue
		}
		offenders = append(offenders, fmt.Sprintf("%s: exec.%s(%q)", site.file, site.fn, site.binary))
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("found exec.LookPath/exec.Command/exec.CommandContext call(s) with a constant binary "+
			"name not in externalBinaryLookupAllowlist; confirm the binary can't reach a real "+
			"cluster/network in demo mode (gate it with k8s.DemoModeEnabled() first if it can), then add "+
			"it to the allowlist:\n%s",
			strings.Join(offenders, "\n"))
	}
}

// moduleRoot locates the repository root by walking up from this test file
// until a go.mod is found, so the guard scans the whole module regardless of
// the working directory `go test` was invoked from.
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to resolve this test file's path")
	}
	dir := filepath.Dir(thisFile)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod above " + thisFile)
		}
		dir = parent
	}
}
