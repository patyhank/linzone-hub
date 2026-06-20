package app

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/patyhank/linzone-hub/internal/protocol"
	airoha "github.com/patyhank/linzone-hub/internal/protocol/airoha"
	"github.com/patyhank/linzone-hub/internal/usb"
	"github.com/sstallion/go-hid"
)

type Service struct{}

func NewService() *Service { return &Service{} }

const udevRules = `# Allow the active local user and the input group to access Sony INZONE HID
# control interfaces. Final assignment prevents older local rules from
# overriding this back to another group such as uucp.
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0e53", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0e4c", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0e61", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0dfd", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0e47", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0ebf", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0ec2", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0ec3", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0fa8", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0f80", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0f81", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0fc0", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0fc1", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0fae", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0faf", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0fb1", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0fb2", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0fb0", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="hidraw", KERNEL=="hidraw*", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0fb3", GROUP:="input", MODE:="0660", TAG+="uaccess"

# Original INZONE H9/H7 0x0E53 control path is a USB serial/COM interface.
# It is not a modem; prevent ModemManager probing from disturbing the CDC ACM
# control channel on systems where ModemManager is enabled.
SUBSYSTEM=="tty", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0e53", ENV{ID_MM_DEVICE_IGNORE}="1", ENV{ID_MM_PORT_IGNORE}="1", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="tty", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0e4c", ENV{ID_MM_DEVICE_IGNORE}="1", ENV{ID_MM_PORT_IGNORE}="1", GROUP:="input", MODE:="0660", TAG+="uaccess"
SUBSYSTEM=="tty", ATTRS{idVendor}=="054c", ATTRS{idProduct}=="0e61", ENV{ID_MM_DEVICE_IGNORE}="1", ENV{ID_MM_PORT_IGNORE}="1", GROUP:="input", MODE:="0660", TAG+="uaccess"

# Mark INZONE batteries as device batteries instead of letting UPower guess them
# as generic/internal batteries.
SUBSYSTEM=="power_supply", KERNEL=="inzone_battery_headset_*", ENV{UPOWER_BATTERY_TYPE}="headset"
SUBSYSTEM=="power_supply", KERNEL=="inzone_battery_left_*", ENV{UPOWER_BATTERY_TYPE}="headset"
SUBSYSTEM=="power_supply", KERNEL=="inzone_battery_right_*", ENV{UPOWER_BATTERY_TYPE}="headset"
SUBSYSTEM=="power_supply", KERNEL=="inzone_battery_case_*", ENV{UPOWER_BATTERY_TYPE}="headset"
SUBSYSTEM=="power_supply", KERNEL=="inzone_battery_headset_*", ATTRS{idProduct}=="0fae", ENV{UPOWER_BATTERY_TYPE}:="mouse"
SUBSYSTEM=="power_supply", KERNEL=="inzone_battery_headset_*", ATTRS{idProduct}=="0faf", ENV{UPOWER_BATTERY_TYPE}:="mouse"
SUBSYSTEM=="power_supply", KERNEL=="inzone_battery_headset_*", ATTRS{idProduct}=="0fb0", ENV{UPOWER_BATTERY_TYPE}:="keyboard"
`

type DeviceSummary struct {
	Index        int    `json:"index"`
	Path         string `json:"path"`
	Model        string `json:"model"`
	Product      string `json:"product"`
	Manufacturer string `json:"manufacturer"`
	Serial       string `json:"serial"`
	VendorID     uint16 `json:"vendorId"`
	ProductID    uint16 `json:"productId"`
	UsagePage    uint16 `json:"usagePage"`
	Usage        uint16 `json:"usage"`
	Interface    int    `json:"interface"`
	Kind         string `json:"kind"`
	IsBuds       bool   `json:"isBuds"`
}

type BatteryState struct {
	Headset *BatteryCell `json:"headset,omitempty"`
	Left    *BatteryCell `json:"left,omitempty"`
	Right   *BatteryCell `json:"right,omitempty"`
	Case    *BatteryCell `json:"case,omitempty"`
}

type BatteryCell struct {
	Percent int `json:"percent"`
	Status  int `json:"status"`
}

type VolumeState struct {
	Mute    int `json:"mute"`
	Raw     int `json:"raw"`
	Percent int `json:"percent"`
}

type AmbientState struct {
	Mode           int  `json:"mode"`
	AmbientRaw     int  `json:"ambientRaw"`
	AmbientPercent int  `json:"ambientPercent"`
	VoiceFocus     bool `json:"voiceFocus"`
}

type MouseState struct {
	CurrentProfile int                `json:"currentProfile"`
	DPI            int                `json:"dpi"`
	ReportRateHz   int                `json:"reportRateHz"`
	LODLevel       int                `json:"lodLevel"`
	SensorSnap     bool               `json:"sensorSnap"`
	MotionSync     bool               `json:"motionSync"`
	LEDBrightness  int                `json:"ledBrightness"`
	LEDRed         int                `json:"ledRed"`
	LEDGreen       int                `json:"ledGreen"`
	LEDBlue        int                `json:"ledBlue"`
	RFStatus       int                `json:"rfStatus"`
	Buttons        []MouseButtonState `json:"buttons"`
}

type MouseButtonState struct {
	ButtonIndex int `json:"buttonIndex"`
	TypeDef     int `json:"typeDef"`
	KeyType     int `json:"keyType"`
	KeyCode1    int `json:"keyCode1"`
	KeyCode2    int `json:"keyCode2"`
}

