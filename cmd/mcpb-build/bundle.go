package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// bundleBinaryPath is where the server sits inside the archive. Zip paths are
// always slash-separated, on every platform -- never filepath.Join here.
const bundleBinaryPath = "server/do0ps"

// bundleFile is one entry to pack.
type bundleFile struct {
	// name is the slash-separated path inside the archive.
	name string
	// source is the file on disk to copy from; empty when content is used.
	source string
	// content is the literal bytes to write; used when source is empty.
	content []byte
	// executable marks an entry the host app has to be able to run.
	executable bool
}

// writeBundle packs files into an .mcpb archive at path.
//
// This is a plain zip with manifest.json at the root, per the bundle spec.
// Using archive/zip rather than shelling out to a zip binary is what makes the
// build work the same on Windows, macOS and Linux -- and it lets the unix
// permission bits be set explicitly, so a bundle packed on Windows (which has
// no execute bit) still unpacks into a runnable binary on macOS and Linux.
func writeBundle(path string, files []bundleFile, modified time.Time) (err error) {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating bundle %s: %w", path, err)
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing bundle %s: %w", path, cerr)
		}
	}()

	archive := zip.NewWriter(out)
	for _, file := range files {
		if err := writeBundleEntry(archive, file, modified); err != nil {
			return err
		}
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("finishing bundle %s: %w", path, err)
	}
	return nil
}

func writeBundleEntry(archive *zip.Writer, file bundleFile, modified time.Time) error {
	mode := os.FileMode(0o644)
	if file.executable {
		mode = 0o755
	}

	header := &zip.FileHeader{Name: file.name, Method: zip.Deflate, Modified: modified}
	header.SetMode(mode)

	entry, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("adding %s to the bundle: %w", file.name, err)
	}

	if file.source == "" {
		if _, err := entry.Write(file.content); err != nil {
			return fmt.Errorf("writing %s into the bundle: %w", file.name, err)
		}
		return nil
	}

	src, err := os.Open(file.source)
	if err != nil {
		return fmt.Errorf("reading %s: %w", file.source, err)
	}
	defer src.Close()

	if _, err := io.Copy(entry, src); err != nil {
		return fmt.Errorf("copying %s into the bundle: %w", file.source, err)
	}
	return nil
}

// encodeManifest renders manifest.json exactly as it is written into the
// bundle, so -print-manifest shows what a client would actually read.
func encodeManifest(m manifest) ([]byte, error) {
	encoded, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding manifest: %w", err)
	}
	return append(encoded, '\n'), nil
}
