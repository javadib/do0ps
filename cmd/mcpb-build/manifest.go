package main

import (
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

// buildManifest assembles the manifest for one target platform.
//
// Ordered map literals would be nicer, but the spec pins no field order and
// encoding/json sorts map keys, so a typed struct keeps the output stable and
// diffable across builds.
func buildManifest(version, goos string) manifest {
	binary := bundleBinaryPath + exeSuffix(goos)
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
		Documentation: repositoryURL + "/blob/develop/docs/mcp-bundle.md",
		Support:       repositoryURL + "/issues",
		Server: server{
			Type: "binary",
			// Both paths are spelled out rather than left to the host app.
			//
			// ${__dirname} is required: Claude Desktop does not resolve a
			// relative command against the extension directory, it spawns the
			// string as given and fails with ENOENT.
			//
			// The .exe is required for the same reason. The bundle spec says
			// hosts append it on Windows, but Claude Desktop 1.32885 spawned
			// "server/do0ps" verbatim there — no extension added. Naming the
			// real file is what actually works, and each bundle targets one
			// platform, so nothing is guessed here.
			EntryPoint: binary,
			MCPConfig: mcpConfig{
				Command: "${__dirname}/" + binary,
				Args:    []string{"--stdio"},
				// Filled in by the host app from the settings the user typed;
				// both are optional, and empty means "run the server inside
				// this bundle" (see user_config below).
				Env: map[string]string{
					"DO0PS_SERVER_URL": "${user_config.server_url}",
					"DO0PS_AUTH_TOKEN": "${user_config.auth_token}",
				},
			},
		},
		UserConfig: map[string]userConfigField{
			"server_url": {
				Type:  "string",
				Title: "do0ps server URL",
				Description: "Optional. The MCP endpoint of a self-hosted do0ps server, including the /mcp path — " +
					"for example https://do0ps.example.com/mcp. Leave this empty to run the server inside this " +
					"extension instead, which needs no server and no token.",
				Required: false,
				Default:  "",
			},
			"auth_token": {
				Type:  "string",
				Title: "Access token",
				Description: "The bearer token for that server, from its MCP_AUTH_TOKENS allow-list. Required only " +
					"when a server URL is set above. This is not your hosting provider API key — that one is never " +
					"stored, you give it to the assistant per request.",
				Required:  false,
				Sensitive: true,
				Default:   "",
			},
		},
		Tools:    toolSummaries(),
		Keywords: []string{"mcp", "devops", "hosting", "dns", "cdn", "ssl", "vps", "parspack", "iran"},
		Compatibility: compatibility{
			Platforms: []string{platforms[goos]},
			// A static Go binary carries its own runtime, so there is nothing
			// for the host app to install or version-check.
			Runtimes: map[string]string{},
		},
	}
}

// toolSummaries lists the registered tools for the manifest.
//
// Reading them out of the adapter's own registry is the point: the manifest a
// chat client sees at install time cannot drift from the tools the binary
// serves. The zero UseCases is deliberate — Tools only captures the use cases
// inside handler closures, and no handler runs here.
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

	// UserConfig declares the fields the host app asks the user to fill in and
	// substitutes into mcp_config above.
	UserConfig map[string]userConfigField `json:"user_config,omitempty"`
}

// userConfigField is one setting shown in the client's extension settings UI.
type userConfigField struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive,omitempty"`
	Default     string `json:"default,omitempty"`
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
