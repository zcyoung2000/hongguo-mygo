#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="$ROOT_DIR/mygo.json"

CONFIG="$(python3 -c '
import json, pathlib, sys
data = json.loads(pathlib.Path(sys.argv[1]).read_text())
print(data.get("name", ""), data.get("out", "dist"), sep="\n")
' "$CONFIG_FILE")"

APP_NAME="$(sed -n '1p' <<<"$CONFIG")"
OUTPUT_DIR="$(sed -n '2p' <<<"$CONFIG")"

if [[ -z "$APP_NAME" ]]; then
  echo "error: app name is missing from mygo.json" >&2
  exit 1
fi

cd "$ROOT_DIR"

echo "==> Building $APP_NAME (CGO_ENABLED=0)"
CGO_ENABLED=0 go run github.com/egoist/mygo/cmd/mygo build -skip-dmg

BUILT_APP="$ROOT_DIR/$OUTPUT_DIR/darwin-arm64/$APP_NAME.app"
INSTALL_APP="/Applications/$APP_NAME.app"

if [[ ! -d "$BUILT_APP" ]]; then
  echo "error: built app not found: $BUILT_APP" >&2
  exit 1
fi

echo "==> Stopping $APP_NAME"
PLIST="$BUILT_APP/Contents/Info.plist"
EXECUTABLE="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$PLIST")"
PROCESS_PATH="$INSTALL_APP/Contents/MacOS/$EXECUTABLE"
PIDS="$(pgrep -f "$PROCESS_PATH" || true)"

if [[ -n "$PIDS" ]]; then
  kill $PIDS
  for _ in {1..20}; do
    PIDS="$(pgrep -f "$PROCESS_PATH" || true)"
    [[ -z "$PIDS" ]] && break
    sleep 0.1
  done

  PIDS="$(pgrep -f "$PROCESS_PATH" || true)"
  if [[ -n "$PIDS" ]]; then
    kill -9 $PIDS
  fi
fi

echo "==> Installing to /Applications"
# rsync removes resources left by previous installs; ditto only overlays files.
rsync -a --delete "$BUILT_APP/" "$INSTALL_APP/"

# Ad-hoc signed replacements can keep a stale icon in LaunchServices/Finder/Dock.
touch "$INSTALL_APP" "$INSTALL_APP/Contents/Info.plist"
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
if [[ -x "$LSREGISTER" ]]; then
  "$LSREGISTER" -f "$INSTALL_APP" >/dev/null 2>&1 || true
fi
killall Dock >/dev/null 2>&1 || true
killall Finder >/dev/null 2>&1 || true

echo "==> Opening $APP_NAME"
open "$INSTALL_APP"

echo "==> Done"
