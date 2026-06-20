package usb

import (
	"fmt"
	"sort"

	"github.com/sstallion/go-hid"
)

// Sony VID
const SonyVID = 0x054C

const (
	UsagePageAirohaRace = 0xFF13
)

// Supported PIDs from technical reference
var SupportedPIDs = map[uint16]string{
	// Headsets
	0x0E53: "INZONE H9 (wired)",
	0x0E4C: "INZONE H9 (PS5)",
	0x0E61: "INZONE H9 Dongle",
	0x0DFD: "INZONE H7 (3-pole)",
	0x0E47: "INZONE H7 (USB)",
	0x0EBF: "INZONE H5",
	0x0EC2: "INZONE Buds",
	0x0FA8: "INZONE H10",
	0x0F80: "INZONE E9 (3-pole)",
	0x0F81: "INZONE E9 (4-pole)",
	0x0FC0: "INZONE H6 Air (4-pole)",
	0x0FC1: "INZONE H6 Air (3-pole)",
	// Mouse
	0x0FAE: "INZONE Mouse-A (wired)",
	0x0FAF: "INZONE Mouse-A Dongle",
	0x0FB1: "INZONE Mouse-A (bootloader)",
	0x0FB2: "INZONE Mouse-A Dongle (bootloader)",
	// Keyboard
	0x0FB0: "INZONE KBD-H75",
	0x0FB3: "INZONE KBD-H75 (bootloader)",
}

// DeviceInfo wraps go-hid DeviceInfo with INZONE-specific metadata.
// Note: go-hid fields: Path, VendorID, ProductID, SerialNbr, MfrStr, ProductStr,
// UsagePage, Usage, InterfaceNbr, etc.
type DeviceInfo struct {
	hid.DeviceInfo
	Model string
}

// Device is an alias for the underlying HID device handle from go-hid.
// We re-export it under the usb package so that CLI commands and higher layers
// do not need to directly import github.com/sstallion/go-hid.
type Device = hid.Device

// IsSupported checks if the device is a known INZONE device
func IsSupported(vid, pid uint16) bool {
	return vid == SonyVID && SupportedPIDs[pid] != ""
}

// IsBuds returns true for INZONE Buds / GTW PIDs that use Airoha AB1565 relay (Protocol C).
func IsBuds(pid uint16) bool {
	return pid == 0x0EC2 || pid == 0x0EC3
}

func IsMouse(pid uint16) bool {
	switch pid {
	case 0x0FAE, 0x0FAF, 0x0FB1, 0x0FB2:
		return true
	default:
		return false
	}
}

func IsKeyboard(pid uint16) bool {
	return pid == 0x0FB0 || pid == 0x0FB3
}

func IsHeadset(pid uint16) bool {
	if IsMouse(pid) || IsKeyboard(pid) {
		return false
	}
	return IsSupported(SonyVID, pid)
}

func IsLegacySerialHeadset(pid uint16) bool {
	switch pid {
	case 0x0E53, 0x0E4C, 0x0E61:
		return true
	default:
		return false
	}
}

// GetModelName returns the human-readable model name
func GetModelName(pid uint16) string {
	if name, ok := SupportedPIDs[pid]; ok {
		return name
	}
	return fmt.Sprintf("Unknown (PID 0x%04X)", pid)
}

func isProtocolAInterface(info *hid.DeviceInfo) bool {
	if IsBuds(info.ProductID) {
		return info.UsagePage == 0xFF00 || info.UsagePage == 0xFF90
	}
	return info.UsagePage == 0xFF00 || info.UsagePage == 0xFF90 || info.UsagePage == 0xFF04
}

func isPreferredProtocolAInterface(info *hid.DeviceInfo) bool {
	return isProtocolAInterface(info)
}

func isBudsRaceInterface(info *hid.DeviceInfo) bool {
	return IsBuds(info.ProductID) && info.UsagePage == UsagePageAirohaRace
}

func isLikelySonyControlInterface(info *hid.DeviceInfo) bool {
	if IsBuds(info.ProductID) {
		return info.UsagePage == 0xFF04 || info.UsagePage == 0xFF03 || info.UsagePage == 0xFF01
	}
	if isProtocolAInterface(info) || isBudsRaceInterface(info) {
		return false
	}
	return info.UsagePage != 0
}

