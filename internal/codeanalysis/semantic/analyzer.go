package semantic

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
)

// ModificationType categorizes the kind of change made to a symbol.
type ModificationType int

const (
	ModificationUnknown   ModificationType = iota
	ModificationSignature                  // function signature changed (params, return values)
	ModificationBody                       // function body altered
	ModificationComment                    // only comments changed
	ModificationImport                     // import list changed
	ModificationExport                     // symbol export status changed (internal → exported or vice versa)
)

// SymbolKind describes what kind of symbol was modified.
type SymbolKind int

const (
	KindUnknown SymbolKind = iota
	KindFunc
	KindType
	KindVar
	KindConst
	KindMethod
)

// Symbol represents a code symbol with its metadata.
type Symbol struct {
	Name      string
	Kind      SymbolKind
	Exported  bool
	Package   string
	Position  string // file:line
	ModType   ModificationType
	Callers   []string // functions that call this symbol
	Callees   []string // functions this symbol calls
	Signature string   // full signature for funcs/types
	IsTest    bool     // _test.go file
}

// Summary holds the semantic analysis result for a file change.
type Summary struct {
	FilePath      string
	ModifiedCount int
	ExportedCount int
	Symbols       []Symbol
	RiskScore     float64  // 0.0 (safe) → 1.0 (high risk)
	RiskReasons   []string // human-readable explanations
	ModTypeCounts map[ModificationType]int
}

// Analyzer extracts semantic information from Go code changes.
type Analyzer struct {
	fset        *token.FileSet
	projectRoot string
}

// NewAnalyzer creates a new semantic analyzer.
func NewAnalyzer(projectRoot string) *Analyzer {
	return &Analyzer{
		fset:        token.NewFileSet(),
		projectRoot: projectRoot,
	}
}

