# INZONE Asset Extraction Guide

這份文件說明怎麼從官方 `INZONEHub_Setup_1.0.19.0.exe` 自動拆出 setup 內的 `INZONEHub.dll`，再把 Linux GUI 會用到的素材抽出來，並整理型號和圖示的對應方式。

整個流程只能使用 Linux 原生工具，不使用也不需要 `wine`。

## 檔案

- `extract_inzone_assets.sh`
- `extract_installshield_payload.go`
- `INZONEHub_Setup_1.0.19.0.exe`

## 需求

需要 `ilspycmd`、`go`、`7z`。

對 `INZONEHub_Setup_1.0.19.0.exe`，腳本會用 Go 找出 Sony 包裝器裡的 nested PE，再取出 MSI/OLE resource，接著用 Linux 原生 `7z` 解出 `Data1.cab` stream，最後從 CAB 取出 setup 內的 `inzonehub.dll` 和 `app_notify_icon.png`。

不支援 `wine`，也不應在 PKGBUILD 或 CI 裡依賴 `wine`。

如果你已經用過前面的環境，通常只要：

```bash
export PATH="$PATH:/home/patyhank/.dotnet/tools"
```

確認：

```bash
ilspycmd --help
```

installer/archive 輸入另需：

```bash
go version
7z i
```

Go helper 只解析/切出 PE resource，`7z` 只處理 MSI/OLE 和 CAB stream；不會執行 Windows 安裝器。

## 使用方式

主要用法是直接指定官方 setup：

```bash
chmod +x ./extract_inzone_assets.sh
./extract_inzone_assets.sh ./INZONEHub_Setup_1.0.19.0.exe ./inzone-assets
```

也可以指定其他輸出目錄：

```bash
./extract_inzone_assets.sh ./INZONEHub_Setup_1.0.19.0.exe ./extracted-assets
```

已解包目錄或 DLL 路徑只保留給除錯用，不是打包流程的前置需求：

```bash
./extract_inzone_assets.sh ./unpacked-installer-dir ./extracted-assets
./extract_inzone_assets.sh ./INZONEHub.dll ./extracted-assets
```

`INZONEHub_Setup_1.0.19.0.exe` 外層是 Sony Packaging Tool。實際素材來源在 nested PE 的 MSI/OLE resource `BINARY/#4000/#1041` 裡，裡面有 `Data1.cab` stream。`extract_installshield_payload.go` 會自動走這條路徑，不需要手動準備 `INZONEHub.dll`。

## 輸出內容

腳本會產生：

- `inzone-assets/raw/resources/`：從 DLL 內嵌資源解出的圖檔
- `inzone-assets/resources/`：安裝目錄中本來就存在的零散圖檔
- `inzone-assets/model_icon_map.json`：型號對應 icon/material 的 mapping
- `inzone-assets/README.txt`

## GUI 直接可用的主圖示

INZONE Hub 的主裝置圖示不是每個型號各一張，而是按品類共用：

| IconName | 檔案 |
|---|---|
| `MENU_HEADSET` | `menu_headset.png` |
| `MENU_BUDS` | `menu_buds.png` |
| `MENU_MOUSE` | `menu_mouse.png` |
| `MENU_KEYBORD` | `menu_keybord.png` |
| `MENU_IE` | `menu_ie.png` |

在原始程式中，主 icon 是這樣組的：

```csharp
Icon = new BitmapImage(new Uri("/Resources/" + iconName + ".png", UriKind.Relative));
```

所以 Linux GUI 最穩的做法是直接沿用這組分類 icon。

## 型號對應

### Headset / Buds

