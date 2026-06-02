package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/phi42/ad-enforcement-tool/rule"
	"github.com/phi42/ad-plugin-archgo/archgo"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
)

type pluginInfo struct {
	Modes        []string `json:"modes"`
	ConfigPrefix string   `json:"config_prefix"`
}

var info = pluginInfo{
	Modes:        []string{"compile"},
	ConfigPrefix: "archgo",
}

var rootCmd = &cobra.Command{
	Use:   "Install this plugin using `ade plugin install` and then run it via `ade compile`",
	Short: "archgo code generator for ADR rules (code rules only)",
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	},
}

func Execute() {
	if len(os.Args) == 2 && os.Args[1] == "--info" {
		out, err := json.Marshal(info)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: marshaling plugin info: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
		os.Exit(0)
	}
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
		_ = rootCmd.Help()
		os.Exit(0)
	}
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	// read protobuf Spec from stdin
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	var spec rule.Spec
	if err := proto.Unmarshal(payload, &spec); err != nil {
		return fmt.Errorf("unmarshal Spec protobuf: %w", err)
	}

	var skipped int
	for _, r := range spec.Rules {
		if r.GetIsFileRule() || r.GetIsCustomRule() {
			skipped++
		}
	}
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "warn: %d rule(s) skipped (plugin can only handle code rules)\n", skipped)
	}

	return runCompile(&spec)
}

func runCompile(spec *rule.Spec) error {
	outDir := spec.GetPluginConfig()["output-dir"]
	if outDir == "" {
		outDir = "."
	}

	// Detect the Go module path from the target project's go.mod by walking up
	// from the output directory, so the generated config.Load() call references
	// the target module rather than the ADE module.
	modulePath, err := archgo.DetectModulePath(outDir)
	if err != nil {
		return fmt.Errorf("detecting module path from %q: %w", outDir, err)
	}

	td, err := archgo.BuildArchGoTemplateData(spec, modulePath)
	if err != nil {
		return fmt.Errorf("building template data: %w", err)
	}

	content, err := archgo.RenderArchGoTemplate(td)
	if err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	filename, err := writeGeneratedFile(spec, outDir, content)
	if err != nil {
		return fmt.Errorf("writing generated test to file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "generated %s for rules in ADR [%s]\n", filename, spec.GetAdr().GetTitle())
	return nil
}

// writeGeneratedFile creates outDir if needed and writes content to outDir/filename.
func writeGeneratedFile(spec *rule.Spec, outDir string, content []byte) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", outDir, err)
	}

	filename := archgo.GenFileName(spec.GetAdr().GetId())
	outPath := filepath.Join(outDir, filename)
	if err := os.WriteFile(outPath, content, 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", outPath, err)
	}
	return filename, nil
}
