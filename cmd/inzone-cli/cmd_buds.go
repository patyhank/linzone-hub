package main

import (
	"fmt"
	"time"

	airoha "github.com/patyhank/linzone-hub/internal/protocol/airoha"
	"github.com/patyhank/linzone-hub/internal/usb"
	"github.com/spf13/cobra"
)

var budsCmd = &cobra.Command{
	Use:   "buds",
	Short: "INZONE Buds specific controls",
	Long: `Commands that talk to INZONE Buds (PID 0x0EC2 / 0x0EC3) over the Buds control interface.

Normal Buds status queries use the standard HID path:
  go run ./cmd/inzone-cli buds battery 0
  go build -o inzone ./cmd/inzone-cli
  ./inzone buds listen 0 5000

Make sure you have proper udev rules (see "inzone udev").
`,
}

func init() {
	rootCmd.AddCommand(budsCmd)

	// buds battery <idx>
	budsCmd.AddCommand(&cobra.Command{
		Use:   "battery <device-index>",
		Short: "Read battery status (left / right / case)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx := mustAtoi(args[0])
			var lastErr error
			for attempt := 1; attempt <= 3; attempt++ {
				buds, d, cleanup, err := openBudsControl(idx)
				if err != nil {
					return err
				}
				bi, err := buds.GetBattery()
				cleanup()
				if err == nil {
					fmt.Printf("INZONE Buds battery (%s):\n", d.Model)
					fmt.Printf("  Left : %3d%%  (status=%d)\n", bi.LeftPercent, bi.LeftStatus)
					fmt.Printf("  Right: %3d%%  (status=%d)\n", bi.RightPercent, bi.RightStatus)
					fmt.Printf("  Case : %3d%%  (status=%d)\n", bi.CasePercent, bi.CaseStatus)
					return nil
				}
				lastErr = err
				if attempt < 3 {
					time.Sleep(300 * time.Millisecond)
				}
			}
			return fmt.Errorf("get battery: %w", lastErr)
		},
	})

	// buds probe <idx>
	// Passively listens for observed Buds notifications.
	budsCmd.AddCommand(&cobra.Command{
		Use:   "probe <device-index>",
		Short: "Listen for notifications from Buds",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx := mustAtoi(args[0])
			buds, d, cleanup, err := openBudsControl(idx)
			if err != nil {
				return err
			}
			defer cleanup()

			fmt.Printf("Listening on %s (PID 0x%04X, path=%s) for 2s...\n\n", d.Model, d.ProductID, d.Path)
			raws := buds.ListenRaw(2 * time.Second)
			if len(raws) == 0 {
				fmt.Println("No notifications received during listen window.")
				return nil
			}
			for i, r := range raws {
				fmt.Printf("RAW[%02d]: % X\n", i, r)
				printBudsNotification(r)
			}
			return nil
		},
	})

	// buds listen <idx> [duration-ms]
	// Pure passive capture. Extremely useful to see what the buds send by themselves.
	budsCmd.AddCommand(&cobra.Command{
		Use:   "listen <device-index> [duration-ms]",
		Short: "Open Buds control interface and dump raw HID reports",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx := mustAtoi(args[0])
			dur := 2000 // default 2 seconds
			if len(args) >= 2 {
				dur = mustAtoi(args[1])
			}
			if dur < 100 {
				dur = 100
			}
			if dur > 30000 {
				dur = 30000
			}

			buds, d, cleanup, err := openBudsControl(idx)
			if err != nil {
				return err
			}
			defer cleanup()

			fmt.Printf("Listening on %s (PID 0x%04X) for %d ms...\n", d.Model, d.ProductID, dur)
			fmt.Println("(Many buds only start pushing battery/ANC/wearing after the control channel is opened.)")

			raws := buds.ListenRaw(time.Duration(dur) * time.Millisecond)
			if len(raws) == 0 {
				fmt.Println("No reports received during listen window.")
				fmt.Println("Try: longer duration, or first run 'buds probe' to wake the device, or 'buds battery' / 'info 0'.")
				return nil
			}
			for i, r := range raws {
				fmt.Printf("RAW[%02d]: % X\n", i, r)
				printBudsNotification(r)
			}
			return nil
		},
	})

	// buds raw <idx> <target> <hex-bytes...>
	// Very low-level escape hatch.
	// Example: inzone buds raw 0 remote 05 5A 00
	budsCmd.AddCommand(&cobra.Command{
		Use:   "raw <device-index> <target> <byte0> [byte1 ...]",
		Short: "Send a raw RACE packet to the Buds relay HID (advanced)",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			idx := mustAtoi(args[0])
			targetStr := args[1]

			client, info, cleanup, err := openBudsRelay(idx)
			if err != nil {
				return err
			}
			defer cleanup()

			target := parseRelayTarget(targetStr)

			inner := make([]byte, 0, len(args)-2)
			for i := 2; i < len(args); i++ {
				b := byte(mustAtoi(args[i]))
				inner = append(inner, b)
			}

			resp, err := client.SendCommand(target, inner)
			if err != nil {
				return fmt.Errorf("raw send: %w", err)
			}
			fmt.Printf("Sent to target 0x%02X on %s\n", target, info.Model)
			fmt.Printf("Response target: 0x%02X\n", resp.Target)
			fmt.Printf("Response inner (%d bytes): % X\n", len(resp.Inner), resp.Inner)
			fmt.Printf("Full raw frame: % X\n", resp.Raw)
			return nil
		},
	})
}

