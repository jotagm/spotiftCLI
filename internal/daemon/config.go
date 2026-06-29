package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// configDir returns ~/.spotify-cli
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find home directory: %w", err)
	}
	return filepath.Join(home, ".spotify-cli"), nil
}

// ConfigPath returns the path to the go-librespot config file.
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yml"), nil
}

// WriteConfig generates the go-librespot config.yml.
func WriteConfig(deviceName string, port int) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	audioBackend := detectAudioBackend()

	cfgPath := filepath.Join(dir, "config.yml")
	// Interactive credentials: go-librespot logs in to the user's account over an
	// outbound connection after a one-time browser authorization. This avoids
	// mDNS/zeroconf discovery, which does not work reliably across WSL's NAT.
	// log_level is "info" so the authentication link is printed to stdout.
	content := fmt.Sprintf(`device_name: %q
device_type: computer
audio_backend: %s
zeroconf_enabled: false
credentials:
  type: interactive
server:
  enabled: true
  address: localhost
  port: %d
volume_steps: 100
log_level: info
`, deviceName, audioBackend, port)

	return os.WriteFile(cfgPath, []byte(content), 0644)
}

// detectAudioBackend returns "pulseaudio" if pactl is available, otherwise "alsa".
func detectAudioBackend() string {
	if _, err := exec.LookPath("pactl"); err == nil {
		return "pulseaudio"
	}
	return "alsa"
}
