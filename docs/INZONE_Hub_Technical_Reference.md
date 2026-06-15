# Sony INZONE Hub - Complete Technical Reference

> Reverse-engineered from INZONE Hub v1.x (INZONEHub.dll, AirohaHidCoreLib.dll)
> All devices use USB VID `054C` (Sony Corporation)

---

## Table of Contents

1. [Supported Devices](#1-supported-devices)
2. [Transport Layer](#2-transport-layer)
3. [Protocol A: Keyboard & Mouse HID](#3-protocol-a-keyboard--mouse-hid)
4. [Protocol B: Headset HCI](#4-protocol-b-headset-hci)
5. [Protocol C: Airoha AB1565 Relay (Earbuds)](#5-protocol-c-airoha-ab1565-relay)
6. [Firmware Update](#6-firmware-update)
7. [Appendix: Enum Reference Tables](#7-appendix-enum-reference-tables)

---

## 1. Supported Devices

### 1.1 Headsets

| PID (hex) | PID (dec) | Model Code | Protocol | Product Name | Connection |
|-----------|-----------|------------|----------|-------------|------------|
| `0x0E53` | 3667 | HDX_2959 | COM (serial 460800) | INZONE H9 | USB wired |
| `0x0E4C` | 3660 | HDX_2959 | PS5 | INZONE H9 | PS5 USB |
| `0x0E61` | 3681 | HDX_2959 | OTA | INZONE H9 Dongle | Wireless dongle |
| `0x0DFD` | 3581 | HDX_2961 | H3 | INZONE H7 (3-pole) | USB wired |
| `0x0E47` | 3655 | HDX_2961 | H3 | INZONE H7 (USB) | USB wired |
| `0x0EBF` | 3775 | GH_M2 | HID | INZONE H5 (YY2976) | USB wired |
| `0x0EC2` | 3778 | GTW | HID | INZONE Buds (YY2977) | USB wired |
| `0x0FA8` | 4008 | GH_H2 | HID | INZONE H10 (YY2987) | USB wired |
| `0x0F80` | 3968 | GH_IE | H3 | INZONE E9 (YY2989, 3-pole) | USB wired |
| `0x0F81` | 3969 | GH_IE | H3 | INZONE E9 (YY2989, 4-pole) | USB wired |
| `0x0FC0` | 4032 | GH_OB | H3 | INZONE H6 Air (YY2990, 4-pole) | USB wired |
| `0x0FC1` | 4033 | GH_OB | H3 | INZONE H6 Air (YY2990, 3-pole) | USB wired |

**PS5 variants** (no HID control, audio only):

| PID (hex) | Model | Protocol |
|-----------|-------|----------|
| `0x0EC0` | GH_M2 | PS5 |
| `0x0EC3` | GTW | PS5 (mobile) |
| `0x0FA9` | GH_H2 | PS5 (mobile) |

### 1.2 Mouse (INZONE Mouse-A, YY2991)

| PID (hex) | PID (dec) | Protocol | HID Interface | Description |
|-----------|-----------|----------|---------------|-------------|
| `0x0FAE` | 4014 | MOUSE_MOUSE | MI_02 | Mouse (wired USB) |
| `0x0FAF` | 4015 | MOUSE_DONGLE | MI_02 | RF Dongle |
| `0x0FB1` | 4017 | MOUSE_MOUSE_BOOTLOADER | VID_054C&PID_0FB1 | Mouse bootloader |
| `0x0FB2` | 4018 | MOUSE_DONGLE_BOOTLOADER | VID_054C&PID_0FB2 | Dongle bootloader |

### 1.3 Keyboard (INZONE KBD-H75, YY2992)

| PID (hex) | PID (dec) | Protocol | HID Interface | Description |
|-----------|-----------|----------|---------------|-------------|
| `0x0FB0` | 4016 | KEYBOARD + KEYBOARD_VOLUME | MI_02 (main) + MI_03 (volume knob) | Keyboard |
| `0x0FB3` | 4019 | KEYBOARD_BOOTLOADER | VID_054C&PID_0FB3 | Keyboard bootloader |

---

## 2. Transport Layer

All device communication uses **USB HID reports** via the Windows HID API (`hid.dll`): `HidD_SetOutputReport` / `HidD_GetInputReport`.

### 2.1 HID Report Parameters

| Parameter | Headset | Keyboard/Mouse | Volume Knob |
|-----------|---------|----------------|-------------|
| Usage Page | `0xFF04` (65284) | `0xFF00` (65280) / `0xFF90` (65424) | `0xFF05` (65285) |
| Report ID | 2 | 0 | 4 |
| In Report Length | 64 bytes | 65 bytes | 3 bytes |
| Out Report Length | 64 bytes | 65 bytes | 0 (read-only) |
| Data Length in Header | Yes (byte 1) | No | No |
| Send Interval | 1 ms | 5 ms (kbd) / 40 ms (mouse) | N/A |
| Write Timeout | 1000 ms | 1000 ms | 1000 ms |

### 2.2 HID Report Framing

**Headset (with data-length header):**
```
Output Report (64 bytes):
  [0] Report ID = 0x02
  [1] Data length N
  [2..2+N-1] Payload (max 62 bytes)
  [2+N..63] Zero padding

Input Report (64 bytes):
  [0] Report ID = 0x02
  [1] Data length N
  [2..2+N-1] Payload
```

**Keyboard/Mouse (no data-length header):**
```
Output Report (65 bytes):
  [0] Report ID = 0x00
  [1..64] Payload (max 64 bytes, zero-padded)

Input Report (65 bytes):
  [0] Report ID = 0x00
  [1..64] Payload
```

If payload exceeds the report capacity, it is fragmented across multiple HID reports with `Thread.Sleep(sendInterval)` between each.

### 2.3 COM Serial Port (Legacy H9 Headset)

For the original INZONE H9 (PID `0x0E53`), communication uses a raw serial port instead of HID:
- **Baud rate:** 460800
- **Data bits:** 8
- **Parity:** None
- **Stop bits:** 1
- **Flow control:** None
- **RTS/DTR:** Both asserted (enabled)

HCI packets are sent as raw bytes over the serial port with no additional framing.

### 2.4 Device Discovery

The app uses Windows `SetupDiGetClassDevs` with `GUID_DEVINTERFACE_HID` to enumerate HID devices. It matches devices by:
1. Parsing `VID_XXXX&PID_XXXX` from the device instance ID
2. Checking against the `supportedDevices` table
3. For HID devices, also matching `MI_XX&COLXX` (USB interface/collection number)

---

## 3. Protocol A: Keyboard & Mouse HID

### 3.1 Packet Structure

All keyboard/mouse commands use a common 4-byte header + variable payload:

```
Offset  Size  Field       Description
[0]     1     Command     Operation type (GET/SET/NOTIFY)
[1]     1     Index       High byte of 16-bit command ID
[2]     1     Packet      Low/high nibble value (sub-index)
[3]     1     Length      Payload data length
[4..N]  N     Data        Payload bytes
```

**Command byte values:**
| Value | Name | Direction |
|-------|------|-----------|
| `0xA0` (160) | GET | Host -> Device (request), Device -> Host (response) |
| `0x20` (32) | SET | Host -> Device |
| `0x99` (153) | NOTIFY | Device -> Host (unsolicited) |

**16-bit Command Index:** `(Index << 8) | Command`
- Example: GET_DEVICE_INFORMATION = 160 = `(0 << 8) | 0xA0`
- Example: SET_PROFILE_NUMBER = 32 = `(0 << 8) | 0x20`

### 3.2 Response Format (SET commands)

When the device acknowledges a SET command:
```
[0..3]  Standard header (echo of command)
[4..5]  Status (ushort LE): OK = 0xACDC (44268), NG = 0xFAEC (64236)
[6]     Error code (only if NG):
          0 = COMMAND_CONTENT_ERROR
          1 = KEY_CONTENT_ERROR
          2 = INDEX_CONTENT_ERROR
          3 = LENGTH_CONTENT_ERROR
          4 = DATA_CONTENT_ERROR
          10 = SEND_COMMAND_AGAIN
```

### 3.3 Mouse Commands

#### 3.3.1 Command Table

| Command Name | ID | Type | Payload |
|-------------|-----|------|---------|
| `GET_DEVICE_INFOMATION` | 160 (0x00A0) | GET | 18 bytes |
| `GET_PROFILE_ENABLE` | 416 (0x01A0) | GET | 1 byte |
| `GET_PROFILE_NAME` | 672 (0x02A0) | GET | Multi-packet |
| `GET_WIRELESS_MOUSE_STATUS` | 928 (0x03A0) | GET | 1 byte |
| `GET_CURRENT_BATTERY_INFO` | 1184 (0x04A0) | GET | 1 byte |
| `GET_PROFILE_FUNCTION` | 1440 (0x05A0) | GET | 9 bytes |
| `GET_BUTTONS_RE_DEFINED_KEY_INFO` | 1696 (0x06A0) | GET | 15 bytes |
| `GET_BUTTONS_TYPE_INFO` | 1952 (0x07A0) | GET | 5 bytes |
| `GET_BUTTONS_DEFAULT_INFO` | 2208 (0x08A0) | GET | 15 bytes |
| `GET_CHARGE_STATUS` | 2464 (0x09A0) | GET | 1 byte |
| `SET_PROFILE_NUMBER` | 32 (0x0020) | SET | 1 byte |
| `SET_PROFILE_ENABLE` | 288 (0x0120) | SET | 1 byte |
| `SET_PROFILE_NAME` | 544 (0x0220) | SET | Multi-packet |
| `SET_LOD_LEVEL` | 800 (0x0320) | SET | 1 byte |
| `SET_LED_LIGHTING` | 1312 (0x0520) | SET | 4 bytes |
| `SET_SENSOR_SNAP` | 1568 (0x0620) | SET | 1 byte |
| `SET_DPI_LEVEL_VALUE` | 1824 (0x0720) | SET | 2 bytes |
| `SET_MOTION_SYNC` | 2080 (0x0820) | SET | 1 byte |
| `SET_BUTTONS` | 2336 (0x0920) | SET | 6 bytes |
| `SET_REPORT_RATE` | 4128 (0x1020) | SET | 1 byte |
| `SAVE_TO_PROFILE` | 20512 (0x5020) | SET | Variable |
| `RESET_TO_DEFAULT` | 25120 (0x6220) | SET | 8 bytes |
| `NOTIFY_CURRENT_PROFILE_NUMBER` | 153 (0x0099) | NOTIFY | 1 byte |
| `NOTIFY_WIRELESS_MOUSE_STATUS` | 409 (0x0199) | NOTIFY | 1 byte |
| `NOTIFY_FOR_CHARGE_STATUS` | 665 (0x0299) | NOTIFY | 1 byte |
| `NOTIFY_FOR_CURRENT_BATTERY` | 921 (0x0399) | NOTIFY | 1 byte |

#### 3.3.2 GET_DEVICE_INFOMATION (160) - 18 bytes

```
Offset  Size  Field
[0..1]  2     PID (ushort LE)
[2..3]  2     VID (ushort LE)
[4]     1     Mouse FW Revision
[5]     1     Mouse FW Build
[6]     1     Mouse FW Minor
[7]     1     Mouse FW Major
[8]     1     USB Dongle FW Revision
[9]     1     USB Dongle FW Build
[10]    1     USB Dongle FW Minor
[11]    1     USB Dongle FW Major
[12]    1     RF Dongle FW Revision
[13]    1     RF Dongle FW Build
[14]    1     RF Dongle FW Minor
[15]    1     RF Dongle FW Major
[16]    1     Current Profile (1-4)
[17]    1     Color SKU (0=Black, 1=Orange)
```

FW Version construction: `new Version(param[7], param[6], param[5], param[4])`

#### 3.3.3 GET_PROFILE_FUNCTION (1440) - 9 bytes

```
Offset  Size  Field
[0]     1     Report Rate (0-4)
[1]     1     Sensor Snap / Angle Snapping (0=Off, 1=On)
[2]     1     LOD / Lift-Off Distance (0=0.7mm, 1=1.0mm, 2=2.0mm)
[3..4]  2     DPI (ushort LE, encoded: wire = (actual - 50) / 50)
[5]     1     Motion Sync (0=Off, 1=On)
[6]     1     LED Brightness (0-255)
[7]     1     LED Color Red (0-255)
[8]     1     LED Color Green (0-255)
[9]     1     LED Color Blue (0-255)
```

**DPI encoding:**
```
wire_value = (actual_dpi - 50) / 50
actual_dpi = wire_value * 50 + 50
```
Valid DPI range: multiples of 50, starting at 50.

**Report Rate encoding:**
```
value 0 = 500 Hz
value 1 = 1000 Hz
value 2 = 2000 Hz
value 3 = 4000 Hz
value 4 = 8000 Hz
Formula: hz = 500 << value
```

#### 3.3.4 SET_BUTTONS (2336) - 6 bytes per button

```
Offset  Size  Field
[0]     1     Type Definition (0xFF=Default, 0x00=Off, 0x40=Remapped)
[1]     1     Padding (0x00)
[2]     1     Padding (0x00)
[3]     1     Key Type (0xFF=Empty, 0=Mouse, 1=Keyboard, 2=Consumer)
[4]     1     Key Code 1
[5]     1     Key Code 2
```

**5 buttons:** Left(1), Right(2), Middle(3), Side1(4), Side2(5)

**Mouse key codes:** 1=Left, 2=Right, 3=Mid, 4=Button4, 5=Button5

**Consumer key codes:** 0xCD=Play/Pause, 0xB7=Stop, 0xB6=Prev, 0xB5=Next, 0xE2=Mute, 0xEA=Vol-, 0xE9=Vol+

**Modifier keys (KeyCode2):** 0x01=LCtrl, 0x02=LShift, 0x04=LAlt, 0x08=LWin, 0x10=RCtrl, 0x20=RShift, 0x40=RAlt, 0x80=RWin

#### 3.3.5 GET_BUTTONS_RE_DEFINED_KEY_INFO (1696) - 15 bytes

3 bytes per button (5 buttons), sequential:
```
[KeyType:1][KeyCode1:1][KeyCode2:1] x 5
```

#### 3.3.6 GET_BUTTONS_TYPE_INFO (1952) - 5 bytes

1 byte per button:
```
[TypeDef_btn1][TypeDef_btn2][TypeDef_btn3][TypeDef_btn4][TypeDef_btn5]
```

#### 3.3.7 SET_LED_LIGHTING (1312) - 4 bytes

```
[Brightness:1][Red:1][Green:1][Blue:1]
```

#### 3.3.8 SAVE_TO_PROFILE (20512) - Variable

Data split by `Packet.LowByte`:
- **LowByte=0:** ProfileFunction (9 bytes)
- **LowByte=1:** Button TypeDefinitions (5 bytes)
- **LowByte=2:** Button KeyAssignments (15 bytes)

#### 3.3.9 RESET_TO_DEFAULT (25120) - 8 bytes

Magic bytes: `0x36 0x31 0x18 0x38 0x27 0x98 0x10 0x94`

#### 3.3.10 GET_WIRELESS_MOUSE_STATUS (928) - 1 byte

```
RF_STATUS: 0xCC (204) = Connected, 0xDD (221) = Disconnected
```

#### 3.3.11 GET_CURRENT_BATTERY_INFO (1184) - 1 byte

```
BATTERY: 0=Low, 1=LowMid, 2=HighMid, 3=High
```

#### 3.3.12 GET_CHARGE_STATUS (2464) - 1 byte

```
CHARGE_STATUS: 0=NonCharging, 1=Charging
```

#### 3.3.13 Mouse Initialization Sequence

```
1. GET_WIRELESS_MOUSE_STATUS (928)
2. GET_DEVICE_INFOMATION (160)
3. GET_CURRENT_BATTERY_INFO (1184)
4. GET_CHARGE_STATUS (2464)
5. GET_PROFILE_ENABLE (416)
6. For each profile 1-4:
   a. GET_PROFILE_FUNCTION (1440)
   b. GET_PROFILE_NAME (672)
   c. GET_BUTTONS_RE_DEFINED_KEY_INFO (1696)
   d. GET_BUTTONS_TYPE_INFO (1952)
   e. GET_BUTTONS_DEFAULT_INFO (2208)
```

---

### 3.4 Keyboard Commands

#### 3.4.1 Command Table

| Command Name | ID | Type |
|-------------|-----|------|
| `GET_DEVICE_INFORMATION` | 160 | GET |
| `GET_PROFILE_NAME` | 416 | GET (multi-packet) |
| `GET_LIGHTING_EFFECT_INFORMATION` | 672 | GET |
| `GET_PALETTE_CUSTOM_COLOR_SETTING1` | 928 | GET |
| `GET_GAME_FUNCTION_ENABLE_DISABLE` | 1184 | GET |
| `GET_KEY_SETTING_INFO` | 1440 | GET (multi-packet) |
| `GET_KEY_TRIGGER_POINT` | 1696 | GET (multi-packet) |
| `GET_KEY_RAPID_TRIGGER_MAKE_POINT` | 1952 | GET (multi-packet) |
| `GET_KEY_RAPID_TRIGGER_BREAK_POINT` | 2208 | GET (multi-packet) |
| `GET_KEY_WITH_FN_KEY_SETTING_INFO` | 2464 | GET (multi-packet) |
| `GET_PALETTE_CUSTOM_COLOR_SETTING2` | 2720 | GET |
| `GET_MACRO_KEY_INFO` | 4256 | GET |
| `GET_MACRO_KEY_DATA_01` .. `_10` | 4512..6816 | GET |
| `GET_ALL_LIGHTING_EFFECT_INFORMATION` | 17056 | GET |
| `GET_DEFAULT_*` (all defaults) | 12960..15264 | GET |
| `SET_CURRENT_PROFILE` | 32 | SET |
| `SET_PROFILE_NAME` | 288 | SET (multi-packet) |
| `SET_LIGHTING_EFFECT_INFORMATION` | 544 | SET |
| `SET_PALETTE_CUSTOM_COLOR_SETTING1` | 800 | SET |
| `SET_GAME_MODE_FUNCTION_ENABLE_DISABLE` | 1056 | SET |
| `SET_KEY_SETTING_INFO` | 1312 | SET (multi-packet) |
| `SET_KEY_TRIGGER_POINT` | 1568 | SET (multi-packet) |
| `SET_KEY_RAPID_TRIGGER_MAKE_POINT` | 1824 | SET (multi-packet) |
| `SET_KEY_RAPID_TRIGGER_BREAK_POINT` | 2080 | SET (multi-packet) |
| `SET_KEY_WITH_FN_KEY_SETTING_INFO` | 2336 | SET (multi-packet) |
| `SET_PALETTE_CUSTOM_COLOR_SETTING2` | 2592 | SET |
| `SET_MACRO_KEY_INFO` | 4128 | SET |
| `SET_MACRO_KEY_DATA_01` .. `_10` | 4384..6688 | SET |
| `SET_FUNCTIONS_OF_PROFILE_TO_DEFAULT` | 8224 | SET |
| `KEY_CALIBRATION_0_MM_START_STOP` | 8736 | SET |
| `KEY_CALIBRATION_3_4_MM_START_STOP` | 8992 | SET |
| `SET_SAVE_PROFILE_INFO` | 12320 | SET |
| `SET_ALL_LIGHTING_EFFECT_INFORMATION` | 16928 | SET |
| `NOTIFY_CURRENT_PROFILE_NUMBER` | 153 | NOTIFY |
| `NOTIFY_CURRENT_LIGHTING_EFFECT` | 409 | NOTIFY |
| `NOTIFY_VOLUME_KNOB_DIRECTION` | 665 | NOTIFY |
| `NOTIFY_ILLUMINATION_MODE_SAVE` | 921 | NOTIFY |
| `NOTIFY_ILLUMINATION_MODE_CANCEL` | 1177 | NOTIFY |
| `NOTIFY_PROFESSIONAL_MODE_CHANGED` | 1433 | NOTIFY |

#### 3.4.2 GET_DEVICE_INFORMATION (160) - 15 bytes

```
Offset  Size  Field
[0..1]  2     PID (ushort LE)
[2..3]  2     VID (ushort LE)
[4]     1     MCU1 FW Major
[5]     1     MCU1 FW Build
[6]     1     MCU1 FW Minor
[7]     1     MCU1 FW Revision
[8]     1     MCU2 FW Major
[9]     1     MCU2 FW Build
[10]    1     MCU2 FW Minor
[11]    1     MCU2 FW Revision
[12]    1     Current Profile (1-4)
[13]    1     Multi Language (0=US, 1=JP, 2=UK, 3=FR, 4=NOR, 5=DE)
[14]    1     Color Code (0=Black, 1=Orange)
```

#### 3.4.3 Lighting Effect Structure (6 bytes per effect)

```
Offset  Size  Field
[0]     1     Speed (0=NoNeed, 1=Low, 2=Middle, 3=MiddleHigh, 4=High)
[1]     1     Color Type (see table below)
[2]     1     Custom Red (used when ColorType=8)
[3]     1     Custom Green
[4]     1     Custom Blue
[5]     1     Brightness (0-10, each = 10%)
```

**Lighting Effects:**

| Value | Name | Description |
|-------|------|-------------|
| 0 | NO_LIGHT_EFFECT | Off |
| 1 | COLOR_CYCLE | Rainbow cycle |
| 2 | STATIC_COLOR_SINGLE_COLOR | Static single color |
| 3 | COLOR_WAVE | Color wave |
| 4 | BREATH | Breathing effect |
| 5 | SNAKE | Snake effect |
| 6 | REACTIVE | Reactive key press |
| 7 | FOUNTAIN | Fountain effect |
| 8 | LASER | Laser effect |
| 9 | TRAFFIC | Traffic effect |
| 10 | RIPPLE | Ripple effect |
| 11 | INZONE_ORIGINAL_1 | INZONE custom 1 |
| 12 | INZONE_ORIGINAL_2 | INZONE custom 2 |
| 13 | PALETTE_CUSTOM_COLOR1 | Per-key custom 1 |
| 14 | PALETTE_CUSTOM_COLOR2 | Per-key custom 2 |

**Color Types:**

| Value | Name | Description |
|-------|------|-------------|
| 0 | NO_COLOR | No color |
| 1 | SINGLE_COLOR_WHITE | White (255,255,255) |
| 2 | SINGLE_COLOR_RED | Red (255,0,0) |
| 3 | SINGLE_COLOR_GREEN | Green (0,255,0) |
| 4 | SINGLE_COLOR_BLUE | Blue (0,0,255) |
| 5 | SINGLE_COLOR_YELLOW | Yellow (255,255,0) |
| 6 | SINGLE_COLOR_PURPLE | Purple (255,0,255) |
| 7 | SINGLE_COLOR_CYAN | Cyan (0,255,255) |
| 8 | SINGLE_CUSTOM_COLOR | Custom RGB (bytes 2-4) |
| 9 | SPECTRUM_GRADIENT | Spectrum gradient |
| 10 | INZONE_GRADIENT | INZONE gradient |
| 11 | SEVEN_COLOR_CYCLE | 7-color cycle |
| 12 | SPECTRUM_COLOR_CYCLE | Spectrum cycle |
| 13 | INZONE_PURPLE_COLOR_CYCLE | INZONE purple cycle |
| 14 | WARM_COLOR_CYCLE | Warm color cycle |
| 15 | COLD_COLOR_CYCLE | Cold color cycle |

**SET single effect (544):**
```
[Location:1 (effect enum value)] [Speed:1] [ColorType:1] [R:1] [G:1] [B:1] [Brightness:1]
= 7 bytes
```

**SET ALL effects (16928):**
```
14 effects x 6 bytes each = 84 bytes (split across multiple 60-byte packets)
```

#### 3.4.4 Game Mode Function (1 byte, bit-packed)

```
Bit 0: Game Mode
Bit 1: Windows Key Lock
Bit 2: Alt+F4 Lock
Bit 3: Alt+Tab Lock
Bit 4: Professional Mode (Ultra Low Latency)
Bit 5: Caps Lock LED
Bit 6: FN Key LED
Bit 7: (unused)
```

#### 3.4.5 Key Assignment (4 bytes per key)

```
[0] Key Type
[1] Key Code 1
[2] Key Code 2
[3] Key Code 3
```

**Key Types:**

| Value | Name | Description |
|-------|------|-------------|
| 0 | STANDARD_KEY | Standard keyboard key |
| 2 | MOUSE_BUTTON | Mouse button |
| 3 | CONSUMER_KEY | Media/consumer key |
| 5 | COMBINE_KEY | Key combination (uses all 3 key codes) |
| 6 | MACRO_KEY | Macro assignment |
| 7 | SPECIAL_KEY | Special function key |
| 15 | NO_FUNCTION | Disabled |
| 240 | DEFAULT_FUNCTION | Factory default |

#### 3.4.6 Key Trigger Point (1 byte per key)

```
Value 0-255: Actuation point (default = 15)
```

#### 3.4.7 Rapid Trigger Make/Break Point (1 byte per key, bit-packed)

```
Bit 7:    Enabled (1=enabled, 0=disabled)
Bit 6:    Point Linked (1=linked to actuation point, 0=independent)
Bits 5-0: Distance (0-63, actual mm = distance / 10.0)
```

Example: `0xC5` = Enabled(1) + Linked(1) + Distance=5 (0.5mm)

#### 3.4.8 Per-Key Custom Color (Palette) (3 bytes per key)

```
[Red:1][Green:1][Blue:1]
```

Sent via `SET_PALETTE_CUSTOM_COLOR_SETTING1` (800) or `SETTING2` (2592).

#### 3.4.9 Profile Name (Multi-packet)

Each character is padded to 4 bytes. Max 15 characters per 60-byte packet.

```
Packet 1 (LowByte=0): [CharCount:1][PaddedName:up to 59 bytes]
Packet 2 (LowByte=1): [PaddedName:up to 60 bytes]
...
```

Total packets needed: `ceil(CharCount / 15)`

#### 3.4.10 Save Profile (SET_SAVE_PROFILE_INFO = 12320) - 3 bytes

Dirty flags telling the device which sections to commit to flash:

**Byte 0:**
```
Bit 7: Profile Name
Bit 6: Lighting Effect
Bit 5: Palette Custom Color 1
Bit 4: Key Settings
Bit 3: Key Trigger Points
Bit 2: Rapid Trigger Make Points
Bit 1: Rapid Trigger Break Points
Bit 0: (unused)
```

**Byte 1:**
```
Bit 7: Macro Key 8
Bit 6: Macro Key 7
Bit 5: Macro Key 6
Bit 4: Macro Key 5
Bit 3: Macro Key 4
Bit 2: Macro Key 3
Bit 1: Macro Key 2
Bit 0: Macro Key 1
```

**Byte 2:**
```
Bit 7-6: (unused)
Bit 5: Palette Custom Color 2
Bit 4: FN Key Settings
Bit 3-2: (unused)
Bit 1: Macro Key 10
Bit 0: Macro Key 9
```

#### 3.4.11 Calibration Commands

```
KEY_CALIBRATION_0_MM_START_STOP (8736):
  Payload: 1 byte - 0x55=Start, 0xAA=Stop, 0x5A=Abort

KEY_CALIBRATION_3_4_MM_START_STOP (8992):
  Payload: 1 byte - 0x55=Start, 0xAA=Stop, 0x5A=Abort
```

#### 3.4.12 Key Location Map

```
 0=ESC   1=F1    2=F2    3=F3    4=F4    5=F5    6=F6    7=F7
 8=F8    9=F9   10=F10  11=F11  12=F12  13=RES1  14=RES2

15=~    16=1    17=2    18=3    19=4    20=5    21=6    22=7
23=8    24=9    25=0    26=-    27==    28=BS   29=DEL  30=PGUP  31=RES3

32=TAB  33=Q    34=W    35=E    36=R    37=T    38=Y    39=U
40=I    41=O    42=P    43=[    44=]    45=\    46=INS  47=PGDN  48=RES4

49=CAPS 50=A    51=S    52=D    53=F    54=G    55=H    56=J
57=K    58=L    59=;    60='    61=ENT

62=LSFT 63=Z    64=X    65=C    66=V    67=B    68=N    69=M
70=,    71=.    72=/    73=RSFT  74=UP

75=LCTL 76=LWIN 77=LALT 78=SPC  79=RES5  80=RALT 81=FN  82=RCTL
83=LEFT 84=DOWN 85=RIGHT

86=MUTE 87=VOL+ 88=VOL-  89=RES6  90=RES7
```

Note: Locations 86-88 (volume keys) are skipped in trigger point and rapid trigger commands.

#### 3.4.13 Volume Knob (Separate HID Interface)

The keyboard volume knob uses a separate HID interface:
- **Usage Page:** `0xFF05` (65285)
- **Report ID:** 4
- **In Report Length:** 3 bytes
- **Out:** None (read-only)

```
NOTIFY_VOLUME_KNOB_DIRECTION (665):
  1 byte: 1=Clockwise, 2=CounterClockwise
```

---

## 4. Protocol B: Headset HCI

### 4.1 HCI Packet Structure

#### 4.1.1 Command Packet (PC -> Headset)

```
Offset  Size  Field              Description
[0]     1     Packet Type        0x01 (COMMAND)
[1..2]  2     Opcode             LE ushort. OGF in bits[15..10], OCF in bits[9..0]. Default=0xFC00
[3]     1     Param Length        8 + actual_param_length
[4..5]  2     SonyKeyID          LE ushort. Always = 0xC396 (50070)
[6]     1     Address             [7..4]=DstID, [3..0]=SrcID
[7]     1     Event ID            See EVENT_ID table
[8]     1     Event Type          See EVENT_TYPE table
[9..10] 2     Transaction ID     LE ushort, incremented per event type
[11..N] N     Parameter           Payload (max 50 bytes per fragment)
[11+N]  1     Checksum            Sum of bytes[4..10+N] & 0xFF
```

**Total: 12 + N bytes**

#### 4.1.2 Event Packet (Headset -> PC)

```
Offset  Size  Field              Description
[0]     1     Packet Type        0x04 (EVENT)
[1]     1     Event Code         Always 0xFF
[2]     1     Param Length        8 + actual_param_length
[3]     1     Dummy              Always 0x00
[4..5]  2     SonyKeyID          LE ushort. Always = 0xC396 (50070)
[6]     1     Address             [7..4]=DstID, [3..0]=SrcID
[7]     1     Event ID
[8]     1     Event Type
[9..10] 2     Transaction ID     LE ushort
[11..N] N     Parameter           Payload
[11+N]  1     Checksum            Sum of bytes[3..10+N] & 0xFF
```

**Checksum difference:** Command checksum starts at offset 4, Event checksum starts at offset 3.

#### 4.1.3 Checksum Algorithm

```
checksum = 0
for i from headerLength to (totalLength - 2):
    checksum += packet[i]
return checksum & 0xFF
```

- Command: `headerLength = 4` (sum from SonyKeyID through end of param)
- Event: `headerLength = 3` (sum from Dummy through end of param)

### 4.2 Address Field

```
Byte 6: [7..4] = DstID, [3..0] = SrcID

Values:
  PC = 1
  TX = 2  (dongle/transmitter)
  RX = 4  (headset/receiver)
```

### 4.3 Event Type Flags

```
GET        = 0x01   (request data)
SET        = 0x02   (write data)
RET        = 0x10   (return/response)
NTFY       = 0x20   (notification)
NTFY_ACTIVE= 0xA0   (active notification)
```

### 4.4 EVENT_ID Table

| ID | Name | Direction | Settable |
|----|------|-----------|----------|
| 1 | 2GHZ_CONNECT_STATUS | TX | No |
| 2 | MODEL_INFO | RX | No |
| 3 | FW_VERSION | RX | No |
| 4 | BATTERY_INFO | RX | No |
| 5 | HOST_SELECT_SWITCH | RX | Yes |
| 6 | ALL_FUNCTION_SETTINGS_PART1 | RX | No |
| 7 | ALL_FUNCTION_SETTINGS_PART2 | RX | No |
| 8 | ALL_FUNCTION_SETTINGS_PART3 | RX | No |
| 9 | BOOT_STATUS | TX | No |
| 33 | HEADPHONE_VOLUME | RX | Yes |
| 34 | GAME_CHAT_MIX_BALANCE | RX | Yes |
| 35 | SIDETONE_VOLUME | RX | Yes |
| 36 | MIC_VOLUME | RX | Yes |
| 37 | SURROUND_SETTING | RX | Yes |
| 65 | AMB_SETTING | RX | Yes |
| 66 | NC_TOGGLE_SETTING | RX | Yes |
| 67 | NC_STARTUP_MODE | RX | Yes |
| 97 | BT_STATUS | RX | No |
| 98 | BT_SOUND_QUALITY | RX | Yes |
| 99 | BT_STARTUP_MODE | RX | Yes |
| 129 | AUTO_POWER_OFF | RX | Yes |
| 130 | LED_SETTING | RX | Yes |
| 131 | VP_LANGUAGE | RX | Yes |
| 132 | GUIDANCE | RX | Yes |
| 133 | CONNECTION_DESTINATION_MODE | RX | Yes |
| 134 | WEARING_DETECTOR_CAPABILITY | RX | No |
| 135 | WEARING_DETECTOR_STATUS | RX | Yes |
| 136 | WEARING_DETECTOR_PARAM | RX | Yes |
| 137 | WEARING_DETECTOR_EXTENDED_PARAM | RX | Yes |
| 138 | IMPRESS_INFO | RX | No |
| 139 | IMPRESS_DATA | RX | No |
| 140 | ASSIGNABLE_SETTINGS_CAPABILITY | RX | No |
| 141 | ASSIGNABLE_SETTINGS_PARAM | RX | Yes |
| 142 | INCOMING_PERMISSION | RX | Yes |
| 143 | MIC_ATTACHED_STATUS | RX | No |
| 160 | UPDATE_FIRMWARE | - | FW update |

### 4.5 Fragmentation

Parameters larger than 50 bytes are split into multiple fragments:
- Each fragment carries up to **50 bytes** of parameter data
- All fragments share the same **TransactionID**
- Final fragment has `Param.Length < 50`
- `ParamLength` field = `8 + fragment_param_length`

### 4.6 Key Parameter Wire Formats

#### 4.6.1 MODEL_INFO (Event ID 2)

**Standard headset (6 bytes):**
```
[0] Model ID (0=H9, 1=H7, 2=H3, 3=H5, 4=Buds, 5=H10, 6=E9, 7=H6Air)
[1] Destination
[2] Serial Number (low byte)
[3] Serial Number (high byte)
[4] Model Color
[5] Model Status
```

**INZONE Buds / GTW (32 bytes):**
```
[0..5]   Same as above
[6..13]  Dongle Serial Number (8 bytes ASCII)
[14..21] Left Earpiece Serial (8 bytes ASCII)
[22..29] Right Earpiece Serial (8 bytes ASCII)
[30]     Left Color ID
[31]     Right Color ID
```

#### 4.6.2 FW_VERSION (Event ID 3)

**Standard (8 bytes):**
```
[0..3] RX (headset) version:
  [0] = major
  [1..2] = minor (12-bit) + build high nibble
  [3] = build low nibble shifted
[4..7] TX (dongle) version (same encoding)
```

**GTW/Buds (12 bytes):** Adds [8..11] for right earpiece version.

#### 4.6.3 BATTERY_INFO (Event ID 4)

**Standard (2 bytes):**
```
[0] Battery Status (0=Discharging, 1=Charging)
[1] Remaining Battery (0-100%)
```

**GTW/Buds (6 bytes):**
```
[0] Left Battery Status
[1] Left Remaining Battery
[2] Right Battery Status
[3] Right Remaining Battery
[4] Case Battery Status
[5] Case Remaining Battery
```

#### 4.6.4 HEADPHONE_VOLUME (Event ID 33) - 3 bytes

```
[0] Mute (0/1)
[1] Volume Value (raw)
[2] Volume Percent (0-100)
```

#### 4.6.5 GAME_CHAT_MIX_BALANCE (Event ID 34) - 1 byte

```
[0] Mix Balance value
```

#### 4.6.6 SIDETONE_VOLUME (Event ID 35) - 2 bytes

```
[0] Sidetone Value (raw)
[1] Sidetone Percent (0-100)
```

#### 4.6.7 MIC_VOLUME (Event ID 36) - 3 bytes

```
[0] Mic Mute (0/1)
[1] Mic Volume Value (raw)
[2] Mic Volume Percent (0-100)
```

#### 4.6.8 AMB_SETTING (Event ID 65) - 4 bytes

```
[0] NC Setting (noise cancellation mode)
[1] Ambient Volume Value (raw)
[2] Ambient Volume Percent (0-100)
[3] Voice Focus (0/1)
```

#### 4.6.9 ALL_FUNCTION_SETTINGS_PART1 (Event ID 6)

**Standard (9 bytes):**
```
[0] Model ID
[1] Battery Status
[2] Remaining Battery
[3] Headphone Mute
[4] Headphone Volume Value
[5] Headphone Volume Percent
[6] Game/Chat Mix Balance
[7] Sidetone Volume Value
[8] Sidetone Volume Percent
```

**GTW/Buds (13 bytes):** Adds right earpiece + case battery (4 extra bytes).

#### 4.6.10 ALL_FUNCTION_SETTINGS_PART2 (Event ID 7) - 10 bytes

```
[0] Mic Mute
[1] Mic Volume Value
[2] Mic Volume Percent
[3] NC Setting
[4] Ambient Volume Value
[5] Ambient Volume Percent
[6] Voice Focus
[7] Off Enable
[8] NC Enable
[9] AMB Enable
```

#### 4.6.11 ALL_FUNCTION_SETTINGS_PART3 (Event ID 8) - Variable

```
[0] NC Startup Mode
[1] BT Status
[2] BT Connection Status
[3] BT Sound Quality
[4] BT Startup Mode
[5] Auto Power Off Time
[6] LED Setting
[7] VP Language (model > H7 only)
[8] Guidance (model != H5 only)
[9] Incoming Permission (GTW/Buds only)
[10] Mic Attached Status (H10 only)
```

### 4.7 Headset Enumeration Sequence

```
1.  _2GHZ_CONNECT_STATUS (GET)
2.  MODEL_INFO (GET)
3.  BATTERY_INFO (GET)
4.  [H9/H7 only]: TX_GET_BOOTMODE (OTA opcode 0xFC00)
    [H9/H7 only]: RX_SEND_USER_DATA (OTA opcode 0xFFB3)
    [Others]:     BOOT_STATUS (GET)
5.  [GTW only]: CONNECTION_DESTINATION_MODE (GET)
6.  [GTW only]: WEARING_STATUS_DETECTOR_STATUS (SET + NTFY)
7.  [GTW only]: WEARING_DETECTOR_CAPABILITY (GET)
8.  [GTW only]: ASSIGNABLE_SETTINGS_CAPABILITY (GET)
9.  FW_VERSION (GET)
10. ALL_FUNCTION_SETTINGS_PART1 (GET)
11. ALL_FUNCTION_SETTINGS_PART2 (GET)
12. ALL_FUNCTION_SETTINGS_PART3 (GET)
13. [GTW only]: ASSIGNABLE_SETTINGS_PARAM (GET)
-> Enumeration complete
```

### 4.8 OTA (Firmware Update) Packets

**OTA Command Packet** (simpler format, no SonyKeyID):
```
[0]     1     Packet Type = 0x01
[1..2]  2     OpCode LE (0xFC00=TX_GET_BOOTMODE, 0xFFB3=RX_SEND_USER_DATA)
[3]     1     Param Length
[4..N]  N     Parameter
```

**OTA Event Packet:**
```
[0]     1     Packet Type = 0x04
[1]     1     Event Code = 0x0E (HCI Command Complete)
[2]     1     Param Length
[3]     1     Num HCI Command Packets = 1
[4..5]  2     OpCode LE
[6]     1     HCI Status (0=success)
[7..N]  N     Parameter
```

---

## 5. Protocol C: Airoha AB1565 Relay

The `AirohaHidCoreLib.dll` (native C++ library) handles communication with earbuds using the Airoha AB1565 chipset via a relay protocol.

### 5.1 Product Categories

```
AB1565              - Standard Airoha AB1565
AB1565_dual_chip    - Dual chip configuration
AB1565_LEA_DONGLE   - LE Audio dongle
AB1565_DUAL_DONGLE  - Dual-mode dongle
```

### 5.2 Transport

Uses the same Windows HID API (`HidD_SetOutputReport` / `HidD_GetInputReport`) with a `HidTransport` layer that supports:
- **Local HID** - direct communication with the device
- **Dongle relay** - relay packets through the USB dongle to the earbuds

### 5.3 Relay Packet Structure

The relay protocol wraps HID packets with routing information:
- `cmd recipient` - target device identifier
- `recipientCount` - number of recipients
- Relay response validation: checks response length, type, and race ID

### 5.4 FOTA (Firmware Over-The-Air)

`AirohaRelayFotaControl` handles firmware updates for earbud firmware via the relay protocol, with priority levels: None, Low, Middle, High.

---

## 6. Firmware Update

### 6.1 Update Information

Firmware update info is fetched from Sony servers:
```
https://info.update.sony.net/HP002/{serviceId}/info/info.xml
```

**Service IDs:**
| Model | Service ID |
|-------|-----------|
| H9/H7 | MDRID295900 |
| H5 | MDRID297600 |
| Buds | MDRID297701 |
| H10 | MDRID298700 |

### 6.2 Update Libraries

| DLL | Purpose |
|-----|---------|
| `FwUpdate_HeadSet.dll` | Headset firmware update |
| `HidFwUpdate_Headset.dll` | HID-based headset FW update |
| `HidFwUpdate_HeadSet_HDX2987.dll` | H10-specific FW update |
| `KeyboardMouseFwUpdateDll.dll` | Keyboard/mouse FW update |
| `EarbudsFwUpdate.dll` | Earbuds FW update |
| `FwUpdateSharedLib.dll` | Shared FW update utilities |
| `FwUpdate_Monitor.dll` | Monitor FW update |

### 6.3 Mouse/Keyboard Dongle Update

The mouse dongle firmware can be updated independently:
- `MOUSE_DONGLE_BOOTLOADER` (PID `0x0FB2`) - dongle in bootloader mode
- `MOUSE_MOUSE_BOOTLOADER` (PID `0x0FB1`) - mouse in bootloader mode
- `KEYBOARD_BOOTLOADER` (PID `0x0FB3`) - keyboard in bootloader mode

### 6.4 Hub Firmware Update

The INZONE Hub itself has firmware that can be updated via:
- `GLHubUpdateToolCli/` directory containing `GLHubUpdateToolCli.exe`
- Hub firmware files in `GLHubUpdateToolCli/HubFw/`
- Driver files in `GLHubUpdateToolCli/HubDriver/`

---

## 7. Appendix: Enum Reference Tables

### 7.1 COMMUNICATION_PROTOCOL

```
COM                      = 0x00001   Serial port
HID                      = 0x00002   HID report
OTA                      = 0x00004   Over-the-air
H3                       = 0x00008   H3 wired protocol
PS5                      = 0x00010   PS5 connection
HEADSET_MASK             = 0x000FF
MOUSE_DONGLE             = 0x00100   Mouse via dongle
MOUSE_MOUSE              = 0x00200   Mouse via wired USB
MOUSE_DONGLE_BOOTLOADER  = 0x00400
MOUSE_MOUSE_BOOTLOADER   = 0x00800
MOUSE_MASK               = 0x0FF00
KEYBOARD                 = 0x10000   Keyboard main interface
KEYBOARD_VOLUME          = 0x20000   Keyboard volume knob
KEYBOARD_BOOTLOADER      = 0x40000
KEYBOARD_MASK            = 0xFF0000
```

### 7.2 MODEL_ID (Headsets)

```
0 = HDX_2959 (INZONE H9)
1 = HDX_2960 (INZONE H7)
2 = HDX_2961 (INZONE H3)
3 = GH_M2    (INZONE H5 / YY2976)
4 = GTW      (INZONE Buds / YY2977)
5 = GH_H2    (INZONE H10 / YY2987)
6 = GH_IE    (INZONE E9 / YY2989)
7 = GH_OB    (INZONE H6 Air / YY2990)
```

### 7.3 PID (Mouse & Keyboard)

```
4014 (0x0FAE) = HDX_2991_MOUSE           (INZONE Mouse-A wired)
4015 (0x0FAF) = HDX_2991_DONGLE          (INZONE Mouse-A dongle)
4016 (0x0FB0) = HDX_2992                 (INZONE KBD-H75)
4017 (0x0FB1) = HDX_2991_MOUSE_BOOTLOADER
4018 (0x0FB2) = HDX_2991_DONGLE_BOOTLOADER
4019 (0x0FB3) = HDX_2992_BOOTLOADER
```

### 7.4 PROFILE

```
1 = Profile One
2 = Profile Two
3 = Profile Three
4 = Profile Four
```

### 7.5 PROFILE_ENABLE (Flags)

```
Bit 0 = Profile 1
Bit 1 = Profile 2
Bit 2 = Profile 3
Bit 3 = Profile 4
```

### 7.6 RF_STATUS

```
0xCC (204) = Connected
0xDD (221) = Disconnected
```

### 7.7 RECEIVE_STATUS

```
0xACDC (44268) = OK
0xFAEC (64236) = NG (error)
```

### 7.8 ERROR_CODE

```
0  = COMMAND_CONTENT_ERROR
1  = KEY_CONTENT_ERROR
2  = INDEX_CONTENT_ERROR
3  = LENGTH_CONTENT_ERROR
4  = DATA_CONTENT_ERROR
10 = SEND_COMMAND_AGAIN
```

### 7.9 HCI_PACKET_TYPE

```
1 = COMMAND
4 = EVENT
```

### 7.10 ADDRESS

```
1 = PC
2 = TX (dongle)
4 = RX (headset)
```

### 7.11 SPECIAL_KEY (Keyboard)

```
164 = FN
240 = FN_LOCK
241 = PERFORMANCE_MODE
242 = PROFILE_CYCLE
243 = LIGHTING_CONFIG_MODE
244 = BACKLIGHT_BRIGHTNESS_DOWN
245 = BACKLIGHT_BRIGHTNESS_UP
246-254 = INZONEHUB_CONTROL1-9
```

### 7.12 ILLUMINATION_MODES

```
0 = NORMAL
1 = SETTING
2 = SAVED
3 = CANCELLED
```

### 7.13 CALIBRATION

```
0x55 (85)  = START
0xAA (170) = STOP
0x5A (90)  = ABORT
```

---

*Document generated from decompilation of Sony INZONE Hub. For educational and interoperability research purposes only.*
