package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type options struct {
	version string
	output  string
	goBin   string
}

type buildTarget struct {
	pkg      string
	filename string
	goos     string
	goarch   string
	version  bool
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "netprobe-release: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	options, err := parseOptions(args, stderr)
	if err != nil {
		return err
	}
	if strings.ContainsAny(options.version, "\r\n\t ") {
		return errors.New("version must not contain whitespace")
	}
	downloads := filepath.Join(options.output, "downloads")
	if err := os.RemoveAll(downloads); err != nil {
		return fmt.Errorf("clear downloads directory: %w", err)
	}
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		return fmt.Errorf("create downloads directory: %w", err)
	}

	targets := []buildTarget{
		{pkg: "./cmd/netprobe-server", filename: filepath.Join(options.output, "netprobe-server")},
	}
	for _, goos := range []string{"linux", "windows"} {
		for _, goarch := range []string{"amd64", "arm64"} {
			extension := ""
			if goos == "windows" {
				extension = ".exe"
			}
			targets = append(targets, buildTarget{
				pkg:      "./cmd/netcheck",
				filename: filepath.Join(downloads, "netcheck-"+goos+"-"+goarch+extension),
				goos:     goos,
				goarch:   goarch,
				version:  true,
			})
		}
	}
	for _, goarch := range []string{"amd64", "arm64"} {
		targets = append(targets, buildTarget{
			pkg:      "./cmd/netprobe-deploy",
			filename: filepath.Join(downloads, "netprobe-deploy-linux-"+goarch),
			goos:     "linux",
			goarch:   goarch,
			version:  true,
		})
	}

	for _, target := range targets {
		fmt.Fprintf(stdout, "Building %s\n", target.filename)
		if err := build(ctx, options, target, stdout, stderr); err != nil {
			return err
		}
		if filepath.Dir(target.filename) == downloads {
			if err := writeChecksum(target.filename); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	defaultVersion := firstValue(os.Getenv("VERSION"), os.Getenv("GITHUB_REF_NAME"), "dev")
	defaultOutput := firstValue(os.Getenv("OUT"), "dist")
	defaultGo := firstValue(os.Getenv("GO"), "go")
	var result options
	flags := flag.NewFlagSet("netprobe-release", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&result.version, "version", defaultVersion, "release version")
	flags.StringVar(&result.output, "out", defaultOutput, "output directory")
	flags.StringVar(&result.goBin, "go", defaultGo, "Go executable")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected positional argument %q", flags.Arg(0))
	}
	if strings.TrimSpace(result.version) == "" || strings.TrimSpace(result.output) == "" || strings.TrimSpace(result.goBin) == "" {
		return options{}, errors.New("version, output directory, and Go executable are required")
	}
	return result, nil
}

func build(ctx context.Context, options options, target buildTarget, stdout, stderr io.Writer) error {
	arguments := []string{"build", "-trimpath"}
	linkerFlags := "-s -w"
	if target.version {
		linkerFlags += " -X main.version=" + options.version
	}
	arguments = append(arguments, "-ldflags="+linkerFlags, "-o", target.filename, target.pkg)
	process := exec.CommandContext(ctx, options.goBin, arguments...)
	process.Env = buildEnvironment(os.Environ(), target.goos, target.goarch)
	process.Stdout = stdout
	process.Stderr = stderr
	if err := process.Run(); err != nil {
		return fmt.Errorf("build %s: %w", target.filename, err)
	}
	return nil
}

func buildEnvironment(current []string, goos, goarch string) []string {
	filtered := make([]string, 0, len(current)+3)
	for _, item := range current {
		if strings.HasPrefix(item, "CGO_ENABLED=") || strings.HasPrefix(item, "GOOS=") || strings.HasPrefix(item, "GOARCH=") {
			continue
		}
		filtered = append(filtered, item)
	}
	filtered = append(filtered, "CGO_ENABLED=0")
	if goos != "" {
		filtered = append(filtered, "GOOS="+goos)
	}
	if goarch != "" {
		filtered = append(filtered, "GOARCH="+goarch)
	}
	return filtered
}

func writeChecksum(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open %s for checksum: %w", filename, err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return fmt.Errorf("checksum %s: %w", filename, err)
	}
	line := fmt.Sprintf("%x  %s\n", hash.Sum(nil), filepath.Base(filename))
	if err := os.WriteFile(filename+".sha256", []byte(line), 0o644); err != nil {
		return fmt.Errorf("write checksum for %s: %w", filename, err)
	}
	return nil
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
