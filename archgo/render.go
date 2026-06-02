package archgo

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"text/template"
)

// RenderArchGoTemplate executes the embedded Go test template with the
// provided template data and returns the rendered, gofmt-formatted file
// contents. If formatting fails the unformatted source is returned alongside
// the error so the caller can inspect the problem.
func RenderArchGoTemplate(td *templateData) ([]byte, error) {
	tmpl, err := template.New("archgo").Parse(archGoTemplateFile)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var b bytes.Buffer
	if err := tmpl.Execute(&b, td); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	src, err := format.Source(b.Bytes())
	if err != nil {
		return b.Bytes(), fmt.Errorf("formatting generated source: %w", err)
	}
	return src, nil
}

//go:embed test.tmpl
var archGoTemplateFile string
