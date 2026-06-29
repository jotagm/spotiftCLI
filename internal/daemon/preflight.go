package daemon

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// EnsureAudioDeps verifies that an audio client go-librespot can use is present.
//
// On Linux it needs PulseAudio (pactl/libpulse) or ALSA (aplay). If neither is
// found — common on a fresh WSL install — it offers to install the PulseAudio
// client via apt, but only after explicit consent. It never runs sudo silently:
// the install inherits the terminal so the user types their own password.
//
// It is a no-op on non-Linux platforms and when a client is already present.
func EnsureAudioDeps() {
	if runtime.GOOS != "linux" {
		return
	}
	if hasAudioClient() {
		return
	}

	fmt.Println("[!] No audio client found (pactl/aplay).")
	fmt.Println("    go-librespot needs PulseAudio or ALSA to play sound.")

	install := aptInstallSteps()
	if install == nil {
		fmt.Println("    Install a PulseAudio client with your package manager")
		fmt.Println("    (packages: pulseaudio-utils, libpulse0).")
		return
	}

	fmt.Print("    Install it now with sudo apt-get? [y/N] ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if strings.ToLower(strings.TrimSpace(line)) != "y" {
		fmt.Println("    Skipped. Install manually:")
		fmt.Println("      sudo apt-get install -y pulseaudio-utils libpulse0")
		return
	}

	if err := runSteps(install); err != nil {
		fmt.Fprintf(os.Stderr, "[✗] Install failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "    Run manually: sudo apt-get install -y pulseaudio-utils libpulse0")
		return
	}
	fmt.Println("[✓] Audio client installed.")
}

// hasAudioClient reports whether a usable audio client binary is on PATH.
func hasAudioClient() bool {
	for _, bin := range []string{"pactl", "aplay"} {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}
	return false
}

// aptInstallSteps returns the apt-get commands to install the audio runtime
// libraries go-librespot needs, or nil when apt-get is not the system package
// manager (other distros are left to the user, since package names differ).
//
// libasound2 is included even though we configure the PulseAudio backend: the
// go-librespot binary is dynamically linked against libasound and won't load
// without it.
func aptInstallSteps() [][]string {
	if _, err := exec.LookPath("apt-get"); err != nil {
		return nil
	}
	pkgs := []string{"pulseaudio-utils", "libpulse0", alsaPackage()}
	return [][]string{
		{"apt-get", "update"},
		append([]string{"apt-get", "install", "-y"}, pkgs...),
	}
}

// alsaPackage picks the ALSA runtime library package name available on this
// system. Ubuntu 24.04+ renamed it to libasound2t64 (the time_t transition);
// older releases use libasound2.
func alsaPackage() string {
	for _, p := range []string{"libasound2t64", "libasound2"} {
		if exec.Command("apt-cache", "show", p).Run() == nil {
			return p
		}
	}
	return "libasound2t64"
}

// runSteps runs each command under sudo, inheriting the terminal so sudo can
// prompt for the password.
func runSteps(steps [][]string) error {
	for _, args := range steps {
		cmd := exec.Command("sudo", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("sudo %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}
