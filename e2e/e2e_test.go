//go:build e2e

// Package e2e runs P-Manager and P-Node as real processes and checks that an
// account can reach the internet through every supported proxy method, first
// with distinct node ports and then with Remote RR and Relay RR2RR sharing one.
//
// Run with: make e2e (go test -tags e2e ./e2e/). Set E2E_SSH=1 to include
// Relay RR2SSH, which needs passwordless ssh to 127.0.0.1 for the current user.
// Set E2E_PQ=1 to include Relay RR2PQ, which needs root on Linux (paqet sends
// raw packets over a veth pair the test creates): make e2e-docker runs the
// whole test that way in a privileged container.
package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	adminPassword = "password" // default admin password (internal/data/main_settings.go)
	sshPort       = 22
)

// Third-party binary directories by GOOS/GOARCH (see internal/config/paths.go).
var xrayDirs = map[string]string{
	"darwin/arm64": "third_party/xray-macos-arm64",
	"linux/amd64":  "third_party/xray-linux-64",
	"linux/arm64":  "third_party/xray-linux-arm64",
}

var paqetDirs = map[string]string{
	"darwin/arm64": "third_party/paqet-macos-arm64",
	"linux/amd64":  "third_party/paqet-linux-64",
	"linux/arm64":  "third_party/paqet-linux-arm64",
}

// The veth pair paqet runs over when E2E_PQ=1: the manager side and the node side.
const (
	pqManagerIface = "pq0"
	pqManagerAddr  = "10.200.0.1"
	pqNodeIface    = "pq1"
	pqNodeAddr     = "10.200.0.2"
)

