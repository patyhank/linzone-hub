# UPower Integration Notes

LINZONE Hub reads device battery data from Sony HID / KBM protocols for in-app display.

To make the same battery appear under:

```text
/org/freedesktop/UPower/devices/
```

the data must be exposed to the UPower daemon through a system-level source, normally a kernel `power_supply` device under:

```text
/sys/class/power_supply/
```

UPower does not provide a general D-Bus provider API for normal desktop apps to inject arbitrary device batteries. A Wails GUI process should not try to own or spoof `org.freedesktop.UPower`; that name belongs to the system UPower daemon.

Practical options:

1. Keep battery display in LINZONE Hub by reading HID directly.
2. Add a HID kernel driver that publishes LINZONE HID battery as a real `power_supply`, then UPower will export it under `/org/freedesktop/UPower/devices/`.
3. For Bluetooth-only devices, BlueZ Battery Provider can publish `org.bluez.Battery1`, but that is not the UPower devices path and is not suitable for 2.4G dongle / USB HID devices.

This repository contains an experimental HID kernel module at:

```text
kernel/hid-inzone-battery-dkms/
```

It currently targets the devices available for testing:

- INZONE H9 PS5 / dongle
- INZONE Buds / GTW, with separate left / right / case power supplies
- Sony INZONE HCI `BATTERY_INFO`
- kernel `power_supply` registration

Expected result after loading the module:

```text
/sys/class/power_supply/inzone_battery_*
```

The kernel module reports these supplies with `Scope=Device`, and the udev rules set `UPOWER_BATTERY_TYPE` to headset / mouse / keyboard where applicable. This avoids presenting INZONE peripherals as internal/system batteries. UPower should then create a matching D-Bus object under `/org/freedesktop/UPower/devices/`.
