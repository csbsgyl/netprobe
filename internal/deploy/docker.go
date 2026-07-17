package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const dockerInstallURL = "https://get.docker.com"

type dockerInstaller interface {
	Install(context.Context, io.Writer, io.Writer) error
}

type officialDockerInstaller struct {
	Client   *http.Client
	Commands commandRunner
}

func (installer officialDockerInstaller) Install(ctx context.Context, stdout, stderr io.Writer) error {
	client := installer.Client
	if client == nil {
		client = &http.Client{Timeout: time.Minute}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, dockerInstallURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "netprobe-deploy")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download Docker installer: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8<<10))
		return fmt.Errorf("download Docker installer: HTTP %s", response.Status)
	}
	file, err := os.CreateTemp("", "netprobe-docker-install-*.sh")
	if err != nil {
		return fmt.Errorf("create Docker installer: %w", err)
	}
	name := file.Name()
	defer os.Remove(name)
	limited := &io.LimitedReader{R: response.Body, N: 4<<20 + 1}
	if _, err := io.Copy(file, limited); err != nil {
		_ = file.Close()
		return fmt.Errorf("save Docker installer: %w", err)
	}
	if limited.N <= 0 {
		_ = file.Close()
		return errors.New("Docker installer exceeds the size limit")
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("save Docker installer: %w", err)
	}
	if err := installer.Commands.Run(ctx, command{Name: "sh", Args: []string{name}}, stdout, stderr); err != nil {
		return fmt.Errorf("run Docker's official installer: %w", err)
	}
	return nil
}

func dockerReady(ctx context.Context, commands commandRunner) error {
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	output, err := commands.Output(checkCtx, command{Name: "docker", Args: []string{"compose", "version"}})
	if err != nil {
		detail := strings.TrimSpace(output)
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("Docker Engine with Compose v2 is unavailable: %s", detail)
	}
	if _, err := commands.Output(checkCtx, command{Name: "docker", Args: []string{"info"}}); err != nil {
		return fmt.Errorf("Docker daemon is not ready: %w", err)
	}
	return nil
}
