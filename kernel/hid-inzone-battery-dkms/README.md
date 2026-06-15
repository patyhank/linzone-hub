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

- INZONE H9 PS5 / dongle PIDs: `0x0E4C`, `0x0E61`
- INZONE Buds / GTW PIDs: `0x0EC2`, `0x0EC3`
- Buds exports separate left / right / case batteries

The original H9 wired PID `0x0E53` is not enabled here because the technical notes identify it as a legacy COM/serial path, not this HID battery path.

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
