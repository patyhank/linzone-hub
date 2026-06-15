package main

import (
	"github.com/patyhank/linzone-hub/internal/usb"
	"github.com/spf13/cobra"
)

var udevCmd = &cobra.Command{
	Use:   "udev",
	Short: "Print recommended udev rules for normal-user access on Linux",
	Run: func(cmd *cobra.Command, args []string) {
		usb.PrintUdevRule()
	},
}

func init() {
	rootCmd.AddCommand(udevCmd)
}