type SimpleState struct {
	EventID int   `json:"eventId"`
	Value   *int  `json:"value,omitempty"`
	Raw     []int `json:"raw"`
}

type DeviceState struct {
	Device          DeviceSummary `json:"device"`
	ModelID         *int          `json:"modelId,omitempty"`
	Color           *int          `json:"color,omitempty"`
	SerialNumber    *int          `json:"serialNumber,omitempty"`
	Status          *int          `json:"status,omitempty"`
	FirmwareRaw     string        `json:"firmwareRaw,omitempty"`
	ModelRaw        string        `json:"modelRaw,omitempty"`
	Battery         BatteryState  `json:"battery"`
	HeadphoneVolume *VolumeState  `json:"headphoneVolume,omitempty"`
	MicVolume       *VolumeState  `json:"micVolume,omitempty"`
	Ambient         *AmbientState `json:"ambient,omitempty"`
	Mouse           *MouseState   `json:"mouse,omitempty"`
	GameChatMix     *SimpleState  `json:"gameChatMix,omitempty"`
	Sidetone        *VolumeState  `json:"sidetone,omitempty"`
	Surround        *SimpleState  `json:"surround,omitempty"`
	BTStatus        *SimpleState  `json:"btStatus,omitempty"`
	BTSoundQuality  *SimpleState  `json:"btSoundQuality,omitempty"`
	BTStartupMode   *SimpleState  `json:"btStartupMode,omitempty"`
	AutoPowerOff    *SimpleState  `json:"autoPowerOff,omitempty"`
	NCStartupMode   *SimpleState  `json:"ncStartupMode,omitempty"`
	ConnectionMode  *SimpleState  `json:"connectionMode,omitempty"`
	Assignable      *SimpleState  `json:"assignable,omitempty"`
	Warnings        []string      `json:"warnings"`
}

type FeatureStatus struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

func (s *Service) ListDevices() ([]DeviceSummary, error) {
	devs, err := inzoneDevices()
	if err != nil {
		return nil, err
	}
	out := make([]DeviceSummary, 0, len(devs))
	for i, d := range devs {
		out = append(out, summarizeDevice(i, d))
	}
	return out, nil
}

