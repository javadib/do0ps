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

func TestManifestMatchesToolRegistry(t *testing.T) {
	m := buildManifest("1.2.3", platforms["windows"])

	if m.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", m.Version)
	}
	if m.Server.MCPConfig.Command != bundleBinaryPath {
		t.Errorf("command = %q, want %q", m.Server.MCPConfig.Command, bundleBinaryPath)
	}
	// The host app appends .exe itself on Windows; naming it here would give
	// the client do0ps.exe.exe to run.
	if filepath.Ext(m.Server.MCPConfig.Command) != "" {
		t.Errorf("command = %q, want no file extension", m.Server.MCPConfig.Command)
	}
	if len(m.Server.MCPConfig.Args) == 0 || m.Server.MCPConfig.Args[0] != "--stdio" {
		t.Errorf("args = %v, want the server started with --stdio", m.Server.MCPConfig.Args)
	}
	if len(m.Compatibility.Platforms) != 1 || m.Compatibility.Platforms[0] != "win32" {
		t.Errorf("platforms = %v, want [win32]", m.Compatibility.Platforms)
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