func openBudsControl(idx int) (*airoha.BudsDevice, usb.DeviceInfo, func(), error) {
	devs, err := usb.Enumerate()
	if err != nil {
		return nil, usb.DeviceInfo{}, nil, err
	}
	if idx < 0 || idx >= len(devs) {
		return nil, usb.DeviceInfo{}, nil, fmt.Errorf("device index %d out of range (have %d)", idx, len(devs))
	}
	d := devs[idx]
	if !usb.IsBuds(d.ProductID) {
		fmt.Printf("Warning: device %d is %s (PID 0x%04X), not a known Buds PID. Trying anyway.\n", idx, d.Model, d.ProductID)
	}
	control, err := usb.FindBudsControlInterface(d)
	if err != nil {
		return nil, d, nil, err
	}

	hiddev, err := usb.Open(control)
	if err != nil {
		return nil, control, nil, fmt.Errorf("open device: %w\n\nTry: sudo or `inzone udev` + udev rules", err)
	}

	buds := airoha.OpenBuds(hiddev)
	cleanup := func() {
		_ = buds.Close()
	}
	return buds, control, cleanup, nil
}

func openBudsRelay(idx int) (*airoha.AirohaClient, usb.DeviceInfo, func(), error) {
	devs, err := usb.Enumerate()
	if err != nil {
		return nil, usb.DeviceInfo{}, nil, err
	}
	if idx < 0 || idx >= len(devs) {
		return nil, usb.DeviceInfo{}, nil, fmt.Errorf("device index %d out of range (have %d)", idx, len(devs))
	}
	base := devs[idx]
	raceInfo, err := usb.FindBudsRaceInterface(base)
	if err != nil {
		return nil, base, nil, err
	}
	hiddev, err := usb.Open(raceInfo)
	if err != nil {
		return nil, raceInfo, nil, fmt.Errorf("open relay device: %w", err)
	}
	client := airoha.NewAirohaClient(hiddev)
	cleanup := func() {
		_ = hiddev.Close()
	}
	return client, raceInfo, cleanup, nil
}

func parseRelayTarget(s string) byte {
	switch s {
	case "local", "LOCAL", "0":
		return airoha.TargetLocal
	case "remote", "REMOTE", "128", "0x80":
		return airoha.TargetRemote
	default:
		n := mustAtoi(s)
		return byte(n)
	}
}

func printBudsNotification(raw []byte) {
	n, err := airoha.DecodeBudsNotification(raw)
	if err != nil {
		return
	}
	fmt.Printf("  Decoded: event=%d type=0x%02X addr=0x%02X txid=0x%04X param=% X\n",
		n.EventID, n.EventType, n.Address, n.TxID, n.Param)
	if bi, _, err := airoha.DecodeBudsBatteryNotification(raw); err == nil {
		fmt.Printf("  Battery: Left=%d%%(status=%d) Right=%d%%(status=%d) Case=%d%%(status=%d)\n",
			bi.LeftPercent, bi.LeftStatus, bi.RightPercent, bi.RightStatus, bi.CasePercent, bi.CaseStatus)
	}
}
