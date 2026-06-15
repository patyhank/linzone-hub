package main

import (
	"fmt"

	"github.com/patyhank/linzone-hub/internal/usb"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run basic USB communication test",
	Long:  `Attempts to enumerate devices and perform a minimal open/close cycle for testing HID access.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== INZONE Hub CLI - USB Test ===")
		fmt.Println()

		devices, err := usb.Enumerate()
		if err != nil {
			return fmt.Errorf("enumeration failed: %w", err)
		}

		fmt.Printf("Enumeration: found %d supported device(s)\n", len(devices))
		if len(devices) == 0 {
			fmt.Println("No devices to test. Connect an INZONE device and retry.")
			return nil
		}

		for i, d := range devices {
			fmt.Printf("\n[%d] Testing %s (0x%04X)\n", i, d.Model, d.ProductID)
			dev, err := usb.Open(d)
			if err != nil {
				fmt.Printf("  OPEN FAILED: %v\n", err)
				continue
			}
			fmt.Println("  OPEN OK")
			dev.Close()
			fmt.Println("  CLOSE OK")
		}

		fmt.Println("\n=== Test complete ===")
		return nil
	},
}
