package main

import (
	"fmt"
	"time"

	"github.com/patyhank/linzone-hub/internal/protocol"
	"github.com/patyhank/linzone-hub/internal/usb"
	"github.com/spf13/cobra"
)

var serialDebugCmd = &cobra.Command{
	Use:   "serial-debug [index]",
	Short: "Debug legacy serial/COM headset traffic",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		devices, err := usb.Enumerate()
		if err != nil {
			return err
		}
		var idx int
		if _, err := fmt.Sscanf(args[0], "%d", &idx); err != nil {
			return fmt.Errorf("invalid index: %s", args[0])
		}
		if idx < 0 || idx >= len(devices) {
			return fmt.Errorf("index out of range: %d (have %d devices)", idx, len(devices))
		}
		d := devices[idx]
		if !usb.IsLegacySerialHeadset(d.ProductID) {
			return fmt.Errorf("%s is not a legacy serial headset", d.Model)
		}
		ports, err := protocol.FindSonySerialPorts(d.VendorID, d.ProductID)
		if err != nil {
			return err
		}
		allPorts, err := protocol.SerialPortsList()
		if err == nil {
			fmt.Printf("System serial ports: %v\n", allPorts)
		}
		for _, port := range ports {
			detail := port.Path
			if port.Interface != "" || port.Description != "" {
				detail = fmt.Sprintf("%s (interface=%s %s)", port.Path, port.Interface, port.Description)
			}
			for _, baud := range []int{460800, 115200, 921600, 230400} {
				fmt.Printf("Opening serial port: %s baud=%d\n", detail, baud)
				serial, err := protocol.OpenSerialHeadsetWithBaud(port.Path, baud)
				if err != nil {
					fmt.Printf("  open error: %v\n", err)
					continue
				}
				if bits, err := serial.ModemStatusBits(); err == nil {
					fmt.Printf("  Modem input bits: CTS=%v DSR=%v RI=%v DCD=%v\n", bits.CTS, bits.DSR, bits.RI, bits.DCD)
				}
				probes := []struct {
					eventID byte
					addr    byte
					name    string
				}{
					{protocol.Evt2GHZConnectStatus, byte((protocol.AddrTX << 4) | protocol.AddrPC), "2GHZ_CONNECT_STATUS PC->TX"},
					{protocol.EvtModelInfo, byte((protocol.AddrRX << 4) | protocol.AddrPC), "MODEL_INFO PC->RX"},
					{protocol.EvtBatteryInfo, byte((protocol.AddrRX << 4) | protocol.AddrPC), "BATTERY_INFO PC->RX"},
					{protocol.EvtFWVersion, byte((protocol.AddrRX << 4) | protocol.AddrPC), "FW_VERSION PC->RX"},
				}
				for i, probe := range probes {
					packet := protocol.BuildHciGetWithAddress(probe.eventID, probe.addr, uint16(i+1), nil)
					fmt.Printf("  TX %s addr=0x%02X: % X\n", probe.name, probe.addr, packet)
					if err := serial.WritePacket(packet); err != nil {
						fmt.Printf("    write error: %v\n", err)
						continue
					}
					raw, err := serial.ReadRaw(300 * time.Millisecond)
					if err != nil {
						fmt.Printf("    RX error: %v\n", err)
						continue
					}
					fmt.Printf("    RX raw: % X\n", raw)
				}
				_ = serial.Close()
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serialDebugCmd)
}
