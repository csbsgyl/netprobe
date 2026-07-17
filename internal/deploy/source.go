package deploy

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxSourceArchiveSize = 128 << 20
	maxSourceExtractSize = 512 << 20
)

type sourceInstaller interface {
	Install(context.Context, string, string, string) error
}

type githubSourceInstaller struct {
	Client *http.Client
}

func (installer githubSourceInstaller) Install(ctx context.Context, repository, reference, destination string) error {
	client := installer.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	archiveURL := githubArchiveURL(repository, reference)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return fmt.Errorf("create source request: %w", err)
	}
	request.Header.Set("User-Agent", "netprobe-deploy")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download NetProbe source: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8<<10))
		return fmt.Errorf("download NetProbe source: HTTP %s", response.Status)
	}
	if response.ContentLength > maxSourceArchiveSize {
		return fmt.Errorf("source archive is too large: %d bytes", response.ContentLength)
	}
	limited := &io.LimitedReader{R: response.Body, N: maxSourceArchiveSize + 1}
	if err := extractSourceArchive(limited, destination); err != nil {
		return err
	}
	if limited.N <= 0 {
		return errors.New("source archive exceeds the size limit")
	}
	return nil
}

func githubArchiveURL(repository, reference string) string {
	segments := strings.Split(reference, "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	return "https://codeload.github.com/" + repository + "/tar.gz/" + strings.Join(segments, "/")
}

func extractSourceArchive(reader io.Reader, destination string) error {
	compressed, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("open source archive: %w", err)
	}
	defer compressed.Close()
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create install directory: %w", err)
	}
	archive := tar.NewReader(compressed)
	var extracted int64
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read source archive: %w", err)
		}
		name, ok := archiveRelativePath(header.Name)
		if !ok {
			continue
		}
		if header.Size < 0 || extracted+header.Size > maxSourceExtractSize {
			return errors.New("expanded source archive exceeds the size limit")
		}
		extracted += header.Size
		target, err := secureTargetPath(destination, name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create source directory %s: %w", name, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := writeArchiveFile(archive, target, header.FileInfo().Mode().Perm()); err != nil {
				return fmt.Errorf("extract source file %s: %w", name, err)
			}
		default:
			return fmt.Errorf("source archive contains unsupported entry %s", name)
		}
	}
	return nil
}

func archiveRelativePath(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	clean := path.Clean(name)
	if clean == "." || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	parts := strings.Split(clean, "/")
	if len(parts) < 2 {
		return "", false
	}
	relative := path.Join(parts[1:]...)
	return relative, relative != "."
}

func secureTargetPath(destination, relative string) (string, error) {
	target := filepath.Join(destination, filepath.FromSlash(relative))
	check, err := filepath.Rel(destination, target)
	if err != nil || check == ".." || strings.HasPrefix(check, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source archive path escapes install directory: %s", relative)
	}
	return target, nil
}

func writeArchiveFile(reader io.Reader, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if mode == 0 {
		mode = 0o644
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	return errors.Join(copyErr, closeErr)
}
