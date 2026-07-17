package deploy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractSourceArchiveStripsRepositoryDirectory(t *testing.T) {
	archive := sourceArchive(t, map[string]string{
		"netprobe-v1/deploy/compose.yaml": "services:\n",
		"netprobe-v1/README.md":           "# NetProbe\n",
	})
	destination := t.TempDir()
	if err := extractSourceArchive(bytes.NewReader(archive), destination); err != nil {
		t.Fatalf("extractSourceArchive returned error: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(destination, "deploy", "compose.yaml"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(contents) != "services:\n" {
		t.Fatalf("unexpected contents: %q", contents)
	}
}

func TestExtractSourceArchiveSkipsTraversal(t *testing.T) {
	archive := sourceArchive(t, map[string]string{
		"netprobe-v1/../../outside": "unsafe",
		"netprobe-v1/README.md":     "safe",
	})
	parent := t.TempDir()
	destination := filepath.Join(parent, "install")
	if err := extractSourceArchive(bytes.NewReader(archive), destination); err != nil {
		t.Fatalf("extractSourceArchive returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "outside")); !os.IsNotExist(err) {
		t.Fatalf("traversal file exists or stat failed unexpectedly: %v", err)
	}
}

func TestGitHubArchiveURLPreservesBranchSegments(t *testing.T) {
	got := githubArchiveURL("owner/repo", "feature/deploy-v2")
	want := "https://codeload.github.com/owner/repo/tar.gz/feature/deploy-v2"
	if got != want {
		t.Fatalf("githubArchiveURL = %q, want %q", got, want)
	}
}

func sourceArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	for name, contents := range files {
		if err := archive.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(contents)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
