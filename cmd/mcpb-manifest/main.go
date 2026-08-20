// Command mcpb-manifest generates the manifest.json for an MCP bundle (.mcpb).
//
// It is build tooling, not part of the server: scripts/build-mcpb.sh runs it
// once per target platform while assembling the bundle. The tool list is read
// straight out of the MCP adapter's registry, so the manifest a chat client
// reads at install time can never drift from the tools the binary actually
// serves -- adding a tool to internal/adapters/mcp is enough.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/javadib/do0ps/internal/adapters/mcp"
)

// manifestVersion is the MCP bundle spec revision this generator targets.
// See https://github.com/anthropics/mcpb/blob/main/MANIFEST.md.
const manifestVersion = "0.3"

const repositoryURL = "https://github.com/javadib/do0ps"

// platforms maps GOOS onto the platform identifiers the bundle spec uses.
var platforms = map[string]string{
	"darwin":  "darwin",
	"linux":   "linux",
	"windows": "win32",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mcpb-manifest: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	version := flag.String("version", "0.0.0", "bundle version, without a leading v")
	goos := flag.String("os", "", "target GOOS: darwin, linux or windows")
	out := flag.String("out", "-", `file to write the manifest to, or "-" for stdout`)
	flag.Parse()

	platform, ok := platforms[*goos]
	if !ok {
		return fmt.Errorf("-os must be one of darwin, linux or windows, got %q", *goos)
	}

	encoded, err := json.MarshalIndent(build(*version, platform), "", "  ")
	if err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}
	encoded = append(encoded, '\n')

	if *out == "-" {
		_, err := os.Stdout.Write(encoded)
		return err
	}
	if err := os.WriteFile(*out, encoded, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", *out, err)
	}
	return nil
}

// build assembles the manifest for one target platform.
//
// Ordered map literals would be nicer, but the spec pins no field order and
// encoding/json sorts map keys, so a typed struct keeps the output stable and
// diffable across builds.
func build(version, platform string) manifest {
	return manifest{
		ManifestVersion: manifestVersion,
		Name:            "do0ps",
		DisplayName:     "do0ps",
		Version:         version,
		Description:     "Manage servers, DNS, CDN and SSL at Iranian hosting providers through chat.",
		LongDescription: "do0ps connects your chat assistant to Iranian hosting providers, starting with " +
			"Parspack. Ask it to provision a VPS, point a domain at it, order an SSL certificate, or tune a " +
			"CDN zone, and it calls the provider API for you — including long operations, which it tracks in " +
			"the background so you can ask how they are going.\n\n" +
			"Your provider API key is never stored by this bundle: you pass it as a parameter on each tool " +
			"call, and it lives only in the chat session.",
		Author:        author{Name: "javadib", URL: repositoryURL},
		Repository:    &repository{Type: "git", URL: repositoryURL + ".git"},
		Homepage:      repositoryURL,
		Documentation: repositoryURL + "/blob/step/ph1/docs/mcp-bundle.md",
		Support:       repositoryURL + "/issues",
		Server: server{
			Type: "binary",
			// The host app appends .exe on Windows, so the manifest names the
			// binary the same way on every platform.
			EntryPoint: "server/do0ps",
			MCPConfig: mcpConfig{
				Command: "server/do0ps",
				Args:    []string{"--stdio"},
				Env:     map[string]string{},
			},
		},
		Tools:    toolSummaries(),
		Keywords: []string{"mcp", "devops", "hosting", "dns", "cdn", "ssl", "vps", "parspack", "iran"},
		Compatibility: compatibility{
			Platforms: []string{platform},
			// A static Go binary carries its own runtime, so there is nothing
			// for the host app to install or version-check.
			Runtimes: map[string]string{},
		},
	}
}

// toolSummaries lists the registered tools for the manifest.
//
// The zero UseCases is deliberate: Tools only captures the use cases inside
// handler closures, and no handler runs here -- this reads names and
// descriptions, which are static.
func toolSummaries() []toolSummary {
	registered := mcp.Tools(mcp.UseCases{})

	summaries := make([]toolSummary, 0, len(registered))
	for _, tool := range registered {
		summaries = append(summaries, toolSummary{Name: tool.Name, Description: tool.Description})
	}
	return summaries
}

type manifest struct {
	ManifestVersion string        `json:"manifest_version"`
	Name            string        `json:"name"`
	DisplayName     string        `json:"display_name"`
	Version         string        `json:"version"`
	Description     string        `json:"description"`
	LongDescription string        `json:"long_description"`
	Author          author        `json:"author"`
	Repository      *repository   `json:"repository,omitempty"`
	Homepage        string        `json:"homepage,omitempty"`
	Documentation   string        `json:"documentation,omitempty"`
	Support         string        `json:"support,omitempty"`
	Server          server        `json:"server"`
	Tools           []toolSummary `json:"tools"`
	Keywords        []string      `json:"keywords,omitempty"`
	Compatibility   compatibility `json:"compatibility"`
}

type author struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type repository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type server struct {
	Type       string    `json:"type"`
	EntryPoint string    `json:"entry_point"`
	MCPConfig  mcpConfig `json:"mcp_config"`
}

type mcpConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

type toolSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type compatibility struct {
	Platforms []string          `json:"platforms"`
	Runtimes  map[string]string `json:"runtimes"`
}