func deviceKey(info *hid.DeviceInfo) string {
	if IsMouse(info.ProductID) || IsKeyboard(info.ProductID) {
		if info.SerialNbr != "" {
			return fmt.Sprintf("kbm:serial:%s pid:%04x", info.SerialNbr, info.ProductID)
		}
		if info.ProductStr != "" {
			return fmt.Sprintf("kbm:product:%s pid:%04x", info.ProductStr, info.ProductID)
		}
		return fmt.Sprintf("kbm:pid:%04x", info.ProductID)
	}
	if info.Path != "" {
		return fmt.Sprintf("path:%s pid:%04x", info.Path, info.ProductID)
	}
	if info.SerialNbr != "" {
		return fmt.Sprintf("serial:%s pid:%04x", info.SerialNbr, info.ProductID)
	}
	if info.ProductStr != "" {
		return fmt.Sprintf("product:%s pid:%04x", info.ProductStr, info.ProductID)
	}
	return fmt.Sprintf("pid:%04x", info.ProductID)
}

func devicePreferenceScore(info *hid.DeviceInfo) int {
	score := 0
	switch {
	case isBudsRaceInterface(info):
		score = -100
	case (IsMouse(info.ProductID) || IsKeyboard(info.ProductID)) && info.UsagePage == 0xFF00:
		score = 160
	case (IsMouse(info.ProductID) || IsKeyboard(info.ProductID)) && info.UsagePage == 0xFF90:
		score = 150
	case (IsMouse(info.ProductID) || IsKeyboard(info.ProductID)) && info.UsagePage == 0xFF04:
		score = 140
	case IsBuds(info.ProductID) && info.UsagePage == 0xFF04:
		score = 140
	case IsBuds(info.ProductID) && info.UsagePage == 0xFF01:
		score = 120
	case IsBuds(info.ProductID) && info.UsagePage == 0xFF03:
		score = 110
	case info.UsagePage == 0xFF01:
		score = 120
	case info.UsagePage == 0xFF03:
		score = 110
	case isProtocolAInterface(info):
		score = 100
	case isLikelySonyControlInterface(info):
		score = 80
	case info.UsagePage != 0:
		score = 40
	default:
		score = 10
	}
	if info.InterfaceNbr >= 0 {
		score += 1
	}
	return score
}

func samePhysicalDevice(base DeviceInfo, candidate *hid.DeviceInfo) bool {
	if base.VendorID != candidate.VendorID || base.ProductID != candidate.ProductID {
		return false
	}
	if base.SerialNbr != "" && candidate.SerialNbr != "" {
		return base.SerialNbr == candidate.SerialNbr
	}
	if base.ProductStr != "" && candidate.ProductStr != "" {
		return base.ProductStr == candidate.ProductStr
	}
	return true
}

