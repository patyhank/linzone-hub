package main

import (
	"fmt"
	"strconv"

	"github.com/patyhank/linzone-hub/internal/protocol"
	"github.com/patyhank/linzone-hub/internal/usb"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Change device settings (CLI test commands)",
	Long: `Send SET commands to INZONE devices for testing.

Examples:
  inzone set profile 0 2
  inzone set dpi 0 800
  inzone set led 0 200 255 0 128
  inzone set volume 0 75
  inzone set sidetone 0 30
	  inzone set ambient 0 ambient 60 1
	`,
}

func init() {
	rootCmd.AddCommand(setCmd)

	// profile (works for mouse + keyboard + some headsets via profile number)
	setProfileCmd := &cobra.Command{
		Use:   "profile <device-index> <1-4>",
		Short: "Switch active profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, prof, err := parseIdxProfile(args[0], args[1])
			if err != nil {
				return err
			}
			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			defer dev.Close()

			// For Protocol A devices (mouse/kbd) use the KBM setter
			if isProtocolA(d) {
				if err := protocol.SetProfileNumber(dev, prof); err != nil {
					return err
				}
				fmt.Printf("Set profile to %d on %s\n", prof, d.Model)
				return nil
			}

			// For headsets we can try HCI if we know the event, but many headsets use
			// a different "HOST_SELECT_SWITCH" or similar. For now we only do Protocol A here.
			// Headset profile switching is often done via "CONNECTION_DESTINATION_MODE" or similar.
			return fmt.Errorf("profile switch is not implemented for this headset interface yet")
		},
	}
	setCmd.AddCommand(setProfileCmd)

	// dpi (mouse only)
	setDpiCmd := &cobra.Command{
		Use:   "dpi <device-index> <dpi>",
		Short: "Set DPI (mouse). Value must be multiple of 50, >=50",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, dpi, err := parseIdxUint16(args[0], args[1])
			if err != nil {
				return err
			}
			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			defer dev.Close()

			if !isProtocolA(d) {
				return fmt.Errorf("DPI setting is only for mouse/keyboard")
			}
			if err := protocol.SetDPI(dev, dpi); err != nil {
				return err
			}
			fmt.Printf("Set DPI to %d on %s\n", dpi, d.Model)
			return nil
		},
	}
	setCmd.AddCommand(setDpiCmd)

	// led (mouse + some keyboards)
	setLedCmd := &cobra.Command{
		Use:   "led <device-index> <brightness 0-255> <r> <g> <b>",
		Short: "Set LED color/brightness",
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx := mustAtoi(args[0])
			bri := byte(mustAtoi(args[1]))
			r := byte(mustAtoi(args[2]))
			g := byte(mustAtoi(args[3]))
			b := byte(mustAtoi(args[4]))

			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			defer dev.Close()

			if !isProtocolA(d) {
				return fmt.Errorf("LED setting via this command is for mouse/keyboard")
			}
			if err := protocol.SetLEDLighting(dev, bri, r, g, b); err != nil {
				return err
			}
			fmt.Printf("Set LED bri=%d rgb=(%d,%d,%d) on %s\n", bri, r, g, b, d.Model)
			return nil
		},
	}
	setCmd.AddCommand(setLedCmd)

	// lod (mouse)
	setLodCmd := &cobra.Command{
		Use:   "lod <device-index> <0|1|2>",
		Short: "Set lift-off distance (0=0.7mm, 1=1.0mm, 2=2.0mm)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx := mustAtoi(args[0])
			level := byte(mustAtoi(args[1]))
			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			defer dev.Close()

			if !isProtocolA(d) {
				return fmt.Errorf("LOD is for mouse")
			}
			if err := protocol.SetLOD(dev, level); err != nil {
				return err
			}
			fmt.Printf("Set LOD level=%d on %s\n", level, d.Model)
			return nil
		},
	}
	setCmd.AddCommand(setLodCmd)

	// report-rate (mouse)
	setRateCmd := &cobra.Command{
		Use:   "report-rate <device-index> <500|1000|2000|4000|8000>",
		Short: "Set USB report rate / polling rate",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx := mustAtoi(args[0])
			hz := mustAtoi(args[1])
			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			defer dev.Close()

			if !isProtocolA(d) {
				return fmt.Errorf("report-rate is for mouse")
			}
			if err := protocol.SetReportRate(dev, hz); err != nil {
				return err
			}
			fmt.Printf("Set report rate %d Hz on %s\n", hz, d.Model)
			return nil
		},
	}
	setCmd.AddCommand(setRateCmd)

	// === Headset controls (Protocol B) ===

	setVolumeCmd := &cobra.Command{
		Use:   "volume <device-index> <0-100>",
		Short: "Set headphone volume",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, pct, err := parseIdxUint8(args[0], args[1])
			if err != nil {
				return err
			}
			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			if usb.IsBuds(d.ProductID) {
				_ = dev.Close()
				buds, info, cleanup, err := openBudsControl(idx)
				if err != nil {
					return err
				}
				defer cleanup()
				if err := buds.SetHeadphoneVolume(pct); err != nil {
					return fmt.Errorf("set Buds volume: %w", err)
				}
				fmt.Printf("Set headphone volume ~%d%% on %s\n", pct, info.Model)
				return nil
			}
			defer dev.Close()
			if err := protocol.SetHeadphoneVolume(dev, false, pct, pct); err != nil {
				return fmt.Errorf("set volume: %w (this interface may not support headset volume control)", err)
			}
			fmt.Printf("Set headphone volume ~%d%% on %s\n", pct, d.Model)
			return nil
		},
	}
	setCmd.AddCommand(setVolumeCmd)

	setSidetoneCmd := &cobra.Command{
		Use:   "sidetone <device-index> <0-100>",
		Short: "Set sidetone volume (headset)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, pct, err := parseIdxUint8(args[0], args[1])
			if err != nil {
				return err
			}
			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			defer dev.Close()

			if err := protocol.SetSidetone(dev, pct, pct); err != nil {
				return fmt.Errorf("set sidetone: %w", err)
			}
			fmt.Printf("Set sidetone ~%d%% on %s\n", pct, d.Model)
			return nil
		},
	}
	setCmd.AddCommand(setSidetoneCmd)

	setMixCmd := &cobra.Command{
		Use:   "mix <device-index> <0-100>",
		Short: "Set game/chat mix balance (center ~50)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, bal, err := parseIdxUint8(args[0], args[1])
			if err != nil {
				return err
			}
			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			defer dev.Close()

			if err := protocol.SetGameChatMix(dev, bal); err != nil {
				return fmt.Errorf("set mix: %w", err)
			}
			fmt.Printf("Set game/chat mix to %d on %s\n", bal, d.Model)
			return nil
		},
	}
	setCmd.AddCommand(setMixCmd)

	setMicCmd := &cobra.Command{
		Use:   "mic <device-index> <0-100>",
		Short: "Set microphone volume (headset)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx, pct, err := parseIdxUint8(args[0], args[1])
			if err != nil {
				return err
			}
			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			if usb.IsBuds(d.ProductID) {
				_ = dev.Close()
				buds, info, cleanup, err := openBudsControl(idx)
				if err != nil {
					return err
				}
				defer cleanup()
				if err := buds.SetMicVolume(pct); err != nil {
					return fmt.Errorf("set Buds mic: %w", err)
				}
				fmt.Printf("Set mic volume ~%d%% on %s\n", pct, info.Model)
				return nil
			}
			defer dev.Close()

			if err := protocol.SetMicVolume(dev, false, pct, pct); err != nil {
				return fmt.Errorf("set mic: %w", err)
			}
			fmt.Printf("Set mic volume ~%d%% on %s\n", pct, d.Model)
			return nil
		},
	}
	setCmd.AddCommand(setMicCmd)

	setAmbientCmd := &cobra.Command{
		Use:   "ambient <device-index> <mode> <0-100> [voice-focus 0|1]",
		Short: "Set Ambient/NC mode and level",
		Args:  cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx := mustAtoi(args[0])
			mode, err := parseAmbientMode(args[1])
			if err != nil {
				return err
			}
			level, err := parsePercent(args[2])
			if err != nil {
				return err
			}
			voiceFocus := false
			if len(args) == 4 {
				voiceFocus, err = parseBool01(args[3])
				if err != nil {
					return err
				}
			}

			dev, d, err := openDeviceByIndex(idx)
			if err != nil {
				return err
			}
			if usb.IsBuds(d.ProductID) {
				_ = dev.Close()
				buds, info, cleanup, err := openBudsControl(idx)
				if err != nil {
					return err
				}
				defer cleanup()
				if err := buds.SetAmbient(mode, level, voiceFocus); err != nil {
					return fmt.Errorf("set Buds ambient: %w", err)
				}
				fmt.Printf("Set Ambient/NC mode=%d level=%d voiceFocus=%t on %s\n", mode, level, voiceFocus, info.Model)
				return nil
			}
			defer dev.Close()

			if err := protocol.SetAmbient(dev, mode, level, level, voiceFocus); err != nil {
				return fmt.Errorf("set ambient: %w", err)
			}
			fmt.Printf("Set Ambient/NC mode=%d level=%d voiceFocus=%t on %s\n", mode, level, voiceFocus, d.Model)
			return nil
		},
	}
	setCmd.AddCommand(setAmbientCmd)
}