func TestProxyMethods(t *testing.T) {
	managerRepo, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	nodeRepo := os.Getenv("P_NODE_DIR")
	if nodeRepo == "" {
		nodeRepo = filepath.Join(managerRepo, "..", "p-node")
	}
	withSsh := os.Getenv("E2E_SSH") == "1"
	withPq := os.Getenv("E2E_PQ") == "1"

	platform := runtime.GOOS + "/" + runtime.GOARCH
	xrayDir, ok := xrayDirs[platform]
	if !ok {
		t.Skipf("no xray binary for %s", platform)
	}
	xrayBinary := filepath.Join(managerRepo, xrayDir, "xray")
	if _, err := os.Stat(xrayBinary); err != nil {
		t.Skipf("xray binary not found at %s (run make local-setup)", xrayBinary)
	}
	paqetDir := paqetDirs[platform]
	if withPq {
		if runtime.GOOS != "linux" || os.Geteuid() != 0 {
			t.Fatal("E2E_PQ=1 needs root on Linux (use make e2e-docker)")
		}
		for _, repo := range []string{managerRepo, nodeRepo} {
			if _, err := os.Stat(filepath.Join(repo, paqetDir, "paqet")); err != nil {
				t.Fatalf("paqet binary not found in %s/%s", repo, paqetDir)
			}
		}
	}

	// paqet needs real Ethernet interfaces (loopback has no MAC), so the
	// manager and the node each get one end of a veth pair.
	nodeHost := "127.0.0.1"
	var managerPaqet, nodePaqet map[string]any
	if withPq {
		managerMac, nodeMac := setupVeth(t)
		nodeHost = pqNodeAddr
		managerPaqet = map[string]any{"interface": pqManagerIface, "address": pqManagerAddr, "router_mac": nodeMac}
		nodePaqet = map[string]any{"interface": pqNodeIface, "address": pqNodeAddr, "router_mac": managerMac}
	}

	// The target the proxied requests must reach.
	const body = "p-manager e2e ok"
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer target.Close()

	work := t.TempDir()
	managerBin := build(t, managerRepo, filepath.Join(work, "p-manager"))
	nodeBin := build(t, nodeRepo, filepath.Join(work, "p-node"))

	ports := freePorts(t, 11)
	managerPort, nodePort := ports[0], ports[1]
	directRr, remoteRr, relayRr2RrManager, relayRr2RrNode, reverseRr, relayRr2Ssh, reverseRrPortal := ports[2], ports[3], ports[4], ports[5], ports[6], ports[7], ports[8]
	relayRr2Pq, relayRr2PqNode := ports[9], ports[10]
	if !withSsh {
		relayRr2Ssh = 0
	}
	if !withPq {
		relayRr2Pq, relayRr2PqNode = 0, 0
	}

	// --- P-Node ---
	nodeRoot := prepareRoot(t, nodeRepo, filepath.Join(work, "node"), xrayDir, paqetDir, map[string]any{
		"http_server": map[string]any{"port": nodePort},
		"logger":      map[string]any{"level": "debug", "format": "2006-01-02 15:04:05.000"},
		"paqet":       nodePaqet,
	})
	nodeProc := startProcess(t, nodeBin, nodeRoot)
	nodeUrl := fmt.Sprintf("http://127.0.0.1:%d", nodePort)
	waitHttp(t, nodeUrl+"/", "p-node")

	var nodeDb struct {
		Settings struct {
			HttpToken string `json:"http_token"`
		} `json:"settings"`
	}
	readJsonFile(t, filepath.Join(nodeRoot, "storage/database/data.json"), &nodeDb)

	// --- P-Manager ---
	managerRoot := prepareRoot(t, managerRepo, filepath.Join(work, "manager"), xrayDir, paqetDir, map[string]any{
		"http_server": map[string]any{"host": "127.0.0.1", "port": managerPort},
		"logger":      map[string]any{"level": "debug", "format": "2006-01-02 15:04:05.000"},
		"paqet":       managerPaqet,
	})
	managerProc := startProcess(t, managerBin, managerRoot)
	managerUrl := fmt.Sprintf("http://127.0.0.1:%d", managerPort)
	waitHttp(t, managerUrl+"/api/admin/platform", "p-manager")

	t.Cleanup(func() {
		if t.Failed() {
			dumpLogs(t, "manager", managerRoot)
			dumpLogs(t, "node", nodeRoot)
		}
		managerProc.stop()
		nodeProc.stop()
	})

	admin := &apiClient{base: managerUrl + "/api/admin", token: adminPassword}

	// Xray settings: keep the generated Reality keys, enable every method.
	var xs map[string]any
	admin.call(t, http.MethodGet, "/xray-settings", nil, &xs)
	if xs["reality_private_key"] == "" {
		t.Fatal("manager did not generate reality keys")
	}
	xs["direct_rr_port"] = directRr
	xs["remote_rr_port"] = remoteRr
	xs["relay_rr_2_rr_manager_port"] = relayRr2RrManager
	xs["relay_rr_2_rr_node_port"] = relayRr2RrNode
	xs["reverse_rr_manager_port"] = reverseRr
	xs["reverse_rr_portal_port"] = reverseRrPortal
	xs["relay_rr_2_ssh_port"] = relayRr2Ssh
	xs["relay_rr_2_ssh_connections"] = 1
	xs["relay_rr_2_pq_port"] = relayRr2Pq
	xs["relay_rr_2_pq_node_port"] = relayRr2PqNode
	admin.call(t, http.MethodPost, "/xray-settings", xs, nil)

	// Node: pushed over HTTP; SSH only when the environment provides it.
	sshUser := "root"
	if u, err := user.Current(); err == nil {
		sshUser = u.Username
	}
	var node struct {
		Id        string `json:"id"`
		PullToken string `json:"pull_token"`
	}
	admin.call(t, http.MethodPost, "/nodes", map[string]any{
		"host":         nodeHost,
		"http_token":   nodeDb.Settings.HttpToken,
		"http_port":    nodePort,
		"ssh_user":     sshUser,
		"ssh_port":     sshPort,
		"ssh_enabled":  withSsh,
		"push_enabled": true,
	}, &node)

	// Node pulls too, so both sync paths are exercised.
	nodeApi := &apiClient{base: nodeUrl, token: nodeDb.Settings.HttpToken}
	nodeApi.call(t, http.MethodPost, "/manager", map[string]any{
		"url":   fmt.Sprintf("%s/api/node/%s", managerUrl, node.Id),
		"token": node.PullToken,
	}, nil)

	var account struct {
		Id string `json:"id"`
	}
	admin.call(t, http.MethodPost, "/accounts", map[string]any{
		"name": "e2e", "enabled": true, "quota": 0,
	}, &account)

	// Wait until the node runs the account's inbound, i.e. the config arrived.
	waitFor(t, 60*time.Second, "node to receive the remote-rr inbound", func() bool {
		var xc struct {
			Inbounds []struct {
				Tag string `json:"tag"`
			} `json:"inbounds"`
		}
		if err := readJson(filepath.Join(nodeRoot, "storage/app/xray.json"), &xc); err != nil {
			return false
		}
		for _, in := range xc.Inbounds {
			if in.Tag == "remote-rr" {
				return true
			}
		}
		return false
	})

	var shown struct {
		Proxies map[string]string `json:"proxies"`
	}
	(&apiClient{base: managerUrl + "/api/user"}).call(t, http.MethodGet, "/account/"+account.Id, nil, &shown)

	want := []string{"direct-rr", "remote-rr-" + nodeHost, "relay-rr2rr", "reverse-rr"}
	if withSsh {
		want = append(want, "relay-rr2ssh")
	}
	if withPq {
		want = append(want, "relay-rr2paqet")
	}
	for _, method := range want {
		method := method
		link, ok := shown.Proxies[method+"@127.0.0.1"]
		if !ok {
			t.Errorf("no %s link in %v", method, keys(shown.Proxies))
			continue
		}
		t.Run(method, func(t *testing.T) {
			got := fetchThroughLink(t, xrayBinary, filepath.Join(work, "client-"+method), link, target.URL)
			if got != body {
				t.Fatalf("got %q through %s, want %q", got, method, body)
			}
		})
	}
	if !withSsh {
		t.Log("relay-rr2ssh skipped; set E2E_SSH=1 to include it")
	}
	if !withPq {
		t.Log("relay-rr2paqet skipped; run make e2e-docker to include it")
	}

	// Logs: the manager lists and streams its own log files and, proxied through
	// it, the node's.
	t.Run("logs", func(t *testing.T) {
		for _, src := range []struct{ name, base string }{
			{"manager", "/logs"},
			{"node", "/nodes/" + node.Id + "/logs"},
		} {
			var files []struct {
				Name string `json:"name"`
			}
			admin.call(t, http.MethodGet, src.base, nil, &files)
			found := false
			for _, f := range files {
				found = found || f.Name == "app-std.log"
			}
			if !found {
				t.Fatalf("%s: app-std.log not listed in %v", src.name, files)
			}
			line := streamFirstLine(t, managerUrl+"/api/admin"+src.base+"/app-std.log?lines=5", adminPassword)
			if !strings.Contains(line, `"level"`) {
				t.Fatalf("%s: unexpected log line %q", src.name, line)
			}
		}
	})

	// Shared node port: Remote RR and Relay RR2RR on the same node port, so the
	// node serves the account and the manager's relay from one inbound.
	t.Run("shared-node-port", func(t *testing.T) {
		xs["relay_rr_2_rr_node_port"] = remoteRr
		admin.call(t, http.MethodPost, "/xray-settings", xs, nil)

		waitFor(t, 60*time.Second, "node to serve the relay from the remote-rr inbound", func() bool {
			var xc struct {
				Inbounds []struct {
					Tag      string `json:"tag"`
					Port     int    `json:"port"`
					Settings struct {
						Clients []struct {
							Id string `json:"id"`
						} `json:"clients"`
					} `json:"settings"`
				} `json:"inbounds"`
			}
			if err := readJson(filepath.Join(nodeRoot, "storage/app/xray.json"), &xc); err != nil {
				return false
			}
			merged := false
			for _, in := range xc.Inbounds {
				if in.Tag == "relay-rr2rr" {
					return false
				}
				if in.Tag == "remote-rr" && in.Port == remoteRr && len(in.Settings.Clients) == 2 {
					merged = true
				}
			}
			return merged
		})

		for _, method := range []string{"remote-rr-" + nodeHost, "relay-rr2rr"} {
			method := method
			link := shown.Proxies[method+"@127.0.0.1"]
			t.Run(method, func(t *testing.T) {
				got := fetchThroughLink(t, xrayBinary, filepath.Join(work, "client-shared-"+method), link, target.URL)
				if got != body {
					t.Fatalf("got %q through %s on the shared node port, want %q", got, method, body)
				}
			})
		}
	})
}

