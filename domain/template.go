package domain

import (
	"github.com/phi42/ad-enforcement-tool/rule"

	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// DetectModulePath walks up from startDir looking for a go.mod file and returns
// the module path declared in it. Returns an error if no go.mod is found.
func DetectModulePath(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "go.mod")
		if f, err := os.Open(candidate); err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "module ") {
					return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
				}
			}
			return "", fmt.Errorf("module directive not found in %s", candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no go.mod found in %s or any parent directory", startDir)
}

// depRuleData holds data for one arch-go DependenciesRule entry.
type depRuleData struct {
	Package   string
	AllowOnly []string
	Forbid    []string
}

// cycleRuleData holds data for one arch-go CyclesRule entry.
type cycleRuleData struct {
	Package string
}

// tmplData is the root data passed to the generated test template.
type tmplData struct {
	ModulePath   string
	DepRules     []depRuleData
	CycleRules   []cycleRuleData
	SkippedRules []string // rules not expressible in arch-go
	AdrID        string
	AdrTitle     string
	HasRules     bool
}

// resolveTarget resolves a rule.TargetRef to a Go package pattern suitable for arch-go.
// For scoped subjects ("class in Domain"), the scope pattern is used as the package.
// "regex:" prefixes on inline match patterns are stripped since arch-go uses glob patterns.
func resolveTarget(target *rule.TargetRef, selMap map[string]*rule.Selector) (string, error) {
	if target == nil {
		return "", fmt.Errorf("target is nil")
	}
	// Scoped target: "component in Scope" — use scope as the package boundary
	if target.Scope != nil {
		return resolveTarget(target.Scope, selMap)
	}
	if target.IsInline {
		// Strip "regex:" prefix — arch-go uses path glob patterns, not regex
		return strings.TrimPrefix(target.Value, "regex:"), nil
	}
	// Named selector reference
	if s, ok := selMap[target.Value]; ok {
		return s.Pattern, nil
	}
	return "", fmt.Errorf("unknown selector %q", target.Value)
}

// BuildTemplateData converts a rule.Spec into template data for arch-go test generation.
// modulePath is the Go module path of the target project (from go.mod).
// Rules that cannot be expressed in arch-go are collected in SkippedRules and
// emitted as comments in the generated file.
func BuildTemplateData(spec *rule.Spec, modulePath string) (*tmplData, error) {
	selMap := make(map[string]*rule.Selector, len(spec.Selectors))
	for _, s := range spec.Selectors {
		selMap[s.Name] = s
	}

	var deps []depRuleData
	var cycles []cycleRuleData
	var skipped []string

	for _, r := range spec.Rules {
		// File system rules have no arch-go equivalent
		if r.IsFileRule || len(r.Checks) > 0 {
			skipped = append(skipped, fmt.Sprintf("%q: file system rules are not supported by arch-go", r.Name))
			continue
		}

		switch r.Kind {
		case rule.RuleKind_RULE_DEPEND_ONLY:
			from, err := resolveTarget(r.From, selMap)
			if err != nil {
				return nil, fmt.Errorf("rule %q: cannot resolve subject: %w", r.Name, err)
			}
			var targets []string
			for _, t := range r.Targets {
				p, err := resolveTarget(t, selMap)
				if err != nil {
					return nil, fmt.Errorf("rule %q: cannot resolve target: %w", r.Name, err)
				}
				targets = append(targets, p)
			}
			deps = append(deps, depRuleData{Package: from, AllowOnly: targets})

		case rule.RuleKind_RULE_NOT_DEPEND:
			from, err := resolveTarget(r.From, selMap)
			if err != nil {
				return nil, fmt.Errorf("rule %q: cannot resolve subject: %w", r.Name, err)
			}
			var targets []string
			for _, t := range r.Targets {
				p, err := resolveTarget(t, selMap)
				if err != nil {
					return nil, fmt.Errorf("rule %q: cannot resolve target: %w", r.Name, err)
				}
				targets = append(targets, p)
			}
			deps = append(deps, depRuleData{Package: from, Forbid: targets})

		case rule.RuleKind_RULE_ACYCLIC:
			from, err := resolveTarget(r.From, selMap)
			if err != nil {
				return nil, fmt.Errorf("rule %q: cannot resolve subject: %w", r.Name, err)
			}
			cycles = append(cycles, cycleRuleData{Package: from})

		// The following rule kinds have no arch-go equivalent:
		//   RULE_IMPLEMENT / RULE_NOT_IMPLEMENT — arch-go NamingRule only constrains
		//     naming of implementors, it cannot assert that structs MUST implement an interface
		//   RULE_EXTEND / RULE_NOT_EXTEND — Go has no classical inheritance
		//   RULE_ANNOTATE / RULE_NOT_ANNOTATE — Go has no runtime annotations
		//   RULE_ACCESSED_BY — arch-go only checks outgoing dependencies
		//   RULE_IN / RULE_NOT_IN — arch-go has no package location checks
		//   RULE_MATCH / RULE_NOT_MATCH — arch-go has no regex naming rules
		//   RULE_VISIBILITY — arch-go has no rule.Visibility checks
		//   RULE_TYPE_CONSTRAINT — abstract/sealed/static have no Go equivalent
		default:
			skipped = append(skipped, fmt.Sprintf("%q (%v): not supported by arch-go", r.Name, r.Kind))
		}
	}

	td := &tmplData{
		ModulePath:   modulePath,
		DepRules:     deps,
		CycleRules:   cycles,
		SkippedRules: skipped,
		AdrID:        spec.Adr.Id,
		AdrTitle:     spec.Adr.Title,
		HasRules:     len(deps) > 0 || len(cycles) > 0,
	}
	return td, nil
}

// ParseTemplate renders the arch-go test file from template data and formats
// it with gofmt so the output is always canonical Go source.
func ParseTemplate(td *tmplData) ([]byte, error) {
	tmpl, err := template.New("archgo").Parse(testTemplateFile)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, td); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}
	src, err := format.Source(b.Bytes())
	if err != nil {
		// Return unformatted source so the caller can inspect the problem
		return b.Bytes(), fmt.Errorf("formatting generated source: %w", err)
	}
	return src, nil
}

//go:embed archgo.tmpl
var testTemplateFile string
