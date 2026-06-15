package protocol

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/sstallion/go-hid"
)

const hciReadTimeout = 600 * time.Millisecond

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
	EvtHeadphoneVolume   = 33
	EvtGameChatMix       = 34
	EvtSidetoneVolume    = 35
	EvtMicVolume         = 36
	EvtAmbSetting        = 65
	EvtNCToggle          = 66
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
	if param == nil {
		param = []byte{}
	}
	addr := byte((AddrRX << 4) | AddrPC) // dst in high nibble, src in low nibble
	p := HciPacket{
		PacketType: HciCmdPacketType,
		Opcode:     0xFC00,
		ParamLen:   byte(8 + len(param)),
		SonyKeyID:  SonyKeyID,
		Address:    addr,
		EventID:    eventID,
		EventType:  FlagGET,
		TxID:       0, // will be managed by higher layer if needed
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
	return &BatteryInfo{
		Status:  param[0],
		Percent: param[1],
	}, nil
}

// GetHeadsetInfo performs a minimal enumeration sequence and returns model + battery if available.
func GetHeadsetInfo(dev *hid.Device) (model *HeadsetModelInfo, batt *BatteryInfo, fw []byte, err error) {
	// 1. 2GHZ_CONNECT_STATUS (GET)
	pkt := BuildHciGet(Evt2GHZConnectStatus, nil)
	if err = SendHciReport(dev, pkt); err != nil {
		return
	}
	_, _ = RecvHciReport(dev) // best-effort, ignore for initial probe

	// 2. MODEL_INFO
	pkt = BuildHciGet(EvtModelInfo, nil)
	if err = SendHciReport(dev, pkt); err != nil {
		return
	}
	if r, e := RecvHciReport(dev); e == nil {
		if ev2, _ := ParseHciEvent(r); ev2 != nil && ev2.EventID == EvtModelInfo {
			model, _ = ParseModelInfo(ev2.Param)
		}
	}

	// 3. BATTERY_INFO
	pkt = BuildHciGet(EvtBatteryInfo, nil)
	if err = SendHciReport(dev, pkt); err != nil {
		return
	}
	if r, e := RecvHciReport(dev); e == nil {
		if ev2, _ := ParseHciEvent(r); ev2 != nil && ev2.EventID == EvtBatteryInfo {
			batt, _ = ParseBatteryInfo(ev2.Param)
		}
	}

	// 4. FW_VERSION (best effort)
	pkt = BuildHciGet(EvtFWVersion, nil)
	if err = SendHciReport(dev, pkt); err == nil {
		if r, e := RecvHciReport(dev); e == nil {
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
	if param == nil {
		param = []byte{}
	}
	addr := byte((AddrRX << 4) | AddrPC)
	p := HciPacket{
		PacketType: HciCmdPacketType,
		Opcode:     0xFC00,
		ParamLen:   byte(8 + len(param)),
		SonyKeyID:  SonyKeyID,
		Address:    addr,
		EventID:    eventID,
		EventType:  FlagSET,
		TxID:       0,
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
	pkt := BuildHciSet(eventID, param)
	if err := SendHciReport(dev, pkt); err != nil {
		return err
	}
	// Best effort: consume possible RET / confirmation. Ignore content and errors.
	_, _ = RecvHciReport(dev)
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
