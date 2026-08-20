// Command mcpb-build packs the do0ps MCP server into installable MCP bundles
// (.mcpb) -- the artifact an end user drops into a chat client.
//
// It is build tooling, not part of the running server. It shells out to `go
// build` once per target and writes each archive with archive/zip, so the only
// prerequisite is the Go toolchain itself: no bash, no zip binary, no make.
// That is deliberate -- it has to run the same way on Windows, macOS and
// Linux. See docs/mcp-bundle.md.
//
// Usage:
//
//	go run ./cmd/mcpb-build                        # every default target
//	go run ./cmd/mcpb-build -targets host          # just this machine
//	go run ./cmd/mcpb-build -version 1.2.3
//	go run ./cmd/mcpb-build -print-manifest -targets linux/amd64
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// defaultTargets are the GOOS/GOARCH pairs a release ships.
//
// The bundle spec distinguishes operating systems but has no notion of CPU
// architecture, so each pair gets its own bundle rather than one archive that
// could hand an Intel binary to an Apple silicon machine.
var defaultTargets = []string{
	"darwin/arm64",
	"darwin/amd64",
	"linux/amd64",
	"linux/arm64",
	"windows/amd64",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mcpb-build: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	versionFlag := flag.String("version", "", "bundle version without a leading v; defaults to the current git tag, or a commit-stamped dev version")
	targetsFlag := flag.String("targets", "", `comma-separated GOOS/GOARCH pairs, or "host" for this machine; defaults to every released target`)
	outFlag := flag.String("out", filepath.Join("dist", "mcpb"), "directory to write the bundles into")
	printManifest := flag.Bool("print-manifest", false, "print the manifest.json for the first target and exit, without building anything")
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		return err
	}

	targets, err := parseTargets(*targetsFlag)
	if err != nil {
		return err
	}

	version := *versionFlag
	if version == "" {
		version = resolveVersion(root)
	}
	version = strings.TrimPrefix(version, "v")

	if *printManifest {
		encoded, err := encodeManifest(buildManifest(version, platforms[targets[0].goos]))
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(encoded)
		return err
	}

	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("the Go toolchain is required to build a bundle, and `go` is not on PATH: %w", err)
	}

	outDir := *outFlag
	if !filepath.IsAbs(outDir) {
		outDir = filepath.Join(root, outDir)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating the output directory %s: %w", outDir, err)
	}

	fmt.Printf("Building do0ps MCP bundles, version %s\n", version)

	// One timestamp for every entry in this run, so two bundles built from the
	// same tree differ only where their contents differ.
	stamp := time.Now()

	for _, t := range targets {
		path, err := buildBundle(root, goBin, outDir, version, t, stamp)
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		fmt.Printf("  %s\n", filepath.ToSlash(rel))
	}
	return nil
}

// target is one GOOS/GOARCH pair to build for.
type target struct {
	goos   string
	goarch string
}

func (t target) String() string { return t.goos + "/" + t.goarch }

// buildBundle compiles the server for one target and packs it into a bundle,
// returning the archive's path.
func buildBundle(root, goBin, outDir, version string, t target, stamp time.Time) (string, error) {
	staging, err := os.MkdirTemp("", "mcpb-build-")
	if err != nil {
		return "", fmt.Errorf("creating a staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	binaryName := "do0ps"
	if t.goos == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(staging, binaryName)

	if err := compile(root, goBin, binaryPath, version, t); err != nil {
		return "", err
	}

	encoded, err := encodeManifest(buildManifest(version, platforms[t.goos]))
	if err != nil {
		return "", err
	}

	files := []bundleFile{
		{name: "manifest.json", content: encoded},
		// The archive path stays "server/do0ps" even for the .exe: the host app
		// appends the extension itself on Windows, which is why the manifest
		// names it without one.
		{name: bundleBinaryPath + exeSuffix(t.goos), source: binaryPath, executable: true},
	}

	// The install docs travel inside the bundle, so an unpacked copy explains
	// itself. Optional: a partial checkout should still be able to build.
	readme := filepath.Join(root, "docs", "mcp-bundle.md")
	if _, err := os.Stat(readme); err == nil {
		files = append(files, bundleFile{name: "README.md", source: readme})
	}

	path := filepath.Join(outDir, fmt.Sprintf("do0ps-%s-%s-%s.mcpb", version, t.goos, t.goarch))
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("replacing %s: %w", path, err)
	}
	if err := writeBundle(path, files, stamp); err != nil {
		return "", err
	}
	return path, nil
}

// compile cross-compiles cmd/server for one target.
//
// CGO stays off so the binary is fully static (the SQLite driver is pure Go for
// the same reason) and runs on a machine with no toolchain installed.
// -trimpath keeps local build paths out of the shipped binary; -s -w drops the
// symbol table, which is dead weight in a bundle.
func compile(root, goBin, out, version string, t target) error {
	cmd := exec.Command(goBin, "build",
		"-trimpath",
		"-ldflags", "-s -w -X main.version="+version,
		"-o", out,
		"./cmd/server",
	)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+t.goos,
		"GOARCH="+t.goarch,
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building cmd/server for %s: %w", t, err)
	}
	return nil
}

func exeSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

// parseTargets turns the -targets flag into the list to build.
func parseTargets(raw string) ([]target, error) {
	specs := defaultTargets
	switch {
	case raw == "host":
		specs = []string{runtime.GOOS + "/" + runtime.GOARCH}
	case raw != "":
		specs = strings.Split(raw, ",")
	}

	targets := make([]target, 0, len(specs))
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}

		goos, goarch, ok := strings.Cut(spec, "/")
		if !ok || goos == "" || goarch == "" {
			return nil, fmt.Errorf("target %q must look like linux/amd64", spec)
		}
		if _, supported := platforms[goos]; !supported {
			return nil, fmt.Errorf("target %q: the MCP bundle spec covers darwin, linux and windows only", spec)
		}
		targets = append(targets, target{goos: goos, goarch: goarch})
	}

	if len(targets) == 0 {
		return nil, errors.New("no targets to build")
	}
	return targets, nil
}

// repoRoot walks up from the working directory to the module root, so the
// build works from any subdirectory rather than only from the repository root.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locating the working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod found in any parent directory: run this from inside the repository")
		}
		dir = parent
	}
}

// resolveVersion prefers an exact git tag and falls back to a commit-stamped
// dev version, so a locally built bundle is still identifiable once installed.
// git is optional: a checkout without it still builds.
func resolveVersion(root string) string {
	if tag, ok := git(root, "describe", "--tags", "--exact-match"); ok {
		return strings.TrimPrefix(tag, "v")
	}
	if sha, ok := git(root, "rev-parse", "--short", "HEAD"); ok {
		return "0.0.0-dev+" + sha
	}
	return "0.0.0-dev"
}

func git(root string, args ...string) (string, bool) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root

	out, err := cmd.Output()
	if err != nil {
		return "", false
	}

	value := strings.TrimSpace(string(out))
	return value, value != ""
}