func (s *Service) GetDeviceState(index int) (*DeviceState, error) {
	base, err := deviceByIndex(index)
	if err != nil {
		return nil, err
	}
	info := summarizeDevice(index, base)
	state := &DeviceState{Device: info, Warnings: []string{}}
	if usb.IsLegacySerialHeadset(base.ProductID) {
		model, batt, fw, serialState, err := readLegacySerialHeadsetState(base)
		if err != nil {
			state.Warnings = append(state.Warnings, err.Error())
		}
		if model != nil {
			state.ModelID = intPtr(int(model.ModelID))
			state.Color = intPtr(int(model.Color))
			state.SerialNumber = intPtr(int(model.Serial))
			state.Status = intPtr(int(model.Status))
		}
		if batt != nil {
			state.Battery.Headset = batteryCell(batt.Percent, batt.Status)
		}
		state.FirmwareRaw = hex.EncodeToString(fw)
		if serialState != nil {
			state.HeadphoneVolume = serialState.HeadphoneVolume
			state.MicVolume = serialState.MicVolume
			state.Ambient = serialState.Ambient
			state.GameChatMix = serialState.GameChatMix
			state.Sidetone = serialState.Sidetone
			state.Surround = serialState.Surround
			state.BTStatus = serialState.BTStatus
			state.BTSoundQuality = serialState.BTSoundQuality
			state.BTStartupMode = serialState.BTStartupMode
			state.AutoPowerOff = serialState.AutoPowerOff
			state.NCStartupMode = serialState.NCStartupMode
			state.ConnectionMode = serialState.ConnectionMode
			state.Assignable = serialState.Assignable
			state.Warnings = append(state.Warnings, serialState.Warnings...)
		}
		if model != nil || batt != nil || len(fw) > 0 || serialState != nil {
			return state, nil
		}
	}

	dev, openInfo, cleanup, err := openDevice(index)
	if err != nil {
		state.Warnings = append(state.Warnings, err.Error())
		return state, nil
	}
	defer cleanup()
	state.Device = openInfo

	if info.IsBuds {
		buds := airoha.OpenBuds(dev)
		if startup, err := buds.GetStartupInfo(); err == nil {
			state.ModelRaw = hex.EncodeToString(startup.Model)
			state.FirmwareRaw = hex.EncodeToString(startup.FW)
			if startup.Battery != nil {
				state.Battery.Left = batteryCell(startup.Battery.LeftPercent, startup.Battery.LeftStatus)
				state.Battery.Right = batteryCell(startup.Battery.RightPercent, startup.Battery.RightStatus)
				state.Battery.Case = batteryCell(startup.Battery.CasePercent, startup.Battery.CaseStatus)
			}
		} else {
			state.Warnings = append(state.Warnings, err.Error())
		}
		if vol, err := buds.GetHeadphoneVolume(); err == nil {
			state.HeadphoneVolume = budsVolume(vol)
		}
		if vol, err := buds.GetMicVolume(); err == nil {
			state.MicVolume = budsVolume(vol)
		}
		if amb, err := buds.GetAmbient(); err == nil {
			state.Ambient = &AmbientState{Mode: int(amb.NCMode), AmbientRaw: int(amb.AmbientRaw), AmbientPercent: int(amb.AmbientPercent), VoiceFocus: amb.VoiceFocus != 0}
		}
		return state, nil
	}

	if isKBM(info.ProductID) {
		if usb.IsMouse(info.ProductID) {
			mouseState := &MouseState{}
			if batt, err := protocol.GetMouseBatteryInfo(dev); err == nil {
				status := 0
				if batt.IsCharging {
					status = 1
				}
				state.Battery.Headset = &BatteryCell{Percent: batt.Percent, Status: status}
				mouseState.RFStatus = int(batt.RFStatus)
			} else {
				state.Warnings = append(state.Warnings, fmt.Sprintf("mouse battery: %v", err))
			}
			if raw, err := protocol.GetDeviceInformationKBM(dev); err == nil {
				if info, err := protocol.ParseMouseInfo(raw); err == nil {
					mouseState.CurrentProfile = int(info.CurrentProfile)
				} else {
					state.Warnings = append(state.Warnings, fmt.Sprintf("mouse device info: %v", err))
				}
			} else {
				state.Warnings = append(state.Warnings, fmt.Sprintf("mouse device info: %v", err))
			}
			if fn, err := protocol.GetMouseProfileFunction(dev); err == nil {
				mouseState.DPI = int(fn.DPI)
				mouseState.ReportRateHz = fn.ReportRateHz
				mouseState.LODLevel = int(fn.LODLevel)
				mouseState.SensorSnap = fn.SensorSnap
				mouseState.MotionSync = fn.MotionSync
				mouseState.LEDBrightness = int(fn.LEDBrightness)
				mouseState.LEDRed = int(fn.LEDRed)
				mouseState.LEDGreen = int(fn.LEDGreen)
				mouseState.LEDBlue = int(fn.LEDBlue)
			} else {
				state.Warnings = append(state.Warnings, fmt.Sprintf("mouse profile function: %v", err))
			}
			if buttons, err := protocol.GetMouseButtons(dev); err == nil {
				mouseState.Buttons = make([]MouseButtonState, len(buttons))
				for i, button := range buttons {
					mouseState.Buttons[i] = MouseButtonState{
						ButtonIndex: int(button.ButtonIndex),
						TypeDef:     int(button.TypeDef),
						KeyType:     int(button.KeyType),
						KeyCode1:    int(button.KeyCode1),
						KeyCode2:    int(button.KeyCode2),
					}
				}
			} else {
				state.Warnings = append(state.Warnings, fmt.Sprintf("mouse buttons: %v", err))
			}
			state.Mouse = mouseState
		}
		return state, nil
	}

	model, batt, fw, err := readHeadsetInfo(index, dev)
	if err != nil {
		state.Warnings = append(state.Warnings, err.Error())
	}
	if model != nil {
		state.ModelID = intPtr(int(model.ModelID))
		state.Color = intPtr(int(model.Color))
		state.SerialNumber = intPtr(int(model.Serial))
		state.Status = intPtr(int(model.Status))
	}
	if batt != nil {
		state.Battery.Headset = batteryCell(batt.Percent, batt.Status)
	}
	state.FirmwareRaw = hex.EncodeToString(fw)
	if vol, err := getVolume(dev, protocol.EvtHeadphoneVolume); err == nil {
		state.HeadphoneVolume = vol
	}
	if vol, err := getVolume(dev, protocol.EvtMicVolume); err == nil {
		state.MicVolume = vol
	}
	if amb, err := getAmbient(dev); err == nil {
		state.Ambient = amb
	}
	state.GameChatMix = getSimpleState(dev, protocol.EvtGameChatMix, &state.Warnings)
	state.Sidetone = getSidetone(dev, &state.Warnings)
	state.Surround = getSimpleState(dev, protocol.EvtSurroundSetting, &state.Warnings)
	state.BTStatus = getSimpleState(dev, protocol.EvtBTStatus, &state.Warnings)
	state.BTSoundQuality = getSimpleState(dev, protocol.EvtBTSoundQuality, &state.Warnings)
	state.BTStartupMode = getSimpleState(dev, protocol.EvtBTStartupMode, &state.Warnings)
	state.AutoPowerOff = getSimpleState(dev, protocol.EvtAutoPowerOff, &state.Warnings)
	state.NCStartupMode = getSimpleState(dev, protocol.EvtNCStartupMode, &state.Warnings)
	state.ConnectionMode = getSimpleState(dev, protocol.EvtConnectionDest, &state.Warnings)
	state.Assignable = getSimpleState(dev, protocol.EvtAssignableParam, &state.Warnings)
	return state, nil
}

func (s *Service) SetHeadphoneVolume(index int, percent int) error {
	pct, err := percentByte(percent)
	if err != nil {
		return err
	}
	raw := percentRawByte(percent)
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtHeadphoneVolume, []byte{0, raw, pct})
	}
	return withHeadset(index, func(dev *hid.Device, info DeviceSummary) error {
		if info.IsBuds {
			if percent < 0 || percent > 30 {
				return fmt.Errorf("INZONE Buds headphone volume must be 0-30")
			}
			return airoha.OpenBuds(dev).SetHeadphoneVolume(pct)
		}
		return protocol.SetHeadphoneVolume(dev, false, raw, pct)
	})
}

func (s *Service) SetMicVolume(index int, percent int) error {
	pct, err := percentByte(percent)
	if err != nil {
		return err
	}
	raw := percentRawByte(percent)
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtMicVolume, []byte{0, raw, pct})
	}
	return withHeadset(index, func(dev *hid.Device, info DeviceSummary) error {
		if info.IsBuds {
			return airoha.OpenBuds(dev).SetMicVolume(pct)
		}
		return protocol.SetMicVolume(dev, false, raw, pct)
	})
}

