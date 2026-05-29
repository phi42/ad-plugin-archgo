package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	"github.com/phi42/ad-enforcement-tool/rule"
	"github.com/phi42/ad-plugin-arch-go/domain"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

var rootCmd = &cobra.Command{
	Use:   "arch-go",
	Short: "Arch-Go code generator for ADR-based DSL rules",
	Run: func(cmd *cobra.Command, args []string) {
		setupPluginLogger()
		if err := run(); err != nil {
			slog.Error("plugin failed", "error", err)
			os.Exit(1)
		}
	},
}

func setupPluginLogger() {
	level := slog.LevelInfo
	skipWarn := false
	switch os.Getenv("ADE_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "quiet":
		level = slog.LevelError
	case "no-warnings":
		skipWarn = true
	}
	slog.SetDefault(slog.New(newCLIHandler(os.Stderr, level, skipWarn)))
}

func Execute() {
	if len(os.Args) == 2 && os.Args[1] == "--info" {
		fmt.Println(`{"modes":["compile"],"config_prefix":"arch-go"}`)
		os.Exit(0)
	}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	// read and parse intermediate representation
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	var spec rule.SpecIR
	if err := proto.Unmarshal(payload, &spec); err != nil {
		return fmt.Errorf("cannot unmarshal protobuf SpecIR: %w", err)
	}

	// Warn about any rules this plugin does not handle. arch-go only
	// generates tests for code rules; file and custom rules are skipped.
	for _, r := range spec.Rules {
		if r.GetIsFileRule() || r.GetIsCustomRule() {
			slog.Warn(fmt.Sprintf("rule %q skipped (arch-go handles code rules only)", r.GetName()))
		}
	}

	outDir := spec.GetPluginConfig()["output-dir"]
	if outDir == "" {
		outDir = "."
	}

	// Detect the Go module path from the target project's go.mod.
	// Walk up from the output directory so the generated config.Load() call
	// references the correct module, not the ADE module.
	modulePath, err := domain.DetectModulePath(outDir)
	if err != nil {
		return fmt.Errorf("cannot detect module path from %q (no go.mod found): %w", outDir, err)
	}

	// build template data based on IR and parse with template file
	td, err := domain.BuildTemplateData(&spec, modulePath)
	if err != nil {
		return fmt.Errorf("building template data: %w", err)
	}

	output, err := domain.ParseTemplate(td)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	// write output to file
	adr := spec.GetAdr()
	adrID := "UNKNOWN"
	if adr != nil && adr.GetId() != "" {
		adrID = adr.GetId()
	}

	fileName := fmt.Sprintf("ADR_%s_archgo_test.go", sanitizeFileToken(adrID))

	outPath := filepath.Join(outDir, fileName)
	if err := os.WriteFile(outPath, output, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", outPath, err)
	}

	slog.Info(fmt.Sprintf("generated %s for rules in ADR [%s]", filepath.Base(outPath), adr.Title))
	return nil
}

var nonFileToken = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func sanitizeFileToken(s string) string {
	if s == "" {
		return "UNKNOWN"
	}
	return nonFileToken.ReplaceAllString(s, "_")
}
