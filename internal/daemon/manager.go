package daemon

import (
	"fmt"
	"net/http"
	"os/exec"
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
}

// NewManager creates a Manager from the application config.
func NewManager(cfg *config.Config) *Manager {
	binPath, _ := BinaryPath()
	cfgPath, _ := ConfigPath()
	return &Manager{
		binaryPath: binPath,
		configPath: cfgPath,
		port:       cfg.DaemonPort,
	}
}

// Start downloads the binary if needed, writes config, then launches the daemon.
// It polls the HTTP API until it responds or the 30-second timeout expires.
func (m *Manager) Start(cfg *config.Config) error {
	binPath, err := EnsureBinary()
	if err != nil {
		return err
	}
	m.binaryPath = binPath

	if err := WriteConfig(cfg.DeviceName, cfg.DaemonPort); err != nil {
		return fmt.Errorf("writing daemon config: %w", err)
	}

	fmt.Println("[i] Starting go-librespot daemon...")

	m.cmd = exec.Command(m.binaryPath, "--config-dir", configDirPath())
	m.cmd.SysProcAttr = &syscall.SysProcAttr{}

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("starting daemon: %w", err)
	}

	fmt.Printf("[i] Daemon PID %d — waiting for API to be ready...\n", m.cmd.Process.Pid)

	if err := m.waitReady(30 * time.Second); err != nil {
		m.Stop()
		return err
	}

	fmt.Println("[✓] Daemon is ready.")
	return nil
}

// Stop sends SIGTERM to the daemon process.
func (m *Manager) Stop() {
	if m.cmd == nil || m.cmd.Process == nil {
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
