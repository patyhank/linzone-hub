package airoha

import (
	"fmt"
	"time"

	"github.com/patyhank/linzone-hub/internal/protocol"
	"github.com/sstallion/go-hid"
)

// BudsDevice talks to the Sony HCI control path used for normal Buds state.
type BudsDevice struct {
	dev    *hid.Device
	nextTx uint16
}

func OpenBuds(dev *hid.Device) *BudsDevice {
	return &BudsDevice{dev: dev, nextTx: 1}
}

func (b *BudsDevice) Close() error {
	if b.dev != nil {
		return b.dev.Close()
	}
	return nil
}

func (b *BudsDevice) allocTx() uint16 {
	id := b.nextTx
	b.nextTx++
	if b.nextTx == 0 {
		b.nextTx = 1
	}
	return id
}

func (b *BudsDevice) GetBattery() (*BatteryInfo, error) {
	info, err := GetBudsStartupInfo(b.dev, b.allocTx)
	if err != nil {
		return nil, err
	}
	if info.Battery == nil {
		return nil, fmt.Errorf("battery not present in Buds startup notifications")
	}
	return info.Battery, nil
}

func (b *BudsDevice) GetStartupInfo() (*BudsStartupInfo, error) {
	return GetBudsStartupInfo(b.dev, b.allocTx)
}

func (b *BudsDevice) GetHeadphoneVolume() (*BudsVolumeInfo, error) {
	n, err := QueryBudsEvent(b.dev, b.allocTx(), protocol.EvtHeadphoneVolume)
	if err != nil {
		return nil, err
	}
	return decodeBudsVolumeInfo(n.Param)
}

func (b *BudsDevice) SetHeadphoneVolume(percent byte) error {
	_, err := SendBudsHciSet(b.dev, b.allocTx(), protocol.EvtHeadphoneVolume, []byte{0, percent, percent}, budsHciReadTimeout)
	return err
}

func (b *BudsDevice) GetMicVolume() (*BudsVolumeInfo, error) {
	n, err := QueryBudsEvent(b.dev, b.allocTx(), protocol.EvtMicVolume)
	if err != nil {
		return nil, err
	}
	return decodeBudsVolumeInfo(n.Param)
}

func (b *BudsDevice) SetMicVolume(percent byte) error {
	_, err := SendBudsHciSet(b.dev, b.allocTx(), protocol.EvtMicVolume, []byte{0, percent, percent}, budsHciReadTimeout)
	return err
}

func (b *BudsDevice) GetAmbient() (*BudsAmbientInfo, error) {
	n, err := QueryBudsEvent(b.dev, b.allocTx(), protocol.EvtAmbSetting)
	if err != nil {
		return nil, err
	}
	return decodeBudsAmbientInfo(n.Param)
}

func (b *BudsDevice) SetAmbient(ncMode, ambientPercent byte, voiceFocus bool) error {
	vf := byte(0)
	if voiceFocus {
		vf = 1
	}
	_, err := SendBudsHciSet(b.dev, b.allocTx(), protocol.EvtAmbSetting, []byte{ncMode, ambientPercent, ambientPercent, vf}, budsHciReadTimeout)
	return err
}

// Probe captures whatever the Buds control interface emits without assuming relay framing.
func (b *BudsDevice) Probe() (map[string][]byte, error) {
	results := make(map[string][]byte)
	for i, raw := range CollectBudsNotifications(b.dev, 2*time.Second, 16) {
		results[fmt.Sprintf("notification_%02d_raw", i)] = append([]byte(nil), raw...)
		if n, err := DecodeBudsNotification(raw); err == nil {
			results[fmt.Sprintf("notification_%02d_param", i)] = append([]byte(nil), n.Param...)
		}
	}
	return results, nil
}

func (b *BudsDevice) ListenRaw(duration time.Duration) [][]byte {
	return CollectBudsNotifications(b.dev, duration, 32)
}
