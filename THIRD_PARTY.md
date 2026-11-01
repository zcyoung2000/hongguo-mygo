# Third-Party Components

## OpenHongguo

- Upstream: OpenHongguo (<https://github.com/beilunyang/openhongguo>)
- License: Apache-2.0
- Usage: Go SDK and request-signing helpers used for catalog, account, playback, and comment integration.

## MyGo

- Upstream: MyGo (<https://github.com/egoist/mygo>)
- License: MIT
- Usage: Native macOS windowing, WKWebView embedding, IPC bridge, and packaging.

## FFmpeg

macOS release builds bundle a static FFmpeg executable selected for the target architecture. The binary is fetched by `scripts/fetch-ffmpeg.sh` and embedded in `<App>.app/Contents/Resources`.

- Upstream: FFmpeg (<https://ffmpeg.org/>)
- Build source: OSXExperts (<https://www.osxexperts.net/>)
- License: GPL; the packaged builds enable GPL components
- Source: download the matching FFmpeg source from <https://ffmpeg.org/download.html>

Redistribution of these GPL binaries must follow the applicable GPL terms, including providing the corresponding source or a written source offer.

## SQLite Driver

- Package: `modernc.org/sqlite`
- License: BSD-3-Clause; SQLite itself is public domain.
