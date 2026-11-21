package ssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ssh writes no time, so each line of its output gets one; stdout and stderr
// share a file and a writer.
func TestProcessLogLinesHaveTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh.log")
	p := &Process{stdoutPath: path, stderrPath: path}
	stdout, stderr, err := p.openLogFiles()
	if err != nil {
		t.Fatal(err)
	}
	if stdout != stderr {
		t.Fatal("one log file should get one writer")
	}
	cmd := exec.Command("sh", "-c", "echo out; echo 'ssh: connect to host 1.2.3.4 port 22: Connection timed out' >&2; printf partial")
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	p.closeOutputFiles(stdout, stderr)

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	stamped := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} \S`)
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %q", b)
	}
	for _, line := range lines {
		if !stamped.MatchString(line) {
			t.Errorf("line without a time: %q", line)
		}
	}
}

func TestNextRetryDelay(t *testing.T) {
	cases := []struct {
		name      string
		prev, ran time.Duration
		want      time.Duration
	}{
		{"first start", 0, time.Second, minRetryDelay},
		{"quick failure doubles", 2 * time.Second, time.Second, 4 * time.Second},
		{"keeps doubling", 8 * time.Second, 10 * time.Second, 16 * time.Second},
		{"capped", 16 * time.Second, time.Second, maxRetryDelay},
		{"stays capped", maxRetryDelay, time.Second, maxRetryDelay},
		{"died on keepalive still backs off", 4 * time.Second, 45 * time.Second, 8 * time.Second},
		{"stable run resets", maxRetryDelay, stableRun, minRetryDelay},
	}
	for _, c := range cases {
		if got := nextRetryDelay(c.prev, c.ran); got != c.want {
			t.Errorf("%s: nextRetryDelay(%v, %v) = %v, want %v", c.name, c.prev, c.ran, got, c.want)
		}
	}
}
