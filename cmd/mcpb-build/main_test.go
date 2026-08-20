package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestParseTargets(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		got, err := parseTargets("")
		if err != nil {
			t.Fatalf("parseTargets(\"\") error = %v", err)
		}
		if len(got) != len(defaultTargets) {
			t.Errorf("got %d targets, want %d", len(got), len(defaultTargets))
		}
	})

	t.Run("host", func(t *testing.T) {
		got, err := parseTargets("host")
		if err != nil {
			t.Fatalf("parseTargets(host) error = %v", err)
		}
		want := target{goos: runtime.GOOS, goarch: runtime.GOARCH}
		if len(got) != 1 || got[0] != want {
			t.Errorf("parseTargets(host) = %v, want [%v]", got, want)
		}
	})

	t.Run("explicit list", func(t *testing.T) {
		got, err := parseTargets("linux/amd64, darwin/arm64")
		if err != nil {
			t.Fatalf("parseTargets error = %v", err)
		}
		want := []target{{goos: "linux", goarch: "amd64"}, {goos: "darwin", goarch: "arm64"}}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("parseTargets = %v, want %v", got, want)
		}
	})

	for _, spec := range []string{"linux", "linux/", "/amd64", "freebsd/amd64", ","} {
		t.Run("rejects "+spec, func(t *testing.T) {
			if _, err := parseTargets(spec); err == nil {
				t.Errorf("parseTargets(%q) succeeded, want an error", spec)
			}
		})
	}
}

// TestManifestSpawnPath guards the two things that made Claude Desktop fail
// with "spawn server/do0ps ENOENT": a command relative to nothing in
// particular, and a Windows binary named without its extension.
func TestManifestSpawnPath(t *testing.T) {
	cases := []struct {
		goos         string
		wantCommand  string
		wantEntry    string
		wantPlatform string
	}{
		{goos: "windows", wantCommand: "${__dirname}/server/do0ps.exe", wantEntry: "server/do0ps.exe", wantPlatform: "win32"},
		{goos: "darwin", wantCommand: "${__dirname}/server/do0ps", wantEntry: "server/do0ps", wantPlatform: "darwin"},
		{goos: "linux", wantCommand: "${__dirname}/server/do0ps", wantEntry: "server/do0ps", wantPlatform: "linux"},
	}

	for _, tc := range cases {
		t.Run(tc.goos, func(t *testing.T) {
			m := buildManifest("1.2.3", tc.goos)

			// Hosts spawn this string as given; without ${__dirname} it
			// resolves against a working directory nobody chose.
			if m.Server.MCPConfig.Command != tc.wantCommand {
				t.Errorf("command = %q, want %q", m.Server.MCPConfig.Command, tc.wantCommand)
			}
			if m.Server.EntryPoint != tc.wantEntry {
				t.Errorf("entry_point = %q, want %q", m.Server.EntryPoint, tc.wantEntry)
			}
			if len(m.Compatibility.Platforms) != 1 || m.Compatibility.Platforms[0] != tc.wantPlatform {
				t.Errorf("platforms = %v, want [%s]", m.Compatibility.Platforms, tc.wantPlatform)
			}
		})
	}
}

// TestManifestUserConfig checks that the settings a user fills in actually
// reach the process: a field with no matching env substitution is a field that
// silently does nothing.
func TestManifestUserConfig(t *testing.T) {
	m := buildManifest("1.2.3", "linux")

	for _, field := range []string{"server_url", "auth_token"} {
		declared, ok := m.UserConfig[field]
		if !ok {
			t.Fatalf("user_config has no %q field", field)
		}
		if declared.Title == "" || declared.Description == "" {
			t.Errorf("user_config.%s needs a title and a description: %+v", field, declared)
		}

		want := "${user_config." + field + "}"
		found := false
		for _, value := range m.Server.MCPConfig.Env {
			if value == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no env entry substitutes %s; env = %v", want, m.Server.MCPConfig.Env)
		}
	}

	// Neither is required: an empty pair means "run the server in-process",
	// which is the zero-configuration install.
	if m.UserConfig["server_url"].Required || m.UserConfig["auth_token"].Required {
		t.Error("neither settings field may be required: the local mode needs neither")
	}
	// The token is a credential; a host that renders it in plain text is a
	// shoulder-surfing problem.
	if !m.UserConfig["auth_token"].Sensitive {
		t.Error("auth_token must be marked sensitive")
	}
}

func TestManifestMatchesToolRegistry(t *testing.T) {
	m := buildManifest("1.2.3", "windows")

	if m.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", m.Version)
	}
	if len(m.Server.MCPConfig.Args) == 0 || m.Server.MCPConfig.Args[0] != "--stdio" {
		t.Errorf("args = %v, want the server started with --stdio", m.Server.MCPConfig.Args)
	}

	// The manifest exists to describe the served tools; an empty list would
	// install as an extension with nothing in it.
	if len(m.Tools) == 0 {
		t.Fatal("manifest lists no tools")
	}
	for _, tool := range m.Tools {
		if tool.Name == "" || tool.Description == "" {
			t.Errorf("tool %+v needs both a name and a description", tool)
		}
	}
}

func TestWriteBundle(t *testing.T) {
	dir := t.TempDir()

	binary := filepath.Join(dir, "do0ps")
	if err := os.WriteFile(binary, []byte("binary"), 0o600); err != nil {
		t.Fatalf("writing the fake binary: %v", err)
	}

	path := filepath.Join(dir, "test.mcpb")
	err := writeBundle(path, []bundleFile{
		{name: "manifest.json", content: []byte(`{"name":"do0ps"}`)},
		{name: bundleBinaryPath, source: binary, executable: true},
	}, time.Now())
	if err != nil {
		t.Fatalf("writeBundle: %v", err)
	}

	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("opening the bundle: %v", err)
	}
	defer archive.Close()

	entries := make(map[string]*zip.File, len(archive.File))
	for _, f := range archive.File {
		entries[f.Name] = f
	}

	// manifest.json must sit at the archive root: it is the one file the spec
	// requires a host app to find.
	manifestEntry, ok := entries["manifest.json"]
	if !ok {
		t.Fatalf("bundle has no manifest.json at the root; entries: %v", entries)
	}
	var decoded map[string]any
	body := readEntry(t, manifestEntry)
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Errorf("manifest.json is not valid JSON: %v", err)
	}

	// The execute bit is written explicitly rather than taken from the host
	// filesystem, so bundles packed on Windows still run on macOS and Linux.
	server, ok := entries[bundleBinaryPath]
	if !ok {
		t.Fatalf("bundle has no %s; entries: %v", bundleBinaryPath, entries)
	}
	if mode := server.Mode(); mode&0o111 == 0 {
		t.Errorf("%s mode = %v, want the execute bits set", bundleBinaryPath, mode)
	}
}

func readEntry(t *testing.T, f *zip.File) []byte {
	t.Helper()

	rc, err := f.Open()
	if err != nil {
		t.Fatalf("opening %s: %v", f.Name, err)
	}
	defer rc.Close()

	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading %s: %v", f.Name, err)
	}
	return body
}
