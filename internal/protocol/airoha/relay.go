package airoha

import (
	"fmt"
	"time"

	"github.com/sstallion/go-hid"
)

const (
	RaceReportOut = 0x06
	RaceReportIn  = 0x07

	TargetLocal  = 0x00
	TargetRemote = 0x80

	MaxRelayPayload = 512
)

// RelayFrame is the outer Airoha Race HID wrapper.
// The RACE transaction details live inside Inner.
type RelayFrame struct {
	Target byte
	Inner  []byte
	Raw    []byte
}

type AirohaClient struct {
	dev     *hid.Device
	timeout time.Duration
}

func NewAirohaClient(dev *hid.Device) *AirohaClient {
	return &AirohaClient{dev: dev, timeout: 800 * time.Millisecond}
}

func (c *AirohaClient) SetTimeout(d time.Duration) {
	c.timeout = d
}

func BuildRelayFrame(target byte, inner []byte) *RelayFrame {
	return &RelayFrame{
		Target: target,
		Inner:  append([]byte(nil), inner...),
	}
}

func (f *RelayFrame) Encode() []byte {
	payload := make([]byte, 0, 1+len(f.Inner))
	payload = append(payload, f.Target)
	payload = append(payload, f.Inner...)
	f.Raw = append([]byte(nil), payload...)
	return payload
}

func DecodeRelayFrame(data []byte) (*RelayFrame, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("relay frame too short")
	}
	return &RelayFrame{
		Target: data[0],
		Inner:  append([]byte(nil), data[1:]...),
		Raw:    append([]byte(nil), data...),
	}, nil
}

func (c *AirohaClient) SendFrame(f *RelayFrame) error {
	payload := f.Encode()
	if len(payload) > 0xFF {
		return fmt.Errorf("relay payload too large: %d", len(payload))
	}
	report := make([]byte, 0, 2+len(payload))
	report = append(report, RaceReportOut, byte(len(payload)))
	report = append(report, payload...)
	_, err := c.dev.Write(report)
	return err
}

func (c *AirohaClient) RecvFrame() (*RelayFrame, error) {
	buf := make([]byte, MaxRelayPayload)
	n, err := c.dev.ReadWithTimeout(buf, c.timeout)
	if err != nil {
		if err == hid.ErrTimeout {
			return nil, fmt.Errorf("airoha relay read timeout")
		}
		return nil, err
	}
	if n < 3 {
		return nil, fmt.Errorf("short airoha relay read")
	}
	if buf[0] != RaceReportIn {
		return nil, fmt.Errorf("unexpected relay report id 0x%02X", buf[0])
	}
	length := int(buf[1])
	if length > n-2 {
		length = n - 2
	}
	return DecodeRelayFrame(buf[2 : 2+length])
}

func (c *AirohaClient) SendCommand(target byte, inner []byte) (*RelayFrame, error) {
	if err := c.SendFrame(BuildRelayFrame(target, inner)); err != nil {
		return nil, fmt.Errorf("send relay: %w", err)
	}
	return c.RecvFrame()
}

func (c *AirohaClient) RecvRaw() ([]byte, error) {
	buf := make([]byte, MaxRelayPayload)
	n, err := c.dev.ReadWithTimeout(buf, c.timeout)
	if err != nil {
		if err == hid.ErrTimeout {
			return nil, fmt.Errorf("airoha relay read timeout")
		}
		return nil, err
	}
	if n <= 0 {
		return nil, fmt.Errorf("empty read from airoha relay")
	}
	return append([]byte(nil), buf[:n]...), nil
}

func (c *AirohaClient) CollectRaw(count int, timeout time.Duration) [][]byte {
	if count <= 0 {
		count = 8
	}
	deadline := time.Now().Add(timeout)
	var out [][]byte
	for len(out) < count && time.Now().Before(deadline) {
		raw, err := c.RecvRaw()
		if err != nil {
			break
		}
		out = append(out, raw)
	}
	return out
}