| MODEL_ID | Internal | Marketing Name | Icon |
|---|---|---|---|
| `HDX_2959` | `HDX-2959` | `INZONE H9` | `menu_headset.png` |
| `HDX_2960` | `HDX-2960` | `INZONE H7` | `menu_headset.png` |
| `HDX_2961` | `HDX-2961` | `INZONE H3` | `menu_headset.png` |
| `GH_M2` | `YY2976` | `INZONE H5` | `menu_headset.png` |
| `GTW` | `YY2977` | `INZONE Buds` | `menu_buds.png` |
| `GH_H2` | `YY2987` | `INZONE H9 II` | `menu_headset.png` |
| `GH_IE` | `YY2989` | `INZONE E9` | `menu_ie.png` |
| `GH_OB` | `YY2990` | `INZONE H6 Air` | `menu_headset.png` |

### Mouse / Keyboard

| Model | Marketing Name | Icon |
|---|---|---|
| `YY2991` | `INZONE Mouse-A` | `menu_mouse.png` |
| `YY2992` | `INZONE KBD-H75` | `menu_keybord.png` |

## PID 對應

| PID | Device |
|---|---|
| `0x0E53`, `0x0E4C`, `0x0E61` | INZONE H9 |
| `0x0DFD`, `0x0E47` | INZONE H3 |
| `0x0EBF`, `0x0EC0` | INZONE H5 |
| `0x0EC2`, `0x0EC3` | INZONE Buds |
| `0x0FA8`, `0x0FA9` | INZONE H9 II |
| `0x0F80`, `0x0F81` | INZONE E9 |
| `0x0FC0`, `0x0FC1` | INZONE H6 Air |
| `0x0FAE`, `0x0FAF`, `0x0FB1`, `0x0FB2` | INZONE Mouse-A |
| `0x0FB0`, `0x0FB3` | INZONE KBD-H75 |

## 值得優先抽出的素材

### 主類別 icon

- `menu_headset.png`
- `menu_buds.png`
- `menu_mouse.png`
- `menu_keybord.png`
- `menu_ie.png`

### Buds 相關

- `bud_s.png`
- `ear_l.png`
- `ear_r.png`
- `ear_l_mid.png`
- `ear_r_mid.png`
- `tw_case_mini_s.png`
- `status_left_mini_s.png`
- `status_right_mini_s.png`

### 電量 / 狀態

- `battery_disconnect_s.png`
- `battery_charging_s.png`
- `battery_1_s.png`
- `battery_1_2_s.png`
- `battery_2_s.png`
- `battery_2_2_s.png`
- `battery_3_s.png`
- `battery_3_2_s.png`
- `battery_4_s.png`
- `battery_4_2_s.png`
- `bluetooth_on_s.png`
- `bluetooth_connected_s.png`
- `bluetooth_pairing_s.png`
- `bluetooth_off_s.png`
- `mic_on_mini_s.png`
- `mic_mute_mini_s.png`
- `mic_mute_mini_disable_s.png`

### Mouse 相關

- `mouse_top.png`
- `mouse_top_fnc.png`
- `mouse_wired_s.png`
- `mouse_dongle_s.png`
- `logo_fnc_o.png`

## Linux GUI 建議

如果你只是要先把 GUI 做出來，建議先這樣用：

1. 主設備列表直接用 `model_icon_map.json` 的 `icon_file`
2. Buds 詳情頁用：
   - `bud_s.png`
   - `ear_l.png`
   - `ear_r.png`
   - `tw_case_mini_s.png`
3. 電量顯示用 `battery_*_s.png`
4. 藍牙狀態用 `bluetooth_*_s.png`
5. 滑鼠頁面用 `mouse_top.png` / `mouse_top_fnc.png`

這樣可以先快速做出一版接近官方風格的 UI。

## 備註

- 這些素材大多是嵌在 `INZONEHub.dll` 裡，不是安裝目錄上的普通檔案。
- 除了主 icon 外，很多圖是狀態圖示，不是特定型號專屬大圖。
- 官方程式對不同 headset 型號主要是共用 `menu_headset.png`，不是每個 headset 一張獨立主圖。
- 後續 PKGBUILD / AppImage 打包流程應以官方 setup 和這個腳本產生 assets；不要要求預先放入 `INZONEHub.dll`，也不要加入 `wine` 步驟。