// helpers

func parseIdxProfile(a, b string) (int, byte, error) {
	idx := mustAtoi(a)
	p := mustAtoi(b)
	if p < 1 || p > 4 {
		return 0, 0, fmt.Errorf("profile must be 1-4")
	}
	return idx, byte(p), nil
}

func parseIdxUint16(a, b string) (int, uint16, error) {
	idx := mustAtoi(a)
	v := mustAtoi(b)
	if v < 0 {
		return 0, 0, fmt.Errorf("value must be >= 0")
	}
	return idx, uint16(v), nil
}

func parseIdxUint8(a, b string) (int, byte, error) {
	idx := mustAtoi(a)
	v, err := parseByteRange(b, 255)
	if err != nil {
		return 0, 0, err
	}
	return idx, v, nil
}

func parsePercent(s string) (byte, error) {
	return parseByteRange(s, 100)
}

func parseByteRange(s string, max int) (byte, error) {
	v := mustAtoi(s)
	if v < 0 || v > max {
		return 0, fmt.Errorf("value out of range 0-%d", max)
	}
	return byte(v), nil
}

func parseBool01(s string) (bool, error) {
	v := mustAtoi(s)
	switch v {
	case 0:
		return false, nil
	case 1:
		return true, nil
	default:
		return false, fmt.Errorf("value must be 0 or 1")
	}
}

