// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package rt

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/cockroachdb/eddie/pkg/util"

	"github.com/cockroachdb/eddie/pkg/contract"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Enforcer is the main entrypoint for a generated linter binary.
// The generated code will just configure an instance of Enforcer
// and call its Main() method.
type Enforcer struct {
	// If true, we will only consider types to implement an interface
	// if there is an explicit assertion of the form:
	//   var _ Intf = &Impl{}
	AssertedInterfaces bool
	// Contracts contains providers for the various Contract types.
	// This map is the primary point of code-generation.
	Contracts contract.Providers
	// Allows the working directory to be overridden.
	Dir string
	// The name of the generated linter.
	Name string
	// An optional Logger to receive diagnostic messages.
	Logger *log.Logger
	// The package-patterns to enforce contracts upon.
	Packages []string
	// If true, Main() will call os.Exit(1) if any reports are generated.
	SetExitStatus bool
	// If true, the test sources for the package will be included.
	Tests bool

	allPackages map[string]*packages.Package
	hints       *contract.Hints
	oracle      *contract.TypeOracle
	pkgs        []*packages.Package
	ssaPgm      *ssa.Program

	mu struct {
		sync.Mutex
		results Results
	}
}

// Execute allows an Enforcer to be called programmatically.
func (e *Enforcer) Execute(ctx context.Context) (Results, error) {
	absDir, err := filepath.Abs(e.Dir)
	if err != nil {
		return nil, err
	}
	e.Dir = absDir
	if len(e.Packages) == 0 {
		return nil, errors.New("no packages specified")
	}
	// Load the source
	cfg := &packages.Config{
		Dir:   e.Dir,
		Fset:  token.NewFileSet(),
		Mode:  packages.LoadAllSyntax,
		Tests: e.Tests,
	}
	pkgs, err := packages.Load(cfg, e.Packages...)
	if err != nil {
		return nil, err
	}
	e.pkgs = pkgs

	e.allPackages = flattenImports(pkgs)

	// Prep SSA program. We'll defer building the packages until we
	// need to present a function to a Contract.
	e.ssaPgm, _ = ssautil.AllPackages(pkgs, 0 /* mode */)

	// Look for contract declarations on the AST side before we go through
	// the bother of converting to SSA form
	aliases, assertions, tgts, err := e.findContracts(ctx)
	if err != nil {
		return nil, err
	}

	// Expand aliases and initialize Contract instances.
	work, err := e.expandAll(aliases, assertions, tgts)
	if err != nil {
		return nil, err
	}

	// Defer building SSA nodes until we know that all configuration is good.
	e.ssaPgm.Build()

	// Now, we can run the contracts.
	err = e.enforceAll(ctx, work)

	e.mu.Lock()

	// Filter-without-allocating slice trick to remove empty, top-level
	// results. These will occur if a Contract calls Context.Reporter(),
	// but doesn't actually log anything.
	ret := e.mu.results[:0]
	for _, res := range e.mu.results {
		if res.Data.Len() > 0 || res.Children.Len() > 0 {
			ret = append(ret, res)
		}
	}

	return ret, err
}

// Main is called by the generated main() code.
func (e *Enforcer) Main() {
	verbose := false
	enforce := &cobra.Command{
		Use:           "enforce [packages]",
		Short:         "Enforce contracts defined in the given packages",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sig := make(chan os.Signal, 1)
			defer close(sig)

			signal.Notify(sig, syscall.SIGINT)
			defer signal.Stop(sig)

			go func() {
				if _, open := <-sig; open {
					cmd.Println("Interrupted")
					cancel()
				}
			}()

			e.Packages = args
			if verbose {
				e.Logger = log.New(os.Stdout, "" /* prefix */, 0 /* flags */)
			}
			results, err := e.Execute(ctx)

			// We can sort the top-level and second-level results to create
			// more stable output, but we don't want to sort the remainder
			// of the levels, since we don't know what the Contracts intended.
			sort.Sort(results)
			for _, result := range results {
				sort.Sort(result.Children)
				cmd.Printf("%s\n\n", result.StringRelative(e.Dir))
			}
			if err == nil && e.SetExitStatus {
				err = errors.New("reports generated")
			}
			return err
		},
	}
	enforce.Flags().BoolVar(&e.AssertedInterfaces, "asserted_only",
		false, "only consider explicit type assertions")
	enforce.Flags().StringVarP(&e.Dir, "dir", "d",
		".", "override the current working directory")
	enforce.Flags().BoolVar(&e.SetExitStatus, "set_exit_status",
		false, "return a non-zero exit code if errors are reported")
	enforce.Flags().BoolVarP(&e.Tests, "tests", "t",
		false, "include test sources in the analysis")
	enforce.Flags().BoolVarP(&verbose, "verbose", "v",
		false, "enable additional diagnostic messages")

	root := &cobra.Command{
		Use: e.Name,
	}
	root.AddCommand(
		enforce,
		&cobra.Command{
			Use:   "contracts",
			Short: "Lists all defined contracts",
			RunE: func(cmd *cobra.Command, _ []string) error {
				for name, provider := range e.Contracts {
					if provider.Help == "" {
						cmd.Println("contract:" + name)
					}
					if provider.Help != "" {
						cmd.Println(provider.Help)
					}
					cmd.Println()
				}
				return nil
			},
		})

	if err := root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}

