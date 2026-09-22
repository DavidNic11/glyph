package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/glyph/api/internal/model"
)

// readAllStdin reads the entirety of standard input.
func readAllStdin() ([]byte, error) {
	return io.ReadAll(os.Stdin)
}

// printJSON writes v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// ─── Pages ──────────────────────────────────────────────────────────────────

func renderPages(cfg *config, pages []model.Page) error {
	if cfg.jsonOut {
		return printJSON(pages)
	}
	if len(pages) == 0 {
		fmt.Println("No pages.")
		return nil
	}
	w := newTabWriter()
	fmt.Fprintln(w, "ID\tTYPE\tTITLE\tPRIVATE")
	for _, p := range pages {
		fmt.Fprintf(w, "%s\t%s\t%s\t%t\n", p.ID, dash(string(p.Type)), truncate(p.Title, 50), p.IsPrivate)
	}
	return w.Flush()
}

func renderPage(_ *config, p *model.Page) error {
	return printJSON(p)
}

// ─── Tasks ──────────────────────────────────────────────────────────────────

func renderTasks(cfg *config, tasks []model.Task) error {
	if cfg.jsonOut {
		return printJSON(tasks)
	}
	if len(tasks) == 0 {
		fmt.Println("No tasks.")
		return nil
	}
	w := newTabWriter()
	fmt.Fprintln(w, "ID\tSTATUS\tPRIORITY\tTITLE")
	for _, t := range tasks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.ID, dash(string(t.Status)), dash(string(t.Priority)), truncate(t.Title, 50))
	}
	return w.Flush()
}

func renderTask(cfg *config, t *model.Task) error {
	return printJSON(t)
}

// ─── Lanes ──────────────────────────────────────────────────────────────────

func renderLanes(cfg *config, lanes []model.Lane) error {
	if cfg.jsonOut {
		return printJSON(lanes)
	}
	if len(lanes) == 0 {
		fmt.Println("No lanes.")
		return nil
	}
	w := newTabWriter()
	fmt.Fprintln(w, "ID\tORDER\tTITLE")
	for _, l := range lanes {
		fmt.Fprintf(w, "%s\t%d\t%s\n", l.ID, l.Order, truncate(l.Title, 50))
	}
	return w.Flush()
}

func renderLane(cfg *config, l *model.Lane) error {
	return printJSON(l)
}

// ─── Templates ────────────────────────────────────────────────────────────────

func renderTemplates(cfg *config, templates []model.Template) error {
	if cfg.jsonOut {
		return printJSON(templates)
	}
	if len(templates) == 0 {
		fmt.Println("No templates.")
		return nil
	}
	w := newTabWriter()
	fmt.Fprintln(w, "ID\tNAME\tDEFAULT")
	for _, t := range templates {
		fmt.Fprintf(w, "%s\t%s\t%t\n", t.ID, truncate(t.Name, 50), t.IsDefault)
	}
	return w.Flush()
}

func renderTemplate(cfg *config, t *model.Template) error {
	return printJSON(t)
}
