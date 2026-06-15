package protocol

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/sstallion/go-hid"
)

const hciReadTimeout = 600 * time.Millisecond

type HciFraming int

const (
	HciFramingHeadset HciFraming = iota
	HciFramingRawReport
)

func (f HciFraming) String() string {
	switch f {
	case HciFramingHeadset:
		return "headset-report-id-2-length"
	case HciFramingRawReport:
		return "raw-report-id-0"
	default:
		return "unknown"
	}
}

// Protocol B (Headset HCI) constants
const (
	HciCmdPacketType = 0x01
	HciEvtPacketType = 0x04
	SonyKeyID        = 0xC396

	// Common Event IDs (from tech ref)
	Evt2GHZConnectStatus = 1
	EvtModelInfo         = 2
	EvtFWVersion         = 3
	EvtBatteryInfo       = 4
	EvtHostSelectSwitch  = 5
	EvtFunctionPart1     = 6
	EvtFunctionPart2     = 7
	EvtFunctionPart3     = 8
	EvtHeadphoneVolume   = 33
	EvtGameChatMix       = 34
	EvtSidetoneVolume    = 35
	EvtMicVolume         = 36
	EvtSurroundSetting   = 37
	EvtAmbSetting        = 65
	EvtNCToggle          = 66
	EvtNCStartupMode     = 67
	EvtBTStatus          = 97
	EvtBTSoundQuality    = 98
	EvtBTStartupMode     = 99
	EvtAutoPowerOff      = 129
	EvtLEDSetting        = 130
	EvtConnectionDest    = 133
	EvtAssignableCap     = 140
	EvtAssignableParam   = 141
	EvtIncomingPerm      = 142
)

// HciAddress IDs
const (
	AddrPC = 1
	AddrTX = 2
	AddrRX = 4
)

// HciFlags
const (
	FlagGET  = 0x01
	FlagSET  = 0x02
	FlagRET  = 0x10
	FlagNTFY = 0x20
)

// HciPacket represents a parsed or to-be-sent HCI packet (command or event)
type HciPacket struct {
	PacketType byte
	Opcode     uint16 // only for commands (usually 0xFC00)
	EventCode  byte   // 0xFF for events
	ParamLen   byte
	SonyKeyID  uint16
	Address    byte // high nibble Dst, low nibble Src
	EventID    byte
	EventType  byte
	TxID       uint16
	Param      []byte
	Checksum   byte
}

// checksum for command: sum from SonyKeyID (offset 4) to end of param
func checksumCmd(p *HciPacket) byte {
	sum := 0
	sum += int(p.SonyKeyID & 0xFF)
	sum += int(p.SonyKeyID >> 8)
	sum += int(p.Address)
	sum += int(p.EventID)
	sum += int(p.EventType)
	sum += int(p.TxID & 0xFF)
	sum += int(p.TxID >> 8)
	for _, b := range p.Param {
		sum += int(b)
	}
	return byte(sum & 0xFF)
}

// BuildHciGet constructs a GET command packet for the given EventID.
// ParamLen = 8 + len(param). For simple GETs, param can be empty or small.
func BuildHciGet(eventID byte, param []byte) []byte {
	return BuildHciGetWithAddress(eventID, byte((AddrRX<<4)|AddrPC), 0, param)
}

func BuildHciGetWithAddress(eventID byte, address byte, txID uint16, param []byte) []byte {
	if param == nil {
		param = []byte{}
	}
	p := HciPacket{
		PacketType: HciCmdPacketType,
		Opcode:     0xFC00,
		ParamLen:   byte(8 + len(param)),
		SonyKeyID:  SonyKeyID,
		Address:    address,
		EventID:    eventID,
		EventType:  FlagGET,
		TxID:       txID,
		Param:      param,
	}
	p.Checksum = checksumCmd(&p)

	buf := make([]byte, 0, 12+len(param))
	buf = append(buf, p.PacketType)
	buf = append(buf, byte(p.Opcode&0xFF), byte(p.Opcode>>8))
	buf = append(buf, p.ParamLen)
	buf = append(buf, byte(p.SonyKeyID&0xFF), byte(p.SonyKeyID>>8))
	buf = append(buf, p.Address, p.EventID, p.EventType)
	buf = append(buf, byte(p.TxID&0xFF), byte(p.TxID>>8))
	buf = append(buf, p.Param...)
	buf = append(buf, p.Checksum)
	return buf
}

