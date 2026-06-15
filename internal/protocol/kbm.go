package protocol

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/sstallion/go-hid"
)

const kbmReadTimeout = 800 * time.Millisecond

// Protocol A (Keyboard/Mouse) constants
const (
	ReportIDKBM   = 0x00
	ReportSizeKBM = 65 // 1 (report id) + 64 payload

	CmdGET    = 0xA0
	CmdSET    = 0x20
	CmdNOTIFY = 0x99

	// Common command IDs (16-bit: index<<8 | cmd)
	GETDeviceInformation     = 0x00A0
	GETCurrentBatteryInfo    = 0x04A0
	GETWirelessMouseStatus   = 0x03A0
	GETChargeStatus          = 0x09A0
	MouseBatteryLevelLow     = 0
	MouseBatteryLevelLowMid  = 1
	MouseBatteryLevelHighMid = 2
	MouseBatteryLevelHigh    = 3
)

// KBMHeader is the 4-byte protocol A header
type KBMHeader struct {
	Cmd    byte
	Index  byte
	Packet byte
	Length byte
}

// SendKBMReport sends a single output report using Protocol A framing (no length prefix in wire for K/M).
// The payload must already be formatted as [header 4 bytes][data...]
func SendKBMReport(dev *hid.Device, payload []byte) error {
	if len(payload) > 64 {
		return fmt.Errorf("payload too large for single KBM report: %d", len(payload))
	}
	report := make([]byte, ReportSizeKBM)
	report[0] = ReportIDKBM
	copy(report[1:], payload)
	_, err := dev.Write(report)
	return err
}

// RecvKBMReport reads one input report (with timeout) and returns the payload (without report ID).
func RecvKBMReport(dev *hid.Device) ([]byte, error) {
	buf := make([]byte, ReportSizeKBM)
	n, err := dev.ReadWithTimeout(buf, kbmReadTimeout)
	if err != nil {
		if err == hid.ErrTimeout {
			return nil, fmt.Errorf("read timeout (no response from device)")
		}
		return nil, err
	}
	if n < 1 {
		return nil, fmt.Errorf("short read")
	}
	// buf[0] should be report ID 0
	return buf[1:n], nil
}

// BuildKBMGet builds a GET request packet for the given 16-bit command.
func BuildKBMGet(cmd16 uint16) []byte {
	idx := byte(cmd16 >> 8)
	c := byte(cmd16 & 0xFF)
	return []byte{c, idx, 0, 0} // length 0 for GET
}

// ParseKBMResponse parses header + status for SET responses, or header + data for GET responses.
func ParseKBMResponse(data []byte) (KBMHeader, []byte, error) {
	if len(data) < 4 {
		return KBMHeader{}, nil, fmt.Errorf("response too short")
	}
	h := KBMHeader{
		Cmd:    data[0],
		Index:  data[1],
		Packet: data[2],
		Length: data[3],
	}
	payload := data[4:]
	if int(h.Length) > len(payload) {
		// Some devices may not echo length strictly; trust actual data
	}
	return h, payload[:min(len(payload), int(h.Length))], nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetDeviceInformationKBM sends GET_DEVICE_INFORMATION (160) and returns the 18-byte raw info (mouse) or 15-byte (kbd).
func GetDeviceInformationKBM(dev *hid.Device) ([]byte, error) {
	return GetKBMCommand(dev, GETDeviceInformation)
}

func GetKBMCommand(dev *hid.Device, cmd uint16) ([]byte, error) {
	req := BuildKBMGet(cmd)
	if err := SendKBMReport(dev, req); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	resp, err := RecvKBMReport(dev)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	h, data, err := ParseKBMResponse(resp)
	if err != nil {
		return nil, err
	}
	if h.Cmd != CmdGET {
		// Some devices may echo differently; we still return the data payload
	}
	_ = h
	return data, nil
}

type MouseBatteryInfo struct {
	Level      byte
	Percent    int
	IsCharging bool
	RFStatus   byte
}

func GetMouseBatteryInfo(dev *hid.Device) (*MouseBatteryInfo, error) {
	batteryRaw, err := GetKBMCommand(dev, GETCurrentBatteryInfo)
	if err != nil {
		return nil, fmt.Errorf("get battery: %w", err)
	}
	if len(batteryRaw) < 1 {
		return nil, fmt.Errorf("battery response too short")
	}

	info := &MouseBatteryInfo{
		Level:   batteryRaw[0],
		Percent: MouseBatteryLevelToPercent(batteryRaw[0]),
	}

	if chargeRaw, err := GetKBMCommand(dev, GETChargeStatus); err == nil && len(chargeRaw) > 0 {
		info.IsCharging = chargeRaw[0] == 1
	}
	if rfRaw, err := GetKBMCommand(dev, GETWirelessMouseStatus); err == nil && len(rfRaw) > 0 {
		info.RFStatus = rfRaw[0]
	}

	return info, nil
}

func MouseBatteryLevelToPercent(level byte) int {
	switch level {
	case MouseBatteryLevelLow:
		return 10
	case MouseBatteryLevelLowMid:
		return 40
	case MouseBatteryLevelHighMid:
		return 70
	case MouseBatteryLevelHigh:
		return 100
	default:
		return 0
	}
}

// ParseMouseDeviceInfo parses the 18-byte GET_DEVICE_INFORMATION response for mice.
type MouseDeviceInfo struct {
	PID            uint16
	VID            uint16
	MouseFW        [4]byte // Major, Minor, Build, Revision ? per spec order
	DongleUSB      [4]byte
	DongleRF       [4]byte
	CurrentProfile byte
	ColorSKU       byte
}

func ParseMouseInfo(raw []byte) (*MouseDeviceInfo, error) {
	if len(raw) < 18 {
		return nil, fmt.Errorf("mouse info needs 18 bytes, got %d", len(raw))
	}
	return &MouseDeviceInfo{
		PID:            binary.LittleEndian.Uint16(raw[0:2]),
		VID:            binary.LittleEndian.Uint16(raw[2:4]),
		MouseFW:        [4]byte{raw[7], raw[6], raw[5], raw[4]}, // major,minor,build,rev as per doc
		DongleUSB:      [4]byte{raw[11], raw[10], raw[9], raw[8]},
		DongleRF:       [4]byte{raw[15], raw[14], raw[13], raw[12]},
		CurrentProfile: raw[16],
		ColorSKU:       raw[17],
	}, nil
}

// --- SET commands (Protocol A) ---

// BuildKBMSet builds a SET request packet.
// cmd16: e.g. 0x0020 for SET_PROFILE_NUMBER
// packet: usually 0, or sub-packet index for multi-packet commands
// data: payload (length <= 60 or so)
func BuildKBMSet(cmd16 uint16, packet byte, data []byte) []byte {
	idx := byte(cmd16 >> 8)
	c := byte(cmd16 & 0xFF)
	hdr := []byte{c, idx, packet, byte(len(data))}
	return append(hdr, data...)
}

// SendKBMSetAndAck sends a SET and waits for the ACK response.
// Returns nil on 0xACDC (OK), error on NG or timeout or bad status.
func SendKBMSetAndAck(dev *hid.Device, cmd16 uint16, packet byte, data []byte) error {
	pkt := BuildKBMSet(cmd16, packet, data)
	if err := SendKBMReport(dev, pkt); err != nil {
		return fmt.Errorf("write set: %w", err)
	}
	resp, err := RecvKBMReport(dev)
	if err != nil {
		return fmt.Errorf("read ack: %w", err)
	}
	h, payload, err := ParseKBMResponse(resp)
	if err != nil {
		return err
	}
	_ = h // header should echo the SET we sent
	if len(payload) < 2 {
		return fmt.Errorf("ack too short")
	}
	status := binary.LittleEndian.Uint16(payload[0:2])
	switch status {
	case 0xACDC:
		return nil
	case 0xFAEC:
		if len(payload) >= 3 {
			return fmt.Errorf("device returned NG (error code %d)", payload[2])
		}
		return fmt.Errorf("device returned NG")
	default:
		return fmt.Errorf("unexpected SET ack status 0x%04X", status)
	}
}

// SetProfileNumber sets current profile (1-4). Command 0x0020.
func SetProfileNumber(dev *hid.Device, profile byte) error {
	if profile < 1 || profile > 4 {
		return fmt.Errorf("profile must be 1-4")
	}
	return SendKBMSetAndAck(dev, 0x0020, 0, []byte{profile})
}

// SetDPI sets DPI for current profile. Command 0x0720.
// dpi must be multiple of 50, >=50 (device dependent max).
func SetDPI(dev *hid.Device, dpi uint16) error {
	if dpi < 50 || (dpi-50)%50 != 0 {
		return fmt.Errorf("dpi must be >=50 and multiple of 50")
	}
	wire := (dpi - 50) / 50
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(wire))
	return SendKBMSetAndAck(dev, 0x0720, 0, buf)
}

