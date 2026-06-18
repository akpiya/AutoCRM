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
  local binary="$macos_dir/autocrm"
  local zip_path="$dist_dir/autocrm-macos-$arch.zip"

  rm -rf "$work_dir"
  mkdir -p "$macos_dir"

  CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" go build \
    -ldflags "$ldflags" \
    -o "$binary" \
    ./cmd/autocrm

  cat > "$contents_dir/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleExecutable</key>
  <string>autocrm</string>
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

Other commands:
  ./AutoCRM.app/Contents/MacOS/autocrm doctor
  ./AutoCRM.app/Contents/MacOS/autocrm run
  ./AutoCRM.app/Contents/MacOS/autocrm uninstall

This app is not notarized by Apple. macOS may require you to approve it in
Privacy & Security before it can run.
EOF

  (cd "$work_dir" && zip -q -r "$zip_path" AutoCRM.app README.txt)
  echo "wrote $zip_path"
}

cd "$repo_root"
build_one arm64
build_one amd64
