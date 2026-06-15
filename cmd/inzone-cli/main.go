package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "inzone",
	Short:         "INZONE Hub - Linux CLI controller for Sony INZONE devices",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `INZONE Hub CLI - Command-line controller for Sony INZONE gaming peripherals.

Supports headsets (H9, H7, H5, H10, etc.), mice, and keyboards via USB HID.

This is the command-line testing version. A GUI version using Wails v3 is planned.`,
}

func init() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(testCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
