package airoha

import (
	"encoding/binary"
	"fmt"
)

// Inner command / response opcodes and structures for the Airoha relay on INZONE Buds.
//
// These are best-effort based on:
// - Sony INZONE Hub reference (Protocol C mentions AB1565, FOTA, touch keys, ANC/Ambient, wearing detector, etc.)
// - Public Airoha AB1565 / RACE protocol fragments from other earbud projects
// - Observation that battery, ANC, wearing, and key mapping are handled per-earpiece
//
// The "Inner" field of a RelayFrame carries an Airoha/RACE-style command:
//   [0]   Cmd / OpCode
//   [1]   SubCmd / ParamType (often)
//   [2..] Parameters
//
// Responses usually echo the Cmd/SubCmd and add status + payload.
//
// WARNING: These constants and layouts are speculative and may require capture-based fixes.

const (
	// Top-level command groups (very approximate).
	// Many Airoha devices use 0x01XX for GET/SET of "system" things and 0x02XX/0x12XX for audio/ANC.
	CmdGetBattery         = 0x01
	CmdGetFirmwareVersion = 0x02
	CmdGetDeviceInfo      = 0x03

	CmdSetAncMode = 0x10 // ANC / Ambient / Off
	CmdGetAncMode = 0x11

	CmdSetTouchKey = 0x20 // touch / gesture mapping
	CmdGetTouchKey = 0x21

	CmdGetWearingStatus = 0x30 // in-ear detection
	CmdSetWearingConfig = 0x31

	CmdGetEqPreset = 0x40
	CmdSetEqPreset = 0x41

	CmdFotaControl = 0xA0 // firmware update control
)

// AncMode values (typical for many Airoha-based buds).
const (
	AncModeOff     = 0x00
	AncModeOn      = 0x01 // strong ANC
	AncModeAmbient = 0x02 // transparency / ambient
	AncModeCustom  = 0x03 // device-specific custom level
)

// BatteryInfo represents per-earpiece + case battery reported over the relay.
type BatteryInfo struct {
	LeftStatus   byte // 0=discharging, 1=charging, etc.
	LeftPercent  byte
	RightStatus  byte
	RightPercent byte
	CaseStatus   byte
	CasePercent  byte
}

// DecodeBattery tries to interpret an inner payload as battery info.
// Real layout on INZONE Buds is likely 6 bytes: [Lstat, L%, Rstat, R%, Cstat, C%].
func DecodeBattery(inner []byte) (*BatteryInfo, error) {
	if len(inner) < 6 {
		return nil, fmt.Errorf("battery payload too short (%d bytes)", len(inner))
	}
	return &BatteryInfo{
		LeftStatus:   inner[0],
		LeftPercent:  inner[1],
		RightStatus:  inner[2],
		RightPercent: inner[3],
		CaseStatus:   inner[4],
		CasePercent:  inner[5],
	}, nil
}

// BuildAncSet builds an inner payload for setting ANC/Ambient mode.
func BuildAncSet(mode byte, extra ...byte) []byte {
	p := []byte{CmdSetAncMode, mode}
	p = append(p, extra...)
	return p
}

// BuildGetBattery builds a simple GET_BATTERY inner command.
func BuildGetBattery() []byte {
	return []byte{CmdGetBattery, 0x00}
}

// BuildGetWearing builds a GET_WEARING_STATUS inner command.
func BuildGetWearing() []byte {
	return []byte{CmdGetWearingStatus, 0x00}
}

// DecodeWearingStatus is a placeholder decoder.
// On many buds this returns something like [leftInEar, rightInEar] or richer structs.
func DecodeWearingStatus(inner []byte) (left bool, right bool, err error) {
	if len(inner) < 2 {
		return false, false, fmt.Errorf("wearing payload too short")
	}
	left = inner[0] != 0
	right = inner[1] != 0
	return left, right, nil
}

