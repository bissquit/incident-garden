package notifications

import (
	"bytes"
	"embed"
	"fmt"
	"html"
	"strings"
	"text/template"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//go:embed templates/*.tmpl
var templatesFS embed.FS

// Renderer renders notifications from templates.
type Renderer struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

// NewRenderer creates a new renderer and loads all templates.
func NewRenderer() (*Renderer, error) {
	funcMap := template.FuncMap{
		"title":          titleCase,
		"upper":          strings.ToUpper,
		"lower":          strings.ToLower,
		"formatTime":     formatTime,
		"formatDuration": formatDuration,
		"statusEmoji":    statusEmoji,
		"severityEmoji":  severityEmoji,
		"typeEmoji":      typeEmoji,
		"escapeHTML":     html.EscapeString,
	}

	r := &Renderer{
		templates: make(map[string]*template.Template),
		funcMap:   funcMap,
	}

	// Load all templates
	channelTypes := []string{"email", "telegram", "mattermost"}
	messageTypes := []string{"initial", "update", "resolved", "completed", "cancelled"}

	for _, channel := range channelTypes {
		for _, msg := range messageTypes {
			name := fmt.Sprintf("%s_%s", channel, msg)
			filename := fmt.Sprintf("templates/%s.tmpl", name)

			content, err := templatesFS.ReadFile(filename)
			if err != nil {
				return nil, fmt.Errorf("read template %s: %w", filename, err)
			}

			tmpl, err := template.New(name).Funcs(funcMap).Parse(string(content))
			if err != nil {
				return nil, fmt.Errorf("parse template %s: %w", name, err)
			}

			r.templates[name] = tmpl
		}
	}

	return r, nil
}

// Render renders a notification payload for the specified channel type.
// Returns subject and body.
func (r *Renderer) Render(channelType domain.ChannelType, payload NotificationPayload) (subject, body string, err error) {
	subject = r.renderSubject(payload)

	templateName := fmt.Sprintf("%s_%s", channelType, payload.MessageType)
	tmpl, ok := r.templates[templateName]
	if !ok {
		return "", "", fmt.Errorf("template not found: %s", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, payload); err != nil {
		return "", "", fmt.Errorf("execute template %s: %w", templateName, err)
	}

	body = strings.TrimSpace(buf.String())
	return subject, body, nil
}

// renderSubject generates the notification subject line.
func (r *Renderer) renderSubject(payload NotificationPayload) string {
	var prefix string
	switch payload.MessageType {
	case MessageTypeInitial:
		if payload.Event.Type == "incident" {
			prefix = "Incident"
		} else {
			prefix = "Scheduled Maintenance"
		}
	case MessageTypeUpdate:
		prefix = "Update"
	case MessageTypeResolved:
		prefix = "Resolved"
	case MessageTypeCompleted:
		prefix = "Completed"
	case MessageTypeCancelled:
		prefix = "Cancelled"
	default:
		prefix = "Notification"
	}

	return fmt.Sprintf("[%s] %s", prefix, payload.Event.Title)
}

// Template functions

var titleCaser = cases.Title(language.English)

func titleCase(s string) string {
	return titleCaser.String(s)
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("Jan 2, 2006 15:04 UTC")
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}

func statusEmoji(status string) string {
	switch strings.ToLower(status) {
	case "investigating":
		return "🔍"
	case "identified":
		return "🔎"
	case "monitoring":
		return "👀"
	case "resolved":
		return "✅"
	case "scheduled":
		return "📅"
	case "in_progress":
		return "🔧"
	case "completed":
		return "✅"
	default:
		return "📋"
	}
}

func severityEmoji(severity string) string {
	switch strings.ToLower(severity) {
	case "minor":
		return "🟡"
	case "major":
		return "🟠"
	case "critical":
		return "🔴"
	default:
		return "⚪"
	}
}

func typeEmoji(eventType string) string {
	switch strings.ToLower(eventType) {
	case "incident":
		return "🔴"
	case "maintenance":
		return "🔧"
	default:
		return "📋"
	}
}