// AnalyzeFile analyzes a Go source file and returns its semantic structure.
// It extracts function signatures, types, exported symbols, and call relationships.
// The analysis is performed on the current (post-change) state of the file.
func (a *Analyzer) AnalyzeFile(filePath string) (*Summary, error) {
	node, err := parser.ParseFile(a.fset, filePath, nil, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file %s: %w", filePath, err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check(filePath, a.fset, []*ast.File{node}, nil)
	if err != nil {
		// Type checking can fail on incomplete code — still return partial results
		pkg = nil
	}

	summary := &Summary{
		FilePath:      filePath,
		ModTypeCounts: make(map[ModificationType]int),
	}

	a.extractSymbols(node, pkg, summary)
	a.computeRisk(summary)

	return summary, nil
}

// AnalyzeDiff analyzes a before/after pair of file contents.
// Returns a Summary describing what changed between them.
// The diff is computed by comparing the symbol sets before and after.
func (a *Analyzer) AnalyzeDiff(filePath string, before, after string) (*Summary, error) {
	beforeSum, err := a.analyzeContent(filePath, before)
	if err != nil {
		return nil, err
	}

	afterSum, err := a.analyzeContent(filePath, after)
	if err != nil {
		return nil, err
	}

	// Diff: identify symbols that changed
	symbolsChanged := make(map[string]bool)
	beforeNames := make(map[string]Symbol)
	for _, s := range beforeSum.Symbols {
		beforeNames[s.Name] = s
	}

	var changedSymbols []Symbol
	var exportedChanged, modifiedCount int

	for _, s := range afterSum.Symbols {
		beforeSym, exists := beforeNames[s.Name]
		if !exists {
			// New symbol
			s.ModType = ModificationSignature
			changedSymbols = append(changedSymbols, s)
			modifiedCount++
			if s.Exported {
				exportedChanged++
			}
		} else if s.Signature != beforeSym.Signature {
			// Signature changed
			s.ModType = ModificationSignature
			changedSymbols = append(changedSymbols, s)
			modifiedCount++
			if s.Exported {
				exportedChanged++
			}
		} else if s.Exported != beforeSym.Exported {
			// Export status changed
			s.ModType = ModificationExport
			changedSymbols = append(changedSymbols, s)
			modifiedCount++
			exportedChanged++
		}
		symbolsChanged[s.Name] = true
	}

	// Detect removed symbols (exist in before but not after)
	for _, s := range beforeSum.Symbols {
		if !symbolsChanged[s.Name] {
			// Symbol was removed — still tracked in modified (implicit body removal)
			s.ModType = ModificationBody
			changedSymbols = append(changedSymbols, s)
			modifiedCount++
			if s.Exported {
				exportedChanged++
			}
		}
	}

	// Build final summary
	sum := &Summary{
		FilePath:      filePath,
		ModifiedCount: modifiedCount,
		ExportedCount: exportedChanged,
		Symbols:       changedSymbols,
		ModTypeCounts: make(map[ModificationType]int),
	}

	for _, s := range changedSymbols {
		sum.ModTypeCounts[s.ModType]++
	}

	a.computeRisk(sum)

	// Build diff summary by merging changed symbols into full file summary
	if len(changedSymbols) > 0 {
		// Call graph is computed on the full after state
		for i := range sum.Symbols {
			if i < len(afterSum.Symbols) {
				sum.Symbols[i].Callers = afterSum.Symbols[i].Callers
				sum.Symbols[i].Callees = afterSum.Symbols[i].Callees
			}
		}
	}

	return sum, nil
}

func (a *Analyzer) analyzeContent(filePath, content string) (*Summary, error) {
	node, err := parser.ParseFile(a.fset, filePath, content, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse content: %w", err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, _ := conf.Check(filePath, a.fset, []*ast.File{node}, nil)

	sum := &Summary{FilePath: filePath, ModTypeCounts: make(map[ModificationType]int)}
	a.extractSymbols(node, pkg, sum)

	return sum, nil
}

// extractSymbols walks the AST and collects all symbols with their metadata.
func (a *Analyzer) extractSymbols(node *ast.File, pkg *types.Package, sum *Summary) {
	scope := pkg.Scope()
	seen := make(map[string]bool)

	// Collect all top-level declarations
	ast.Inspect(node, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			if seen[decl.Name.Name] {
				return true
			}
			seen[decl.Name.Name] = true

			sym := a.makeSymbol(decl.Name.Name, KindFunc, decl.Name.IsExported(), node, pkg, decl)
			if decl.Recv != nil {
				sym.Kind = KindMethod
			}
			sum.Symbols = append(sum.Symbols, sym)
			if sym.Exported {
				sum.ExportedCount++
			}

		case *ast.GenDecl:
			if decl.Tok == token.IMPORT {
				sum.ModTypeCounts[ModificationImport]++
				return true
			}
			for _, spec := range decl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				var kind SymbolKind
				if decl.Tok == token.TYPE {
					kind = KindType
				} else if decl.Tok == token.CONST {
					kind = KindConst
				} else {
					kind = KindVar
				}
				for _, name := range valueSpec.Names {
					if seen[name.Name] {
						continue
					}
					seen[name.Name] = true
					sym := a.makeValueSymbol(name.Name, kind, name.IsExported(), valueSpec)
					sum.Symbols = append(sum.Symbols, sym)
					if sym.Exported {
						sum.ExportedCount++
					}
				}
			}
		}
		return true
	})

	// Build call graph: for each function, find what it calls
	a.buildCallGraph(node, pkg, scope, sum)
}

func (a *Analyzer) makeSymbol(name string, kind SymbolKind, exported bool, node *ast.File, pkg *types.Package, decl *ast.FuncDecl) Symbol {
	pos := a.fset.Position(decl.Pos())
	sig := a.signatureString(pkg, name)

	pkgPath := ""
	if pkg != nil {
		pkgPath = pkg.Path()
	}

	sym := Symbol{
		Name:      name,
		Kind:      kind,
		Exported:  exported,
		Package:   pkgPath,
		Position:  fmt.Sprintf("%s:%d", pos.Filename, pos.Line),
		Signature: sig,
		IsTest:    strings.HasSuffix(pos.Filename, "_test.go"),
	}

	// Classify modification type from comments vs code
	if hasOnlyComments(decl.Body, node.Comments) {
		sym.ModType = ModificationComment
	} else {
		sym.ModType = ModificationBody
	}

	return sym
}

