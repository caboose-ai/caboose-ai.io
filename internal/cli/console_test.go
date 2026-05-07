package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestConsoleLinkDoesNotEscapeANSIInURL(t *testing.T) {
	origStderr := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stderr pipe: %v", err)
	}
	os.Stderr = writePipe
	t.Cleanup(func() {
		os.Stderr = origStderr
		readPipe.Close()
		writePipe.Close()
	})

	console := NewConsole()
	console.Link("admin recovery link", "https://example.test/recovery")
	writePipe.Close()

	out, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("reading console output: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "https://example.test/recovery") {
		t.Fatalf("console output does not include URL: %q", text)
	}
	if strings.Contains(text, `\x1b`) {
		t.Fatalf("console output contains escaped ANSI bytes: %q", text)
	}
}
