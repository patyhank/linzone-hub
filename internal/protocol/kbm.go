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
	GETProfileFunction       = 0x05A0
	GETButtonsRedefinedInfo  = 0x06A0
	GETButtonsTypeInfo       = 0x07A0
	GETWirelessMouseStatus   = 0x03A0
	GETChargeStatus          = 0x09A0
	SETProfileNumber         = 0x0020
	SETLODLevel              = 0x0320
	SETLEDLighting           = 0x0520
	SETSensorSnap            = 0x0620
	SETDPILevelValue         = 0x0720
	SETMotionSync            = 0x0820
	SETButtons               = 0x0920
	SETReportRate            = 0x1020
	SAVETOProfile            = 0x5020
	RESETToDefault           = 0x6220
	MouseBatteryLevelLow     = 0
	MouseBatteryLevelLowMid  = 1
	MouseBatteryLevelHighMid = 2
	MouseBatteryLevelHigh    = 3
)

var mouseResetToDefaultMagic = []byte{0x36, 0x31, 0x18, 0x38, 0x27, 0x98, 0x10, 0x94}

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
	// HID report ID 0 is backend-dependent: some reads include a leading 0,
	// while others return the payload directly. Keep both forms working.
	if buf[0] == ReportIDKBM && n > 1 && (buf[1] == CmdGET || buf[1] == CmdSET || buf[1] == CmdNOTIFY) {
		return buf[1:n], nil
	}
	return buf[:n], nil
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

type MouseProfileFunction struct {
	ReportRateHz  int
	SensorSnap    bool
	LODLevel      byte
	DPI           uint16
	MotionSync    bool
	LEDBrightness byte
	LEDRed        byte
	LEDGreen      byte
	LEDBlue       byte
}

type MouseButton struct {
	ButtonIndex byte
	TypeDef     byte
	KeyType     byte
	KeyCode1    byte
	KeyCode2    byte
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

func GetMouseProfileFunction(dev *hid.Device) (*MouseProfileFunction, error) {
	raw, err := GetKBMCommand(dev, GETProfileFunction)
	if err != nil {
		return nil, err
	}
	if len(raw) < 10 {
		return nil, fmt.Errorf("mouse profile function needs 10 bytes, got %d", len(raw))
	}
	return &MouseProfileFunction{
		ReportRateHz:  reportRateValueToHz(raw[0]),
		SensorSnap:    raw[1] != 0,
		LODLevel:      raw[2],
		DPI:           binary.LittleEndian.Uint16(raw[3:5])*50 + 50,
		MotionSync:    raw[5] != 0,
		LEDBrightness: raw[6],
		LEDRed:        raw[7],
		LEDGreen:      raw[8],
		LEDBlue:       raw[9],
	}, nil
}

func GetMouseButtons(dev *hid.Device) ([]MouseButton, error) {
	types, err := GetKBMCommand(dev, GETButtonsTypeInfo)
	if err != nil {
		return nil, fmt.Errorf("get button types: %w", err)
	}
	keys, err := GetKBMCommand(dev, GETButtonsRedefinedInfo)
	if err != nil {
		return nil, fmt.Errorf("get button keys: %w", err)
	}
	if len(types) < 5 {
		return nil, fmt.Errorf("button type info needs 5 bytes, got %d", len(types))
	}
	if len(keys) < 15 {
		return nil, fmt.Errorf("button key info needs 15 bytes, got %d", len(keys))
	}
	buttons := make([]MouseButton, 5)
	for i := range buttons {
		offset := i * 3
		buttons[i] = MouseButton{
			ButtonIndex: byte(i + 1),
			TypeDef:     types[i],
			KeyType:     keys[offset],
			KeyCode1:    keys[offset+1],
			KeyCode2:    keys[offset+2],
		}
	}
	return buttons, nil
}

func mouseProfileFunctionPayload(fn *MouseProfileFunction) ([]byte, error) {
	if fn == nil {
		return nil, fmt.Errorf("mouse profile function is nil")
	}
	reportRate, err := reportRateHzToValue(fn.ReportRateHz)
	if err != nil {
		return nil, err
	}
	if fn.DPI < 50 || (fn.DPI-50)%50 != 0 {
		return nil, fmt.Errorf("dpi must be >=50 and multiple of 50")
	}
	out := make([]byte, 10)
	out[0] = reportRate
	if fn.SensorSnap {
		out[1] = 1
	}
	out[2] = fn.LODLevel
	binary.LittleEndian.PutUint16(out[3:5], (fn.DPI-50)/50)
	if fn.MotionSync {
		out[5] = 1
	}
	out[6] = fn.LEDBrightness
	out[7] = fn.LEDRed
	out[8] = fn.LEDGreen
	out[9] = fn.LEDBlue
	return out, nil
}

func reportRateValueToHz(value byte) int {
	switch value {
	case 0, 1, 2, 3, 4:
		return 500 << value
	default:
		return 0
	}
}

func reportRateHzToValue(hz int) (byte, error) {
	switch hz {
	case 500:
		return 0, nil
	case 1000:
		return 1, nil
	case 2000:
		return 2, nil
	case 4000:
		return 3, nil
	case 8000:
		return 4, nil
	default:
		return 0, fmt.Errorf("unsupported report rate %d (use 500/1000/2000/4000/8000)", hz)
	}
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
		status, ok := findKBMStatus(resp)
		if !ok {
			return fmt.Errorf("ack too short")
		}
		return kbmStatusError(status, nil)
	}
	status := binary.LittleEndian.Uint16(payload[0:2])
	if status != 0xACDC && status != 0xFAEC {
		if fallback, ok := findKBMStatus(resp); ok {
			status = fallback
		}
	}
	return kbmStatusError(status, payload)
}