func (s *Service) SetSidetone(index int, percent int) error {
	pct, err := percentByte(percent)
	if err != nil {
		return err
	}
	raw := percentRawByte(percent)
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtSidetoneVolume, []byte{raw, pct})
	}
	return withHeadset(index, func(dev *hid.Device, info DeviceSummary) error {
		if info.IsBuds {
			return airoha.OpenBuds(dev).SetSidetone(pct)
		}
		return protocol.SetSidetone(dev, raw, pct)
	})
}

func (s *Service) SetGameChatMix(index int, balance int) error {
	pct, err := percentByte(balance)
	if err != nil {
		return err
	}
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtGameChatMix, []byte{pct})
	}
	return withHeadset(index, func(dev *hid.Device, info DeviceSummary) error {
		if info.IsBuds {
			return airoha.OpenBuds(dev).SetGameChatMix(pct)
		}
		return protocol.SetGameChatMix(dev, pct)
	})
}

func (s *Service) SetAmbient(index int, mode int, level int, voiceFocus bool) error {
	if mode < 0 || mode > 3 {
		return fmt.Errorf("ambient mode must be 0-3")
	}
	pct, err := percentByte(level)
	if err != nil {
		return err
	}
	raw := percentRawByte(level)
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		vf := byte(0)
		if voiceFocus {
			vf = 1
		}
		return withLegacySerialSet(base, protocol.EvtAmbSetting, []byte{byte(mode), raw, pct, vf})
	}
	return withHeadset(index, func(dev *hid.Device, info DeviceSummary) error {
		if info.IsBuds {
			return airoha.OpenBuds(dev).SetAmbient(byte(mode), pct, voiceFocus)
		}
		return protocol.SetAmbient(dev, byte(mode), raw, pct, voiceFocus)
	})
}

func (s *Service) SetSurround(index int, enabled bool) error {
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		value := byte(0)
		if enabled {
			value = 1
		}
		return withLegacySerialSet(base, protocol.EvtSurroundSetting, []byte{value})
	}
	return withHeadset(index, func(dev *hid.Device, info DeviceSummary) error {
		if info.IsBuds {
			return airoha.OpenBuds(dev).SetSurround(enabled)
		}
		return protocol.SetSurround(dev, enabled)
	})
}

func (s *Service) SetNCStartupMode(index int, mode int) error {
	value, err := boundedByte(mode, 0, 3, "NC startup mode")
	if err != nil {
		return err
	}
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtNCStartupMode, []byte{value})
	}
	return headsetOnly(index, "NC startup mode", func(dev *hid.Device) error {
		return protocol.SetNCStartupMode(dev, value)
	})
}

func (s *Service) SetBTSoundQuality(index int, mode int) error {
	value, err := boundedByte(mode, 0, 2, "Bluetooth sound quality")
	if err != nil {
		return err
	}
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtBTSoundQuality, []byte{value})
	}
	return headsetOnly(index, "Bluetooth sound quality", func(dev *hid.Device) error {
		return protocol.SetBTSoundQuality(dev, value)
	})
}

func (s *Service) SetBTStartupMode(index int, mode int) error {
	value, err := boundedByte(mode, 0, 2, "Bluetooth startup mode")
	if err != nil {
		return err
	}
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtBTStartupMode, []byte{value})
	}
	return headsetOnly(index, "Bluetooth startup mode", func(dev *hid.Device) error {
		return protocol.SetBTStartupMode(dev, value)
	})
}

func (s *Service) SetAutoPowerOff(index int, minutes int) error {
	switch minutes {
	case 0, 5, 15, 30, 60, 180:
	default:
		return fmt.Errorf("auto power off must be one of 0, 5, 15, 30, 60, 180")
	}
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtAutoPowerOff, []byte{byte(minutes)})
	}
	return headsetOnly(index, "auto power off", func(dev *hid.Device) error {
		return protocol.SetAutoPowerOff(dev, byte(minutes))
	})
}

func (s *Service) SetConnectionDestinationMode(index int, mode int) error {
	value, err := boundedByte(mode, 0, 3, "connection destination mode")
	if err != nil {
		return err
	}
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtConnectionDest, []byte{value})
	}
	return headsetOnly(index, "connection destination mode", func(dev *hid.Device) error {
		return protocol.SetConnectionDestinationMode(dev, value)
	})
}

func (s *Service) SetIncomingPermission(index int, enabled bool) error {
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		value := byte(0)
		if enabled {
			value = 1
		}
		return withLegacySerialSet(base, protocol.EvtIncomingPerm, []byte{value})
	}
	return headsetOnly(index, "incoming permission", func(dev *hid.Device) error {
		return protocol.SetIncomingPermission(dev, enabled)
	})
}

func (s *Service) SetAssignableAction(index int, slot int, action int) error {
	slotByte, err := boundedByte(slot, 0, 15, "assignable slot")
	if err != nil {
		return err
	}
	actionByte, err := boundedByte(action, 0, 255, "assignable action")
	if err != nil {
		return err
	}
	if base, ok, err := legacyDeviceByIndex(index); err != nil {
		return err
	} else if ok {
		return withLegacySerialSet(base, protocol.EvtAssignableParam, []byte{slotByte, actionByte})
	}
	return headsetOnly(index, "assignable settings", func(dev *hid.Device) error {
		return protocol.SetAssignableSettings(dev, []byte{slotByte, actionByte})
	})
}

