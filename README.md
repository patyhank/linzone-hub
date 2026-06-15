# LINZONE Hub

LINZONE Hub is an experimental Linux controller for Sony INZONE devices.

It provides a Wails/Svelte GUI, a Go CLI, Arch Linux packaging, and an optional DKMS kernel module that exposes supported INZONE battery data through Linux `power_supply` so UPower and desktop shells can see it.

> This project is community-built and is not affiliated with Sony.

## Features

- Desktop GUI for Sony INZONE device status and controls.
- CLI tooling for probing and debugging INZONE HID / serial protocols.
- INZONE Buds battery support for left earbud, right earbud, and case.
- INZONE H9 / H7 USB VCOM serial support for supported H9/H7 modes.
- Experimental DKMS `power_supply` driver for UPower integration.
- Arch Linux `PKGBUILD` files for both the app and DKMS module.
- Extracted INZONE Hub assets for icons and device artwork.

## Supported Hardware

Currently tested hardware:

- INZONE Buds / GTW (`054c:0ec2`)
- INZONE H9 / H7 USB VCOM modes:
  - `054c:0e53`
  - `054c:0e4c`
  - `054c:0e61`

The codebase also contains mappings for additional INZONE headset, mouse, and keyboard PIDs, but untested devices may need protocol tuning.

The DKMS battery module attempts broader experimental coverage for known INZONE runtime PIDs:

- HID HCI headsets: H9 / H7 family, H5, H10, E9, and H6 Air.
- INZONE Buds / GTW, including separate left / right / case battery supplies.
- Protocol A keyboard / mouse battery level reporting for INZONE Mouse-A and KBD-H75.

Bootloader PIDs are intentionally excluded. Some H9 / H7 modes expose their main control path through USB VCOM / CDC ACM, so the kernel module only covers battery data available through the HID path.

## Battery / UPower Integration

Desktop battery pages usually read device batteries from UPower. UPower does not offer a normal userspace D-Bus API for arbitrary apps to inject `/org/freedesktop/UPower/devices/*` batteries.

For USB / 2.4 GHz INZONE devices, LINZONE Hub uses a DKMS HID module to register Linux `power_supply` devices under:

```text
/sys/class/power_supply/inzone_battery_*
```

UPower then exposes those as:

```text
/org/freedesktop/UPower/devices/
```

See:

- [docs/UPower_Integration.md](docs/UPower_Integration.md)
- [kernel/hid-inzone-battery-dkms/README.md](kernel/hid-inzone-battery-dkms/README.md)

## Install on Arch Linux

Build the GUI app package:

```bash
cd packaging/arch
makepkg -si
```

Build the DKMS battery module package:

```bash
cd packaging/arch
makepkg -si -p PKGBUILD.dkms
```

Then load the module:

```bash
sudo modprobe hid_inzone_battery
```

If device permissions are missing, reload udev rules or use the GUI repair action:

```bash
sudo udevadm control --reload-rules
sudo udevadm trigger
```

## Build From Source

Requirements:

- Go
- Node.js / npm
- Wails v3 CLI
- GTK / WebKitGTK development libraries
- hidapi / libusb development libraries

Install frontend dependencies:

```bash
npm --prefix frontend ci
```

Run checks:

```bash
npm --prefix frontend run check
go test ./...
```

Build frontend and Linux binary:

```bash
npm --prefix frontend run build
go build -tags production -trimpath -buildvcs=false -ldflags="-w -s" -o bin/linzone-hub
```

Or use Task:

```bash
task build
```

## CLI Usage

List devices:

```bash
go run ./cmd/inzone-cli list
```

Read device info:

```bash
go run ./cmd/inzone-cli info 0
```

Debug H9/H7 USB VCOM serial traffic:

```bash
go run ./cmd/inzone-cli serial-debug 0
```

## Development Notes

- INZONE Buds use an Airoha / GTW relay path and expose left / right / case battery data.
- Some H9/H7 modes expose control through USB VCOM / CDC ACM, not the visible HID interface.
- The DKMS module is intentionally separate from the userspace app because kernel `power_supply` devices are the practical path for UPower integration.
- Generated Wails bindings live under `frontend/bindings/`.

## CI

GitHub Actions runs:

- Go tests
- Svelte checks
- frontend production build
- Linux production binary build
- Arch package smoke tests for `PKGBUILD` and `PKGBUILD.dkms`

## License

The userspace application and repository content are licensed under the MIT License. See [LICENSE](LICENSE).

The DKMS kernel module under `kernel/hid-inzone-battery-dkms/` is licensed as `GPL-2.0-only`, as indicated by its SPDX header and Arch package metadata.
