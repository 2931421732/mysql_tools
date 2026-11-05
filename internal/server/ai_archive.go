package server

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

func isArchiveName(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".zip") || strings.HasSuffix(n, ".tar") ||
		strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".tgz")
}

func textArchiveEntry(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".log", ".out", ".err", ".trace", ".txt", ".text", ".md", ".markdown", ".json", ".jsonl",
		".yaml", ".yml", ".toml", ".ini", ".conf", ".properties", ".xml", ".html", ".css",
		".go", ".py", ".js", ".ts", ".jsx", ".tsx", ".java", ".kt", ".c", ".h", ".cpp", ".hpp", ".cc",
		".cs", ".php", ".rb", ".rs", ".swift", ".sh", ".sql", ".vue", ".svelte", ".diff", ".patch",
		".dockerfile", ".env", ".gitignore", ".gradle", ".pom", ".lock":
		return true
	}
	return ext == "" || strings.EqualFold(filepath.Base(name), "Dockerfile") || strings.EqualFold(filepath.Base(name), "Makefile")
}

type archiveEntry struct {
	Name string
	Size int64
	Data []byte
	Err  error
}

func readZipArchive(data []byte) ([]archiveEntry, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	out := make([]archiveEntry, 0)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || len(out) >= 2000 {
			continue
		}
		e := archiveEntry{Name: f.Name, Size: int64(f.UncompressedSize64)}
		if e.Size > 2<<20 || !textArchiveEntry(f.Name) {
			out = append(out, e)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			e.Err = err
			out = append(out, e)
			continue
		}
		e.Data, e.Err = io.ReadAll(io.LimitReader(rc, 2<<20+1))
		rc.Close()
		out = append(out, e)
	}
	return out, nil
}

func readTarArchive(data []byte, gz bool) ([]archiveEntry, error) {
	var r io.Reader = bytes.NewReader(data)
	if gz {
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		r = gr
	}
	tr := tar.NewReader(r)
	out := make([]archiveEntry, 0)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, err
		}
		if h.FileInfo().IsDir() || len(out) >= 2000 {
			continue
		}
		e := archiveEntry{Name: h.Name, Size: h.Size}
		if h.Size > 2<<20 || !textArchiveEntry(h.Name) {
			out = append(out, e)
			continue
		}
		e.Data, e.Err = io.ReadAll(io.LimitReader(tr, 2<<20+1))
		out = append(out, e)
	}
	return out, nil
}

// archiveAttachmentText extracts archives in memory and returns a bounded
// manifest plus text/log/source contents. It never writes to disk.
func archiveAttachmentText(data []byte, name string) (string, error) {
	var entries []archiveEntry
	var err error
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		entries, err = readZipArchive(data)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		entries, err = readTarArchive(data, true)
	default:
		entries, err = readTarArchive(data, false)
	}
	if err != nil && len(entries) == 0 {
		return "", fmt.Errorf("无法解压 %s: %w", name, err)
	}
	var b strings.Builder
	var textBytes int
	fmt.Fprintf(&b, "[archive: %s]\nentries: %d\n", name, len(entries))
	for _, e := range entries {
		name := filepath.ToSlash(strings.TrimLeft(e.Name, "/"))
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		fmt.Fprintf(&b, "\n--- %s (%d bytes) ---\n", name, e.Size)
		if e.Err != nil {
			fmt.Fprintf(&b, "[unreadable: %v]\n", e.Err)
			continue
		}
		if len(e.Data) == 0 {
			fmt.Fprintf(&b, "[binary or empty; not included]\n")
			continue
		}
		if textBytes+len(e.Data) > 60000 {
			remaining := 60000 - textBytes
			if remaining > 0 {
				b.Write(e.Data[:remaining])
				textBytes += remaining
			}
			b.WriteString("\n…[archive text limit reached]…\n")
			break
		}
		b.Write(e.Data)
		textBytes += len(e.Data)
	}
	if err != nil {
		fmt.Fprintf(&b, "\n[archive ended with warning: %v]\n", err)
	}
	return b.String(), nil
}
