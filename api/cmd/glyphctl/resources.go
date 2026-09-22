package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/glyph/api/internal/model"
)

// splitTags parses a comma-separated tag list into a slice, trimming spaces and
// dropping empty entries. An empty input yields an empty (non-nil) slice.
func splitTags(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// setIfPresent copies flag f's value into body[key] using get, but only if the
// flag was actually set on the command line. Used by the update commands so an
// unset flag never overwrites a stored field.
func setIfPresent(fs *flag.FlagSet, name, key string, body map[string]any, get func() any) {
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			body[key] = get()
		}
	})
}

// requireID returns the first positional argument as a resource id, or an
// error when the argument is missing or looks like a flag.
func requireID(args []string, verb string) (string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", fmt.Errorf("%s requires an id argument", verb)
	}
	return args[0], nil
}

// ─── Pages ──────────────────────────────────────────────────────────────────

func cmdPages(cfg *config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: glyphctl pages list|get|create|delete|content-get|content-set")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		if err := cfg.requireToken(); err != nil {
			return err
		}
		var pages []model.Page
		if err := cfg.client.doJSON("GET", "/pages", nil, &pages); err != nil {
			return err
		}
		return renderPages(cfg, pages)
	case "get":
		id, err := requireID(rest, "pages get")
		if err != nil {
			return err
		}
		if err := cfg.requireToken(); err != nil {
			return err
		}
		var p model.Page
		if err := cfg.client.doJSON("GET", "/pages/"+id, nil, &p); err != nil {
			return err
		}
		return renderPage(cfg, &p)
	case "create":
		return cmdPagesCreate(cfg, rest)
	case "delete":
		id, err := requireID(rest, "pages delete")
		if err != nil {
			return err
		}
		if err := cfg.requireToken(); err != nil {
			return err
		}
		if err := cfg.client.doJSON("DELETE", "/pages/"+id, nil, nil); err != nil {
			return err
		}
		fmt.Printf("Deleted page %s\n", id)
		return nil
	case "content-get":
		id, err := requireID(rest, "pages content-get")
		if err != nil {
			return err
		}
		if err := cfg.requireToken(); err != nil {
			return err
		}
		var content model.PageContent
		if err := cfg.client.doJSON("GET", "/pages/"+id+"/content", nil, &content); err != nil {
			return err
		}
		return printJSON(content)
	case "content-set":
		return cmdPagesContentSet(cfg, rest)
	default:
		return fmt.Errorf("unknown pages subcommand %q", sub)
	}
}

func cmdPagesCreate(cfg *config, args []string) error {
	fs := flag.NewFlagSet("glyphctl pages create", flag.ContinueOnError)
	title := fs.String("title", "", "Page title (required)")
	pageType := fs.String("type", "page", "Node type: page or folder")
	parent := fs.String("parent", "", "Parent page/folder UUID")
	org := fs.String("org", "", "Organization UUID")
	tags := fs.String("tags", "", "Comma-separated tags")
	priority := fs.String("priority", "", "Priority: urgent, high, medium, low, none")
	if err := fs.Parse(args); err != nil {
		return errHandled
	}
	if *title == "" {
		return fmt.Errorf("--title is required")
	}
	if err := cfg.requireToken(); err != nil {
		return err
	}

	body := map[string]any{"title": *title, "type": *pageType}
	if *parent != "" {
		body["parentId"] = *parent
	}
	if *org != "" {
		body["orgId"] = *org
	}
	if *tags != "" {
		body["tags"] = splitTags(*tags)
	}
	if *priority != "" {
		body["priority"] = *priority
	}

	var p model.Page
	if err := cfg.client.doJSON("POST", "/pages", body, &p); err != nil {
		return err
	}
	return renderPage(cfg, &p)
}

func cmdPagesContentSet(cfg *config, args []string) error {
	fs := flag.NewFlagSet("glyphctl pages content-set", flag.ContinueOnError)
	file := fs.String("file", "", "Path to a file holding the ProseMirror JSON document ('-' for stdin)")
	inline := fs.String("content", "", "ProseMirror JSON document as a string")
	schemaVersion := fs.Int("schema-version", 1, "ProseMirror schema version")
	if err := fs.Parse(args); err != nil {
		return errHandled
	}
	id, err := requireID(fs.Args(), "pages content-set")
	if err != nil {
		return err
	}
	if err := cfg.requireToken(); err != nil {
		return err
	}

	var raw []byte
	switch {
	case *inline != "":
		raw = []byte(*inline)
	case *file == "-":
		raw, err = readAllStdin()
	case *file != "":
		raw, err = os.ReadFile(*file)
	default:
		return fmt.Errorf("provide the document via --file or --content")
	}
	if err != nil {
		return err
	}
	if !json.Valid(raw) {
		return fmt.Errorf("content is not valid JSON")
	}

	body := map[string]any{
		"content":       json.RawMessage(raw),
		"schemaVersion": *schemaVersion,
	}
	var content model.PageContent
	if err := cfg.client.doJSON("PUT", "/pages/"+id+"/content", body, &content); err != nil {
		return err
	}
	return printJSON(content)
}

