#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./extract_inzone_assets.sh [DLL_PATH] [OUTPUT_DIR]

Examples:
  ./extract_inzone_assets.sh
  ./extract_inzone_assets.sh ./INZONEHub.dll ./extracted-assets

Notes:
  - Requires ilspycmd in PATH.
  - Extracts embedded image-like resources from INZONEHub.dll.
  - Also writes a model/icon mapping JSON for Linux GUI use.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

DLL_PATH="${1:-./INZONEHub.dll}"
OUTPUT_DIR="${2:-./inzone-assets}"
RESOURCE_PREFIX="INZONEHub.g.resources/"
TMP_DIR="$(mktemp -d)"
LIST_FILE="$TMP_DIR/resources.txt"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

if ! command -v ilspycmd >/dev/null 2>&1; then
  echo "error: ilspycmd not found in PATH" >&2
  echo "hint: export PATH=\"$PATH:/home/patyhank/.dotnet/tools\"" >&2
  exit 1
fi

if [[ ! -f "$DLL_PATH" ]]; then
  echo "error: DLL not found: $DLL_PATH" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR/raw" "$OUTPUT_DIR/resources"

echo "[*] Listing embedded resources from $DLL_PATH"
ilspycmd --list-resources "$DLL_PATH" > "$LIST_FILE"

extract_resource() {
  local resource_name="$1"
  local relative_path
  local destination_dir
  local temp_out
  local extracted_file
  local extracted_name

  relative_path="${resource_name#${RESOURCE_PREFIX}}"
  destination_dir="$OUTPUT_DIR/raw/$(dirname "$relative_path")"
  temp_out="$TMP_DIR/out"

  mkdir -p "$destination_dir"
  rm -rf "$temp_out"
  mkdir -p "$temp_out"

  ilspycmd --resource "$resource_name" "$DLL_PATH" -o "$temp_out" >/dev/null
  extracted_file="$(find "$temp_out" -maxdepth 1 -type f | head -n 1)"
  if [[ -n "$extracted_file" && -f "$extracted_file" ]]; then
    extracted_name="$(basename "$extracted_file")"
    mv "$extracted_file" "$destination_dir/$extracted_name"
    echo "[+] $relative_path -> $extracted_name"
    return
  fi

  echo "[!] failed to extract $resource_name" >&2
}