// Enumerate returns all connected supported INZONE devices (Sony VID + known PID).
func Enumerate() ([]DeviceInfo, error) {
	best := map[string]DeviceInfo{}
	err := hid.Enumerate(SonyVID, 0, func(info *hid.DeviceInfo) error {
		if !IsSupported(info.VendorID, info.ProductID) {
			return nil
		}
		candidate := DeviceInfo{
			DeviceInfo: *info,
			Model:      GetModelName(info.ProductID),
		}
		key := deviceKey(info)
		current, ok := best[key]
		if !ok || devicePreferenceScore(info) > devicePreferenceScore(&current.DeviceInfo) {
			best[key] = candidate
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result := make([]DeviceInfo, 0, len(best))
	for _, d := range best {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ProductID != result[j].ProductID {
			return result[i].ProductID < result[j].ProductID
		}
		return result[i].Path < result[j].Path
	})
	return result, nil
}

// Open opens a device by its info using the stable Path (recommended on Linux).
func Open(info DeviceInfo) (*hid.Device, error) {
	dev, err := hid.OpenPath(info.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open device %s: %w", info.Path, err)
	}
	return dev, nil
}

// EnumerateAll returns every HID interface from Sony VID (for diagnostics).
func EnumerateAll() ([]DeviceInfo, error) {
	var result []DeviceInfo
	err := hid.Enumerate(SonyVID, 0, func(info *hid.DeviceInfo) error {
		result = append(result, DeviceInfo{
			DeviceInfo: *info,
			Model:      GetModelName(info.ProductID),
		})
		return nil
	})
	return result, err
}

// FindBudsControlInterface locates the Sony control collection for the selected Buds device.
func FindBudsControlInterface(base DeviceInfo) (DeviceInfo, error) {
	if !IsBuds(base.ProductID) {
		return DeviceInfo{}, fmt.Errorf("%s is not a Buds device", base.Model)
	}
	if isLikelySonyControlInterface(&base.DeviceInfo) {
		return base, nil
	}

	var match DeviceInfo
	err := hid.Enumerate(SonyVID, 0, func(info *hid.DeviceInfo) error {
		if !samePhysicalDevice(base, info) || !isLikelySonyControlInterface(info) {
			return nil
		}
		match = DeviceInfo{DeviceInfo: *info, Model: GetModelName(info.ProductID)}
		return fmt.Errorf("match found")
	})
	if err != nil && err.Error() != "match found" {
		return DeviceInfo{}, err
	}
	if match.Path == "" {
		if base.Path != "" {
			return base, nil
		}
		return DeviceInfo{}, fmt.Errorf("no Buds Sony control HID collection found for %s", base.Model)
	}
	return match, nil
}

func FindHeadsetControlInterfaces(base DeviceInfo) ([]DeviceInfo, error) {
	if !IsHeadset(base.ProductID) || IsBuds(base.ProductID) {
		return nil, fmt.Errorf("%s is not a standard headset device", base.Model)
	}

	var matches []DeviceInfo
	err := hid.Enumerate(SonyVID, base.ProductID, func(info *hid.DeviceInfo) error {
		if !samePhysicalDevice(base, info) || !isLikelySonyControlInterface(info) {
			return nil
		}
		matches = append(matches, DeviceInfo{DeviceInfo: *info, Model: GetModelName(info.ProductID)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 && base.Path != "" {
		matches = append(matches, base)
	}
	sort.Slice(matches, func(i, j int) bool {
		return devicePreferenceScore(&matches[i].DeviceInfo) > devicePreferenceScore(&matches[j].DeviceInfo)
	})
	return matches, nil
}

func FindProtocolAInterface(base DeviceInfo) (DeviceInfo, error) {
	if !IsMouse(base.ProductID) && !IsKeyboard(base.ProductID) {
		return DeviceInfo{}, fmt.Errorf("%s is not a keyboard/mouse Protocol A device", base.Model)
	}
	if isPreferredProtocolAInterface(&base.DeviceInfo) {
		return base, nil
	}

	var matches []DeviceInfo
	err := hid.Enumerate(SonyVID, base.ProductID, func(info *hid.DeviceInfo) error {
		if !samePhysicalDevice(base, info) || !isPreferredProtocolAInterface(info) {
			return nil
		}
		matches = append(matches, DeviceInfo{DeviceInfo: *info, Model: GetModelName(info.ProductID)})
		return nil
	})
	if err != nil {
		return DeviceInfo{}, err
	}
	if len(matches) == 0 {
		if base.Path != "" {
			return base, nil
		}
		return DeviceInfo{}, fmt.Errorf("no Protocol A HID collection found for %s", base.Model)
	}
	sort.Slice(matches, func(i, j int) bool {
		return devicePreferenceScore(&matches[i].DeviceInfo) > devicePreferenceScore(&matches[j].DeviceInfo)
	})
	return matches[0], nil
}

// FindBudsRaceInterface locates the UsagePage 0xFF13 Airoha Race collection for the selected Buds device.
func FindBudsRaceInterface(base DeviceInfo) (DeviceInfo, error) {
	if !IsBuds(base.ProductID) {
		return DeviceInfo{}, fmt.Errorf("%s is not a Buds device", base.Model)
	}

	var match DeviceInfo
	err := hid.Enumerate(SonyVID, 0, func(info *hid.DeviceInfo) error {
		if !samePhysicalDevice(base, info) || !isBudsRaceInterface(info) {
			return nil
		}
		match = DeviceInfo{DeviceInfo: *info, Model: GetModelName(info.ProductID)}
		return fmt.Errorf("match found")
	})
	if err != nil && err.Error() != "match found" {
		return DeviceInfo{}, err
	}
	if match.Path == "" {
		return DeviceInfo{}, fmt.Errorf("no Buds Race HID collection (UsagePage 0x%04X) found for %s", UsagePageAirohaRace, base.Model)
	}
	return match, nil
}

// PrintUdevRule outputs a recommended udev rule for normal-user access on Linux.
func PrintUdevRule() {
	fmt.Print(`# Recommended udev rule for Sony INZONE devices (Linux)
# Save as: /etc/udev/rules.d/99-sony-inzone.rules
# Then run:
#   sudo udevadm control --reload-rules && sudo udevadm trigger
# And ensure your user is in the group (e.g. plugdev):
#   sudo usermod -aG plugdev $USER && newgrp plugdev

SUBSYSTEM=="hidraw", ATTRS{idVendor}=="054c", MODE="0660", GROUP="plugdev"
SUBSYSTEM=="usb", ATTRS{idVendor}=="054c", MODE="0660", GROUP="plugdev"
`)
}