// SetLEDLighting sets LED [brightness(0-255), R, G, B]. Command 0x0520.
func SetLEDLighting(dev *hid.Device, brightness, r, g, b byte) error {
	return SendKBMSetAndAck(dev, 0x0520, 0, []byte{brightness, r, g, b})
}

// SetLOD sets lift-off distance. Command 0x0320. 0=0.7mm, 1=1.0mm, 2=2.0mm.
func SetLOD(dev *hid.Device, level byte) error {
	if level > 2 {
		return fmt.Errorf("lod level must be 0,1,2")
	}
	return SendKBMSetAndAck(dev, 0x0320, 0, []byte{level})
}

// SetMotionSync enables/disables motion sync. Command 0x0820. 0=off, 1=on.
func SetMotionSync(dev *hid.Device, on bool) error {
	v := byte(0)
	if on {
		v = 1
	}
	return SendKBMSetAndAck(dev, 0x0820, 0, []byte{v})
}

// SetSensorSnap sets angle snapping. Command 0x0620. 0=off, 1=on.
func SetSensorSnap(dev *hid.Device, on bool) error {
	v := byte(0)
	if on {
		v = 1
	}
	return SendKBMSetAndAck(dev, 0x0620, 0, []byte{v})
}

// SetReportRate sets polling rate. Command 0x1020.
// Supported: 500,1000,2000,4000,8000 (maps to 0..4)
func SetReportRate(dev *hid.Device, hz int) error {
	var v byte
	switch hz {
	case 500:
		v = 0
	case 1000:
		v = 1
	case 2000:
		v = 2
	case 4000:
		v = 3
	case 8000:
		v = 4
	default:
		return fmt.Errorf("unsupported report rate %d (use 500/1000/2000/4000/8000)", hz)
	}
	return SendKBMSetAndAck(dev, 0x1020, 0, []byte{v})
}

// SetButtons sets button remapping for one button (6 bytes).
// See technical reference for the 6-byte layout.
// This is low-level; higher level button config helpers can be added later.
func SetButton(dev *hid.Device, buttonIndex byte, data6 [6]byte) error {
	if buttonIndex > 5 {
		return fmt.Errorf("button index 1-5")
	}
	// The command SET_BUTTONS sends 6 bytes per button; the "Packet" field may select the button in some implementations.
	// From the table, SET_BUTTONS (2336=0x0920). Many implementations send one button at a time using Packet field.
	return SendKBMSetAndAck(dev, 0x0920, buttonIndex, data6[:])
}
