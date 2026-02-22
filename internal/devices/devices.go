package devices

import (
	"context"
	"fmt"
	"strings"

	"github.com/zmb3/spotify/v2"
)

type Device struct {
	ID             string
	Name           string
	Type           string
	IsActive       bool
	IsRestricted   bool
	VolumePercent  int
	SupportsVolume bool
}

type DeviceManager struct {
	client *spotify.Client
	ctx    context.Context
}

func NewDeviceManager(client *spotify.Client) *DeviceManager {
	return &DeviceManager{
		client: client,
		ctx:    context.Background(),
	}
}

func (dm *DeviceManager) GetDevices() ([]Device, error) {
	devices, err := dm.client.PlayerDevices(dm.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	result := make([]Device, len(devices))
	for i, d := range devices {
		result[i] = Device{
			ID:             d.ID.String(),
			Name:           d.Name,
			Type:           d.Type,
			IsActive:       d.Active,
			IsRestricted:   d.Restricted,
			VolumePercent:  int(d.Volume),
			SupportsVolume: !d.Restricted,
		}
	}

	return result, nil
}

func (dm *DeviceManager) GetActiveDevice() (*Device, error) {
	devices, err := dm.GetDevices()
	if err != nil {
		return nil, err
	}

	for _, d := range devices {
		if d.IsActive {
			return &d, nil
		}
	}

	return nil, fmt.Errorf("no active device found")
}

func (dm *DeviceManager) TransferPlayback(deviceID string, play bool) error {
	id := spotify.ID(deviceID)
	err := dm.client.TransferPlayback(dm.ctx, id, play)
	if err != nil {
		return fmt.Errorf("failed to transfer playback: %w", err)
	}
	return nil
}

func (dm *DeviceManager) EnsureActiveDevice() (*Device, error) {
	device, err := dm.GetActiveDevice()
	if err == nil {
		return device, nil
	}

	devices, err := dm.GetDevices()
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("no devices available - please open Spotify on a device")
	}

	firstDevice := devices[0]
	err = dm.TransferPlayback(firstDevice.ID, false)
	if err != nil {
		return nil, err
	}

	return &firstDevice, nil
}

func FormatDeviceType(deviceType string) string {
	icons := map[string]string{
		"Computer":    "💻",
		"Smartphone":  "📱",
		"Speaker":     "🔊",
		"TV":          "📺",
		"AVR":         "🎛️",
		"STB":         "📦",
		"AudioDongle": "🎧",
		"GameConsole": "🎮",
		"CastVideo":   "📺",
		"CastAudio":   "🔊",
		"Automobile":  "🚗",
		"Unknown":     "❓",
	}

	icon, exists := icons[deviceType]
	if !exists {
		icon = icons["Unknown"]
	}

	return icon
}

func DisplayDevices(devices []Device) {
	const (
		colorReset  = "\033[0m"
		colorGreen  = "\033[32m"
		colorCyan   = "\033[36m"
		colorYellow = "\033[33m"
		colorGray   = "\033[90m"
		colorBold   = "\033[1m"
	)

	if len(devices) == 0 {
		fmt.Println(colorYellow + "⚠ No devices found" + colorReset)
		fmt.Println(colorGray + "Please open Spotify on at least one device" + colorReset)
		return
	}

	fmt.Println()
	fmt.Println(colorBold + colorCyan + "📱 AVAILABLE DEVICES" + colorReset)
	fmt.Println()

	maxNameLen := 0
	for _, d := range devices {
		if len(d.Name) > maxNameLen {
			maxNameLen = len(d.Name)
		}
	}
	if maxNameLen > 40 {
		maxNameLen = 40
	}

	for i, d := range devices {
		icon := FormatDeviceType(d.Type)

		activeIndicator := "  "
		nameColor := colorGray
		if d.IsActive {
			activeIndicator = colorGreen + "▶ " + colorReset
			nameColor = colorBold
		}

		name := d.Name
		if len(name) > 40 {
			name = name[:37] + "..."
		}

		volumeInfo := ""
		if d.SupportsVolume {
			volumeInfo = fmt.Sprintf(colorGray+"[%d%%]"+colorReset, d.VolumePercent)
		}

		deviceInfo := fmt.Sprintf(colorGray+"%s %s"+colorReset, d.Type, volumeInfo)

		fmt.Printf("%s%s %s %-*s  %s\n",
			activeIndicator,
			icon,
			nameColor+name+colorReset,
			maxNameLen-len(name),
			"",
			deviceInfo,
		)

		if d.IsRestricted {
			fmt.Printf(colorGray + "     ⚠ Volume control restricted" + colorReset + "\n")
		}

		fmt.Printf(colorGray+"     ID: %s"+colorReset+"\n", d.ID)

		if i < len(devices)-1 {
			fmt.Println()
		}
	}

	fmt.Println()

	activeCount := 0
	for _, d := range devices {
		if d.IsActive {
			activeCount++
		}
	}

	if activeCount == 0 {
		fmt.Println(colorYellow + "💡 Tip: Use 'spotify transfer <device-id>' to activate a device" + colorReset)
	}

	fmt.Println()
}

func DisplayDeviceStatus(device *Device) {
	const (
		colorReset = "\033[0m"
		colorGreen = "\033[32m"
		colorCyan  = "\033[36m"
		colorBold  = "\033[1m"
	)

	icon := FormatDeviceType(device.Type)

	fmt.Println()
	fmt.Printf(colorGreen+"▶ "+colorReset+"%s "+colorBold+colorCyan+"%s"+colorReset+"\n",
		icon, device.Name)

	if device.SupportsVolume {
		fmt.Printf("  Volume: %d%%\n", device.VolumePercent)
	}

	fmt.Println()
}

func SelectDeviceInteractive(devices []Device) (*Device, error) {
	if len(devices) == 0 {
		return nil, fmt.Errorf("no devices available")
	}

	if len(devices) == 1 {
		return &devices[0], nil
	}

	for _, d := range devices {
		if d.IsActive {
			return &d, nil
		}
	}

	return &devices[0], nil
}

func FindDeviceByName(devices []Device, name string) (*Device, error) {
	name = strings.ToLower(name)

	for _, d := range devices {
		if strings.ToLower(d.Name) == name {
			return &d, nil
		}
		if strings.Contains(strings.ToLower(d.Name), name) {
			return &d, nil
		}
	}

	return nil, fmt.Errorf("device not found: %s", name)
}