// enforceAll executes all targets.
func (e *Enforcer) enforceAll(ctx context.Context, work []*contextImpl) error {
	g, ctx := errgroup.WithContext(ctx)
	ch := make(chan *contextImpl, 1)

	for i := 0; i < runtime.NumCPU(); i++ {
		g.Go(func() error {
			for {
				select {
				case impl, open := <-ch:
					if !open {
						return nil
					}

					e.printf("enforcing %s: %s %s (%d objects)",
						e.ssaPgm.Fset.Position(impl.declaration.Pos()), impl.Kind(),
						impl.Declaration(), len(impl.Objects()))

					impl.Context = ctx
					if err := impl.contract.Enforce(impl); err != nil {
						impl.Reporter().Printf("%s: %v", impl.contract, err)
					}

				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

sendLoop:
	for i, w := range work {
		select {
		case ch <- w:
			// Nullify the reference to the context once dispatched to
			// allow completed objects to be garbage-collected.
			work[i] = nil
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(ch)

	return g.Wait()
}

// expand expands alias targets into their final form or returns
// terminal targets as-is.
func (e *Enforcer) expand(aliases targetAliases, base *target) ([]*target, error) {
	// Non-terminal targets, which need to be further expanded
	nonTerm := aliases[base.contract]
	if nonTerm == nil {
		return targets{base}, nil
	}

	// The terminal targets, which we will want to return.
	var term targets
	// Detect recursively-defined contracts.  This would only be an
	// issue with contract aliases that are mutually-referent.
	seen := map[*target]bool{base: true}

	for len(nonTerm) > 0 {
		work := append(targets(nil), nonTerm...)
		nonTerm = nonTerm[:0]
		for _, alias := range work {
			if seen[alias] {
				return nil, errors.Errorf("%s detected recursive contract %q",
					base.fset.Position(base.Pos()), alias.contract)
			}
			seen[alias] = true
			if moreExpansions := aliases[alias.contract]; moreExpansions != nil {
				nonTerm = append(nonTerm, moreExpansions...)
			} else {
				dup := *base
				dup.contract = alias.contract
				dup.config = alias.config
				term = append(term, &dup)
			}
		}
	}
	sort.Sort(term)
	return term, nil
}

// expandAll resolves all targets to actual Contract implementations,
// performing alias expansion and configuration. Once this method has
// finished, the Enforcer.contexts field will be populated with all work
// to perform.
func (e *Enforcer) expandAll(
	aliases targetAliases, assertions assertions, tgts targets,
) ([]*contextImpl, error) {
	oracleAssertions := make(contract.Assertions, len(assertions))
	for _, a := range assertions {
		oracleAssertions[a.intf] = append(oracleAssertions[a.intf], a.impl)
	}

	// Populate the shared datastructures.
	e.hints = contract.NewHints()
	e.oracle = contract.NewOracle(e.ssaPgm, oracleAssertions)

	// First pass, expand any aliases.
	expanded := make(targets, 0, len(tgts))
	for _, tgt := range tgts {
		expansion, err := e.expand(aliases, tgt)
		if err != nil {
			return nil, err
		}
		expanded = append(expanded, expansion...)
	}

	// Second pass, construct the contexts to invoke.
	ret := make([]*contextImpl, len(expanded))
	for i, tgt := range expanded {
		ctx, err := e.newContext(tgt)
		if err != nil {
			return nil, err
		}
		ret[i] = ctx
	}

	return ret, nil
}

// findContracts performs AST-level extraction.  Specifically, it will
// find AST nodes which have been annotated with a contract declaration
// as well as type-assertion assignments.
//
// Since we're operating on a per-ast.File basis, we want to operate as
// concurrently as possible. We'll set up a limited number of goroutines
// and feed them (package, file) pairs.
func (e *Enforcer) findContracts(ctx context.Context) (targetAliases, assertions, targets, error) {
	// mu protects the variables shared between goroutines.
	mu := struct {
		sync.Mutex
		aliases    targetAliases
		assertions assertions
		targets    targets
	}{
		aliases: make(targetAliases),
	}

	// addAssertion updates mu.assertions in a safe manner.
	addAssertion := func(a *assertion) {
		e.println("assertion", a)
		mu.Lock()
		mu.assertions = append(mu.assertions, a)
		mu.Unlock()
	}

	// extract will update mu.targets if the provided comments contain
	// a contract declaration. It will also extract contract aliases.
	extract := func(
		pkg *packages.Package,
		comments []*ast.CommentGroup,
		object types.Object,
		enclosing types.Object,
		kind contract.Kind,
	) {
		for _, group := range comments {
			for _, comment := range group.List {
				matches := commentSyntax.FindAllStringSubmatch(comment.Text, -1)
				for _, match := range matches {
					tgt := &target{
						config:    strings.TrimSpace(match[2]),
						contract:  match[1],
						enclosing: enclosing,
						fset:      pkg.Fset,
						kind:      kind,
						object:    object,
						pos:       comment.Pos(),
					}

					e.println("target", tgt)
					mu.Lock()
					// Special case for contract aliases of the form
					//   //contract:Foo { ... }
					//   type Alias contract.Contract
					// The reference to "Contract" will already be resolved to
					// the underlying types.Interface.  We can still know that
					// it's our Contract type by looking for the sole method
					// on the interface to be defined in our contract package.
					if underInt, ok := tgt.object.Type().Underlying().(*types.Interface); ok &&
						underInt.NumMethods() == 1 &&
						util.InPackage(underInt.Method(0), util.Base+"contract") {
						name := tgt.object.Name()
						e.println("alias", name, ":=", tgt)
						mu.aliases[name] = append(mu.aliases[name], tgt)
					} else {
						mu.targets = append(mu.targets, tgt)
					}
					mu.Unlock()
				}
			}
		}
	}

	// process performs the bulk of the work in this method.
	process := func(ctx context.Context, pkg *packages.Package, file *ast.File) error {
		// CommentMap associates each node in the file with
		// its surrounding comments.
		comments := ast.NewCommentMap(pkg.Fset, file, file.Comments)

		// Track current-X's in the visitation below.
		var enclosing types.Object

		// Now we'll inspect the ast.File and look for our magic
		// comment syntax.
		ast.Inspect(file, func(node ast.Node) bool {
			// We'll see a node==nil as the very last call.
			if node == nil {
				return false
			}

			switch t := node.(type) {
			case *ast.Field:
				// Methods of an interface type, such as
				//   type I interface { Foo() }
				// surface as fields with a function type.
				if types.IsInterface(enclosing.Type()) {
					if _, ok := t.Type.(*ast.FuncType); ok {
						extract(pkg, comments[t], pkg.TypesInfo.ObjectOf(t.Names[0]), enclosing, contract.KindInterfaceMethod)
					}
				}
				return false

			case *ast.FuncDecl:
				// Top-level function or method declarations, such as
				//   func Foo() { .... }
				//   func (r Receiver) Bar() { ... }
				kind := contract.KindFunction
				if t.Recv != nil {
					kind = contract.KindMethod
				}
				extract(pkg, comments[t], pkg.TypesInfo.ObjectOf(t.Name), nil, kind)
				// We don't need to descend into function bodies.
				return false

			case *ast.GenDecl:
				switch t.Tok {
				case token.TYPE:
					// Type declarations, such as
					//   type Foo struct { ... }
					//   type Bar interface { ... }
					shouldEnter := false
					for _, spec := range t.Specs {
						tSpec := spec.(*ast.TypeSpec)
						enclosing = pkg.TypesInfo.ObjectOf(tSpec.Name)
						kind := contract.KindType
						if _, ok := tSpec.Type.(*ast.InterfaceType); ok {
							kind = contract.KindInterface
						}

						// Handle the usual case where contract is associated
						// with the type keyword.
						extract(pkg, comments[t], enclosing, nil, kind)
						// Handle unusual case where a type() block is being used
						// and a contract is specified on the entry.
						extract(pkg, comments[tSpec], enclosing, nil, kind)
						// We do need to descend into interfaces to pick up on
						// contracts applied only to interface methods.
						shouldEnter = shouldEnter || kind == contract.KindInterface
					}
					return shouldEnter

				case token.VAR:
					// Assertion declarations, such as
					//   var _ Intf = &Impl{}
					//   var _ Intf = Impl{}
					for _, spec := range t.Specs {
						v := spec.(*ast.ValueSpec)
						if len(v.Values) == 1 && v.Names[0].Name == "_" {
							named, ok := pkg.TypesInfo.TypeOf(v.Type).(*types.Named)
							if !ok || !types.IsInterface(named) {
								continue
							}
							a := &assertion{
								intf: named.Obj(),
								fset: e.ssaPgm.Fset,
								pos:  v.Pos(),
							}
							switch v := pkg.TypesInfo.TypeOf(v.Values[0]).(type) {
							case *types.Named:
								if _, ok := v.Underlying().(*types.Struct); ok {
									a.impl = v.Obj()
								}
							case *types.Pointer:
								if named, ok := v.Elem().(*types.Named); ok {
									if _, ok := named.Underlying().(*types.Struct); ok {
										a.impl = named.Obj()
									}
								}
							}
							if a.impl != nil {
								addAssertion(a)
							}
						}
					}
				}
				return false
			default:
				return true
			}
		})
		return nil
	}

	type work struct {
		pkg  *packages.Package
		file *ast.File
	}
	workCh := make(chan work, 1)

	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < runtime.NumCPU(); i++ {
		g.Go(func() error {
			for {
				select {
				case next, open := <-workCh:
					if !open {
						return nil
					}
					if err := process(ctx, next.pkg, next.file); err != nil {
						return err
					}
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		})
	}

sendLoop:
	for _, pkg := range e.pkgs {
		// See discussion on package.Config type for the naming scheme.
		if e.Tests && !strings.HasSuffix(pkg.ID, ".test]") {
			continue
		}
		if pkg.Errors != nil {
			return nil, nil, nil, errors.Wrap(pkg.Errors[0], "could not load source due to error(s)")
		}

		for _, file := range pkg.Syntax {
			select {
			case workCh <- work{pkg, file}:
			case <-ctx.Done():
				break sendLoop
			}
		}
	}
	close(workCh)

	// Wait for all the goroutines to exit.
	if err := g.Wait(); err != nil {
		return nil, nil, nil, err
	}

	// Produce stable output.
	for _, aliases := range mu.aliases {
		sort.Sort(aliases)
	}
	sort.Sort(mu.assertions)
	sort.Sort(mu.targets)

	return mu.aliases, mu.assertions, mu.targets, nil
}

// newContext builds the contract.Context that contains everything
// we need in order to evaluate the contract.
func (e *Enforcer) newContext(tgt *target) (*contextImpl, error) {
	provider := e.Contracts[tgt.contract]
	if provider == nil {
		return nil, errors.Errorf("%s: cannot find contract named %s",
			tgt.fset.Position(tgt.Pos()), tgt.contract)
	}

	impl := &contextImpl{
		contract: provider.New(),
		hints:    e.hints,
		oracle:   e.oracle,
		program:  e.ssaPgm,
		reporter: func(r *Result) {
			e.mu.Lock()
			e.mu.results = append(e.mu.results, r)
			e.mu.Unlock()
		},
		target: tgt,
	}

	if tgt.config != "" {
		// Disallow unknown fields to help with typos.
		d := json.NewDecoder(strings.NewReader(tgt.config))
		d.DisallowUnknownFields()
		if err := d.Decode(&impl.contract); err != nil {
			return nil, errors.Wrap(err, tgt.fset.Position(tgt.Pos()).String())
		}
	}

	switch tgt.kind {
	case contract.KindInterface:
		decl := e.ssaPgm.Package(tgt.object.Pkg()).Type(tgt.object.Name())
		impl.declaration = decl
		intf := decl.Type().Underlying().(*types.Interface)
		for _, obj := range impl.Oracle().TypeImplementors(intf, e.AssertedInterfaces) {
			impl.objects = append(impl.objects, e.ssaPgm.Package(obj.Pkg()).Type(obj.Name()))
		}

	case contract.KindInterfaceMethod:
		intf := tgt.enclosing.Type().Underlying().(*types.Interface)
		impl.declaration = e.ssaPgm.Package(tgt.enclosing.Pkg()).Type(tgt.enclosing.Name())
		for _, i := range impl.Oracle().MethodImplementors(intf, tgt.object.Name(), e.AssertedInterfaces) {
			impl.objects = append(impl.objects, i)
		}

	case contract.KindFunction, contract.KindMethod:
		fn := tgt.object.(*types.Func)
		impl.declaration = e.ssaPgm.FuncValue(fn)
		impl.objects = []ssa.Member{impl.declaration}

	case contract.KindType:
		impl.declaration = e.ssaPgm.Package(tgt.object.Pkg()).Type(tgt.object.Name())
		impl.objects = []ssa.Member{impl.declaration}

	default:
		panic(errors.Errorf("unimplemented: %s", tgt.kind))
	}

	for _, obj := range impl.objects {
		e.hints.Add(obj, impl.contract)
	}

	return impl, nil
}

// printf will emit a diagnostic message via e.Logger, if one is configured.
func (e *Enforcer) printf(format string, args ...interface{}) {
	if l := e.Logger; l != nil {
		l.Printf(format, args...)
	}
}

// println will emit a diagnostic message via e.Logger, if one is configured.
func (e *Enforcer) println(args ...interface{}) {
	if l := e.Logger; l != nil {
		l.Println(args...)
	}
}