// setupVeth creates the veth pair for paqet and returns the MAC addresses of the
// manager and node ends. Needs root on Linux.
func setupVeth(t *testing.T) (managerMac, nodeMac string) {
	t.Helper()
	_ = exec.Command("ip", "link", "del", pqManagerIface).Run() // leftover from an aborted run
	for _, args := range [][]string{
		{"link", "add", pqManagerIface, "type", "veth", "peer", "name", pqNodeIface},
		{"addr", "add", pqManagerAddr + "/24", "dev", pqManagerIface},
		{"addr", "add", pqNodeAddr + "/24", "dev", pqNodeIface},
		{"link", "set", pqManagerIface, "up"},
		{"link", "set", pqNodeIface, "up"},
	} {
		if b, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
			t.Fatalf("ip %s: %v\n%s", strings.Join(args, " "), err, b)
		}
	}
	t.Cleanup(func() { _ = exec.Command("ip", "link", "del", pqManagerIface).Run() })

	mac := func(name string) string {
		iface, err := net.InterfaceByName(name)
		if err != nil {
			t.Fatal(err)
		}
		return iface.HardwareAddr.String()
	}
	return mac(pqManagerIface), mac(pqNodeIface)
}

// fetchThroughLink runs an Xray client for the share link and fetches the target through it.
func fetchThroughLink(t *testing.T, xrayBinary, dir, link, targetUrl string) string {
	t.Helper()

	u, err := url.Parse(link)
	if err != nil {
		t.Fatalf("bad link %q: %v", link, err)
	}
	port, _ := strconv.Atoi(u.Port())
	q := u.Query()
	httpPort := freePorts(t, 1)[0]

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"inbounds": []any{map[string]any{
			"tag": "in", "protocol": "http", "listen": "127.0.0.1", "port": httpPort, "settings": map[string]any{},
		}},
		"outbounds": []any{map[string]any{
			"tag":      "out",
			"protocol": "vless",
			"settings": map[string]any{"vnext": []any{map[string]any{
				"address": u.Hostname(), "port": port,
				"users": []any{map[string]any{
					"id": u.User.Username(), "flow": q.Get("flow"), "encryption": "none",
				}},
			}}},
			"streamSettings": map[string]any{
				"network":  q.Get("type"),
				"security": q.Get("security"),
				"realitySettings": map[string]any{
					"serverName": q.Get("sni"), "publicKey": q.Get("pbk"), "shortId": "", "fingerprint": "chrome",
				},
			},
		}},
	}
	configPath := filepath.Join(dir, "config.json")
	writeJson(t, configPath, config)

	proc := startProcess(t, xrayBinary, dir, "-c", configPath)
	defer proc.stop()
	waitPort(t, httpPort, "xray client")

	proxyUrl, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", httpPort))
	client := &http.Client{
		Timeout:   20 * time.Second,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)},
	}

	// The chain may still be settling (reverse bridges register asynchronously).
	var lastErr error
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(targetUrl)
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return string(b)
			}
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, b)
		} else {
			lastErr = err
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("request through proxy failed: %v", lastErr)
	return ""
}

