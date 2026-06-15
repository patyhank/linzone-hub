package main

import (
	"fmt"

	"github.com/patyhank/linzone-hub/internal/usb"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Dump all Sony HID interfaces (advanced diagnostics)",
	RunE: func(cmd *cobra.Command, args []string) error {
		all, err := usb.EnumerateAll()
		if err != nil {
			return err
		}
		if len(all) == 0 {
			fmt.Println("No Sony devices found at all.")
			return nil
		}
		fmt.Printf("Found %d Sony HID interface(s):\n\n", len(all))
		for i, d := range all {
			fmt.Printf("[%d] VID=0x%04X PID=0x%04X (%s)\n", i, d.VendorID, d.ProductID, d.Model)
			fmt.Printf("    Path=%s  Product=%q  Manufacturer=%q\n", d.Path, d.ProductStr, d.MfrStr)
			fmt.Printf("    UsagePage=0x%04X Usage=0x%04X Interface=%d\n\n",
				d.UsagePage, d.Usage, d.InterfaceNbr)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
