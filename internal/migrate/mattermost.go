package migrate

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

type Step struct {
	Name   string
	Status string // "running", "done", "skipped", "failed"
	Err    error
}

type Mattermost struct {
	Runner       runner.CommandRunner
	Compose      *docker.ComposeClient
	DockerExec   *docker.ExecClient
	DumpPath     string
	DataDir      string
	Container    string
	DBContainer  string
	DBUser       string
	DBName       string
	PollInterval time.Duration
	PollTimeout  time.Duration
	ProgressFn   func(Step)
}

func NewMattermost(r runner.CommandRunner, compose *docker.ComposeClient, dockerExec *docker.ExecClient) *Mattermost {
	return &Mattermost{
		Runner:       r,
		Compose:      compose,
		DockerExec:   dockerExec,
		DumpPath:     "/tmp/mattermost_dump.pgc",
		DataDir:      "/opt/mattermost/data",
		Container:    "mattermost",
		DBContainer:  "mattermost-db",
		DBUser:       "mattermost",
		DBName:       "mattermost",
		PollInterval: time.Second,
		PollTimeout:  60 * time.Second,
		ProgressFn:   func(Step) {},
	}
}

func (m *Mattermost) Run(ctx context.Context) error {
	steps := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"dump host database", m.dumpHostDB},
		{"stop host mattermost service", m.stopHostService},
		{"start container database", m.startContainerDB},
		{"restore database into container", m.restoreDB},
		{"copy file uploads to container", m.copyData},
		{"start mattermost container", m.startContainer},
		{"verify mattermost is running", m.verify},
		{"clean up dump file", m.cleanup},
	}

	for _, s := range steps {
		m.ProgressFn(Step{Name: s.name, Status: "running"})
		if err := s.fn(ctx); err != nil {
			m.ProgressFn(Step{Name: s.name, Status: "failed", Err: err})
			return fmt.Errorf("%s: %w", s.name, err)
		}
		m.ProgressFn(Step{Name: s.name, Status: "done"})
	}
	return nil
}

func (m *Mattermost) dumpHostDB(ctx context.Context) error {
	_, err := m.Runner.Run(ctx, "sudo", "-u", "postgres", "pg_dump", "-Fc", "-f", m.DumpPath, m.DBName)
	if err != nil {
		return fmt.Errorf("pg_dump: %w", err)
	}
	if err := os.Chmod(m.DumpPath, 0644); err != nil {
		return fmt.Errorf("chmod dump: %w", err)
	}
	return nil
}

func (m *Mattermost) stopHostService(ctx context.Context) error {
	if _, err := m.Runner.Run(ctx, "sudo", "systemctl", "stop", "mattermost"); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	if _, err := m.Runner.Run(ctx, "sudo", "systemctl", "disable", "mattermost"); err != nil {
		return fmt.Errorf("disable: %w", err)
	}
	return nil
}

func (m *Mattermost) startContainerDB(ctx context.Context) error {
	if _, err := m.Compose.UpService(ctx, m.DBContainer); err != nil {
		return err
	}
	return m.waitForPostgres(ctx)
}

func (m *Mattermost) waitForPostgres(ctx context.Context) error {
	deadline := time.After(m.PollTimeout)
	tick := time.NewTicker(m.PollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timed out waiting for postgres to accept connections")
		case <-tick.C:
			_, err := m.DockerExec.Exec(ctx, m.DBContainer, "pg_isready", "-U", m.DBUser)
			if err == nil {
				return nil
			}
		}
	}
}

func (m *Mattermost) restoreDB(ctx context.Context) error {
	if _, err := m.DockerExec.CopyTo(ctx, m.DumpPath, m.DBContainer, "/tmp/dump.pgc"); err != nil {
		return fmt.Errorf("copy dump to container: %w", err)
	}
	_, err := m.DockerExec.Exec(ctx, m.DBContainer,
		"pg_restore", "-U", m.DBUser, "-d", m.DBName, "--clean", "--if-exists", "--no-owner", "/tmp/dump.pgc",
	)
	if err != nil {
		return fmt.Errorf("pg_restore: %w", err)
	}
	return nil
}

func (m *Mattermost) copyData(ctx context.Context) error {
	if _, err := os.Stat(m.DataDir); os.IsNotExist(err) {
		m.ProgressFn(Step{Name: "copy file uploads to container", Status: "skipped"})
		return nil
	}
	if _, err := m.Compose.UpService(ctx, m.Container); err != nil {
		return fmt.Errorf("starting container for volume copy: %w", err)
	}
	if _, err := m.DockerExec.CopyTo(ctx, m.DataDir+"/.", m.Container, "/opt/mattermost/data/"); err != nil {
		return fmt.Errorf("docker cp: %w", err)
	}
	if _, err := m.DockerExec.Exec(ctx, m.Container, "chown", "-R", "2000:2000", "/opt/mattermost/data"); err != nil {
		return fmt.Errorf("chown: %w", err)
	}
	return nil
}

func (m *Mattermost) startContainer(ctx context.Context) error {
	_, err := m.Compose.UpService(ctx, m.Container)
	return err
}

func (m *Mattermost) verify(ctx context.Context) error {
	deadline := time.After(m.PollTimeout)
	tick := time.NewTicker(m.PollInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("mattermost did not become healthy within 60s")
		case <-tick.C:
			out, err := m.Compose.PS(ctx)
			if err != nil {
				continue
			}
			// Mattermost doesn't define a healthcheck in compose, so
			// check that the container is running (not Created/Exited).
			if containsRunning(out, m.Container) {
				return nil
			}
		}
	}
}

func (m *Mattermost) cleanup(_ context.Context) error {
	os.Remove(m.DumpPath)
	return nil
}

func containsRunning(psOutput []byte, container string) bool {
	s := string(psOutput)
	return strings.Contains(s, container) && strings.Contains(s, "running")
}
