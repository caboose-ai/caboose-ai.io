package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/docker"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

func fastPoll(m *Mattermost) {
	m.PollInterval = 10 * time.Millisecond
	m.PollTimeout = 500 * time.Millisecond
}

func TestMattermostMigration_FullRun(t *testing.T) {
	mock := runner.NewMockRunner()
	compose := docker.NewComposeClient(mock, "/fake/compose")
	dockerExec := docker.NewExecClient(mock)

	tmpDir := t.TempDir()
	dumpPath := filepath.Join(tmpDir, "dump.pgc")
	dataDir := filepath.Join(tmpDir, "data")
	os.MkdirAll(dataDir, 0755)

	m := NewMattermost(mock, compose, dockerExec)
	fastPoll(m)
	m.DumpPath = dumpPath
	m.DataDir = dataDir

	var steps []string
	m.ProgressFn = func(s Step) {
		steps = append(steps, s.Name+":"+s.Status)
	}

	mock.On("sudo -u postgres pg_dump", nil, nil)
	mock.On("sudo systemctl stop mattermost", nil, nil)
	mock.On("sudo systemctl disable mattermost", nil, nil)
	mock.On("docker compose", nil, nil)
	mock.On("docker exec mattermost-db pg_isready", nil, nil)
	mock.On("docker cp", nil, nil)
	mock.On("docker exec mattermost-db pg_restore", nil, nil)
	mock.On("docker exec mattermost chown", nil, nil)

	psJSON := `{"Name":"mattermost","State":"running"}` + "\n"
	mock.On("docker compose -f /fake/compose/docker-compose.yml ps", []byte(psJSON), nil)

	// pg_dump creates the file, but with mocks it won't — create a placeholder
	// so Chmod doesn't fail.
	os.WriteFile(dumpPath, []byte("fake"), 0600)

	err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	wantDone := []string{
		"dump host database:done",
		"stop host mattermost service:done",
		"start container database:done",
		"restore database into container:done",
		"copy file uploads to container:done",
		"start mattermost container:done",
		"verify mattermost is running:done",
		"clean up dump file:done",
	}
	for _, want := range wantDone {
		found := false
		for _, got := range steps {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected step %q in progress, got: %v", want, steps)
		}
	}
}

func TestMattermostMigration_DumpFailure(t *testing.T) {
	mock := runner.NewMockRunner()
	compose := docker.NewComposeClient(mock, "/fake/compose")
	dockerExec := docker.NewExecClient(mock)

	m := NewMattermost(mock, compose, dockerExec)
	fastPoll(m)
	m.DumpPath = filepath.Join(t.TempDir(), "dump.pgc")

	mock.On("sudo -u postgres pg_dump", nil, fmt.Errorf("permission denied"))

	var lastStep Step
	m.ProgressFn = func(s Step) { lastStep = s }

	err := m.Run(context.Background())
	if err == nil {
		t.Fatal("expected error from failed pg_dump")
	}
	if !strings.Contains(err.Error(), "pg_dump") {
		t.Errorf("expected pg_dump in error, got: %v", err)
	}
	if lastStep.Status != "failed" {
		t.Errorf("expected last step status 'failed', got: %s", lastStep.Status)
	}
}

func TestMattermostMigration_SkipsDataCopyWhenNoDataDir(t *testing.T) {
	mock := runner.NewMockRunner()
	compose := docker.NewComposeClient(mock, "/fake/compose")
	dockerExec := docker.NewExecClient(mock)

	tmpDir := t.TempDir()
	dumpPath := filepath.Join(tmpDir, "dump.pgc")

	m := NewMattermost(mock, compose, dockerExec)
	fastPoll(m)
	m.DumpPath = dumpPath
	m.DataDir = filepath.Join(tmpDir, "nonexistent")

	var skippedData bool
	m.ProgressFn = func(s Step) {
		if s.Name == "copy file uploads to container" && s.Status == "skipped" {
			skippedData = true
		}
	}

	mock.On("sudo -u postgres pg_dump", nil, nil)
	mock.On("sudo systemctl stop mattermost", nil, nil)
	mock.On("sudo systemctl disable mattermost", nil, nil)
	mock.On("docker compose", nil, nil)
	mock.On("docker exec mattermost-db pg_isready", nil, nil)
	mock.On("docker cp", nil, nil)
	mock.On("docker exec mattermost-db pg_restore", nil, nil)

	psJSON := `{"Name":"mattermost","State":"running"}` + "\n"
	mock.On("docker compose -f /fake/compose/docker-compose.yml ps", []byte(psJSON), nil)

	os.WriteFile(dumpPath, []byte("fake"), 0600)

	err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !skippedData {
		t.Error("expected data copy step to be skipped when DataDir doesn't exist")
	}
}

func TestContainsRunning(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		container string
		want      bool
	}{
		{
			name:      "running container",
			input:     `{"Name":"mattermost","State":"running"}`,
			container: "mattermost",
			want:      true,
		},
		{
			name:      "created container",
			input:     `{"Name":"mattermost","State":"created"}`,
			container: "mattermost",
			want:      false,
		},
		{
			name:      "different container",
			input:     `{"Name":"grafana","State":"running"}`,
			container: "mattermost",
			want:      false,
		},
		{
			name:      "empty output",
			input:     "",
			container: "mattermost",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsRunning([]byte(tt.input), tt.container)
			if got != tt.want {
				t.Errorf("containsRunning(%q, %q) = %v, want %v", tt.input, tt.container, got, tt.want)
			}
		})
	}
}