echo "[*] Extracting image assets"
while IFS= read -r line; do
  case "$line" in
    ${RESOURCE_PREFIX}resources/*.png|${RESOURCE_PREFIX}resources/*.jpg|${RESOURCE_PREFIX}resources/*.jpeg|${RESOURCE_PREFIX}resources/*.bmp|${RESOURCE_PREFIX}resources/*.gif|${RESOURCE_PREFIX}resources/*.ico)
      extract_resource "$line"
      ;;
  esac
done < "$LIST_FILE"

if [[ -f "./resources/APP_NOTIFY_ICON.png" ]]; then
  cp "./resources/APP_NOTIFY_ICON.png" "$OUTPUT_DIR/resources/app_notify_icon.png"
fi

cat > "$OUTPUT_DIR/model_icon_map.json" <<'EOF'
{
  "icon_files": {
    "MENU_HEADSET": "raw/resources/menu_headset.png",
    "MENU_BUDS": "raw/resources/menu_buds.png",
    "MENU_MOUSE": "raw/resources/menu_mouse.png",
    "MENU_KEYBORD": "raw/resources/menu_keybord.png",
    "MENU_IE": "raw/resources/menu_ie.png"
  },
  "models": [
    {
      "model_id": "HDX_2959",
      "internal_name": "HDX-2959",
      "marketing_name": "INZONE H9",
      "icon_name": "MENU_HEADSET",
      "icon_file": "raw/resources/menu_headset.png",
      "pids": ["0x0E53", "0x0E4C", "0x0E61"]
    },
    {
      "model_id": "HDX_2960",
      "internal_name": "HDX-2960",
      "marketing_name": "INZONE H7",
      "icon_name": "MENU_HEADSET",
      "icon_file": "raw/resources/menu_headset.png",
      "pids": []
    },
    {
      "model_id": "HDX_2961",
      "internal_name": "HDX-2961",
      "marketing_name": "INZONE H3",
      "icon_name": "MENU_HEADSET",
      "icon_file": "raw/resources/menu_headset.png",
      "pids": ["0x0DFD", "0x0E47"]
    },
    {
      "model_id": "GH_M2",
      "internal_name": "YY2976",
      "marketing_name": "INZONE H5",
      "icon_name": "MENU_HEADSET",
      "icon_file": "raw/resources/menu_headset.png",
      "pids": ["0x0EBF", "0x0EC0"]
    },
    {
      "model_id": "GTW",
      "internal_name": "YY2977",
      "marketing_name": "INZONE Buds",
      "icon_name": "MENU_BUDS",
      "icon_file": "raw/resources/menu_buds.png",
      "pids": ["0x0EC2", "0x0EC3"]
    },
    {
      "model_id": "GH_H2",
      "internal_name": "YY2987",
      "marketing_name": "INZONE H9 II",
      "icon_name": "MENU_HEADSET",
      "icon_file": "raw/resources/menu_headset.png",
      "pids": ["0x0FA8", "0x0FA9"]
    },
    {
      "model_id": "GH_IE",
      "internal_name": "YY2989",
      "marketing_name": "INZONE E9",
      "icon_name": "MENU_IE",
      "icon_file": "raw/resources/menu_ie.png",
      "pids": ["0x0F80", "0x0F81"]
    },
    {
      "model_id": "GH_OB",
      "internal_name": "YY2990",
      "marketing_name": "INZONE H6 Air",
      "icon_name": "MENU_HEADSET",
      "icon_file": "raw/resources/menu_headset.png",
      "pids": ["0x0FC0", "0x0FC1"]
    },
    {
      "model_id": "HDX_2991",
      "internal_name": "YY2991",
      "marketing_name": "INZONE Mouse-A",
      "icon_name": "MENU_MOUSE",
      "icon_file": "raw/resources/menu_mouse.png",
      "pids": ["0x0FAE", "0x0FAF", "0x0FB1", "0x0FB2"]
    },
    {
      "model_id": "HDX_2992",
      "internal_name": "YY2992",
      "marketing_name": "INZONE KBD-H75",
      "icon_name": "MENU_KEYBORD",
      "icon_file": "raw/resources/menu_keybord.png",
      "pids": ["0x0FB0", "0x0FB3"]
    }
  ],
  "notable_assets": {
    "buds": [
      "raw/resources/bud_s.png",
      "raw/resources/ear_l.png",
      "raw/resources/ear_r.png",
      "raw/resources/tw_case_mini_s.png"
    ],
    "mouse": [
      "raw/resources/mouse_top.png",
      "raw/resources/mouse_top_fnc.png",
      "raw/resources/mouse_wired_s.png",
      "raw/resources/mouse_dongle_s.png"
    ],
    "status": [
      "raw/resources/battery_1_s.png",
      "raw/resources/battery_2_s.png",
      "raw/resources/battery_3_s.png",
      "raw/resources/battery_4_s.png",
      "raw/resources/bluetooth_connected_s.png",
      "raw/resources/mic_mute_mini_s.png"
    ]
  }
}
EOF

cat > "$OUTPUT_DIR/README.txt" <<'EOF'
Extracted from Sony INZONE Hub.

Directories:
  raw/resources/        Embedded resource files extracted from INZONEHub.dll
  resources/            Extra loose files copied from install directory when present
  model_icon_map.json   Model <-> icon/material mapping for Linux GUI use

Common files:
  raw/resources/menu_headset.png
  raw/resources/menu_buds.png
  raw/resources/menu_mouse.png
  raw/resources/menu_keybord.png
  raw/resources/menu_ie.png
EOF

echo "[*] Done"
echo "    Output: $OUTPUT_DIR"
echo "    Mapping: $OUTPUT_DIR/model_icon_map.json"
