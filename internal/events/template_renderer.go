package events

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/bissquit/incident-management/internal/domain"
)

// TemplateRenderer renders Go templates for events.
type TemplateRenderer struct{}

// NewTemplateRenderer creates a new template renderer.
func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{}
}

// Render renders a template string with the given data.
func (tr *TemplateRenderer) Render(tmplStr string, data domain.TemplateData) (string, error) {
	funcMap := template.FuncMap{
		"formatTime": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			return t.Format("2006-01-02 15:04:05 MST")
		},
	}

	tmpl, err := template.New("event").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// Validate checks if a template string is valid.
func (tr *TemplateRenderer) Validate(tmplStr string) error {
	_, err := template.New("validation").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("invalid template syntax: %w", err)
	}
	return nil
}