// SendHciReport sends a Protocol B (headset) output report.
// Headset framing: ReportID=2, byte[1]=data length, payload starts at [2]
func SendHciReport(dev *hid.Device, payload []byte) error {
	if len(payload) > 62 {
		return fmt.Errorf("hci payload too large: %d (max 62)", len(payload))
	}
	report := make([]byte, 64)
	report[0] = 0x02 // Report ID for headsets
	report[1] = byte(len(payload))
	copy(report[2:], payload)
	_, err := dev.Write(report)
	return err
}

// RecvHciReport reads one input report (with timeout) and returns the inner payload (after report ID + len)
func RecvHciReport(dev *hid.Device) ([]byte, error) {
	buf := make([]byte, 64)
	n, err := dev.ReadWithTimeout(buf, hciReadTimeout)
	if err != nil {
		if err == hid.ErrTimeout {
			return nil, fmt.Errorf("read timeout (no response from device)")
		}
		return nil, err
	}
	if n < 2 {
		return nil, fmt.Errorf("short hci read")
	}
	length := int(buf[1])
	if length > n-2 {
		length = n - 2
	}
	return buf[2 : 2+length], nil
}

func SendHciRawReport(dev *hid.Device, payload []byte) error {
	if len(payload) > 64 {
		return fmt.Errorf("hci payload too large: %d (max 64)", len(payload))
	}
	report := make([]byte, 65)
	report[0] = 0x00
	copy(report[1:], payload)
	_, err := dev.Write(report)
	return err
}

func RecvHciRawReport(dev *hid.Device) ([]byte, error) {
	buf := make([]byte, 65)
	n, err := dev.ReadWithTimeout(buf, hciReadTimeout)
	if err != nil {
		if err == hid.ErrTimeout {
			return nil, fmt.Errorf("read timeout (no response from device)")
		}
		return nil, err
	}
	if n < 3 {
		return nil, fmt.Errorf("short raw hci read")
	}

	start := 0
	if buf[0] == 0x00 {
		start = 1
	}
	if start+3 > n {
		return nil, fmt.Errorf("short raw hci read")
	}
	data := buf[start:n]
	for i := 0; i+2 < len(data); i++ {
		if data[i] != HciEvtPacketType || data[i+1] != 0xFF {
			continue
		}
		total := int(data[i+2]) + 4
		if total <= 0 || i+total > len(data) {
			return data[i:], nil
		}
		return data[i : i+total], nil
	}
	return data, nil
}

func SendHciReportWithFraming(dev *hid.Device, payload []byte, framing HciFraming) error {
	if framing == HciFramingRawReport {
		return SendHciRawReport(dev, payload)
	}
	return SendHciReport(dev, payload)
}

func RecvHciReportWithFraming(dev *hid.Device, framing HciFraming) ([]byte, error) {
	if framing == HciFramingRawReport {
		return RecvHciRawReport(dev)
	}
	return RecvHciReport(dev)
}

