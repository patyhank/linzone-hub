package main

import (
	"fmt"

	"github.com/patyhank/linzone-hub/internal/usb"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all connected INZONE devices",
	Long:  `Scan for and list all supported Sony INZONE devices currently connected via USB.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		devices, err := usb.Enumerate()
		if err != nil {
			return err
		}

		if len(devices) == 0 {
			fmt.Println("No INZONE devices found.")
			return nil
		}

		fmt.Printf("Found %d INZONE device(s):\n\n", len(devices))
		for i, d := range devices {
			fmt.Printf("[%d] VID: 0x%04X  PID: 0x%04X  Path: %s\n", i, d.VendorID, d.ProductID, d.Path)
			fmt.Printf("    Product: %s\n", d.ProductStr)
			fmt.Printf("    Manufacturer: %s\n", d.MfrStr)
			fmt.Printf("    Serial: %s\n", d.SerialNbr)
			fmt.Printf("    UsagePage: 0x%04X  Usage: 0x%04X  Interface: %d\n\n",
				d.UsagePage, d.Usage, d.InterfaceNbr)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().BoolP("verbose", "v", false, "Show detailed device information")
}
