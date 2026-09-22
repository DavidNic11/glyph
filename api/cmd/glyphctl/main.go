// Command glyphctl is a command-line client for the Glyph REST API. It talks to
// a running Glyph server over HTTP, authenticating with an OAuth bearer token,
// and exposes the pages, tasks, lanes and templates resources plus a helper for
// minting a client-credentials token.
//
// Configuration is read from flags or the environment:
//
//	--url    / GLYPH_API_URL    Base URL of the server (default http://localhost:8080)
//	--token  / GLYPH_TOKEN      OAuth bearer access token
//	--json                      Emit raw JSON instead of formatted tables
//
// Global flags must appear before the command, e.g.
//
//	glyphctl --url https://glyph.example.com --token "$TOK" tasks list
package main

import (
	"flag"
	"fmt"
	"os"
)

// config holds the resolved global settings shared by every command.
type config struct {
	client   *Client
	jsonOut  bool
	baseURL  string
	hasToken bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if err != errHandled {
			fmt.Fprintln(os.Stderr, "glyphctl: "+err.Error())
		}
		os.Exit(1)
	}
}

func run(argv []string) error {
	root := flag.NewFlagSet("glyphctl", flag.ContinueOnError)
	root.SetOutput(os.Stderr)
	url := root.String("url", envOr("GLYPH_API_URL", "http://localhost:8080"), "Base URL of the Glyph server")
	token := root.String("token", os.Getenv("GLYPH_TOKEN"), "OAuth bearer access token")
	jsonOut := root.Bool("json", false, "Emit raw JSON instead of formatted output")
	root.Usage = func() { usage(root) }
	if err := root.Parse(argv); err != nil {
		return errHandled
	}

	args := root.Args()
	if len(args) == 0 {
		usage(root)
		return errHandled
	}

	cfg := &config{
		client:   newClient(*url, *token),
		jsonOut:  *jsonOut,
		baseURL:  *url,
		hasToken: *token != "",
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "health":
		return cmdHealth(cfg, rest)
	case "auth":
		return cmdAuth(cfg, rest)
	case "pages":
		return cmdPages(cfg, rest)
	case "tasks":
		return cmdTasks(cfg, rest)
	case "lanes":
		return cmdLanes(cfg, rest)
	case "templates":
		return cmdTemplates(cfg, rest)
	case "help", "-h", "--help":
		usage(root)
		return nil
	default:
		return fmt.Errorf("unknown command %q (run 'glyphctl help')", cmd)
	}
}

func usage(root *flag.FlagSet) {
	fmt.Fprint(os.Stderr, `glyphctl — command-line client for the Glyph API

Usage:
  glyphctl [global flags] <command> [subcommand] [flags]

Commands:
  health                     Check server liveness (no auth required)
  auth token                 Mint a bearer token via the client_credentials grant
  pages    list|get|create|delete|content-get|content-set
  tasks    list|get|create|update|delete
  lanes    list|get|create|delete
  templates list|get|create|delete

Global flags:
`)
	root.PrintDefaults()
	fmt.Fprint(os.Stderr, `
Environment:
  GLYPH_API_URL   Default for --url
  GLYPH_TOKEN     Default for --token

Run 'glyphctl <command>' with no subcommand to see that command's help.
`)
}

// errHandled signals that usage/flag output was already written and main should
// exit non-zero without printing another message.
var errHandled = fmt.Errorf("")

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// requireToken returns an error when no bearer token is configured, keeping the
// message consistent across the authenticated commands.
func (c *config) requireToken() error {
	if !c.hasToken {
		return fmt.Errorf("no token configured; pass --token or set GLYPH_TOKEN (see 'glyphctl auth token')")
	}
	return nil
}