func (s *Service) SetMouseProfile(index int, profile int) error {
	value, err := boundedByte(profile, 1, 4, "mouse profile")
	if err != nil {
		return err
	}
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.SetProfileNumber(dev, value)
	})
}

func (s *Service) SetMouseDPI(index int, dpi int) error {
	if dpi < 0 || dpi > 65535 {
		return fmt.Errorf("DPI must be 0-65535")
	}
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.SetDPI(dev, uint16(dpi))
	})
}

func (s *Service) SetMouseLED(index int, brightness int, red int, green int, blue int) error {
	bri, err := boundedByte(brightness, 0, 255, "LED brightness")
	if err != nil {
		return err
	}
	r, err := boundedByte(red, 0, 255, "LED red")
	if err != nil {
		return err
	}
	g, err := boundedByte(green, 0, 255, "LED green")
	if err != nil {
		return err
	}
	b, err := boundedByte(blue, 0, 255, "LED blue")
	if err != nil {
		return err
	}
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.SetLEDLighting(dev, bri, r, g, b)
	})
}

func (s *Service) SetMouseLOD(index int, level int) error {
	value, err := boundedByte(level, 0, 2, "mouse LOD level")
	if err != nil {
		return err
	}
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.SetLOD(dev, value)
	})
}

func (s *Service) SetMouseMotionSync(index int, enabled bool) error {
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.SetMotionSync(dev, enabled)
	})
}

func (s *Service) SetMouseSensorSnap(index int, enabled bool) error {
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.SetSensorSnap(dev, enabled)
	})
}

func (s *Service) SetMouseReportRate(index int, hz int) error {
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.SetReportRate(dev, hz)
	})
}

func (s *Service) SetMouseButton(index int, buttonIndex int, typeDef int, keyType int, keyCode1 int, keyCode2 int) error {
	button, err := boundedByte(buttonIndex, 1, 5, "mouse button")
	if err != nil {
		return err
	}
	typeValue, err := boundedByte(typeDef, 0, 255, "mouse button type definition")
	if err != nil {
		return err
	}
	keyTypeValue, err := boundedByte(keyType, 0, 255, "mouse button key type")
	if err != nil {
		return err
	}
	key1, err := boundedByte(keyCode1, 0, 255, "mouse button key code 1")
	if err != nil {
		return err
	}
	key2, err := boundedByte(keyCode2, 0, 255, "mouse button key code 2")
	if err != nil {
		return err
	}
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.SetMouseButton(dev, button, typeValue, keyTypeValue, key1, key2)
	})
}

func (s *Service) SaveMouseToProfile(index int) error {
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.SaveMouseToProfile(dev)
	})
}

func (s *Service) ResetMouseToDefault(index int) error {
	return withMouse(index, func(dev *hid.Device) error {
		return protocol.ResetMouseToDefault(dev)
	})
}

func (s *Service) RepairUdevPermissions() error {
	if _, err := exec.LookPath("pkexec"); err != nil {
		return fmt.Errorf("pkexec is required to install udev rules: %w", err)
	}

	script := fmt.Sprintf(`set -eu
umask 022
cat > /etc/udev/rules.d/99-inzone-battery.rules <<'INZONE_RULES'
%sINZONE_RULES
chmod 0644 /etc/udev/rules.d/99-inzone-battery.rules
udevadm control --reload-rules
udevadm trigger --subsystem-match=hidraw || true
udevadm trigger --subsystem-match=tty || true
udevadm trigger --subsystem-match=usb --attr-match=idVendor=054c || true
`, udevRules)

	path := filepath.Join(os.TempDir(), "linzone-hub-fix-udev.sh")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		return fmt.Errorf("write temporary udev repair script: %w", err)
	}
	defer os.Remove(path)

	out, err := exec.Command("pkexec", "/bin/sh", path).CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if text != "" {
			return fmt.Errorf("udev permission repair failed: %w: %s", err, text)
		}
		return fmt.Errorf("udev permission repair failed: %w", err)
	}
	return nil
}

func (s *Service) FeatureMatrix() []FeatureStatus {
	return []FeatureStatus{
		{"Device detection", "ready", "Sony VID/PID HID detection for supported headset and Buds IDs."},
		{"Battery", "ready", "Headset battery and Buds left/right/case battery when the device exposes the known HCI events."},
		{"Headphone volume", "ready", "Mapped to HCI event 33 for headsets and Buds."},
		{"Microphone volume", "ready", "Mapped to HCI event 36 for headsets and Buds."},
		{"Sidetone", "partial", "Mapped for standard headset HCI event 35; Buds payload is not confirmed."},
		{"Game / Chat mix", "partial", "Mapped for standard headset HCI event 34; Buds payload is not confirmed."},
		{"Noise canceling / Ambient", "ready", "Mapped to HCI event 65 with mode, ambient level and voice focus."},
		{"Bluetooth controls", "partial", "Sound quality, startup mode, connection mode and auto power off are mapped to documented HCI events."},
		{"Touch / button assignment", "partial", "Assignable settings event is exposed as slot/action writes; exact model layouts may need capture tuning."},
		{"Profiles / 10-band EQ", "ready", "Local profile CRUD/import/export stores EQ and control presets; device EQ writes remain profile-level until command IDs are verified."},
		{"HRTF / 360 Spatial", "partial", "Surround toggle is mapped; personalization/APO calibration stays out of scope on Linux."},
	}
}

