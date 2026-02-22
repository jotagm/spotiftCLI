package daemon

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

// BinaryPath returns the path where the go-librespot binary is stored.
func BinaryPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bin", "go-librespot"), nil
}

// EnsureBinary checks if the binary exists; if not, downloads it.
func EnsureBinary() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("go-librespot is only available for Linux (current OS: %s)", runtime.GOOS)
	}

	binPath, err := BinaryPath()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	fmt.Print("[i] go-librespot not found. Fetching latest release info...\n")

	archSuffix, err := archSuffix()
	if err != nil {
		return "", err
	}

	assetName := fmt.Sprintf("go-librespot_linux_%s.tar.gz", archSuffix)
	downloadURL, version, err := findReleaseAsset(assetName)
	if err != nil {
		return "", fmt.Errorf("finding release asset: %w", err)
	}

	fmt.Printf("[i] Downloading go-librespot %s (%s)...\n", version, assetName)

	if err := downloadAndExtract(downloadURL, binPath); err != nil {
		return "", fmt.Errorf("downloading go-librespot: %w", err)
	}

	fmt.Printf("[✓] Downloaded to %s\n", binPath)
	return binPath, nil
}

// archSuffix maps runtime.GOARCH to the go-librespot asset suffix.
func archSuffix() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "arm64", nil
	case "arm":
		return "armv6", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// findReleaseAsset fetches the latest GitHub release and returns the download URL for the named asset.
func findReleaseAsset(assetName string) (url, version string, err error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/devgianlu/go-librespot/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "spotify-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", fmt.Errorf("decoding releases JSON: %w", err)
	}

	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return asset.BrowserDownloadURL, release.TagName, nil
		}
	}
	return "", "", fmt.Errorf("asset %q not found in release %s", assetName, release.TagName)
}

// downloadAndExtract downloads a .tar.gz from url, extracts the "go-librespot" binary to destPath.
func downloadAndExtract(url, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	total := resp.ContentLength
	reader := &progressReader{r: resp.Body, total: total}

	gr, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}
		if hdr.Name == "go-librespot" || filepath.Base(hdr.Name) == "go-librespot" {
			f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return fmt.Errorf("creating binary: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("writing binary: %w", err)
			}
			f.Close()
			fmt.Println() // newline after progress bar
			return nil
		}
	}
	return fmt.Errorf("go-librespot binary not found in archive")
}

// progressReader wraps an io.Reader and prints a progress bar to stdout.
type progressReader struct {
	r       io.Reader
	total   int64
	current int64
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	p.current += int64(n)

	if p.total > 0 {
		pct := int(float64(p.current) / float64(p.total) * 50)
		bar := ""
		for i := 0; i < 50; i++ {
			if i < pct {
				bar += "="
			} else if i == pct {
				bar += ">"
			} else {
				bar += " "
			}
		}
		fmt.Printf("\r  [%s] %d%%", bar, pct*2)
	}
	return n, err
}
