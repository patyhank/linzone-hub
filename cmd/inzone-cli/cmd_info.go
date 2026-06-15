package main

import (
	"fmt"

	"github.com/patyhank/linzone-hub/internal/protocol"
	"github.com/patyhank/linzone-hub/internal/usb"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info [index]",
	Short: "Get detailed information from a device",
	Long:  `Open a device by index (from 'list' command) and query its basic information.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		devices, err := usb.Enumerate()
		if err != nil {
			return err
		}
		if len(devices) == 0 {
			return fmt.Errorf("no INZONE devices found")
		}

		var idx int
		if _, err := fmt.Sscanf(args[0], "%d", &idx); err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}
		if idx < 0 || idx >= len(devices) {
			return fmt.Errorf("index out of range: %d (have %d devices)", idx, len(devices))
		}

		d := devices[idx]
		fmt.Printf("Opening device [%d]: %s\n", idx, d.Model)
		fmt.Printf("  Path: %s\n", d.Path)
		fmt.Printf("  VID:PID = 0x%04X:0x%04X\n", d.VendorID, d.ProductID)
		fmt.Printf("  UsagePage: 0x%04X  Usage: 0x%04X  Interface: %d\n", d.UsagePage, d.Usage, d.InterfaceNbr)

		// Headset path or Buds/GTW.
		// Special case: INZONE Buds use the exposed Buds control interface
		// for startup state. Race HID is kept separate for relay/FOTA only.
		if usb.IsBuds(d.ProductID) {
			fmt.Println("\nDevice opened successfully. Querying...")
			fmt.Println("INZONE Buds / GTW detected.")
			fmt.Println("Querying Sony HCI control path...")

			buds, _, cleanup, err := openBudsControl(idx)
			if err != nil {
				return err
			}
			defer cleanup()

			if info, err := buds.GetStartupInfo(); err == nil {
				if len(info.Model) > 0 {
					fmt.Printf("MODEL_INFO raw: % X\n", info.Model)
				}
				if info.Battery != nil {
					bi := info.Battery
					fmt.Printf("Battery: Left=%d%%(status=%d) Right=%d%%(status=%d) Case=%d%%(status=%d)\n",
						bi.LeftPercent, bi.LeftStatus, bi.RightPercent, bi.RightStatus, bi.CasePercent, bi.CaseStatus)
				}
				if len(info.FW) > 0 {
					fmt.Printf("FW_VERSION raw: % X\n", info.FW)
				}
			} else if bi, err := buds.GetBattery(); err == nil {
				fmt.Printf("Battery: Left=%d%%(status=%d) Right=%d%%(status=%d) Case=%d%%(status=%d)\n",
					bi.LeftPercent, bi.LeftStatus, bi.RightPercent, bi.RightStatus, bi.CasePercent, bi.CaseStatus)
			} else {
				fmt.Printf("Sony HCI startup information not available: %v\n", err)
				fmt.Println("Try `inzone buds listen 0 5000` to inspect the control collection traffic.")
			}
			if vol, err := buds.GetHeadphoneVolume(); err == nil {
				fmt.Printf("Headphone Volume: mute=%d raw=%d percent=%d\n", vol.Mute, vol.Raw, vol.Percent)
			}
			if mic, err := buds.GetMicVolume(); err == nil {
				fmt.Printf("Mic Volume: mute=%d raw=%d percent=%d\n", mic.Mute, mic.Raw, mic.Percent)
			}
			if ambient, err := buds.GetAmbient(); err == nil {
				fmt.Printf("Ambient/NC: mode=%d ambientRaw=%d ambientPercent=%d voiceFocus=%d\n",
					ambient.NCMode, ambient.AmbientRaw, ambient.AmbientPercent, ambient.VoiceFocus)
			}
			return nil
		}

		dev, err := usb.Open(d)
		if err != nil {
			return fmt.Errorf("%w\n\nHint: On Linux you usually need a udev rule for normal-user access.\nRun `inzone udev` to print the recommended rule, or try with sudo for testing.", err)
		}

		fmt.Println("\nDevice opened successfully. Querying...")

		// Protocol A (mouse / keyboard) - UsagePage 0xFF00 / 0xFF90 / 0xFF04
		if d.UsagePage == 0xFF00 || d.UsagePage == 0xFF90 || d.UsagePage == 0xFF04 {
			defer dev.Close()
			raw, err := protocol.GetDeviceInformationKBM(dev)
			if err != nil {
				return fmt.Errorf("GET_DEVICE_INFORMATION failed: %w", err)
			}
			fmt.Printf("Raw response (%d bytes): % X\n\n", len(raw), raw)

			if len(raw) >= 18 {
				if info, err := protocol.ParseMouseInfo(raw); err == nil {
					fmt.Println("Mouse Device Information:")
					fmt.Printf("  PID: 0x%04X  VID: 0x%04X\n", info.PID, info.VID)
					fmt.Printf("  Current Profile: %d\n", info.CurrentProfile)
					fmt.Printf("  Color SKU: %d\n", info.ColorSKU)
					return nil
				}
			}
			fmt.Println("Received device info (raw shown above).")
			return nil
		}

		defer dev.Close()
		fmt.Println("Attempting headset query...")
		model, batt, fw, err := protocol.GetHeadsetInfo(dev)
		if err != nil {
			fmt.Printf("Headset query error: %v\n", err)
		}
		if model != nil {
			fmt.Printf("MODEL_INFO: ModelID=%d Color=%d Serial=%d Status=%d\n",
				model.ModelID, model.Color, model.Serial, model.Status)
		}
		if batt != nil {
			fmt.Printf("BATTERY: Status=%d Percent=%d%%\n", batt.Status, batt.Percent)
		}
		if len(fw) > 0 {
			fmt.Printf("FW_VERSION raw: % X\n", fw)
		}
		if model == nil && batt == nil {
			fmt.Println("No standard headset responses received on this interface.")
			fmt.Println("This may be a non-standard interface or need a device-specific handler.")
		}
		return nil
	},
}
