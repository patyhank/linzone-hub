# LINZONE Hub

LINZONE Hub 是給 Sony INZONE 裝置使用的實驗性 Linux 控制工具。

專案包含 Wails/Svelte GUI、Go CLI、Arch Linux 打包檔，以及可選的 DKMS kernel module。DKMS 模組會把支援的 INZONE 電量資料註冊成 Linux `power_supply`，讓 UPower 和桌面環境能顯示電量。

> 這是社群專案，與 Sony 官方無關。

## 功能

- Sony INZONE 裝置狀態與控制用桌面 GUI。
- INZONE HID / serial protocol 的 CLI 探測與除錯工具。
- INZONE Buds 左耳、右耳、充電盒電量支援。
- INZONE H9 / H7 USB VCOM serial 模式支援。
- 實驗性的 DKMS `power_supply` driver，提供 UPower 整合。
- Arch Linux app / DKMS module `PKGBUILD`。
- 從 INZONE Hub 擷取出的 icon 與裝置圖片資源。

## 支援硬體

目前已實測：

- INZONE Buds / GTW (`054c:0ec2`)
- INZONE H9 / H7 USB VCOM 模式：
  - `054c:0e53`
  - `054c:0e4c`
  - `054c:0e61`

程式內也有其他 INZONE 耳機、滑鼠、鍵盤 PID 對應，但未測裝置可能還需要調整 protocol。

DKMS 電量模組目前會對已知的 INZONE runtime PID 嘗試較完整的實驗性支援：

- HID HCI 耳機：H9 / H7 系列、H5、H10、E9、H6 Air。
- INZONE Buds / GTW，包含左耳、右耳、充電盒三個獨立電量。
- Protocol A 鍵盤 / 滑鼠電量回報：INZONE Mouse-A 與 KBD-H75。

Bootloader PID 會刻意排除。部分 H9 / H7 模式的主要控制通道是 USB VCOM / CDC ACM，所以 kernel module 只負責 HID path 能取得的電量資料。

## 電量與 UPower 整合

Linux 桌面的電量頁面通常讀 UPower。UPower 沒有提供一般 userspace app 可以任意注入 `/org/freedesktop/UPower/devices/*` 電池的 D-Bus API。

對 USB / 2.4 GHz INZONE 裝置，LINZONE Hub 透過 DKMS HID module 註冊 Linux `power_supply`：

```text
/sys/class/power_supply/inzone_battery_*
```

接著 UPower 會把它們匯出成：

```text
/org/freedesktop/UPower/devices/
```

相關文件：

- [docs/UPower_Integration.md](docs/UPower_Integration.md)
- [kernel/hid-inzone-battery-dkms/README.md](kernel/hid-inzone-battery-dkms/README.md)

## Arch Linux 安裝

建置並安裝 GUI app：

```bash
cd packaging/arch
makepkg -si
```

建置並安裝 DKMS 電量模組：

```bash
cd packaging/arch
makepkg -si -p PKGBUILD.dkms
```

載入模組：

```bash
sudo modprobe hid_inzone_battery
```

如果缺少裝置權限，可以重新載入 udev rules，或使用 GUI 裡的權限修復功能：

```bash
sudo udevadm control --reload-rules
sudo udevadm trigger
```

## 從原始碼建置

需求：

- Go
- Node.js / npm
- Wails v3 CLI
- GTK / WebKitGTK development libraries
- hidapi / libusb development libraries

安裝 frontend dependencies：

```bash
npm --prefix frontend ci
```

執行檢查：

```bash
npm --prefix frontend run check
go test ./...
```

建置 frontend 和 Linux binary：

```bash
npm --prefix frontend run build
go build -tags production -trimpath -buildvcs=false -ldflags="-w -s" -o bin/linzone-hub
```

或使用 Task：

```bash
task build
```

## CLI 使用

列出裝置：

```bash
go run ./cmd/inzone-cli list
```

讀取裝置資訊：

```bash
go run ./cmd/inzone-cli info 0
```

除錯 H9/H7 USB VCOM serial traffic：

```bash
go run ./cmd/inzone-cli serial-debug 0
```

## 開發筆記

- INZONE Buds 使用 Airoha / GTW relay path，並回報左耳、右耳、充電盒電量。
- 部分 H9/H7 模式的主要控制通道是 USB VCOM / CDC ACM，不是畫面上看到的 HID interface。
- DKMS module 和 userspace app 分開，因為 kernel `power_supply` 是目前整合 UPower 的實際可行路徑。
- Wails 產生的 bindings 位於 `frontend/bindings/`。

## CI

GitHub Actions 會執行：

- Go tests
- Svelte checks
- frontend production build
- Linux production binary build
- Arch package smoke tests，包含 `PKGBUILD` 和 `PKGBUILD.dkms`

## 授權

Userspace app 與 repository 內容使用 MIT License。詳見 [LICENSE](LICENSE)。

`kernel/hid-inzone-battery-dkms/` 內的 DKMS kernel module 使用 `GPL-2.0-only`，以檔案內 SPDX header 與 Arch package metadata 為準。
