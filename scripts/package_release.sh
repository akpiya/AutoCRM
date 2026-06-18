#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: scripts/package_release.sh <version>" >&2
  echo "example: scripts/package_release.sh v0.1.0" >&2
  exit 2
fi

version="$1"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist_dir="$repo_root/dist"
commit="$(git -C "$repo_root" rev-parse --short HEAD)"
bundle_version="${version#v}"
ldflags="-s -w"

mkdir -p "$dist_dir"
rm -f "$dist_dir"/autocrm-macos-*.zip

build_one() {
  local arch="$1"
  local work_dir="$dist_dir/autocrm-macos-$arch"
  local app_dir="$work_dir/AutoCRM.app"
  local contents_dir="$app_dir/Contents"
  local macos_dir="$contents_dir/MacOS"
  local resources_dir="$contents_dir/Resources"
  local binary="$macos_dir/autocrm"
  local zip_path="$dist_dir/autocrm-macos-$arch.zip"
  local actool_log="$work_dir/actool.log"
  local asset_catalog_plist="$work_dir/AssetCatalog.plist"

  rm -rf "$work_dir"
  mkdir -p "$macos_dir" "$resources_dir"

  CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" go build \
    -ldflags "$ldflags" \
    -o "$binary" \
    ./cmd/autocrm

  if ! xcrun actool "$repo_root/assets/Assets.xcassets" \
    --compile "$resources_dir" \
    --platform macosx \
    --target-device mac \
    --minimum-deployment-target 12.0 \
    --app-icon AppIcon \
    --output-partial-info-plist "$asset_catalog_plist" >"$actool_log" 2>&1; then
    cat "$actool_log" >&2
    return 1
  fi
  rm -f "$actool_log" "$asset_catalog_plist"

  cat > "$contents_dir/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleExecutable</key>
  <string>autocrm</string>
  <key>CFBundleIconFile</key>
  <string>AppIcon</string>
  <key>CFBundleIconName</key>
  <string>AppIcon</string>
  <key>CFBundleIdentifier</key>
  <string>com.akpiya.autocrm</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>AutoCRM</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>$bundle_version</string>
  <key>CFBundleVersion</key>
  <string>$commit</string>
  <key>LSMinimumSystemVersion</key>
  <string>12.0</string>
</dict>
</plist>
EOF

  cat > "$work_dir/README.txt" <<EOF
AutoCRM $version for macOS $arch

Install:
  ./AutoCRM.app/Contents/MacOS/autocrm install

The installer copies AutoCRM.app to ~/.autocrm/AutoCRM.app, writes a LaunchAgent,
and guides you through granting Full Disk Access to the installed app.

Other commands:
  ~/.autocrm/AutoCRM.app/Contents/MacOS/autocrm doctor
  ~/.autocrm/AutoCRM.app/Contents/MacOS/autocrm run
  ~/.autocrm/AutoCRM.app/Contents/MacOS/autocrm uninstall

This app is not notarized by Apple. macOS may require you to approve it in
Privacy & Security before it can run.
EOF

  (cd "$work_dir" && zip -q -r "$zip_path" AutoCRM.app README.txt)
  echo "wrote $zip_path"
}

cd "$repo_root"
build_one arm64
build_one amd64
