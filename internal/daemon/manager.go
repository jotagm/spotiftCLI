package daemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"cli_spotify/internal/config"
)

// Manager handles the lifecycle of the go-librespot subprocess.
type Manager struct {
	binaryPath string
	configPath string
	cmd        *exec.Cmd
	port       int
	log        safeBuffer
	ready      atomic.Bool
	authURLCh  chan string
}

// safeBuffer is a concurrency-safe buffer that captures the daemon's output so
// it can be surfaced if the daemon fails to start. The output is read from a
// separate goroutine, hence the mutex.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) WriteString(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.WriteString(s)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// NewManager creates a Manager from the application config.
func NewManager(cfg *config.Config) *Manager {
	binPath, _ := BinaryPath()
	cfgPath, _ := ConfigPath()
	return &Manager{
		binaryPath: binPath,
		configPath: cfgPath,
		port:       cfg.DaemonPort,
		authURLCh:  make(chan string, 1),
	}
}

// Start downloads the binary if needed, writes config, then launches the daemon.
//
// On the first run there are no saved credentials, so go-librespot prints a
// Spotify authorization link: Start surfaces it and waits for the user to
// authenticate before returning. Later runs reuse the saved credentials and
// start without interaction.
func (m *Manager) Start(cfg *config.Config) error {
	binPath, err := EnsureBinary(cfg.LibrespotPath)
	if err != nil {
		return err
	}
	m.binaryPath = binPath

	// Offer to install the audio client before writing the config, so a freshly
	// installed pactl is detected and the pulseaudio backend is selected.
	EnsureAudioDeps()

	if err := WriteConfig(cfg.DeviceName, cfg.DaemonPort); err != nil {
		return fmt.Errorf("writing daemon config: %w", err)
	}

	firstRun := !credentialsSaved()

	fmt.Println("[i] Starting go-librespot daemon...")

	m.cmd = exec.Command(m.binaryPath, "--config_dir", configDirPath())
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("creating output pipe: %w", err)
	}
	m.cmd.Stdout = pw
	m.cmd.Stderr = pw

	if err := m.cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("starting daemon: %w", err)
	}
	pw.Close() // the child holds its own copy; close ours so the reader sees EOF

	go m.consumeOutput(pr)

	fmt.Printf("[i] Daemon PID %d\n", m.cmd.Process.Pid)

	if firstRun {
		if err := m.promptLogin(); err != nil {
			m.Stop()
			return err
		}
	}

	fmt.Println("[i] Waiting for the daemon API to be ready...")
	if err := m.waitReady(60 * time.Second); err != nil {
		m.Stop()
		if out := strings.TrimSpace(m.log.String()); out != "" {
			return fmt.Errorf("%w\n  daemon output:\n%s", err, out)
		}
		return err
	}

	m.ready.Store(true)
	fmt.Println("[✓] Daemon is ready.")
	return nil
}

// promptLogin waits for go-librespot to emit the authorization link, shows it,
// and completes the login.
//
// In WSL the browser (on Windows) cannot reach the daemon's 127.0.0.1 callback,
// so after authorizing, the browser fails to open a
// http://127.0.0.1:<port>/login?code=... page. We ask the user to paste that
// callback URL and deliver it to the login server ourselves — from inside WSL,
// where 127.0.0.1 does reach the daemon.
func (m *Manager) promptLogin() error {
	fmt.Println("[i] First run: Spotify login required.")

	select {
	case url := <-m.authURLCh:
		fmt.Println()
		fmt.Println("  ┌─────────────────────────────────────────────────────────────")
		fmt.Println("  │ 1. Open this link in your browser and authorize the app:")
		fmt.Println("  │")
		fmt.Printf("  │      %s\n", url)
		fmt.Println("  │")
		fmt.Println("  │ 2. Your browser will then try to open a http://127.0.0.1:...")
		fmt.Println("  │    page and likely fail to load — that is expected in WSL.")
		fmt.Println("  │    Copy that address from the browser's address bar.")
		fmt.Println("  └─────────────────────────────────────────────────────────────")
		fmt.Println()
	case <-time.After(30 * time.Second):
		fmt.Println("[!] Did not receive a login link within 30s; check the output above.")
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("  Paste the 127.0.0.1 callback URL here (or press Enter if login already succeeded): ")
		line, _ := reader.ReadString('\n')
		cb := strings.TrimSpace(line)
		if cb == "" {
			return nil
		}
		if err := forwardCallback(cb); err != nil {
			fmt.Printf("  [!] Could not deliver that URL: %v — try again.\n", err)
			continue
		}
		fmt.Println("  [✓] Login completed.")
		return nil
	}
}

// forwardCallback delivers the Spotify OAuth callback URL to the daemon's local
// login server (reachable from inside WSL).
func forwardCallback(rawURL string) error {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("not a URL")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("login server returned %d", resp.StatusCode)
	}
	return nil
}

// consumeOutput reads the daemon's combined stdout/stderr line by line. Until
// the daemon is ready it retains lines for error reporting; throughout, it
// detects the Spotify authorization link and forwards it to promptLogin.
func (m *Manager) consumeOutput(r io.ReadCloser) {
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		ln := scanner.Text()
		if !m.ready.Load() {
			m.log.WriteString(ln + "\n")
		}
		if url := authURL(ln); url != "" {
			select {
			case m.authURLCh <- url:
			default:
			}
		}
	}
}

// authURL extracts a Spotify authorization URL from a log line, or "" if none.
func authURL(line string) string {
	i := strings.Index(line, "https://accounts.spotify.com")
	if i < 0 {
		return ""
	}
	url := line[i:]
	if j := strings.IndexAny(url, " \t\""); j >= 0 {
		url = url[:j]
	}
	return url
}

// credentialsSaved reports whether go-librespot already has usable stored
// credentials, i.e. the interactive login has been completed on a previous run.
// A state.json with an empty username (the default) does not count.
func credentialsSaved() bool {
	dir, err := configDir()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		return false
	}
	var state struct {
		Credentials struct {
			Username string          `json:"username"`
			Data     json.RawMessage `json:"data"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return false
	}
	return state.Credentials.Username != "" && len(state.Credentials.Data) > 0 && string(state.Credentials.Data) != "null"
}

// Stop terminates the daemon process, asking it to shut down gracefully where
// the OS supports it (SIGTERM on Unix) and falling back to a hard kill.
func (m *Manager) Stop() {
	if m.cmd == nil || m.cmd.Process == nil {
		return
	}

	// Windows does not support SIGTERM via Process.Signal, so kill directly.
	if runtime.GOOS == "windows" {
		_ = m.cmd.Process.Kill()
		_ = m.cmd.Wait()
		return
	}

	_ = m.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_ = m.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = m.cmd.Process.Kill()
	}
}

// waitReady polls GET http://localhost:{port}/ until 200 or timeout.
func (m *Manager) waitReady(timeout time.Duration) error {
	url := fmt.Sprintf("http://localhost:%d/", m.port)
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode < 500 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not respond within %v", timeout)
}

// configDirPath returns the ~/.spotify-cli path (helper for exec args).
func configDirPath() string {
	dir, _ := configDir()
	return dir
}
