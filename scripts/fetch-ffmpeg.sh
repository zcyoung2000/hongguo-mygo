#!/usr/bin/env bash
set -euo pipefail

case "$(uname -m)" in
  arm64)
    url="https://www.osxexperts.net/ffmpeg9arm.zip"
    expected="591260c945d0eef150e3bf82b0ef988bd36a9cecc18ff05d6679617159f0a95e"
    ;;
  x86_64)
    url="https://www.osxexperts.net/ffmpeg80intel.zip"
    expected="df3f1e3facdc1ae0ad0bd898cdfb072fbc9641bf47b11f172844525a05db8d11"
    ;;
  *)
    echo "unsupported macOS architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

root="$(cd "$(dirname "$0")/.." && pwd)"
output="$root/resources/darwin-$(uname -m)/ffmpeg"

if [[ -x "$output" ]]; then
  actual="$(shasum -a 256 "$output" | awk '{print $1}')"
  if [[ "$actual" == "$expected" ]]; then
    exit 0
  fi
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

curl -fL --retry 3 --connect-timeout 10 -o "$work/ffmpeg.zip" "$url"
unzip -q "$work/ffmpeg.zip" ffmpeg -d "$work"

actual="$(shasum -a 256 "$work/ffmpeg" | awk '{print $1}')"
if [[ "$actual" != "$expected" ]]; then
  echo "ffmpeg checksum mismatch: expected $expected, got $actual" >&2
  exit 1
fi

mkdir -p "$(dirname "$output")"
install -m 0755 "$work/ffmpeg" "$output"