// ParseHciEvent parses an event packet (from device).
func ParseHciEvent(data []byte) (*HciPacket, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("event too short")
	}
	if data[0] != HciEvtPacketType || data[1] != 0xFF {
		return nil, fmt.Errorf("not a valid hci event")
	}
	p := &HciPacket{
		PacketType: data[0],
		EventCode:  data[1],
		ParamLen:   data[2],
		SonyKeyID:  binary.LittleEndian.Uint16(data[4:6]),
		Address:    data[6],
		EventID:    data[7],
		EventType:  data[8],
		TxID:       binary.LittleEndian.Uint16(data[9:11]),
	}
	paramLen := int(p.ParamLen) - 8
	if paramLen < 0 {
		paramLen = 0
	}
	if 11+paramLen+1 > len(data) {
		paramLen = len(data) - 12
	}
	if paramLen > 0 {
		p.Param = make([]byte, paramLen)
		copy(p.Param, data[11:11+paramLen])
	}
	p.Checksum = data[11+paramLen]
	return p, nil
}

// HeadsetModelInfo is a parsed MODEL_INFO (Event ID 2)
type HeadsetModelInfo struct {
	ModelID     byte
	Destination byte
	Serial      uint16
	Color       byte
	Status      byte
}

func ParseModelInfo(param []byte) (*HeadsetModelInfo, error) {
	if len(param) < 6 {
		return nil, fmt.Errorf("model info param too short")
	}
	return &HeadsetModelInfo{
		ModelID:     param[0],
		Destination: param[1],
		Serial:      binary.LittleEndian.Uint16(param[2:4]),
		Color:       param[4],
		Status:      param[5],
	}, nil
}

// BatteryInfo standard (Event ID 4)
type BatteryInfo struct {
	Status  byte // 0=discharging, 1=charging
	Percent byte
}

func ParseBatteryInfo(param []byte) (*BatteryInfo, error) {
	if len(param) < 2 {
		return nil, fmt.Errorf("battery info too short")
	}
	percent := param[1]
	if percent > 100 {
		percent = byte((int(percent)*100 + 100) / 200)
	}
	if percent > 100 {
		percent = 100
	}
	return &BatteryInfo{
		Status:  param[0],
		Percent: percent,
	}, nil
}

// GetHeadsetInfo performs a minimal enumeration sequence and returns model + battery if available.
func GetHeadsetInfo(dev *hid.Device) (model *HeadsetModelInfo, batt *BatteryInfo, fw []byte, err error) {
	return GetHeadsetInfoWithFraming(dev, HciFramingHeadset)
}

func GetHeadsetInfoAuto(dev *hid.Device) (model *HeadsetModelInfo, batt *BatteryInfo, fw []byte, framing HciFraming, err error) {
	framings := []HciFraming{HciFramingHeadset, HciFramingRawReport}
	for _, f := range framings {
		model, batt, fw, err = GetHeadsetInfoWithFraming(dev, f)
		if err == nil && (model != nil || batt != nil || len(fw) > 0) {
			return model, batt, fw, f, nil
		}
	}
	framing = HciFramingHeadset
	return model, batt, fw, framing, err
}

func GetHeadsetInfoWithFraming(dev *hid.Device, framing HciFraming) (model *HeadsetModelInfo, batt *BatteryInfo, fw []byte, err error) {
	// 1. 2GHZ_CONNECT_STATUS (GET)
	pkt := BuildHciGet(Evt2GHZConnectStatus, nil)
	if err = SendHciReportWithFraming(dev, pkt, framing); err != nil {
		return
	}
	_, _ = RecvHciReportWithFraming(dev, framing) // best-effort, ignore for initial probe

	// 2. MODEL_INFO
	pkt = BuildHciGet(EvtModelInfo, nil)
	if err = SendHciReportWithFraming(dev, pkt, framing); err != nil {
		return
	}
	if r, e := RecvHciReportWithFraming(dev, framing); e == nil {
		if ev2, _ := ParseHciEvent(r); ev2 != nil && ev2.EventID == EvtModelInfo {
			model, _ = ParseModelInfo(ev2.Param)
		}
	}

	// 3. BATTERY_INFO
	pkt = BuildHciGet(EvtBatteryInfo, nil)
	if err = SendHciReportWithFraming(dev, pkt, framing); err != nil {
		return
	}
	if r, e := RecvHciReportWithFraming(dev, framing); e == nil {
		if ev2, _ := ParseHciEvent(r); ev2 != nil && ev2.EventID == EvtBatteryInfo {
			batt, _ = ParseBatteryInfo(ev2.Param)
		}
	}

	// 4. FW_VERSION (best effort)
	pkt = BuildHciGet(EvtFWVersion, nil)
	if err = SendHciReportWithFraming(dev, pkt, framing); err == nil {
		if r, e := RecvHciReportWithFraming(dev, framing); e == nil {
			if ev2, _ := ParseHciEvent(r); ev2 != nil && ev2.EventID == EvtFWVersion {
				fw = ev2.Param
			}
		}
	}
	err = nil // we tolerate partial results
	return
}