// BuildTouchKeySet builds a command to remap a touch action.
// The exact key/action encoding is device specific; we pass raw bytes here.
func BuildTouchKeySet(keyID byte, action []byte) []byte {
	p := []byte{CmdSetTouchKey, keyID}
	p = append(p, action...)
	return p
}

// BuildFotaStart builds a FOTA (firmware update) start/control command.
// Priority / flags are passed through; real meaning is in the earbuds FOTA blob.
func BuildFotaStart(priority byte, flags []byte) []byte {
	p := []byte{CmdFotaControl, 0x01 /* start */, priority}
	p = append(p, flags...)
	return p
}

// BuildFotaStop aborts or finalizes a FOTA session.
func BuildFotaStop() []byte {
	return []byte{CmdFotaControl, 0x02 /* stop */}
}

// --- Buds-specific extended structures (from tech ref) ---

// BudsModelInfo is the 32-byte MODEL_INFO returned by INZONE Buds / GTW over the Airoha relay.
// Layout (per INZONE Hub Technical Reference):
//
// [0]    Model ID (4 = GTW / Buds)
// [1]    Destination
// [2..3] Serial (LE)
// [4]    Model Color
// [5]    Model Status
// [6..13]  Dongle Serial (8 bytes ASCII)
// [14..21] Left Earpiece Serial (8 bytes ASCII)
// [22..29] Right Earpiece Serial (8 bytes ASCII)
// [30]     Left Color ID
// [31]     Right Color ID
type BudsModelInfo struct {
	ModelID      byte
	Destination  byte
	Serial       uint16
	Color        byte
	Status       byte
	DongleSerial [8]byte
	LeftSerial   [8]byte
	RightSerial  [8]byte
	LeftColorID  byte
	RightColorID byte
}

// ParseBudsModelInfo parses a 32-byte (or larger) payload into BudsModelInfo.
// It accepts payloads that start with the 6-byte standard header + 26 more bytes.
func ParseBudsModelInfo(payload []byte) (*BudsModelInfo, error) {
	if len(payload) < 32 {
		return nil, fmt.Errorf("buds model info needs at least 32 bytes, got %d", len(payload))
	}
	m := &BudsModelInfo{
		ModelID:      payload[0],
		Destination:  payload[1],
		Serial:       binary.LittleEndian.Uint16(payload[2:4]),
		Color:        payload[4],
		Status:       payload[5],
		LeftColorID:  payload[30],
		RightColorID: payload[31],
	}
	copy(m.DongleSerial[:], payload[6:14])
	copy(m.LeftSerial[:], payload[14:22])
	copy(m.RightSerial[:], payload[22:30])
	return m, nil
}

// BuildGetExtendedModel builds a command that asks for the full 32-byte Buds model info.
// On real devices this is often the same as CmdGetDeviceInfo or a MODEL_INFO equivalent over relay.
func BuildGetExtendedModel() []byte {
	return []byte{CmdGetDeviceInfo, 0x01} // 0x01 = "extended/long form" hint
}

// BudsFirmwareVersion is the 12-byte FW version layout for GTW/Buds (RX + TX + Right earpiece).
type BudsFirmwareVersion struct {
	// Each version is usually 4 bytes: major, minor(12bit)+build high, build low, etc.
	// We store raw for now; user code can decode as needed.
	Left   [4]byte
	Dongle [4]byte
	Right  [4]byte
}

// ParseBudsFirmwareVersion parses 12 bytes.
func ParseBudsFirmwareVersion(payload []byte) (*BudsFirmwareVersion, error) {
	if len(payload) < 12 {
		return nil, fmt.Errorf("buds fw version needs 12 bytes, got %d", len(payload))
	}
	v := &BudsFirmwareVersion{}
	copy(v.Left[:], payload[0:4])
	copy(v.Dongle[:], payload[4:8])
	copy(v.Right[:], payload[8:12])
	return v, nil
}

// BuildGetFirmwareVersion asks for the full Buds FW version (12 bytes expected).
func BuildGetFirmwareVersion() []byte {
	return []byte{CmdGetFirmwareVersion, 0x00}
}
