# hid-inzone-battery

Experimental DKMS HID module for Sony INZONE battery reporting through Linux `power_supply`.

Expected outputs:

```text
/sys/class/power_supply/inzone_battery_headset_*
/sys/class/power_supply/inzone_battery_left_*
/sys/class/power_supply/inzone_battery_right_*
/sys/class/power_supply/inzone_battery_case_*
```

UPower should then expose matching devices under:

```text
/org/freedesktop/UPower/devices/
```

## Current Scope

- HID HCI headsets:
  - INZONE H9 / H7 family: `0x0E53`, `0x0E4C`, `0x0E61`, `0x0DFD`, `0x0E47`
  - INZONE H5: `0x0EBF`
  - INZONE H10: `0x0FA8`
  - INZONE E9: `0x0F80`, `0x0F81`
  - INZONE H6 Air: `0x0FC0`, `0x0FC1`
- INZONE Buds / GTW PIDs: `0x0EC2`, `0x0EC3`
- Buds exports separate left / right / case batteries
- Protocol A keyboard / mouse battery query:
  - INZONE Mouse-A: `0x0FAE`, `0x0FAF`
  - INZONE KBD-H75: `0x0FB0`

Bootloader PIDs are intentionally excluded because they do not expose useful
runtime battery state.

Some H9 / H7 modes expose their main userspace control path through USB VCOM /
CDC ACM. This module only covers HID battery reporting paths that can be
handled by the Linux HID subsystem.

## Install

From this directory:

```bash
sudo dkms install .
sudo modprobe hid_inzone_battery
```

For clang-built kernels such as CachyOS, DKMS may need:

```bash
sudo dkms build -m hid-inzone-battery -v 0.1.12 -- -j"$(nproc)" LLVM=1
sudo dkms install -m hid-inzone-battery -v 0.1.12
sudo modprobe hid_inzone_battery
```

Install udev rules if userspace access through hidraw is also needed:

```bash
sudo install -Dm644 99-inzone-battery.rules /etc/udev/rules.d/99-inzone-battery.rules
sudo udevadm control --reload-rules
sudo udevadm trigger
```
