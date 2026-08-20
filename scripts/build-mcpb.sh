#!/usr/bin/env bash
#
# Build installable MCP bundles (.mcpb) of the do0ps MCP server.
#
# A bundle is a zip with manifest.json at its root and the server next to it.
# The chat client unpacks it and spawns the binary over stdio, so each bundle
# ships one statically linked binary for one target -- there is no runtime for
# the user to install, and nothing to configure by hand.
#
# The bundle spec covers per-OS binaries but has no notion of CPU architecture,
# so this builds one bundle per GOOS/GOARCH pair rather than one fat bundle
# that could pick the wrong binary on Apple silicon.
#
# Usage:
#   scripts/build-mcpb.sh [version] [target ...]
#
#   version   Semantic version without a leading v. Defaults to the current git
#             tag, or 0.0.0-dev+<short sha> when the checkout is not on a tag.
#   target    GOOS/GOARCH pairs. Defaults to $default_targets below.
#
# Output: dist/mcpb/do0ps-<version>-<goos>-<goarch>.mcpb
set -euo pipefail

default_targets=(darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64)

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

out_dir="dist/mcpb"
work_root="dist/.mcpb-work"

# resolve_version prefers an explicit argument, then an exact git tag, and
# falls back to a commit-stamped dev version so local builds are still
# identifiable once installed.
resolve_version() {
	if [ -n "${1:-}" ]; then
		printf '%s' "${1#v}"
		return
	fi
	local tag
	if tag="$(git describe --tags --exact-match 2>/dev/null)"; then
		printf '%s' "${tag#v}"
		return
	fi
	printf '0.0.0-dev+%s' "$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
}

version="$(resolve_version "${1:-}")"
if [ $# -gt 0 ]; then
	shift
fi

targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
	targets=("${default_targets[@]}")
fi

if ! command -v zip >/dev/null 2>&1; then
	echo "build-mcpb: zip is required to pack a bundle" >&2
	exit 1
fi

rm -rf "$work_root"
mkdir -p "$out_dir" "$work_root"

echo "Building do0ps MCP bundles, version $version"

for target in "${targets[@]}"; do
	goos="${target%%/*}"
	goarch="${target##*/}"
	if [ "$goos" = "$target" ] || [ -z "$goarch" ]; then
		echo "build-mcpb: target must look like linux/amd64, got $target" >&2
		exit 1
	fi

	binary="do0ps"
	if [ "$goos" = "windows" ]; then
		binary="do0ps.exe"
	fi

	stage="$work_root/$goos-$goarch"
	mkdir -p "$stage/server"

	# CGO stays off so the binary is fully static (the SQLite driver is pure Go
	# for the same reason) and runs on a machine with no toolchain installed.
	# -trimpath keeps local build paths out of the shipped binary; -s -w drops
	# the symbol table, which is dead weight in a bundle.
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
		go build -trimpath -ldflags "-s -w -X main.version=$version" \
		-o "$stage/server/$binary" ./cmd/server

	# The manifest is generated from the tool registry in the binary's own
	# source, so the two can never disagree about which tools exist.
	go run ./cmd/mcpb-manifest -version "$version" -os "$goos" -out "$stage/manifest.json"

	contents=(manifest.json server)
	if [ -f docs/mcp-bundle.md ]; then
		cp docs/mcp-bundle.md "$stage/README.md"
		contents+=(README.md)
	fi

	bundle="$repo_root/$out_dir/do0ps-$version-$goos-$goarch.mcpb"
	rm -f "$bundle"
	# -X drops platform extra fields: the archive stays a plain zip that any
	# host app can unpack, with manifest.json at the root.
	(cd "$stage" && zip -q -r -X "$bundle" "${contents[@]}")

	echo "  $out_dir/do0ps-$version-$goos-$goarch.mcpb"
done

rm -rf "$work_root"
