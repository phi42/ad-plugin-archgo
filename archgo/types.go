package archgo

// depRuleData holds template data for a single arch-go DependenciesRule entry.
type depRuleData struct {
	Package   string
	AllowOnly []string
	Forbid    []string
}

// cycleRuleData holds template data for a single arch-go CyclesRule entry.
type cycleRuleData struct {
	Package string
}

// templateData holds template data for a full ADR's generated Go test file.
type templateData struct {
	ModulePath   string
	AdrID        string
	AdrTitle     string
	DepRules     []depRuleData
	CycleRules   []cycleRuleData
	SkippedRules []string
	HasRules     bool
}
