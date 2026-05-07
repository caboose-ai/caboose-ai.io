package smoketest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
)

type EvidenceRecorder struct {
	Dir  string
	step int
}

type evidenceEvent struct {
	Step       int               `json:"step"`
	Time       string            `json:"time"`
	Action     string            `json:"action"`
	Label      string            `json:"label"`
	URL        string            `json:"url,omitempty"`
	Screenshot string            `json:"screenshot,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
}

func NewEvidenceRecorder(t *testing.T) *EvidenceRecorder {
	t.Helper()
	if os.Getenv("SMOKETEST_EVIDENCE") == "0" {
		return nil
	}

	dir := filepath.Join(testdataDir(), "evidence", fmt.Sprintf("%s-%d", safeName(t.Name()), time.Now().Unix()))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating evidence directory: %v", err)
	}
	t.Logf("E2E evidence directory: %s", dir)
	return &EvidenceRecorder{Dir: dir}
}

func (r *EvidenceRecorder) Record(t *testing.T, page *rod.Page, action, label string, details map[string]string) {
	t.Helper()
	if r == nil {
		return
	}

	r.step++
	event := evidenceEvent{
		Step:    r.step,
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Action:  action,
		Label:   label,
		Details: details,
	}
	if page != nil {
		info, err := page.Info()
		if err == nil && info != nil {
			event.URL = info.URL
		}
		if data, err := page.Screenshot(false, nil); err == nil {
			name := fmt.Sprintf("%03d-%s.png", r.step, safeName(action+"-"+label))
			if err := os.WriteFile(filepath.Join(r.Dir, name), data, 0644); err == nil {
				event.Screenshot = name
			}
		}
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshaling evidence event: %v", err)
	}

	path := filepath.Join(r.Dir, "events.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("opening evidence log: %v", err)
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Fatalf("writing evidence log: %v", err)
	}
}

func safeName(s string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	name := strings.Trim(re.ReplaceAllString(s, "-"), "-")
	if name == "" {
		return "step"
	}
	if len(name) > 90 {
		return name[:90]
	}
	return name
}