// --- SET support for Protocol B (Headset HCI) ---

// BuildHciSet builds a SET command packet (EventType = FlagSET).
func BuildHciSet(eventID byte, param []byte) []byte {
	return BuildHciSetWithAddress(eventID, byte((AddrRX<<4)|AddrPC), 0, param)
}

func BuildHciSetWithAddress(eventID byte, address byte, txID uint16, param []byte) []byte {
	if param == nil {
		param = []byte{}
	}
	p := HciPacket{
		PacketType: HciCmdPacketType,
		Opcode:     0xFC00,
		ParamLen:   byte(8 + len(param)),
		SonyKeyID:  SonyKeyID,
		Address:    address,
		EventID:    eventID,
		EventType:  FlagSET,
		TxID:       txID,
		Param:      param,
	}
	p.Checksum = checksumCmd(&p)

	buf := make([]byte, 0, 12+len(param))
	buf = append(buf, p.PacketType)
	buf = append(buf, byte(p.Opcode&0xFF), byte(p.Opcode>>8))
	buf = append(buf, p.ParamLen)
	buf = append(buf, byte(p.SonyKeyID&0xFF), byte(p.SonyKeyID>>8))
	buf = append(buf, p.Address, p.EventID, p.EventType)
	buf = append(buf, byte(p.TxID&0xFF), byte(p.TxID>>8))
	buf = append(buf, p.Param...)
	buf = append(buf, p.Checksum)
	return buf
}

// SendHciSet sends a SET without waiting for a specific ACK (many devices just ACK via RET or NTFY).
// It still does a best-effort read to consume any immediate response.
func SendHciSet(dev *hid.Device, eventID byte, param []byte) error {
	return SendHciSetAuto(dev, eventID, param)
}

func SendHciSetAuto(dev *hid.Device, eventID byte, param []byte) error {
	headsetErr := SendHciSetWithFraming(dev, eventID, param, HciFramingHeadset)
	rawErr := SendHciSetWithFraming(dev, eventID, param, HciFramingRawReport)
	if headsetErr == nil || rawErr == nil {
		return nil
	}
	return fmt.Errorf("headset framing: %v; raw framing: %w", headsetErr, rawErr)
}

func SendHciSetWithFraming(dev *hid.Device, eventID byte, param []byte, framing HciFraming) error {
	pkt := BuildHciSet(eventID, param)
	if err := SendHciReportWithFraming(dev, pkt, framing); err != nil {
		return err
	}
	// Best effort: consume possible RET / confirmation. Ignore content and errors.
	_, _ = RecvHciReportWithFraming(dev, framing)
	return nil
}

// Headset SET helpers (values are usually raw + percent where applicable)

// SetHeadphoneVolume sets headphone volume. Param: [mute(0/1), value, percent(0-100)]
func SetHeadphoneVolume(dev *hid.Device, mute bool, value byte, percent byte) error {
	m := byte(0)
	if mute {
		m = 1
	}
	return SendHciSet(dev, EvtHeadphoneVolume, []byte{m, value, percent})
}

// SetSidetone sets sidetone volume. Param: [value, percent]
func SetSidetone(dev *hid.Device, value byte, percent byte) error {
	return SendHciSet(dev, EvtSidetoneVolume, []byte{value, percent})
}

