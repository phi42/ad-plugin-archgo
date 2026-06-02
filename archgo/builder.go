package archgo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/phi42/ad-enforcement-tool/rule"
)

// GenFileName returns the canonical output filename for the generated Go test
// file for the given ADR ID.
func GenFileName(adrID string) string {
	sanitized := identRe.ReplaceAllString(adrID, "_")
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		sanitized = "UNKNOWN"
	}
	return fmt.Sprintf("ADR_%s_archgo_test.go", sanitized)
}

// DetectModulePath walks up from startDir looking for a go.mod file and returns
// the module path declared in it.
func DetectModulePath(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if path, ok := readModulePath(filepath.Join(dir, "go.mod")); ok {
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found in %s or any parent directory", startDir)
		}
		dir = parent
	}
}

// readModulePath reads the module directive from a go.mod file. Returns the
// module path and true on success, "" and false if the file cannot be opened.
func readModulePath(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), true
		}
	}
	_ = scanner.Err()
	return "", false
}

// BuildArchGoTemplateData translates a Spec into template data ready for
// rendering into an archgo-based Go test file. modulePath is the Go module
// path of the target project (from go.mod). Rules that cannot be expressed in
// archgo are collected in SkippedRules and emitted as comments in the
// generated file.
func BuildArchGoTemplateData(spec *rule.Spec, modulePath string) (*templateData, error) {
	selMap := make(map[string]*rule.Selector, len(spec.Selectors))
	for _, s := range spec.Selectors {
		selMap[s.Name] = s
	}

	var deps []depRuleData
	var cycles []cycleRuleData
	var skipped []string

	for _, r := range spec.Rules {
		if r.GetIsFileRule() || r.GetIsCustomRule() {
			continue
		}

		switch r.Kind {
		case rule.RuleKind_RULE_DEPEND_ONLY:
			d, err := buildDepRule(r, selMap, true)
			if err != nil {
				return nil, err
			}
			deps = append(deps, d)

		case rule.RuleKind_RULE_NOT_DEPEND:
			d, err := buildDepRule(r, selMap, false)
			if err != nil {
				return nil, err
			}
			deps = append(deps, d)

		case rule.RuleKind_RULE_ACYCLIC:
			from, err := resolveTarget(r.From, selMap)
			if err != nil {
				return nil, fmt.Errorf("rule %q: cannot resolve subject: %w", r.Name, err)
			}
			cycles = append(cycles, cycleRuleData{Package: from})

		// All other rule kinds have no archgo equivalent; see README for details.
		default:
			skipped = append(skipped, fmt.Sprintf("%q (%v): not supported by archgo", r.Name, r.Kind))
		}
	}

	return &templateData{
		ModulePath:   modulePath,
		AdrID:        spec.Adr.GetId(),
		AdrTitle:     spec.Adr.GetTitle(),
		DepRules:     deps,
		CycleRules:   cycles,
		SkippedRules: skipped,
		HasRules:     len(deps) > 0 || len(cycles) > 0,
	}, nil
}

// buildDepRule translates a dependency rule into a depRuleData. If allowOnly is
// true the targets are placed in the AllowOnly list, otherwise in the Forbid list.
func buildDepRule(r *rule.Rule, selMap map[string]*rule.Selector, allowOnly bool) (depRuleData, error) {
	from, err := resolveTarget(r.From, selMap)
	if err != nil {
		return depRuleData{}, fmt.Errorf("rule %q: cannot resolve subject: %w", r.Name, err)
	}
	targets := make([]string, 0, len(r.Targets))
	for _, t := range r.Targets {
		p, err := resolveTarget(t, selMap)
		if err != nil {
			return depRuleData{}, fmt.Errorf("rule %q: cannot resolve target: %w", r.Name, err)
		}
		targets = append(targets, p)
	}
	d := depRuleData{Package: from}
	if allowOnly {
		d.AllowOnly = targets
	} else {
		d.Forbid = targets
	}
	return d, nil
}

// resolveTarget resolves a TargetRef to a Go package pattern suitable for archgo.
// For scoped subjects ("class in Domain") the scope pattern is used as the
// package boundary. "regex:" prefixes on inline match patterns are stripped
// since archgo uses glob patterns.
func resolveTarget(target *rule.TargetRef, selMap map[string]*rule.Selector) (string, error) {
	if target == nil {
		return "", fmt.Errorf("target is nil")
	}
	if target.Scope != nil {
		return resolveTarget(target.Scope, selMap)
	}
	if target.IsInline {
		return strings.TrimPrefix(target.Value, "regex:"), nil
	}
	if s, ok := selMap[target.Value]; ok {
		return s.Pattern, nil
	}
	return "", fmt.Errorf("unknown selector %q", target.Value)
}

var identRe = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