// ─── Tasks ──────────────────────────────────────────────────────────────────

func cmdTasks(cfg *config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: glyphctl tasks list|get|create|update|delete")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		if err := cfg.requireToken(); err != nil {
			return err
		}
		var tasks []model.Task
		if err := cfg.client.doJSON("GET", "/tasks", nil, &tasks); err != nil {
			return err
		}
		return renderTasks(cfg, tasks)
	case "get":
		id, err := requireID(rest, "tasks get")
		if err != nil {
			return err
		}
		if err := cfg.requireToken(); err != nil {
			return err
		}
		var t model.Task
		if err := cfg.client.doJSON("GET", "/tasks/"+id, nil, &t); err != nil {
			return err
		}
		return renderTask(cfg, &t)
	case "create":
		return cmdTasksCreate(cfg, rest)
	case "update":
		return cmdTasksUpdate(cfg, rest)
	case "delete":
		id, err := requireID(rest, "tasks delete")
		if err != nil {
			return err
		}
		if err := cfg.requireToken(); err != nil {
			return err
		}
		if err := cfg.client.doJSON("DELETE", "/tasks/"+id, nil, nil); err != nil {
			return err
		}
		fmt.Printf("Deleted task %s\n", id)
		return nil
	default:
		return fmt.Errorf("unknown tasks subcommand %q", sub)
	}
}

func cmdTasksCreate(cfg *config, args []string) error {
	fs := flag.NewFlagSet("glyphctl tasks create", flag.ContinueOnError)
	title := fs.String("title", "", "Task title (required)")
	desc := fs.String("description", "", "Task description")
	status := fs.String("status", "", "Status: todo, in-progress, done, cancelled")
	priority := fs.String("priority", "", "Priority: urgent, high, medium, low, none")
	due := fs.String("due", "", "Due date (YYYY-MM-DD)")
	tags := fs.String("tags", "", "Comma-separated tags")
	org := fs.String("org", "", "Organization UUID")
	if err := fs.Parse(args); err != nil {
		return errHandled
	}
	if *title == "" {
		return fmt.Errorf("--title is required")
	}
	if err := cfg.requireToken(); err != nil {
		return err
	}

	body := map[string]any{"title": *title}
	if *desc != "" {
		body["description"] = *desc
	}
	if *status != "" {
		body["status"] = *status
	}
	if *priority != "" {
		body["priority"] = *priority
	}
	if *due != "" {
		body["dueDate"] = *due
	}
	if *tags != "" {
		body["tags"] = splitTags(*tags)
	}
	if *org != "" {
		body["orgId"] = *org
	}

	var t model.Task
	if err := cfg.client.doJSON("POST", "/tasks", body, &t); err != nil {
		return err
	}
	return renderTask(cfg, &t)
}

func cmdTasksUpdate(cfg *config, args []string) error {
	fs := flag.NewFlagSet("glyphctl tasks update", flag.ContinueOnError)
	title := fs.String("title", "", "New title")
	desc := fs.String("description", "", "New description")
	status := fs.String("status", "", "New status: todo, in-progress, done, cancelled")
	priority := fs.String("priority", "", "New priority: urgent, high, medium, low, none")
	due := fs.String("due", "", "New due date (YYYY-MM-DD)")
	tags := fs.String("tags", "", "Comma-separated tags (replaces existing)")
	if err := fs.Parse(args); err != nil {
		return errHandled
	}
	id, err := requireID(fs.Args(), "tasks update")
	if err != nil {
		return err
	}
	if err := cfg.requireToken(); err != nil {
		return err
	}

	body := map[string]any{}
	setIfPresent(fs, "title", "title", body, func() any { return *title })
	setIfPresent(fs, "description", "description", body, func() any { return *desc })
	setIfPresent(fs, "status", "status", body, func() any { return *status })
	setIfPresent(fs, "priority", "priority", body, func() any { return *priority })
	setIfPresent(fs, "due", "dueDate", body, func() any { return *due })
	setIfPresent(fs, "tags", "tags", body, func() any { return splitTags(*tags) })
	if len(body) == 0 {
		return fmt.Errorf("nothing to update; pass at least one field flag")
	}

	var t model.Task
	if err := cfg.client.doJSON("PATCH", "/tasks/"+id, body, &t); err != nil {
		return err
	}
	return renderTask(cfg, &t)
}