// SetGameChatMix sets game/chat balance (0-100, center usually ~50).
func SetGameChatMix(dev *hid.Device, balance byte) error {
	return SendHciSet(dev, EvtGameChatMix, []byte{balance})
}

// SetMicVolume sets mic volume. Param: [mute, value, percent]
func SetMicVolume(dev *hid.Device, mute bool, value byte, percent byte) error {
	m := byte(0)
	if mute {
		m = 1
	}
	return SendHciSet(dev, EvtMicVolume, []byte{m, value, percent})
}

// SetAmbient sets ambient/NC mode.
// From ref: AMB_SETTING (65): [NC mode, ambientValue, ambientPercent, voiceFocus(0/1)]
func SetAmbient(dev *hid.Device, ncMode, ambientValue, ambientPercent byte, voiceFocus bool) error {
	vf := byte(0)
	if voiceFocus {
		vf = 1
	}
	return SendHciSet(dev, EvtAmbSetting, []byte{ncMode, ambientValue, ambientPercent, vf})
}

// SetNCToggle sets NC on/off or mode (device specific).
func SetNCToggle(dev *hid.Device, mode byte) error {
	return SendHciSet(dev, EvtNCToggle, []byte{mode})
}

func GetEvent(dev *hid.Device, eventID byte) (*HciPacket, error) {
	return GetEventWithFraming(dev, eventID, HciFramingHeadset)
}

func GetEventAuto(dev *hid.Device, eventID byte) (*HciPacket, HciFraming, error) {
	framings := []HciFraming{HciFramingHeadset, HciFramingRawReport}
	var lastErr error
	for _, f := range framings {
		pkt, err := GetEventWithFraming(dev, eventID, f)
		if err == nil {
			return pkt, f, nil
		}
		lastErr = err
	}
	return nil, HciFramingHeadset, lastErr
}

func GetEventWithFraming(dev *hid.Device, eventID byte, framing HciFraming) (*HciPacket, error) {
	if err := SendHciReportWithFraming(dev, BuildHciGet(eventID, nil), framing); err != nil {
		return nil, err
	}
	for i := 0; i < 5; i++ {
		raw, err := RecvHciReportWithFraming(dev, framing)
		if err != nil {
			return nil, err
		}
		pkt, err := ParseHciEvent(raw)
		if err == nil && pkt.EventID == eventID {
			return pkt, nil
		}
	}
	return nil, fmt.Errorf("no matching HCI event %d", eventID)
}

func SetSingleByte(dev *hid.Device, eventID byte, value byte) error {
	return SendHciSet(dev, eventID, []byte{value})
}

func SetSurround(dev *hid.Device, enabled bool) error {
	v := byte(0)
	if enabled {
		v = 1
	}
	return SetSingleByte(dev, EvtSurroundSetting, v)
}

func SetNCStartupMode(dev *hid.Device, mode byte) error {
	return SetSingleByte(dev, EvtNCStartupMode, mode)
}

func SetBTSoundQuality(dev *hid.Device, mode byte) error {
	return SetSingleByte(dev, EvtBTSoundQuality, mode)
}

func SetBTStartupMode(dev *hid.Device, mode byte) error {
	return SetSingleByte(dev, EvtBTStartupMode, mode)
}

func SetAutoPowerOff(dev *hid.Device, minutes byte) error {
	return SetSingleByte(dev, EvtAutoPowerOff, minutes)
}

func SetConnectionDestinationMode(dev *hid.Device, mode byte) error {
	return SetSingleByte(dev, EvtConnectionDest, mode)
}

func SetIncomingPermission(dev *hid.Device, enabled bool) error {
	v := byte(0)
	if enabled {
		v = 1
	}
	return SetSingleByte(dev, EvtIncomingPerm, v)
}

func SetAssignableSettings(dev *hid.Device, payload []byte) error {
	return SendHciSet(dev, EvtAssignableParam, payload)
}