// streamFirstLine opens a chunked log stream and returns its first line.
func streamFirstLine(t *testing.T, url, token string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: status %d: %s", url, resp.StatusCode, b)
	}
	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		t.Fatalf("GET %s: no line before the stream ended: %v", url, scanner.Err())
	}
	return scanner.Text()
}

// --- helpers ---

type apiClient struct {
	base  string
	token string
}

func (c *apiClient) call(t *testing.T, method, path string, in, out any) {
	t.Helper()
	var reader io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("%s %s: status %d: %s", method, path, resp.StatusCode, b)
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("%s %s: bad json %q: %v", method, path, b, err)
		}
	}
}

// prepareRoot lays out a working directory the way the app expects it under its cwd.
func prepareRoot(t *testing.T, repo, root, xrayDir, paqetDir string, localConfig map[string]any) string {
	t.Helper()
	for _, d := range []string{"configs", "storage/app", "storage/database", "storage/logs", filepath.Dir(xrayDir)} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	copyFile(t, filepath.Join(repo, "configs/main.defaults.json"), filepath.Join(root, "configs/main.defaults.json"))
	writeJson(t, filepath.Join(root, "configs/main.json"), localConfig)
	for _, dir := range []string{xrayDir, paqetDir} {
		if dir == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(repo, dir)); err != nil {
			continue
		}
		if err := os.Symlink(filepath.Join(repo, dir), filepath.Join(root, dir)); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func build(t *testing.T, repo, out string) string {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = repo
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", repo, err, b)
	}
	return out
}

type process struct {
	cmd *exec.Cmd
}

func startProcess(t *testing.T, bin, dir string, args ...string) *process {
	t.Helper()
	if len(args) == 0 {
		args = []string{"serve"}
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	logFile, err := os.Create(filepath.Join(dir, "stdout.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", bin, err)
	}
	return &process{cmd: cmd}
}

func (p *process) stop() {
	if p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() { _, _ = p.cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill()
	}
}

func waitHttp(t *testing.T, url, what string) {
	t.Helper()
	waitFor(t, 30*time.Second, what+" to answer at "+url, func() bool {
		resp, err := (&http.Client{Timeout: 2 * time.Second}).Get(url)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return true
	})
}

func waitPort(t *testing.T, port int, what string) {
	t.Helper()
	waitFor(t, 15*time.Second, fmt.Sprintf("%s to listen on %d", what, port), func() bool {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	})
}

func waitFor(t *testing.T, timeout time.Duration, what string, ready func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		if ready() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %s", what)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func freePorts(t *testing.T, n int) []int {
	t.Helper()
	ports := make([]int, 0, n)
	listeners := make([]net.Listener, 0, n)
	for len(ports) < n {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners = append(listeners, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	for _, l := range listeners {
		_ = l.Close()
	}
	return ports
}

func readJsonFile(t *testing.T, path string, out any) {
	t.Helper()
	if err := readJson(path, out); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
}

func readJson(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func writeJson(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func dumpLogs(t *testing.T, name, root string) {
	for _, f := range []string{"stdout.log", "storage/logs/app-std.log", "storage/logs/xray-error.log", "storage/logs/paqet.log"} {
		b, err := os.ReadFile(filepath.Join(root, f))
		if err != nil || len(bytes.TrimSpace(b)) == 0 {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if len(lines) > 60 {
			lines = lines[len(lines)-60:]
		}
		t.Logf("--- %s %s (last %d lines) ---\n%s", name, f, len(lines), strings.Join(lines, "\n"))
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