func inzoneDevices() ([]usb.DeviceInfo, error) {
	devs, err := usb.Enumerate()
	if err != nil {
		return nil, err
	}
	out := make([]usb.DeviceInfo, 0, len(devs))
	for _, d := range devs {
		if usb.IsSupported(d.VendorID, d.ProductID) {
			out = append(out, d)
		}
	}
	return out, nil
}

func deviceByIndex(index int) (usb.DeviceInfo, error) {
	devs, err := inzoneDevices()
	if err != nil {
		return usb.DeviceInfo{}, err
	}
	if index < 0 || index >= len(devs) {
		return usb.DeviceInfo{}, fmt.Errorf("device index %d out of range (have %d)", index, len(devs))
	}
	return devs[index], nil
}

func openDevice(index int) (*hid.Device, DeviceSummary, func(), error) {
	base, err := deviceByIndex(index)
	if err != nil {
		return nil, DeviceSummary{}, nil, err
	}
	openInfo := base
	if usb.IsBuds(base.ProductID) {
		control, err := usb.FindBudsControlInterface(base)
		if err != nil {
			return nil, summarizeDevice(index, base), nil, err
		}
		openInfo = control
	} else if isKBM(base.ProductID) {
		control, err := usb.FindProtocolAInterface(base)
		if err != nil {
			return nil, summarizeDevice(index, base), nil, err
		}
		openInfo = control
	}
	dev, err := usb.Open(openInfo)
	if err != nil {
		return nil, summarizeDevice(index, openInfo), nil, fmt.Errorf("open %s: %w", openInfo.Model, err)
	}
	return dev, summarizeDevice(index, openInfo), func() { _ = dev.Close() }, nil
}

func openHeadset(index int) (*hid.Device, DeviceSummary, func(), error) {
	dev, info, cleanup, err := openDevice(index)
	if err != nil {
		return nil, info, cleanup, err
	}
	if !usb.IsHeadset(info.ProductID) {
		cleanup()
		return nil, info, nil, fmt.Errorf("%s is not a headset device", info.Model)
	}
	return dev, info, cleanup, nil
}