func parseAmbientMode(s string) (byte, error) {
	switch s {
	case "off":
		return 0, nil
	case "anc", "on", "nc":
		return 1, nil
	case "ambient":
		return 2, nil
	case "custom":
		return 3, nil
	default:
		return parseByteRange(s, 3)
	}
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(fmt.Errorf("invalid number: %s", s))
	}
	return n
}

func openDeviceByIndex(idx int) (*usb.Device, usb.DeviceInfo, error) {
	devs, err := usb.Enumerate()
	if err != nil {
		return nil, usb.DeviceInfo{}, err
	}
	if idx < 0 || idx >= len(devs) {
		return nil, usb.DeviceInfo{}, fmt.Errorf("device index %d out of range (have %d devices)", idx, len(devs))
	}
	d := devs[idx]
	dev, err := usb.Open(d)
	if err != nil {
		return nil, d, fmt.Errorf("open device %d (%s): %w", idx, d.Model, err)
	}
	return dev, d, nil
}

func isProtocolA(d usb.DeviceInfo) bool {
	if usb.IsBuds(d.ProductID) {
		return false
	}
	// Mouse/Keyboard use Protocol A (UsagePage 0xFF00/0xFF90/0xFF04 or known PIDs)
	if d.UsagePage == 0xFF00 || d.UsagePage == 0xFF90 || d.UsagePage == 0xFF04 {
		return true
	}
	// Also treat known mouse/keyboard PIDs as Protocol A even if UsagePage is 0 on some platforms
	switch d.ProductID {
	case 0x0FAE, 0x0FAF, 0x0FB0, 0x0FB1, 0x0FB2, 0x0FB3:
		return true
	}
	return false
}
