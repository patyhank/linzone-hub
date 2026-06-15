package airoha

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/patyhank/linzone-hub/internal/protocol"
	"github.com/sstallion/go-hid"
)

const (
	budsHciEventPacket = 0x04
	budsHciCmdPacket   = 0x01
	budsHciVendorEvent = 0xFF
	budsHciVendorOp    = 0xFC00
	budsSonyKeyID      = 0xC396
	budsAddrPCRX       = 0x41
	budsFlagGET        = 0x01
	budsHciReportID    = 0x02
	budsHciReportSize  = 64

	budsHciReadTimeout = 1200 * time.Millisecond

	BudsEventModelInfo    = 0x02
	BudsEventFirmwareInfo = 0x03
	BudsEventBatteryInfo  = 0x04
)

// BudsNotification is the Sony HCI-style event observed inside the INZONE Buds
// Airoha HID interface. Real traffic captured from INZONE Buds starts with one
// outer byte before the normal Sony vendor event, for example:
//
//	12 04 FF 0F 00 96 C3 14 04 A0 01 00 00 56 00 54 FF 64 1F ...
//
// Starting at 04 FF this is the same Sony HCI event layout used by headsets:
// PacketType, VendorEvent, ParamLen, 00, SonyKeyID, Address, EventID,
// EventType, TxID, Param..., Checksum. For battery, Param is currently observed
// as LStatus, LPercent, RStatus, RPercent, CaseStatus, CasePercent, extra/check.
type BudsNotification struct {
	OuterPrefix []byte
	PacketType  byte
	EventCode   byte
	ParamLen    byte
	SonyKeyID   uint16
	Address     byte
	EventID     byte
	EventType   byte
	TxID        uint16
	Param       []byte
	Checksum    byte
	Raw         []byte
}