func (a *Analyzer) makeValueSymbol(name string, kind SymbolKind, exported bool, decl ast.Spec) Symbol {
	sym := Symbol{
		Name:     name,
		Kind:     kind,
		Exported: exported,
		ModType:  ModificationBody,
	}
	if decl.Pos().IsValid() {
		sym.Position = a.fset.Position(decl.Pos()).String()
	}
	return sym
}

// signatureString returns a human-readable signature for a function.
func (a *Analyzer) signatureString(pkg *types.Package, name string) string {
	if pkg == nil {
		return name
	}
	obj := pkg.Scope().Lookup(name)
	if obj == nil {
		return name
	}
	if sig, ok := obj.Type().(*types.Signature); ok {
		return sig.String()
	}
	return name
}

// buildCallGraph finds caller/callee relationships between functions in the AST.
func (a *Analyzer) buildCallGraph(node *ast.File, pkg *types.Package, scope *types.Scope, sum *Summary) {
	// Collect all function names first
	funcNames := make(map[string]int)
	for i, s := range sum.Symbols {
		if s.Kind == KindFunc || s.Kind == KindMethod {
			funcNames[s.Name] = i
		}
	}

	// Map from callee name → list of caller indices
	calleeToCallers := make(map[string][]int)

	// Walk the AST and find call expressions
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Get the function being called
		var calleeName string
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			calleeName = fun.Name
		case *ast.SelectorExpr:
			if ident, ok := fun.X.(*ast.Ident); ok {
				calleeName = fmt.Sprintf("%s.%s", ident.Name, fun.Sel.Name)
			}
		}

		if calleeName == "" {
			return true
		}

		// Find which function this call is inside (the caller)
		callerName := a.enclosingFunction(node, call.Pos())
		if callerName == "" {
			return true
		}

		// Register the call relationship
		if callerIdx, ok := funcNames[callerName]; ok {
			calleeToCallers[calleeName] = append(calleeToCallers[calleeName], callerIdx)
			// Add callee to caller's callee list
			sum.Symbols[callerIdx].Callees = append(sum.Symbols[callerIdx].Callees, calleeName)
		}

		return true
	})

	// Propagate callers to the callee symbols
	for calleeName, callerIndices := range calleeToCallers {
		if calleeIdx, ok := funcNames[calleeName]; ok {
			for _, callerIdx := range callerIndices {
				if callerName := sum.Symbols[callerIdx].Name; callerName != "" {
					sum.Symbols[calleeIdx].Callers = append(sum.Symbols[calleeIdx].Callers, callerName)
				}
			}
		}
	}
}

// enclosingFunction returns the name of the function that contains pos.
func (a *Analyzer) enclosingFunction(node *ast.File, pos token.Pos) string {
	var current string
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Pos() <= pos && pos <= fn.End() {
				current = fn.Name.Name
			}
		}
	}
	return current
}

// computeRisk calculates a risk score based on the modification characteristics.
func (a *Analyzer) computeRisk(sum *Summary) {
	if len(sum.Symbols) == 0 {
		sum.RiskScore = 0.0
		return
	}

	var risk float64
	var reasons []string

	// High risk: exported symbols modified
	exportedChanges := 0
	for _, s := range sum.Symbols {
		if s.Exported && s.ModType != ModificationComment {
			exportedChanges++
		}
	}
	if exportedChanges > 0 {
		risk += float64(exportedChanges) * 0.25
		reasons = append(reasons, fmt.Sprintf("%d exported symbol(s) modified", exportedChanges))
	}

	// Medium risk: signature changes (affects callers)
	sigChanges := sum.ModTypeCounts[ModificationSignature]
	if sigChanges > 0 {
		risk += float64(sigChanges) * 0.2
		reasons = append(reasons, fmt.Sprintf("%d signature change(s)", sigChanges))
	}

	// Medium risk: many functions in call graph affected
	callerCounts := 0
	for _, s := range sum.Symbols {
		callerCounts += len(s.Callers)
	}
	if callerCounts > 5 {
		risk += 0.15
		reasons = append(reasons, fmt.Sprintf("%d caller relationships impacted", callerCounts))
	}

	// Low risk: body changes on internal functions only
	bodyChanges := sum.ModTypeCounts[ModificationBody]
	if bodyChanges > 0 && exportedChanges == 0 {
		risk += float64(bodyChanges) * 0.05
		reasons = append(reasons, fmt.Sprintf("%d internal function body change(s)", bodyChanges))
	}

	// Cap at 1.0
	if risk > 1.0 {
		risk = 1.0
	}

	sum.RiskScore = risk
	sum.RiskReasons = reasons
}