func withHeadset(index int, fn func(*hid.Device, DeviceSummary) error) error {
	base, err := deviceByIndex(index)
	if err != nil {
		return err
	}
	if usb.IsBuds(base.ProductID) {
		dev, info, cleanup, err := openHeadset(index)
		if err != nil {
			return err
		}
		defer cleanup()
		return fn(dev, info)
	}
	if !usb.IsHeadset(base.ProductID) {
		return fmt.Errorf("%s is not a headset device", base.Model)
	}
	candidates, err := usb.FindHeadsetControlInterfaces(base)
	if err != nil {
		return err
	}
	var lastErr error
	for _, candidate := range candidates {
		dev, err := usb.Open(candidate)
		if err != nil {
			lastErr = fmt.Errorf("open %s %s: %w", candidate.Model, candidate.Path, err)
			continue
		}
		info := summarizeDevice(index, candidate)
		err = fn(dev, info)
		_ = dev.Close()
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("%s %s: %w", candidate.Model, candidate.Path, err)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no usable headset control interface found for %s", base.Model)
}

func readLegacySerialHeadsetState(base usb.DeviceInfo) (*protocol.HeadsetModelInfo, *protocol.BatteryInfo, []byte, *DeviceState, error) {
	ports, err := protocol.FindSonySerialPorts(base.VendorID, base.ProductID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var lastErr error
	for _, port := range ports {
		serial, err := protocol.OpenSerialHeadset(port.Path)
		if err != nil {
			lastErr = fmt.Errorf("open %s serial port %s: %w", base.Model, port.Path, err)
			continue
		}
		model, batt, fw, err := protocol.GetSerialHeadsetInfo(serial)
		if err != nil {
			lastErr = fmt.Errorf("%s serial query on %s: %w", base.Model, port.Path, err)
		}
		if model == nil && batt == nil && len(fw) == 0 {
			_ = serial.Close()
			continue
		}
		state := &DeviceState{Warnings: []string{}}
		state.HeadphoneVolume = getSerialVolume(serial, protocol.EvtHeadphoneVolume, &state.Warnings)
		state.MicVolume = getSerialVolume(serial, protocol.EvtMicVolume, &state.Warnings)
		state.Ambient = getSerialAmbient(serial, &state.Warnings)
		state.GameChatMix = getSerialSimpleState(serial, protocol.EvtGameChatMix, &state.Warnings)
		state.Sidetone = getSerialSidetone(serial, &state.Warnings)
		state.Surround = getSerialSimpleState(serial, protocol.EvtSurroundSetting, &state.Warnings)
		state.BTStatus = getSerialSimpleState(serial, protocol.EvtBTStatus, &state.Warnings)
		state.BTSoundQuality = getSerialSimpleState(serial, protocol.EvtBTSoundQuality, &state.Warnings)
		state.BTStartupMode = getSerialSimpleState(serial, protocol.EvtBTStartupMode, &state.Warnings)
		state.AutoPowerOff = getSerialSimpleState(serial, protocol.EvtAutoPowerOff, &state.Warnings)
		state.NCStartupMode = getSerialSimpleState(serial, protocol.EvtNCStartupMode, &state.Warnings)
		state.ConnectionMode = getSerialSimpleState(serial, protocol.EvtConnectionDest, &state.Warnings)
		state.Assignable = getSerialSimpleState(serial, protocol.EvtAssignableParam, &state.Warnings)
		_ = serial.Close()
		return model, batt, fw, state, nil
	}
	if lastErr != nil {
		return nil, nil, nil, nil, lastErr
	}
	return nil, nil, nil, nil, fmt.Errorf("no serial response from %s", base.Model)
}

func legacyDeviceByIndex(index int) (usb.DeviceInfo, bool, error) {
	base, err := deviceByIndex(index)
	if err != nil {
		return usb.DeviceInfo{}, false, err
	}
	return base, usb.IsLegacySerialHeadset(base.ProductID), nil
}

func withLegacySerialSet(base usb.DeviceInfo, eventID byte, param []byte) error {
	ports, err := protocol.FindSonySerialPorts(base.VendorID, base.ProductID)
	if err != nil {
		return err
	}
	var lastErr error
	for _, port := range ports {
		serial, err := protocol.OpenSerialHeadset(port.Path)
		if err != nil {
			lastErr = fmt.Errorf("open %s serial port %s: %w", base.Model, port.Path, err)
			continue
		}
		err = protocol.SendSerialHciSet(serial, eventID, param)
		_ = serial.Close()
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("%s serial set event %d on %s: %w", base.Model, eventID, port.Path, err)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no serial tty found for VID:PID %04x:%04x", base.VendorID, base.ProductID)
}

func getSerialVolume(serial *protocol.SerialHeadset, eventID byte, warnings *[]string) *VolumeState {
	pkt, err := protocol.QuerySerialHciEvent(serial, eventID)
	if err != nil {
		appendProbeWarning(warnings, fmt.Sprintf("serial event %d", eventID), err)
		return nil
	}
	if len(pkt.Param) < 3 {
		appendProbeWarning(warnings, fmt.Sprintf("serial event %d", eventID), fmt.Errorf("volume payload too short"))
		return nil
	}
	return &VolumeState{Mute: int(pkt.Param[0]), Raw: int(pkt.Param[1]), Percent: devicePercent(pkt.Param[2])}
}

func getSerialAmbient(serial *protocol.SerialHeadset, warnings *[]string) *AmbientState {
	pkt, err := protocol.QuerySerialHciEvent(serial, protocol.EvtAmbSetting)
	if err != nil {
		appendProbeWarning(warnings, "serial ambient", err)
		return nil
	}
	if len(pkt.Param) < 4 {
		appendProbeWarning(warnings, "serial ambient", fmt.Errorf("payload too short"))
		return nil
	}
	return &AmbientState{Mode: int(pkt.Param[0]), AmbientRaw: int(pkt.Param[1]), AmbientPercent: devicePercent(pkt.Param[2]), VoiceFocus: pkt.Param[3] != 0}
}

func getSerialSidetone(serial *protocol.SerialHeadset, warnings *[]string) *VolumeState {
	pkt, err := protocol.QuerySerialHciEvent(serial, protocol.EvtSidetoneVolume)
	if err != nil {
		appendProbeWarning(warnings, "serial sidetone", err)
		return nil
	}
	if len(pkt.Param) < 2 {
		appendProbeWarning(warnings, "serial sidetone", fmt.Errorf("payload too short"))
		return nil
	}
	return &VolumeState{Raw: int(pkt.Param[0]), Percent: devicePercent(pkt.Param[1])}
}

func getSerialSimpleState(serial *protocol.SerialHeadset, eventID byte, warnings *[]string) *SimpleState {
	pkt, err := protocol.QuerySerialHciEvent(serial, eventID)
	if err != nil {
		appendProbeWarning(warnings, fmt.Sprintf("serial event %d", eventID), err)
		return nil
	}
	raw := make([]int, len(pkt.Param))
	for i, b := range pkt.Param {
		raw[i] = int(b)
	}
	var value *int
	if len(raw) > 0 {
		value = intPtr(raw[0])
	}
	return &SimpleState{EventID: int(eventID), Value: value, Raw: raw}
}

func readHeadsetInfo(index int, opened *hid.Device) (*protocol.HeadsetModelInfo, *protocol.BatteryInfo, []byte, error) {
	model, batt, fw, _, err := protocol.GetHeadsetInfoAuto(opened)
	if err == nil && (model != nil || batt != nil || len(fw) > 0) {
		return model, batt, fw, nil
	}

	base, baseErr := deviceByIndex(index)
	if baseErr != nil || usb.IsBuds(base.ProductID) || !usb.IsHeadset(base.ProductID) {
		return model, batt, fw, err
	}
	candidates, candErr := usb.FindHeadsetControlInterfaces(base)
	if candErr != nil {
		return model, batt, fw, firstErr(err, candErr)
	}

	var lastErr error = err
	for _, candidate := range candidates {
		dev, openErr := usb.Open(candidate)
		if openErr != nil {
			lastErr = openErr
			continue
		}
		m, b, f, _, probeErr := protocol.GetHeadsetInfoAuto(dev)
		_ = dev.Close()
		if probeErr == nil && (m != nil || b != nil || len(f) > 0) {
			return m, b, f, nil
		}
		if probeErr != nil {
			lastErr = fmt.Errorf("%s %s: %w", candidate.Model, candidate.Path, probeErr)
		}
	}
	return model, batt, fw, lastErr
}

func firstErr(primary error, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}

func summarizeDevice(index int, d usb.DeviceInfo) DeviceSummary {
	kind := deviceKind(d.ProductID)
	return DeviceSummary{
		Index:        index,
		Path:         d.Path,
		Model:        d.Model,
		Product:      d.ProductStr,
		Manufacturer: d.MfrStr,
		Serial:       d.SerialNbr,
		VendorID:     d.VendorID,
		ProductID:    d.ProductID,
		UsagePage:    d.UsagePage,
		Usage:        d.Usage,
		Interface:    d.InterfaceNbr,
		Kind:         kind,
		IsBuds:       usb.IsBuds(d.ProductID),
	}
}

func deviceKind(pid uint16) string {
	switch {
	case usb.IsLegacySerialHeadset(pid):
		return "legacy-headset"
	case usb.IsMouse(pid):
		return "mouse"
	case usb.IsKeyboard(pid):
		return "keyboard"
	case usb.IsBuds(pid):
		return "buds"
	default:
		return "headset"
	}
}

func isKBM(pid uint16) bool {
	return usb.IsMouse(pid) || usb.IsKeyboard(pid)
}

func getHCI(dev *hid.Device, eventID byte) (*protocol.HciPacket, error) {
	pkt, _, err := protocol.GetEventAuto(dev, eventID)
	return pkt, err
}

func getVolume(dev *hid.Device, eventID byte) (*VolumeState, error) {
	pkt, err := getHCI(dev, eventID)
	if err != nil {
		return nil, err
	}
	if len(pkt.Param) < 3 {
		return nil, fmt.Errorf("volume payload too short")
	}
	return &VolumeState{Mute: int(pkt.Param[0]), Raw: int(pkt.Param[1]), Percent: devicePercent(pkt.Param[2])}, nil
}

func getAmbient(dev *hid.Device) (*AmbientState, error) {
	pkt, err := getHCI(dev, protocol.EvtAmbSetting)
	if err != nil {
		return nil, err
	}
	if len(pkt.Param) < 4 {
		return nil, fmt.Errorf("ambient payload too short")
	}
	return &AmbientState{Mode: int(pkt.Param[0]), AmbientRaw: int(pkt.Param[1]), AmbientPercent: devicePercent(pkt.Param[2]), VoiceFocus: pkt.Param[3] != 0}, nil
}

func getSidetone(dev *hid.Device, warnings *[]string) *VolumeState {
	pkt, err := getHCI(dev, protocol.EvtSidetoneVolume)
	if err != nil {
		appendProbeWarning(warnings, "sidetone", err)
		return nil
	}
	if len(pkt.Param) < 2 {
		appendProbeWarning(warnings, "sidetone", fmt.Errorf("payload too short"))
		return nil
	}
	return &VolumeState{Raw: int(pkt.Param[0]), Percent: devicePercent(pkt.Param[1])}
}

func getSimpleState(dev *hid.Device, eventID byte, warnings *[]string) *SimpleState {
	pkt, err := getHCI(dev, eventID)
	if err != nil {
		appendProbeWarning(warnings, fmt.Sprintf("event %d", eventID), err)
		return nil
	}
	raw := make([]int, len(pkt.Param))
	for i, b := range pkt.Param {
		raw[i] = int(b)
	}
	var value *int
	if len(raw) > 0 {
		value = intPtr(raw[0])
	}
	return &SimpleState{EventID: int(eventID), Value: value, Raw: raw}
}

func appendProbeWarning(warnings *[]string, label string, err error) {
	if warnings == nil || err == nil {
		return
	}
	*warnings = append(*warnings, fmt.Sprintf("%s: %v", label, err))
}

func headsetOnly(index int, label string, fn func(*hid.Device) error) error {
	return withHeadset(index, func(dev *hid.Device, info DeviceSummary) error {
		if info.IsBuds {
			return fmt.Errorf("%s command for INZONE Buds is not mapped yet", label)
		}
		return fn(dev)
	})
}

func withMouse(index int, fn func(*hid.Device) error) error {
	base, err := deviceByIndex(index)
	if err != nil {
		return err
	}
	if !usb.IsMouse(base.ProductID) {
		return fmt.Errorf("%s is not a mouse device", base.Model)
	}
	dev, info, cleanup, err := openDevice(index)
	if err != nil {
		return err
	}
	defer cleanup()
	if !usb.IsMouse(info.ProductID) {
		return fmt.Errorf("%s is not a mouse device", info.Model)
	}
	return fn(dev)
}

func budsVolume(v *airoha.BudsVolumeInfo) *VolumeState {
	return &VolumeState{Mute: int(v.Mute), Raw: int(v.Raw), Percent: int(v.Percent)}
}

func batteryCell(percent byte, status byte) *BatteryCell {
	return &BatteryCell{Percent: int(percent), Status: int(status)}
}

func devicePercent(value byte) int {
	if value <= 100 {
		return int(value)
	}
	return int((uint16(value)*100 + 127) / 255)
}

func percentByte(v int) (byte, error) {
	if v < 0 || v > 100 {
		return 0, fmt.Errorf("percent must be 0-100")
	}
	return byte(v), nil
}

func percentRawByte(v int) byte {
	if v <= 0 {
		return 0
	}
	if v >= 100 {
		return 255
	}
	return byte((v*255 + 50) / 100)
}

func boundedByte(v int, min int, max int, name string) (byte, error) {
	if v < min || v > max {
		return 0, fmt.Errorf("%s must be %d-%d", name, min, max)
	}
	return byte(v), nil
}

func intPtr(v int) *int { return &v }

func (d DeviceSummary) DisplayName() string {
	parts := []string{d.Model}
	if d.Product != "" && d.Product != d.Model {
		parts = append(parts, d.Product)
	}
	return strings.Join(parts, " / ")
}