// DecodeBudsNotification parses the observed Airoha-wrapped Sony HCI event. It
// searches for the 04 FF vendor event header near the start so it accepts both
// raw reports with the outer prefix and already-stripped HCI payloads.
func DecodeBudsNotification(raw []byte) (*BudsNotification, error) {
	start := -1
	for i := 0; i+1 < len(raw) && i < 8; i++ {
		if raw[i] == budsHciEventPacket && raw[i+1] == budsHciVendorEvent {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("not an airoha-wrapped sony hci event")
	}

	data := raw[start:]
	if len(data) < 12 {
		return nil, fmt.Errorf("buds notification too short (%d bytes)", len(data))
	}
	if data[0] != budsHciEventPacket || data[1] != budsHciVendorEvent {
		return nil, fmt.Errorf("invalid buds notification header")
	}

	p := &BudsNotification{
		OuterPrefix: append([]byte(nil), raw[:start]...),
		PacketType:  data[0],
		EventCode:   data[1],
		ParamLen:    data[2],
		SonyKeyID:   binary.LittleEndian.Uint16(data[4:6]),
		Address:     data[6],
		EventID:     data[7],
		EventType:   data[8],
		TxID:        binary.LittleEndian.Uint16(data[9:11]),
		Raw:         append([]byte(nil), raw...),
	}
	if p.SonyKeyID != budsSonyKeyID {
		return nil, fmt.Errorf("unexpected sony key id 0x%04X", p.SonyKeyID)
	}

	paramLen := int(p.ParamLen) - 8
	if paramLen < 0 {
		paramLen = 0
	}
	if 11+paramLen+1 > len(data) {
		paramLen = len(data) - 12
	}
	if paramLen < 0 {
		paramLen = 0
	}
	if paramLen > 0 {
		p.Param = append([]byte(nil), data[11:11+paramLen]...)
	}
	if 11+paramLen < len(data) {
		p.Checksum = data[11+paramLen]
	}
	return p, nil
}

func DecodeBudsBatteryNotification(raw []byte) (*BatteryInfo, *BudsNotification, error) {
	n, err := DecodeBudsNotification(raw)
	if err != nil {
		return nil, nil, err
	}
	if n.EventID != BudsEventBatteryInfo {
		return nil, n, fmt.Errorf("not a battery notification (event=%d)", n.EventID)
	}
	bi, err := DecodeBattery(n.Param)
	if err != nil {
		return nil, n, err
	}
	return bi, n, nil
}

func budsHciChecksum(address, eventID, eventType byte, txID uint16, param []byte) byte {
	sum := int(byte(budsSonyKeyID&0xFF)) + int(byte((budsSonyKeyID>>8)&0xFF))
	sum += int(address) + int(eventID) + int(eventType)
	sum += int(byte(txID)) + int(byte(txID>>8))
	for _, b := range param {
		sum += int(b)
	}
	return byte(sum & 0xFF)
}

// BuildBudsHciGet builds a Sony HCI GET for the Buds vendor report.
// On the HID wire this is [ReportID=2][payload length][HCI packet...].
func BuildBudsHciGet(eventID byte, txID uint16, param []byte) []byte {
	if param == nil {
		param = []byte{}
	}
	paramLen := byte(8 + len(param))
	checksum := budsHciChecksum(budsAddrPCRX, eventID, budsFlagGET, txID, param)

	payload := make([]byte, 0, 12+len(param))
	payload = append(payload, budsHciCmdPacket)
	payload = append(payload, byte(budsHciVendorOp&0xFF), byte((budsHciVendorOp>>8)&0xFF))
	payload = append(payload, paramLen)
	payload = append(payload, byte(budsSonyKeyID&0xFF), byte((budsSonyKeyID>>8)&0xFF))
	payload = append(payload, budsAddrPCRX, eventID, budsFlagGET)
	payload = append(payload, byte(txID), byte(txID>>8))
	payload = append(payload, param...)
	payload = append(payload, checksum)

	if len(payload) > budsHciReportSize-2 {
		return nil
	}
	report := make([]byte, budsHciReportSize)
	report[0] = budsHciReportID
	report[1] = byte(len(payload))
	copy(report[2:], payload)
	return report
}

func BuildBudsHciSet(eventID byte, txID uint16, param []byte) []byte {
	if param == nil {
		param = []byte{}
	}
	paramLen := byte(8 + len(param))
	checksum := budsHciChecksum(budsAddrPCRX, eventID, protocol.FlagSET, txID, param)

	payload := make([]byte, 0, 12+len(param))
	payload = append(payload, budsHciCmdPacket)
	payload = append(payload, byte(budsHciVendorOp&0xFF), byte((budsHciVendorOp>>8)&0xFF))
	payload = append(payload, paramLen)
	payload = append(payload, byte(budsSonyKeyID&0xFF), byte((budsSonyKeyID>>8)&0xFF))
	payload = append(payload, budsAddrPCRX, eventID, protocol.FlagSET)
	payload = append(payload, byte(txID), byte(txID>>8))
	payload = append(payload, param...)
	payload = append(payload, checksum)

	if len(payload) > budsHciReportSize-2 {
		return nil
	}
	report := make([]byte, budsHciReportSize)
	report[0] = budsHciReportID
	report[1] = byte(len(payload))
	copy(report[2:], payload)
	return report
}

func recvBudsRaw(dev *hid.Device, wait time.Duration) ([]byte, error) {
	buf := make([]byte, 64)
	n, err := dev.ReadWithTimeout(buf, wait)
	if err != nil {
		if err == hid.ErrTimeout {
			return nil, fmt.Errorf("buds hci read timeout")
		}
		return nil, err
	}
	if n <= 0 {
		return nil, fmt.Errorf("empty read from buds hci interface")
	}
	if buf[0] == budsHciReportID && n >= 2 {
		length := int(buf[1])
		if length > n-2 {
			length = n - 2
		}
		return append([]byte(nil), buf[2:2+length]...), nil
	}
	return append([]byte(nil), buf[:n]...), nil
}

func SendBudsHciGet(dev *hid.Device, txID uint16, eventID byte, wait time.Duration) ([]*BudsNotification, error) {
	report := BuildBudsHciGet(eventID, txID, nil)
	if report == nil {
		return nil, fmt.Errorf("buds hci report too large for report id %d", budsHciReportID)
	}
	if _, err := dev.Write(report); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(wait)
	var out []*BudsNotification
	for time.Now().Before(deadline) {
		raw, err := recvBudsRaw(dev, wait)
		if err != nil {
			break
		}
		n, err := DecodeBudsNotification(raw)
		if err != nil {
			continue
		}
		out = append(out, n)
		if n.EventID == eventID {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no buds hci notifications after GET event %d", eventID)
	}
	return out, nil
}

func SendBudsHciSet(dev *hid.Device, txID uint16, eventID byte, param []byte, wait time.Duration) ([]*BudsNotification, error) {
	report := BuildBudsHciSet(eventID, txID, param)
	if report == nil {
		return nil, fmt.Errorf("buds hci report too large for report id %d", budsHciReportID)
	}
	if _, err := dev.Write(report); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(wait)
	var out []*BudsNotification
	for time.Now().Before(deadline) {
		raw, err := recvBudsRaw(dev, wait)
		if err != nil {
			break
		}
		n, err := DecodeBudsNotification(raw)
		if err != nil {
			continue
		}
		out = append(out, n)
		if n.EventID == eventID && (n.EventType == protocol.FlagRET || n.EventType == protocol.FlagNTFY) {
			break
		}
	}
	return out, nil
}

type BudsVolumeInfo struct {
	Mute    byte
	Raw     byte
	Percent byte
}

type BudsAmbientInfo struct {
	NCMode         byte
	AmbientRaw     byte
	AmbientPercent byte
	VoiceFocus     byte
}

func decodeBudsVolumeInfo(param []byte) (*BudsVolumeInfo, error) {
	if len(param) < 3 {
		return nil, fmt.Errorf("volume payload too short")
	}
	return &BudsVolumeInfo{Mute: param[0], Raw: param[1], Percent: param[2]}, nil
}

func decodeBudsAmbientInfo(param []byte) (*BudsAmbientInfo, error) {
	if len(param) < 4 {
		return nil, fmt.Errorf("ambient payload too short")
	}
	return &BudsAmbientInfo{NCMode: param[0], AmbientRaw: param[1], AmbientPercent: param[2], VoiceFocus: param[3]}, nil
}

func QueryBudsEvent(dev *hid.Device, txID uint16, eventID byte) (*BudsNotification, error) {
	notifications, err := SendBudsHciGet(dev, txID, eventID, budsHciReadTimeout)
	if err != nil {
		return nil, err
	}
	for _, n := range notifications {
		if n.EventID == eventID {
			return n, nil
		}
	}
	return nil, fmt.Errorf("no matching Buds event %d", eventID)
}

type BudsStartupInfo struct {
	Model   []byte
	Battery *BatteryInfo
	FW      []byte
}

func GetBudsStartupInfo(dev *hid.Device, nextTx func() uint16) (*BudsStartupInfo, error) {
	info := &BudsStartupInfo{}
	for _, eventID := range []byte{BudsEventModelInfo, BudsEventBatteryInfo, BudsEventFirmwareInfo} {
		notifications, err := SendBudsHciGet(dev, nextTx(), eventID, budsHciReadTimeout)
		if err != nil {
			continue
		}
		for _, n := range notifications {
			switch n.EventID {
			case BudsEventModelInfo:
				info.Model = append([]byte(nil), n.Param...)
			case BudsEventBatteryInfo:
				if bi, err := DecodeBattery(n.Param); err == nil {
					info.Battery = bi
				}
			case BudsEventFirmwareInfo:
				info.FW = append([]byte(nil), n.Param...)
			}
		}
	}
	if len(info.Model) == 0 && info.Battery == nil && len(info.FW) == 0 {
		return nil, fmt.Errorf("no Buds startup information received over Sony HCI control path")
	}
	return info, nil
}

func CollectBudsNotifications(dev *hid.Device, duration time.Duration, limit int) [][]byte {
	if limit <= 0 {
		limit = 32
	}
	deadline := time.Now().Add(duration)
	var out [][]byte
	for len(out) < limit && time.Now().Before(deadline) {
		raw, err := recvBudsRaw(dev, duration)
		if err != nil {
			break
		}
		out = append(out, raw)
	}
	return out
}