// ─── Lanes ──────────────────────────────────────────────────────────────────

func cmdLanes(cfg *config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: glyphctl lanes list|get|create|delete")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		if err := cfg.requireToken(); err != nil {
			return err
		}
		var lanes []model.Lane
		if err := cfg.client.doJSON("GET", "/lanes", nil, &lanes); err != nil {
			return err
		}
		return renderLanes(cfg, lanes)
	case "get":
		id, err := requireID(rest, "lanes get")
		if err != nil {
			return err
		}
		if err := cfg.requireToken(); err != nil {
			return err
		}
		var l model.Lane
		if err := cfg.client.doJSON("GET", "/lanes/"+id, nil, &l); err != nil {
			return err
		}
		return renderLane(cfg, &l)
	case "create":
		return cmdLanesCreate(cfg, rest)
	case "delete":
		id, err := requireID(rest, "lanes delete")
		if err != nil {
			return err
		}
		if err := cfg.requireToken(); err != nil {
			return err
		}
		if err := cfg.client.doJSON("DELETE", "/lanes/"+id, nil, nil); err != nil {
			return err
		}
		fmt.Printf("Deleted lane %s\n", id)
		return nil
	default:
		return fmt.Errorf("unknown lanes subcommand %q", sub)
	}
}

func cmdLanesCreate(cfg *config, args []string) error {
	fs := flag.NewFlagSet("glyphctl lanes create", flag.ContinueOnError)
	title := fs.String("title", "", "Lane title (required)")
	order := fs.Int("order", 0, "Display order")
	if err := fs.Parse(args); err != nil {
		return errHandled
	}
	if *title == "" {
		return fmt.Errorf("--title is required")
	}
	if err := cfg.requireToken(); err != nil {
		return err
	}

	body := map[string]any{"title": *title, "order": *order}
	var l model.Lane
	if err := cfg.client.doJSON("POST", "/lanes", body, &l); err != nil {
		return err
	}
	return renderLane(cfg, &l)
}

// ─── Templates ────────────────────────────────────────────────────────────────

func cmdTemplates(cfg *config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: glyphctl templates list|get|create|delete")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		if err := cfg.requireToken(); err != nil {
			return err
		}
		var templates []model.Template
		if err := cfg.client.doJSON("GET", "/templates", nil, &templates); err != nil {
			return err
		}
		return renderTemplates(cfg, templates)
	case "get":
		id, err := requireID(rest, "templates get")
		if err != nil {
			return err
		}
		if err := cfg.requireToken(); err != nil {
			return err
		}
		var t model.Template
		if err := cfg.client.doJSON("GET", "/templates/"+id, nil, &t); err != nil {
			return err
		}
		return renderTemplate(cfg, &t)
	case "create":
		return cmdTemplatesCreate(cfg, rest)
	case "delete":
		id, err := requireID(rest, "templates delete")
		if err != nil {
			return err
		}
		if err := cfg.requireToken(); err != nil {
			return err
		}
		if err := cfg.client.doJSON("DELETE", "/templates/"+id, nil, nil); err != nil {
			return err
		}
		fmt.Printf("Deleted template %s\n", id)
		return nil
	default:
		return fmt.Errorf("unknown templates subcommand %q", sub)
	}
}

func cmdTemplatesCreate(cfg *config, args []string) error {
	fs := flag.NewFlagSet("glyphctl templates create", flag.ContinueOnError)
	name := fs.String("name", "", "Template name (required)")
	content := fs.String("content", "", "Template body content")
	titleTemplate := fs.String("title-template", "", "Title template string")
	isDefault := fs.Bool("default", false, "Mark as the default template")
	org := fs.String("org", "", "Organization UUID")
	if err := fs.Parse(args); err != nil {
		return errHandled
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	if err := cfg.requireToken(); err != nil {
		return err
	}

	body := map[string]any{"name": *name, "isDefault": *isDefault}
	if *content != "" {
		body["content"] = *content
	}
	if *titleTemplate != "" {
		body["titleTemplate"] = *titleTemplate
	}
	if *org != "" {
		body["orgId"] = *org
	}

	var t model.Template
	if err := cfg.client.doJSON("POST", "/templates", body, &t); err != nil {
		return err
	}
	return renderTemplate(cfg, &t)
}
