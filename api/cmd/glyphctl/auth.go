package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// cmdHealth hits the unauthenticated /health endpoint.
func cmdHealth(cfg *config, args []string) error {
	fs := flag.NewFlagSet("glyphctl health", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return errHandled
	}
	body, status, err := cfg.client.getRaw("/health")
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("server unhealthy (status %d): %s", status, strings.TrimSpace(string(body)))
	}
	fmt.Print(string(body))
	if !strings.HasSuffix(string(body), "\n") {
		fmt.Println()
	}
	return nil
}

// cmdAuth dispatches the auth subcommands.
func cmdAuth(cfg *config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: glyphctl auth token [flags]")
	}
	switch args[0] {
	case "token":
		return cmdAuthToken(cfg, args[1:])
	default:
		return fmt.Errorf("unknown auth subcommand %q", args[0])
	}
}

// cmdAuthToken exchanges client credentials for a bearer access token using the
// client_credentials (+subject) grant on POST /oauth/token.
func cmdAuthToken(cfg *config, args []string) error {
	fs := flag.NewFlagSet("glyphctl auth token", flag.ContinueOnError)
	clientID := fs.String("client-id", os.Getenv("GLYPH_CLIENT_ID"), "OAuth client ID (or GLYPH_CLIENT_ID)")
	clientSecret := fs.String("client-secret", os.Getenv("GLYPH_CLIENT_SECRET"), "OAuth client secret (or GLYPH_CLIENT_SECRET)")
	subject := fs.String("subject", "", "User to act as: user UUID or email (required)")
	org := fs.String("org", "", "Organization UUID the token is scoped to (required)")
	scope := fs.String("scope", "", "Space-separated scopes (default: all the client is configured with)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Mint a bearer token via the OAuth client_credentials grant.\n\nUsage:\n  glyphctl auth token --client-id ID --client-secret SECRET --subject USER --org ORG_ID [--scope \"...\"]\n\nFlags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return errHandled
	}
	if *clientID == "" || *clientSecret == "" {
		return fmt.Errorf("--client-id and --client-secret are required (or set GLYPH_CLIENT_ID / GLYPH_CLIENT_SECRET)")
	}
	if *subject == "" {
		return fmt.Errorf("--subject is required")
	}
	if *org == "" {
		return fmt.Errorf("--org is required")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", *clientID)
	form.Set("client_secret", *clientSecret)
	form.Set("subject", *subject)
	form.Set("org_id", *org)
	if *scope != "" {
		form.Set("scope", *scope)
	}

	var resp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		ActingUserID string `json:"acting_user_id"`
		OrgID        string `json:"org_id"`
	}
	if err := cfg.client.postForm("/oauth/token", form, &resp); err != nil {
		return err
	}

	if cfg.jsonOut {
		return printJSON(resp)
	}
	// Default output is just the token so it can be captured directly, e.g.
	//   export GLYPH_TOKEN=$(glyphctl auth token ...)
	fmt.Println(resp.AccessToken)
	fmt.Fprintf(os.Stderr, "# token_type=%s expires_in=%ds scope=%q\n", resp.TokenType, resp.ExpiresIn, resp.Scope)
	return nil
}