// hasOnlyComments reports whether the function body contains only comments.
func hasOnlyComments(body *ast.BlockStmt, comments []*ast.CommentGroup) bool {
	if body == nil {
		return true
	}
	for _, item := range body.List {
		if _, ok := item.(*ast.ExprStmt); ok {
			return false
		}
		if _, ok := item.(*ast.AssignStmt); ok {
			return false
		}
		if _, ok := item.(*ast.ReturnStmt); ok {
			return false
		}
		if _, ok := item.(*ast.IfStmt); ok {
			return false
		}
		if _, ok := item.(*ast.ForStmt); ok {
			return false
		}
		if _, ok := item.(*ast.RangeStmt); ok {
			return false
		}
		if _, ok := item.(*ast.ExprStmt); ok {
			return false
		}
	}
	return true
}

// FormatModType returns a human-readable string for a modification type.
func FormatModType(m ModificationType) string {
	switch m {
	case ModificationSignature:
		return "signature"
	case ModificationBody:
		return "body"
	case ModificationComment:
		return "comment"
	case ModificationImport:
		return "import"
	case ModificationExport:
		return "export"
	default:
		return "unknown"
	}
}

// FormatKind returns a human-readable string for a symbol kind.
func FormatKind(k SymbolKind) string {
	switch k {
	case KindFunc:
		return "function"
	case KindType:
		return "type"
	case KindVar:
		return "variable"
	case KindConst:
		return "constant"
	case KindMethod:
		return "method"
	default:
		return "unknown"
	}
}

// Fset returns the underlying token.FileSet (for debugging/testing).
func (a *Analyzer) Fset() *token.FileSet {
	return a.fset
}

// FormatSummary returns a human-readable summary string.
func FormatSummary(sum *Summary) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("File: %s\n", sum.FilePath))
	b.WriteString(fmt.Sprintf("Modified symbols: %d (exported: %d)\n", sum.ModifiedCount, sum.ExportedCount))
	b.WriteString(fmt.Sprintf("Risk score: %.2f\n", sum.RiskScore))
	if len(sum.RiskReasons) > 0 {
		b.WriteString("Risk factors:\n")
		for _, r := range sum.RiskReasons {
			b.WriteString(fmt.Sprintf("  - %s\n", r))
		}
	}
	if len(sum.Symbols) > 0 {
		b.WriteString("Symbols:\n")
		for _, s := range sum.Symbols {
			b.WriteString(fmt.Sprintf("  %s %s (%s) %s\n",
				FormatKind(s.Kind), s.Name,
				formatExported(s.Exported),
				FormatModType(s.ModType)))
			if len(s.Callers) > 0 {
				b.WriteString(fmt.Sprintf("    called by: %s\n", strings.Join(s.Callers, ", ")))
			}
			if len(s.Callees) > 0 {
				b.WriteString(fmt.Sprintf("    calls: %s\n", strings.Join(s.Callees, ", ")))
			}
		}
	}
	return b.String()
}

func formatExported(exp bool) string {
	if exp {
		return "exported"
	}
	return "internal"
}

// Pretty print of a symbol (implements fmt.Stringer for debugging).
func (s Symbol) String() string {
	return fmt.Sprintf("Symbol{Name:%s Kind:%s Exported:%v ModType:%s Risk:%.2f}",
		s.Name, FormatKind(s.Kind), s.Exported, FormatModType(s.ModType), 0.0)
}