func findKBMStatus(data []byte) (uint16, bool) {
	for i := 0; i+1 < len(data); i++ {
		status := binary.LittleEndian.Uint16(data[i : i+2])
		if status == 0xACDC || status == 0xFAEC {
			return status, true
		}
	}
	return 0, false
}

func kbmStatusError(status uint16, payload []byte) error {
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
	return SendKBMSetAndAck(dev, SETProfileNumber, 0, []byte{profile})
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
	return SendKBMSetAndAck(dev, SETDPILevelValue, 0, buf)
}

// SetLEDLighting sets LED [brightness(0-255), R, G, B]. Command 0x0520.
func SetLEDLighting(dev *hid.Device, brightness, r, g, b byte) error {
	return SendKBMSetAndAck(dev, SETLEDLighting, 0, []byte{brightness, r, g, b})
}

// SetLOD sets lift-off distance. Command 0x0320. 0=0.7mm, 1=1.0mm, 2=2.0mm.
func SetLOD(dev *hid.Device, level byte) error {
	if level > 2 {
		return fmt.Errorf("lod level must be 0,1,2")
	}
	return SendKBMSetAndAck(dev, SETLODLevel, 0, []byte{level})
}

// SetMotionSync enables/disables motion sync. Command 0x0820. 0=off, 1=on.
func SetMotionSync(dev *hid.Device, on bool) error {
	v := byte(0)
	if on {
		v = 1
	}
	return SendKBMSetAndAck(dev, SETMotionSync, 0, []byte{v})
}

// SetSensorSnap sets angle snapping. Command 0x0620. 0=off, 1=on.
func SetSensorSnap(dev *hid.Device, on bool) error {
	v := byte(0)
	if on {
		v = 1
	}
	return SendKBMSetAndAck(dev, SETSensorSnap, 0, []byte{v})
}

// SetReportRate sets polling rate. Command 0x1020.
// Supported: 500,1000,2000,4000,8000 (maps to 0..4)
func SetReportRate(dev *hid.Device, hz int) error {
	v, err := reportRateHzToValue(hz)
	if err != nil {
		return err
	}
	return SendKBMSetAndAck(dev, SETReportRate, 0, []byte{v})
}

// SetButtons sets button remapping for one button (6 bytes).
// See technical reference for the 6-byte layout.
// This is low-level; higher level button config helpers can be added later.
func SetButton(dev *hid.Device, buttonIndex byte, data6 [6]byte) error {
	if buttonIndex < 1 || buttonIndex > 5 {
		return fmt.Errorf("button index 1-5")
	}
	// The command SET_BUTTONS sends 6 bytes per button; the "Packet" field may select the button in some implementations.
	// From the table, SET_BUTTONS (2336=0x0920). Many implementations send one button at a time using Packet field.
	return SendKBMSetAndAck(dev, SETButtons, buttonIndex, data6[:])
}

func SetMouseButton(dev *hid.Device, buttonIndex, typeDef, keyType, keyCode1, keyCode2 byte) error {
	if typeDef != 0xFF && typeDef != 0x00 && typeDef != 0x40 {
		return fmt.Errorf("button type definition must be 0xFF, 0x00, or 0x40")
	}
	if keyType != 0xFF && keyType > 2 {
		return fmt.Errorf("button key type must be 0xFF, 0, 1, or 2")
	}
	return SetButton(dev, buttonIndex, [6]byte{typeDef, 0, 0, keyType, keyCode1, keyCode2})
}

func SaveMouseToProfile(dev *hid.Device) error {
	fn, err := GetMouseProfileFunction(dev)
	if err != nil {
		return fmt.Errorf("read profile function: %w", err)
	}
	buttons, err := GetMouseButtons(dev)
	if err != nil {
		return fmt.Errorf("read buttons: %w", err)
	}
	profilePayload, err := mouseProfileFunctionPayload(fn)
	if err != nil {
		return err
	}
	typePayload := make([]byte, len(buttons))
	keyPayload := make([]byte, len(buttons)*3)
	for i, button := range buttons {
		typePayload[i] = button.TypeDef
		offset := i * 3
		keyPayload[offset] = button.KeyType
		keyPayload[offset+1] = button.KeyCode1
		keyPayload[offset+2] = button.KeyCode2
	}
	if err := SendKBMSetAndAck(dev, SAVETOProfile, 0, profilePayload); err != nil {
		return fmt.Errorf("save profile function: %w", err)
	}
	if err := SendKBMSetAndAck(dev, SAVETOProfile, 1, typePayload); err != nil {
		return fmt.Errorf("save button types: %w", err)
	}
	if err := SendKBMSetAndAck(dev, SAVETOProfile, 2, keyPayload); err != nil {
		return fmt.Errorf("save button assignments: %w", err)
	}
	return nil
}

func ResetMouseToDefault(dev *hid.Device) error {
	return SendKBMSetAndAck(dev, RESETToDefault, 0, mouseResetToDefaultMagic)
}
