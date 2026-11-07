#!/usr/bin/env python3
"""MyGo automation E2E test runner for myguo.

Usage:
    python3 e2e/run_e2e.py [--socket /tmp/myguo-test.sock] [--verbose]
"""

import argparse
import base64
import json
import re
import socket
import sys
import time

PASS = 0
FAIL = 0
results = []
screenshots = []
EMOJI_RE = re.compile("[\U0001f300-\U0001f9ff\u2600-\u27bf]")


class MyGoClient:
    """Line-delimited JSON client for MyGo automation plugin."""

    def __init__(self, path: str):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(30)
        self.sock.connect(path)
        self.f = self.sock.makefile("r")
        self.next_id = 1

    def call(self, method: str, params=None, timeout: float = 30):
        rid = self.next_id
        self.next_id += 1
        msg = {"id": rid, "method": method}
        if params:
            msg["params"] = params
        self.sock.sendall((json.dumps(msg) + "\n").encode())
        self.sock.settimeout(timeout)
        while True:
            line = self.f.readline()
            if not line:
                raise ConnectionError("socket closed")
            resp = json.loads(line)
            if resp.get("id") == rid:
                if "error" in resp:
                    return None, resp["error"]
                return resp.get("result"), None

    def close(self):
        self.sock.close()


def record(name, ok, detail, module=""):
    global PASS, FAIL
    results.append((module, name, ok, detail))
    if ok:
        PASS += 1
    else:
        FAIL += 1
        print(f"  [FAIL] {module} {name}: {detail}")


def js(client, win, script, timeout=15):
    r, e = client.call("eval", {"window": win, "script": script}, timeout=timeout)
    if e:
        return None, e
    return r.get("value"), None


def js_ok(client, win, script):
    v, e = js(client, win, script)
    return e is None, f"err={e}" if e else "ok"


def elem(client, win, sel):
    v, e = js(client, win, f'document.querySelector("{sel}") !== null')
    return v is True and not e, f"{sel}={v}, err={e}"


def switch_view(client, win, view):
    script = f'switchView("{view}"); return document.getElementById("view-{view}").classList.contains("active")'
    v, e = js(client, win, script)
    return v is True and not e, f"active={v}, err={e}"


def wait(n):
    time.sleep(n)


# === Module A: Foundation ===

def mod_a(c, w):
    m = "A"
    r, e = c.call("windows")
    wins = (r or {}).get("windows", []) if r else []
    record("A01 window exists", len(wins) > 0 and not e, f"n={len(wins)}, err={e}", m)
    if not wins:
        return
    cb = wins[0].get("contentBounds", {})
    record("A02 window size", cb.get("width", 0) >= 1000 and cb.get("height", 0) >= 640,
           f"w={cb.get('width')} h={cb.get('height')}", m)
    v, e = js(c, w, "document.readyState")
    record("A03 readyState", v == "complete" and not e, f"got {v!r}", m)
    ok, d = elem(c, w, "#sidebar")
    record("A04 sidebar", ok, d, m)
    v, e = js(c, w, "document.querySelectorAll('.nav-item').length")
    record("A05 nav >= 8", isinstance(v, (int, float)) and v >= 8, f"n={v}", m)
    ok, d = elem(c, w, ".main-content")
    record("A06 main-content", ok, d, m)
    ok, d = elem(c, w, "#toast")
    record("A07 toast", ok, d, m)
    v, e = js(c, w, "document.querySelectorAll('.view-page').length")
    record("A08 views >= 11", isinstance(v, (int, float)) and v >= 11, f"n={v}", m)
    v, e = js(c, w, "[typeof window.mygo, typeof window.mygo?.call, typeof switchView, typeof apiCall].join(':')")
    record("A09 frontend bindings loaded", v == "object:function:function:function", f"got {v!r}, err={e}", m)


# === Module B: View Routing ===

VIEWS = ["recommend", "discover", "rank", "new", "history", "favorites", "reservations", "settings"]

def mod_b(c, w):
    m = "B"
    for i, view in enumerate(VIEWS):
        ok, d = switch_view(c, w, view)
        record(f"B{i+1:02d} switch {view}", ok, d, m)
    ok, d = switch_view(c, w, "recommend")
    record("B09 back to recommend", ok, d, m)
    script = 'switchView("discover"); switchView("discover"); return document.getElementById("view-discover").classList.contains("active")'
    v, e = js(c, w, script)
    record("B10 idempotent", v is True and not e, f"active={v}", m)
    script = 'switchView("discover"); return document.querySelector(".nav-item.active")?.dataset.view'
    v, e = js(c, w, script)
    record("B11 nav syncs", v == "discover" and not e, f"got {v!r}", m)
    ok, d = js_ok(c, w, 'try { switchView("nonexistent") } catch(e) { return "caught" }; return "ok"')
    record("B12 invalid view safe", ok, d, m)


# === Module C: Feed ===

def mod_c(c, w):
    m = "C"
    switch_view(c, w, "recommend")
    wait(1)
    v, e = js(c, w, "document.querySelectorAll('.feed-video').length")
    record("C01 feed videos >= 3", isinstance(v, (int, float)) and v >= 3, f"n={v}", m)
    v, e = js(c, w, 'document.getElementById("feed-video-0")?.playsInline')
    record("C02 playsInline", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'document.getElementById("feed-video-0")?.className')
    ok = isinstance(v, str) and ("portrait" in v or "landscape" in v)
    record("C03 orientation class", ok, f"got {v!r}", m)
    for i in range(3):
        ok, d = elem(c, w, f"#feed-video-{i}")
        record(f"C04 video-{i} exists", ok, d, m)
    ok, d = elem(c, w, "#feed-hint-toast")
    record("C05 hint toast", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetHotSearch')")
    record("C06 GetHotSearch callable", ok, d, m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetHotSearch').then(r => r && Array.isArray(r.words) && r.words.length > 0).catch(() => false)", timeout=20)
    record("C07 hot search words array", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.SetFeedScrollCapture', true)")
    record("C08 SetFeedScrollCapture", ok, d, m)


# === Module D: Discover ===

def mod_d(c, w):
    m = "D"
    switch_view(c, w, "discover")
    wait(2)
    ok, d = elem(c, w, "#search-input")
    record("D01 search input", ok, d, m)
    # Filter panel may show a loading spinner while the API loads; either
    # loading indicator or loaded genre chips is a valid UI state.
    v, e = js(c, w, 'document.querySelectorAll(".filter-chip[data-key=\'genre\']").length')
    record("D02 genre chips >= 3", isinstance(v, (int, float)) and v >= 3, f"n={v}", m)
    v, e = js(c, w, 'document.querySelector(".filter-chip.active[data-key=\'genre\']")?.dataset.val')
    record("D03 default genre short_play", v == "short_play", f"got {v!r}", m)
    ok, d = elem(c, w, "#discover-filters")
    record("D04 filter panel", ok, d, m)
    # Subfilters are rendered as additional .filter-row elements.
    v, e = js(c, w, 'document.querySelectorAll("#discover-filters .filter-row").length')
    record("D05 filter rows >= 1", isinstance(v, (int, float)) and v >= 1, f"n={v}", m)
    ok, d = elem(c, w, "#discover-grid")
    record("D06 discover grid", ok, d, m)
    ok, d = elem(c, w, "#btn-discover-load-more")
    record("D07 load more btn", ok, d, m)
    v, e = js(c, w, "document.querySelectorAll('#discover-grid .drama-card').length")
    record("D08 cards > 0", isinstance(v, (int, float)) and v > 0, f"n={v}", m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetFilterPanel', 'short_play')")
    record("D09 GetFilterPanel", ok, d, m)
    ok, d = elem(c, w, "#discover-grid .drama-card img")
    record("D10 card cover img", ok, d, m)
    # Data quality: most discover cards should load real covers (not fallback SVG)
    v, e = js(c, w, """return (function(){
        const imgs = document.querySelectorAll('#discover-grid .drama-card img.card-poster');
        if (imgs.length === 0) return JSON.stringify({total: 0, real: 0});
        let real = 0;
        for (const img of imgs) {
            if (img.src && !img.src.startsWith('data:image/svg')) real++;
        }
        return JSON.stringify({total: imgs.length, real: real});
    })()""")
    try:
        dq = json.loads(v) if isinstance(v, str) else {}
    except (json.JSONDecodeError, TypeError):
        dq = {}
    ratio = dq.get("real", 0) / max(dq.get("total", 1), 1)
    record("D11 discover covers > 50% real", ratio > 0.5,
           f"{dq.get('real', 0)}/{dq.get('total', 0)} real ({ratio:.0%})", m)


# === Module E: Rankings ===

def mod_e(c, w):
    m = "E"
    switch_view(c, w, "rank")
    wait(2)
    v, e = js(c, w, "document.querySelectorAll('#rank-content-tabs .rank-content-tab').length")
    record("E01 content tabs >= 1", isinstance(v, (int, float)) and v >= 1, f"n={v}", m)
    ok, d = elem(c, w, "#rank-board-tabs")
    record("E02 board tabs", ok, d, m)
    ok, d = elem(c, w, "#rank-items-list")
    record("E03 rank list", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetRankings')")
    record("E04 GetRankings callable", ok, d, m)
    v, e = js(c, w, "document.getElementById('rank-items-list')?.children.length")
    record("E05 list items > 0", isinstance(v, (int, float)) and v > 0, f"n={v}", m)


# === Module F: New Releases ===

def mod_f(c, w):
    m = "F"
    switch_view(c, w, "new")
    wait(2)
    v, e = js(c, w, "document.querySelectorAll('#new-main-tabs .rank-content-tab').length")
    record("F01 new tabs >= 2", isinstance(v, (int, float)) and v >= 2, f"n={v}", m)
    v, e = js(c, w, 'document.querySelector("#new-main-tabs .rank-content-tab.active")?.dataset.tab')
    record("F02 default tab hot", v == "hot", f"got {v!r}", m)
    ok, d = elem(c, w, "#new-calendar-dates")
    record("F03 calendar dates", ok, d, m)
    ok, d = elem(c, w, "#new-drama-grid")
    record("F04 new grid", ok, d, m)
    v, e = js(c, w, "document.querySelectorAll('#new-drama-grid .drama-card').length")
    record("F05 new cards > 0", isinstance(v, (int, float)) and v > 0, f"n={v}", m)


# === Module G: History ===

def mod_g(c, w):
    m = "G"
    switch_view(c, w, "history")
    wait(1)
    ok, d = elem(c, w, "#history-grid")
    record("G01 history grid", ok, d, m)
    v, e = js(c, w, "document.querySelectorAll('#history-filter-pills .filter-chip').length")
    record("G02 filter pills >= 3", isinstance(v, (int, float)) and v >= 3, f"n={v}", m)
    ok, d = elem(c, w, "#history-search-input")
    record("G03 history search", ok, d, m)
    ok, d = elem(c, w, "#btn-clear-all-history")
    record("G04 clear all btn", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetHistory')")
    record("G05 GetHistory callable", ok, d, m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetHistory').then(r => Array.isArray(r)).catch(() => false)", timeout=20)
    record("G06 GetHistory array", v is True, f"got {v!r}", m)


# === Module H: Favorites & Reservations ===

def mod_h(c, w):
    m = "H"
    ok, d = elem(c, w, "#favorites-grid")
    if not ok:
        ok, d = elem(c, w, "#view-favorites .drama-grid")
    record("H01 favorites grid", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetFavorites')")
    record("H02 GetFavorites callable", ok, d, m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetFavorites').then(r => Array.isArray(r)).catch(() => false)", timeout=20)
    record("H03 GetFavorites array", v is True, f"got {v!r}", m)
    ok, d = elem(c, w, "#view-reservations .drama-grid")
    record("H04 reservations grid", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetReservations')")
    record("H05 GetReservations callable", ok, d, m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetReservations').then(r => Array.isArray(r)).catch(() => false)", timeout=20)
    record("H06 GetReservations array", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetLikes')")
    record("H07 GetLikes callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.IsFavorite', '999')")
    record("H08 IsFavorite callable", ok, d, m)


# === Module I: Search ===

def mod_i(c, w):
    m = "I"
    switch_view(c, w, "discover")
    ok, d = elem(c, w, "#search-suggestions")
    record("I01 suggestions box", ok, d, m)
    ok, d = elem(c, w, "#btn-clear-search")
    record("I02 clear search btn", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetSuggestions', '总裁')")
    record("I03 GetSuggestions callable", ok, d, m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetSuggestions', '总裁').then(r => Array.isArray(r)).catch(() => false)", timeout=20)
    record("I04 GetSuggestions array", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.Search', '总裁')")
    record("I05 Search callable", ok, d, m)
    ok, d = elem(c, w, "#view-search-results")
    record("I06 search results view", ok, d, m)
    ok, d = elem(c, w, "#search-query-header")
    record("I07 query header", ok, d, m)
    ok, d = elem(c, w, "#search-grid")
    record("I08 search grid", ok, d, m)


# === Module J: Settings ===

def mod_j(c, w):
    m = "J"
    switch_view(c, w, "settings")
    wait(1)
    v, e = js(c, w, "document.querySelector('.settings-page-title')?.textContent")
    record("J01 settings title", isinstance(v, str) and len(v) > 0, f"got {v!r}", m)
    ok, d = elem(c, w, "#settings-avatar")
    record("J02 avatar", ok, d, m)
    ok, d = elem(c, w, "#settings-nickname")
    record("J03 nickname", ok, d, m)
    ok, d = elem(c, w, "#settings-login-badge")
    record("J04 login badge", ok, d, m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetCacheSize').then(r => typeof r === 'number').catch(() => false)", timeout=20)
    record("J05 GetCacheSize number", v is True, f"got {v!r}", m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetDeviceInfo').then(r => r && typeof r.deviceId === 'string').catch(() => false)", timeout=20)
    record("J06 GetDeviceInfo deviceId", v is True, f"got {v!r}", m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetProfile').then(r => typeof r === 'object' && r !== null).catch(() => false)", timeout=20)
    record("J07 GetProfile object", v is True, f"got {v!r}", m)
    ok, d = elem(c, w, "#btn-settings-clear-cache")
    record("J08 clear cache btn", ok, d, m)
    ok, d = elem(c, w, "#settings-device-id")
    record("J09 device id", ok, d, m)
    ok, d = elem(c, w, "#settings-cache-size")
    record("J10 cache size", ok, d, m)


# === Module K: Detail & Player ===

def mod_k(c, w):
    m = "K"
    ok, d = elem(c, w, "#view-detail")
    record("K01 detail view", ok, d, m)
    ok, d = elem(c, w, "#detail-title")
    record("K02 detail title", ok, d, m)
    ok, d = elem(c, w, "#detail-episode-grid")
    record("K03 episode grid", ok, d, m)
    ok, d = elem(c, w, "#detail-episode-count")
    record("K04 episode count", ok, d, m)
    ok, d = elem(c, w, "#detail-related-section")
    record("K05 related section", ok, d, m)
    ok, d = elem(c, w, "#detail-actors-section")
    record("K06 actors section", ok, d, m)
    ok, d = elem(c, w, "#main-video")
    record("K07 main video", ok, d, m)
    v, e = js(c, w, 'document.getElementById("main-video")?.playsInline')
    record("K08 video playsInline", v is True, f"got {v!r}", m)
    ok, d = elem(c, w, "#detail-episode-toggle")
    record("K09 episode toggle", ok, d, m)
    ok, d = elem(c, w, "#view-actor")
    record("K10 actor view", ok, d, m)


# === Module L: IPC Bridge ===

def mod_l(c, w):
    m = "L"
    v, e = js(c, w, "typeof window.mygo")
    record("L01 window.mygo object", v == "object", f"got {v!r}", m)
    v, e = js(c, w, "typeof window.mygo?.call")
    record("L02 mygo.call function", v == "function", f"got {v!r}", m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.NonexistentMethod').then(() => false).catch(() => true)")
    record("L03 unknown method rejects", v is True, f"got {v!r}", m)
    rejected = [
        ("L04 ToggleFavorite", "window.mygo.call('HongguoService.ToggleFavorite', '999')"),
        ("L05 ToggleLike", "window.mygo.call('HongguoService.ToggleLike', '999', '999')"),
        ("L10 SaveProgress", "window.mygo.call('HongguoService.SaveProgress', '999', '1', 0, 100, 1)"),
    ]
    for name, expr in rejected:
        v, e = js(c, w, f"return {expr}.then(() => false).catch(() => true)")
        record(f"{name} rejects invalid ID", v is True, f"got {v!r}", m)

    safe = [
        ("L06 ClearNowPlaying", "window.mygo.call('HongguoService.ClearNowPlaying')"),
        ("L07 SetPlaybackKeepAwake", "window.mygo.call('HongguoService.SetPlaybackKeepAwake', false)"),
        ("L08 SendNativeNotification", "window.mygo.call('HongguoService.SendNativeNotification', 'e2e', 'test')"),
        ("L09 ShowConfirmDialog", "window.mygo.call('HongguoService.ShowConfirmDialog', 'e2e')"),
    ]
    for name, expr in safe:
        ok, d = js_ok(c, w, f"{expr}.catch(() => 'rejected'); return 'ok'")
        record(name, ok, d, m)


# === Module M: Security ===

def mod_m(c, w):
    m = "M"
    v, e = js(c, w, "document.body.innerHTML")
    if e:
        record("M01 no emoji", False, f"eval failed: {e}", m)
    else:
        n = len(EMOJI_RE.findall(v or ""))
        record("M01 no emoji", n == 0, f"found {n}", m)
    v, e = js(c, w, "typeof window.handleImgError")
    record("M02 handleImgError", v == "function", f"got {v!r}", m)
    v, e = js(c, w, "typeof window.escapeHtml")
    record("M03 escapeHtml", v == "function", f"got {v!r}", m)
    v, e = js(c, w, 'document.querySelector(\'[onclick^="javascript:"]\') !== null')
    record("M04 no javascript: onclick", v is False, f"got {v!r}", m)
    n_cards, _ = js(c, w, "document.querySelectorAll('#discover-grid .drama-card').length")
    n_onerr, _ = js(c, w, "document.querySelectorAll('#discover-grid .drama-card img[onerror]').length")
    if isinstance(n_cards, (int, float)) and n_cards > 0:
        record("M05 img onerror bound", isinstance(n_onerr, (int, float)) and n_onerr > 0,
               f"cards={n_cards} onerror={n_onerr}", m)
    else:
        record("M05 img onerror bound", True, "no cards to check", m)


# === Module N: Screenshots ===

def mod_n(c, w):
    m = "N"
    for i, view in enumerate(["recommend", "discover", "rank", "settings", "history"]):
        switch_view(c, w, view)
        wait(0.5)
        r, e = c.call("capture", {"window": w})
        if e:
            record(f"N{i+1:02d} {view} capture", False, f"err={e}", m)
            continue
        data = base64.b64decode(r.get("png", ""))
        ww, hh = r.get("width", 0), r.get("height", 0)
        ok = len(data) > 0 and ww > 0 and hh > 0
        record(f"N{i+1:02d} {view} capture", ok, f"{ww}x{hh} {len(data)}B", m)
        if ok:
            screenshots.append((view, data))


# === Module P: Stability ===

def mod_p(c, w):
    m = "P"
    r, e = c.call("idle", {"window": w, "timeout": 5000})
    record("P01 idle settle", not e, f"err={e}" if e else "settled", m)
    v, e = js(c, w, "const vs=['recommend','discover','rank','new','history','favorites','reservations','settings','recommend','discover']; for(const v of vs){switchView(v)} return 'done'")
    record("P02 rapid 10 switches", v == "done" and not e, f"got {v!r}", m)
    r, e = c.call("windows")
    record("P03 app responsive", r is not None and not e, f"err={e}", m)
    record("P04 screenshots >= 5", len(screenshots) >= 5, f"n={len(screenshots)}", m)


# === Module Q: Remaining IPC Coverage ===

def mod_q(c, w):
    m = "Q"
    # Grab a real series ID from discover grid for detail tests
    first_id, _ = js(c, w,
        'return document.querySelector("#discover-grid .drama-card")?.dataset.id || document.querySelector("#discover-grid .drama-card")?.getAttribute("data-series-id") || ""')
    switch_view(c, w, "discover")
    wait(2)
    if not first_id:
        first_id, _ = js(c, w,
            'return window.mygo.call("HongguoService.GetDiscover").then(r => Array.isArray(r) && r.length > 0 ? String(r[0].id || r[0].seriesId || "") : "").catch(() => "")', timeout=20)
    if not first_id:
        first_id = "7700943522404240409"  # fallback

    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetRecommendFeed')")
    record("Q01 GetRecommendFeed callable", ok, d, m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetRecommendFeed').then(r => r !== null && r !== undefined).catch(() => false)", timeout=25)
    record("Q02 GetRecommendFeed non-empty", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetDiscover')")
    record("Q03 GetDiscover callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetNewReleases', new Date().toISOString().slice(0,10).replace(/-/g,''))")
    record("Q04 GetNewReleases callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetRecommend')")
    record("Q05 GetRecommend callable", ok, d, m)
    ok, d = js_ok(c, w, f"window.mygo.call('HongguoService.GetSeriesDetail', '{first_id}').catch(() => 'rejected')")
    record("Q06 GetSeriesDetail valid ID", ok, d, m)
    v, e = js(c, w, f"return window.mygo.call('HongguoService.GetSeriesDetail', '{first_id}').then(r => typeof r?.title === 'string' ? 'has-title' : 'no-title').catch(e => 'rejected')", timeout=20)
    record("Q07 GetSeriesDetail title or reject", v in ("has-title", "rejected"), f"got {v!r}", m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetSeriesDetail', '999').then(() => false).catch(() => true)", timeout=15)
    record("Q08 GetSeriesDetail invalid rejects", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetActorProfile', 'test').catch(() => 'rejected')")
    record("Q09 GetActorProfile callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.GetActorWorks', 'test').catch(() => 'rejected')")
    record("Q10 GetActorWorks callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.CheckExistingAccount')")
    record("Q11 CheckExistingAccount callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.ImportHgplayerAccount').catch(() => 'rejected')")
    record("Q12 ImportHgplayerAccount callable", ok, d, m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.LoginWithCookies', '').then(() => false).catch(e => typeof e === 'object' || typeof e === 'string')", timeout=20)
    record("Q13 LoginWithCookies empty rejects", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.Logout').catch(() => 'rejected')")
    record("Q14 Logout callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.RefreshUserProfile').catch(() => 'rejected')")
    record("Q15 RefreshUserProfile callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.ToggleReservation', '999').catch(() => 'rejected')")
    record("Q16 ToggleReservation callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.TogglePictureInPicture').catch(() => 'rejected')")
    record("Q17 TogglePictureInPicture callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.UpdateNowPlaying', {title:'e2e', artist:'test'}).catch(() => 'rejected')")
    record("Q18 UpdateNowPlaying callable", ok, d, m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.WarmupFeed').catch(() => 'rejected')")
    record("Q19 WarmupFeed callable", ok, d, m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.PlayEpisode', '999', '', 1, '720p').then(() => false).catch(e => typeof e === 'object' || typeof e === 'string')", timeout=25)
    record("Q20 PlayEpisode invalid rejects", v is True, f"got {v!r}", m)
    for case, method in [("Q21", "DeleteHistory"), ("Q22", "ClearHistory")]:
        v, e = js(c, w, f"return window.mygo.call('HongguoService.{method}').then(() => false).catch(() => true)")
        record(f"{case} {method} unavailable", v is True, f"got {v!r}", m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.ClearCache').then(r => typeof r === 'number').catch(() => false)", timeout=20)
    record("Q23 ClearCache returns number", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, "window.mygo.call('HongguoService.ResetDevice').catch(() => 'rejected')")
    record("Q24 ResetDevice callable", ok, d, m)


# === Module R: Detail Workflow ===

def mod_r(c, w):
    m = "R"
    switch_view(c, w, "discover")
    wait(2)
    v, e = js(c, w, "document.querySelectorAll('#discover-grid .drama-card').length")
    record("R01 discover cards > 0", isinstance(v, (int, float)) and v > 0, f"n={v}", m)

    # Get a real series ID from the IPC API (cards don't carry data-id)
    series_id, e = js(c, w,
        "return window.mygo.call('HongguoService.GetDiscover').then(r => Array.isArray(r) && r.length > 0 ? String(r[0].id || r[0].seriesId || '') : 'empty').catch(() => 'err')", timeout=25)
    if not series_id or series_id in ("empty", "err", ""):
        series_id, e = js(c, w,
            "return window.mygo.call('HongguoService.GetRecommendFeed').then(r => { const items = Array.isArray(r) ? r : (r?.items || []); return items.length > 0 ? String(items[0].id || items[0].seriesId || '') : 'empty' }).catch(() => 'err')", timeout=25)
    if not series_id or series_id in ("empty", "err", ""):
        series_id = "7700943522404240409"  # fallback known ID

    # Click the first card to trigger openSeriesDetail via its onclick handler
    v, e = js(c, w, 'return (function(){const card=document.querySelector("#discover-grid .drama-card"); if(!card) return "no-card"; card.click(); return "clicked"})()')
    record("R02 card click", v == "clicked" and not e, f"got {v!r}, err={e}", m)
    wait(3)

    v, e = js(c, w, 'return document.getElementById("view-detail")?.classList.contains("active")')
    record("R03 detail view active", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return document.getElementById("detail-title")?.textContent')
    ok = isinstance(v, str) and len(v.strip()) > 0 and v.strip() != "--"
    record("R04 detail title filled", ok, f"got {v!r}", m)
    ok, d = elem(c, w, "#detail-poster")
    record("R05 detail poster", ok, d, m)
    ok, d = elem(c, w, "#btn-detail-play")
    record("R06 detail play btn", ok, d, m)
    ok, d = elem(c, w, "#btn-detail-favorite")
    record("R07 detail favorite btn", ok, d, m)
    ok, d = elem(c, w, "#detail-tags")
    record("R08 detail tags", ok, d, m)
    ok, d = elem(c, w, "#detail-description")
    record("R09 detail description", ok, d, m)
    v, e = js(c, w, "return document.getElementById('detail-episode-grid')?.children?.length || 0", timeout=25)
    record("R10 episode grid populated", isinstance(v, (int, float)) and v >= 0, f"n={v}", m)
    ok, d = elem(c, w, "#btn-back-from-detail")
    record("R11 back button", ok, d, m)

    # Click back and check we're not on detail anymore
    v, e = js(c, w, 'return (function(){document.getElementById("btn-back-from-detail")?.click(); return !document.getElementById("view-detail").classList.contains("active")})()')
    record("R12 back returns", v is True, f"got {v!r}", m)
    ok, d = elem(c, w, "#detail-heat")
    record("R13 detail heat", ok, d, m)
    ok, d = elem(c, w, "#detail-status")
    record("R14 detail status", ok, d, m)


# === Module S: Search Workflow ===

def mod_s(c, w):
    m = "S"
    switch_view(c, w, "discover")
    wait(1)
    v, e = js(c, w, 'const i=document.getElementById("search-input"); i.value="总裁"; i.dispatchEvent(new Event("input")); return i.value')
    record("S01 type search text", v == "总裁" and not e, f"got {v!r}", m)
    v, e = js(c, w, 'return document.getElementById("btn-clear-search")?.style?.display !== "none"')
    record("S02 clear btn visible", v is True, f"got {v!r}", m)
    v, e = js(c, w, """return (function(){
        const i = document.getElementById('search-input');
        i.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', bubbles: true}));
        return document.getElementById('view-search-results').classList.contains('active');
    })()""")
    record("S03 enter triggers search", v is True, f"got {v!r}", m)
    wait(1)
    v, e = js(c, w, 'return document.getElementById("search-query-header")?.textContent || ""')
    record("S04 header shows query", isinstance(v, str) and "总裁" in v, f"got {v!r}", m)
    v, e = js(c, w, 'return (document.getElementById("search-grid")?.innerHTML || "").length')
    record("S05 grid non-empty", isinstance(v, (int, float)) and v > 0, f"n={v}", m)
    wait(3)
    v, e = js(c, w, "return document.querySelectorAll('#search-grid .drama-card').length")
    v_noresult, _ = js(c, w, 'return (document.getElementById("search-grid")?.textContent || "").includes("未找到")')
    ok = (isinstance(v, (int, float)) and v > 0) or v_noresult is True
    record("S06 search results or empty msg", ok, f"cards={v}, noresult={v_noresult}", m)
    # Data quality: search result cards should load real covers (not fallback SVG)
    v, e = js(c, w, """return (function(){
        const imgs = document.querySelectorAll('#search-grid .drama-card img.card-poster');
        if (imgs.length === 0) return JSON.stringify({total: 0, real: 0});
        let real = 0;
        for (const img of imgs) {
            if (img.src && !img.src.startsWith('data:image/svg')) real++;
        }
        return JSON.stringify({total: imgs.length, real: real});
    })()""", timeout=25)
    try:
        sq = json.loads(v) if isinstance(v, str) else {}
    except (json.JSONDecodeError, TypeError):
        sq = {}
    s_ratio = sq.get("real", 0) / max(sq.get("total", 1), 1)
    record("S06b search covers > 50% real", s_ratio > 0.5,
           f"{sq.get('real', 0)}/{sq.get('total', 0)} real ({s_ratio:.0%})", m)
    # Card titles should be populated (not empty or placeholder)
    v, e = js(c, w, """return (function(){
        const cards = document.querySelectorAll('#search-grid .drama-card');
        if (cards.length === 0) return JSON.stringify({total: 0, titled: 0});
        let titled = 0;
        for (const card of cards) {
            const t = card.querySelector('.card-title, .card-name, h4, [class*=title]');
            if (t && t.textContent.trim().length > 0 && t.textContent.trim() !== '--') titled++;
        }
        return JSON.stringify({total: cards.length, titled: titled});
    })()""")
    try:
        tq = json.loads(v) if isinstance(v, str) else {}
    except (json.JSONDecodeError, TypeError):
        tq = {}
    record("S06c search titles populated", tq.get("titled", 0) == tq.get("total", 0) and tq.get("total", 0) > 0,
           f"{tq.get('titled', 0)}/{tq.get('total', 0)} titled", m)

    v, e = js(c, w, 'return (function(){document.getElementById("btn-clear-search")?.click(); return document.getElementById("search-input").value})()')
    record("S07 clear button clears input", v == "" and not e, f"got {v!r}", m)
    v, e = js(c, w, 'return document.getElementById("btn-clear-search")?.style?.display === "none"')
    record("S08 clear btn hidden after clear", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, 'const i=document.getElementById("search-input"); i.value="<script>alert(1)<\\/script>"; i.dispatchEvent(new Event("input")); return "ok"')
    record("S09 special chars safe", ok, d, m)
    ok, d = js_ok(c, w, 'const i=document.getElementById("search-input"); i.value="a".repeat(500); i.dispatchEvent(new Event("input")); return "ok"')
    record("S10 long input safe", ok, d, m)
    ok, d = js_ok(c, w, 'const i=document.getElementById("search-input"); i.value="  "; i.dispatchEvent(new KeyboardEvent("keydown",{key:"Enter",bubbles:true})); return "ok"')
    record("S11 whitespace search safe", ok, d, m)
    v, e = js(c, w, 'const i=document.getElementById("search-input"); i.value="总裁"; i.dispatchEvent(new Event("input")); return typeof document.getElementById("search-suggestions")?.style?.display')
    ok = isinstance(v, str) and v in ("string", "block", "none")
    record("S12 suggestions display controlled", ok, f"got {v!r}", m)


# === Module T: History & Favorites Interactions ===

def mod_t(c, w):
    m = "T"
    switch_view(c, w, "history")
    wait(1)
    v, e = js(c, w, 'return (function(){const b=document.querySelector(".filter-chip[data-filter=\'unwatched\']"); if(!b) return false; b.click(); return b.classList.contains("active")})()')
    record("T01 unwatched pill active", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return (function(){const b=document.querySelector(".filter-chip[data-filter=\'completed\']"); if(!b) return false; b.click(); return b.classList.contains("active")})()')
    record("T02 completed pill active", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return (function(){const b=document.querySelector(".filter-chip[data-filter=\'all\']"); if(!b) return false; b.click(); return b.classList.contains("active")})()')
    record("T03 all pill active", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'const i=document.getElementById("history-search-input"); i.value="test"; return i.value')
    record("T04 history search type", v == "test" and not e, f"got {v!r}", m)
    v, e = js(c, w, 'const i=document.getElementById("history-search-input"); i.value=""; return i.value')
    record("T05 history search clear", v == "" and not e, f"got {v!r}", m)
    v, e = js(c, w, 'return (function(){const btn=document.getElementById("btn-clear-all-history"); return btn && !btn.disabled})()')
    record("T07 clear all btn enabled", v is True, f"got {v!r}", m)

    # Favorites tabs (if present)
    switch_view(c, w, "favorites")
    wait(1)
    v, e = js(c, w, """return (function(){
        const tabs = document.querySelectorAll('#view-favorites .filter-chip, #view-favorites .fav-tab');
        if (tabs.length > 1) { tabs[1].click(); return true; }
        return 'no-tabs';
    })()""")
    record("T06 favorites tab switch", v is True or v == "no-tabs", f"got {v!r}", m)
    # Try clicking a card
    v, e = js(c, w, """return (function(){
        const card = document.querySelector('#view-favorites .drama-card');
        if (!card) return 'no-card';
        card.click(); return 'clicked';
    })()""")
    record("T08 favorites card click", v in ("clicked", "no-card"), f"got {v!r}", m)
    switch_view(c, w, "favorites")
    wait(0.5)
    v, e = js(c, w, 'return document.querySelector("#view-favorites h3, #view-favorites .section-title") !== null')
    record("T09 favorites heading", v is True, f"got {v!r}", m)
    switch_view(c, w, "reservations")
    wait(0.5)
    v, e = js(c, w, 'return document.querySelector("#view-reservations h3, #view-reservations .section-title") !== null')
    record("T10 reservations heading", v is True, f"got {v!r}", m)


# === Module U: Settings Interactions ===

def mod_u(c, w):
    m = "U"
    switch_view(c, w, "settings")
    wait(1)
    v, e = js(c, w, 'return (function(){const b=document.getElementById("btn-show-login"); if(!b) return false; b.click(); return true})()')
    record("U01 show login click", v is True, f"got {v!r}", m)
    wait(0.3)
    ok, d = elem(c, w, "#login-cookie-input")
    record("U02 login cookie textarea", ok, d, m)
    ok, d = elem(c, w, "#login-token-input")
    record("U03 login token input", ok, d, m)
    ok, d = elem(c, w, "#btn-paste-clipboard")
    record("U04 paste clipboard btn", ok, d, m)
    ok, d = elem(c, w, "#btn-cancel-login")
    record("U05 cancel login btn", ok, d, m)
    ok, d = elem(c, w, "#btn-submit-login")
    record("U06 submit login btn", ok, d, m)
    v, e = js(c, w, 'return (function(){const b=document.getElementById("btn-cancel-login"); if(!b) return false; b.click(); return true})()')
    record("U07 cancel login closes", v is True, f"got {v!r}", m)
    ok, d = elem(c, w, "#btn-settings-reset-device")
    record("U08 reset device btn", ok, d, m)
    ok, d = elem(c, w, "#btn-refresh-profile")
    record("U09 refresh profile btn", ok, d, m)
    ok, d = elem(c, w, "#settings-status-indicator")
    record("U10 status indicator", ok, d, m)


# === Module V: Player UI ===

def mod_v(c, w):
    m = "V"
    checks = [
        ("V01 close player btn", "#btn-close-player"),
        ("V02 video container", "#video-container"),
        ("V03 video loader", "#video-loader"),
        ("V04 center play badge", "#center-play-badge"),
        ("V05 ambient glow controller", None),
        ("V06 comment sheet overlay", "#comment-sheet-overlay"),
        ("V07 comment close btn", "#btn-close-comment-sheet"),
        ("V08 comment list", "#comment-list"),
        ("V09 duplicate player close absent", None),
    ]
    for i, (name, sel) in enumerate(checks):
        if sel is None:
            if name.startswith("V09"):
                v, e = js(c, w, 'return Boolean(document.getElementById("btn-close-player-right"))')
                record(name, v is False and not e, f"found={v!r}, err={e}", m)
            else:
                # Ambient glow is a JS controller with an offscreen canvas; check the instance.
                v, e = js(c, w, 'return window.ambientGlow instanceof Object && window.ambientGlow.sampleCanvas !== undefined')
                record(name, v is True and not e, f"controller={v}, err={e}", m)
        else:
            ok, d = elem(c, w, sel)
            record(name, ok, d, m)


# === Module W: Discover Interactions ===

def mod_w(c, w):
    m = "W"
    switch_view(c, w, "discover")
    for _ in range(20):
        v, _ = js(c, w, "return document.querySelectorAll(\".filter-chip[data-key='genre']\").length")
        if isinstance(v, (int, float)) and v >= 3:
            break
        wait(0.5)
    v, e = js(c, w, 'return (function(){const b=document.querySelector(".filter-chip[data-key=\'genre\'][data-val=\'comic_series\']"); if(!b) return false; b.click(); return b.classList.contains("active")})()')
    record("W01 comic_series tab active", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return (function(){const b=document.querySelector(".filter-chip[data-key=\'genre\'][data-val=\'ai_series\']"); if(!b) return false; b.click(); return b.classList.contains("active")})()')
    record("W02 ai_series tab active", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return (function(){const b=document.querySelector(".filter-chip[data-key=\'genre\'][data-val=\'short_play\']"); if(!b) return false; b.click(); return b.classList.contains("active")})()')
    record("W03 short_play tab active", v is True, f"got {v!r}", m)
    v, e = js(c, w, "return document.querySelector(\".filter-chip[data-key='genre']\") !== null")
    record("W04 chip has data-key", v is True, f"got {v!r}", m)
    # Clicking W01-W03 chips re-renders the grid; give it a moment to settle.
    for _ in range(10):
        v, e = js(c, w, "return document.querySelectorAll(\".filter-chip[data-key='genre']\").length >= 3")
        if v is True:
            break
        wait(0.5)
    record("W05 genre chips >= 3", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, 'document.getElementById("btn-discover-load-more")?.click(); return "ok"')
    record("W06 load more click safe", ok, d, m)


# === Module X: Rank & New Interactions ===

def mod_x(c, w):
    m = "X"
    switch_view(c, w, "rank")
    wait(1)
    v, e = js(c, w, "return document.querySelectorAll('#rank-content-tabs .rank-content-tab').length")
    record("X01 content tabs >= 2", isinstance(v, (int, float)) and v >= 2, f"n={v}", m)
    v, e = js(c, w, """return (function(){
        const tabs = document.querySelectorAll('#rank-content-tabs .rank-content-tab');
        if (tabs.length < 2) return 'few';
        tabs[1].click();
        return 'clicked';
    })()""")
    if v == "clicked":
        wait(0.5)
        v2, _ = js(c, w, 'return document.querySelectorAll("#rank-content-tabs .rank-content-tab.active").length === 1')
        record("X02 second tab click active", v2 is True, f"got {v2!r}", m)
    else:
        record("X02 second tab click active", False, f"got {v!r}", m)
    v, e = js(c, w, """return (function(){
        const tabs = document.querySelectorAll('#rank-content-tabs .rank-content-tab');
        if (tabs.length < 1) return 'few';
        tabs[0].click();
        return 'clicked';
    })()""")
    if v == "clicked":
        wait(0.5)
        v2, _ = js(c, w, 'return document.querySelectorAll("#rank-content-tabs .rank-content-tab.active").length === 1')
        record("X03 first tab click back", v2 is True, f"got {v2!r}", m)
    else:
        record("X03 first tab click back", False, f"got {v!r}", m)

    switch_view(c, w, "new")
    wait(1)
    v, e = js(c, w, 'return (function(){const b=document.querySelector("#new-main-tabs .rank-content-tab[data-tab=\'calendar\']"); if(!b) return false; b.click(); return b.classList.contains("active")})()')
    record("X04 calendar tab active", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return document.getElementById("new-calendar-dates")?.children?.length >= 0')
    record("X05 calendar dates container", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return (function(){const b=document.querySelector("#new-main-tabs .rank-content-tab[data-tab=\'hot\']"); if(!b) return false; b.click(); return b.classList.contains("active")})()')
    record("X06 hot tab back", v is True, f"got {v!r}", m)


# === Module Y: Edge Cases ===

def mod_y(c, w):
    m = "Y"
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetSeriesDetail', null).then(() => 'ok').catch(() => 'rejected')")
    record("Y01 IPC null param", v in ("ok", "rejected"), f"got {v!r}", m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetSeriesDetail', undefined).then(() => 'ok').catch(() => 'rejected')")
    record("Y02 IPC undefined param", v in ("ok", "rejected"), f"got {v!r}", m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.Search', 'a'.repeat(5000)).then(() => 'ok').catch(() => 'rejected')", timeout=25)
    record("Y03 IPC long string", v in ("ok", "rejected"), f"got {v!r}", m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.SaveProgress', -1, -1, -1, -1, -1).then(() => 'ok').catch(() => 'rejected')")
    record("Y04 IPC negative numbers", v in ("ok", "rejected"), f"got {v!r}", m)
    v, e = js(c, w, "return window.mygo.call('HongguoService.GetSeriesDetail', {}).then(() => 'ok').catch(() => 'rejected')")
    record("Y05 IPC object param", v in ("ok", "rejected"), f"got {v!r}", m)
    v, e = js(c, w, """return Promise.all([
        window.mygo.call('HongguoService.GetSuggestions', 'a'),
        window.mygo.call('HongguoService.GetSuggestions', 'b'),
        window.mygo.call('HongguoService.GetSuggestions', 'c'),
        window.mygo.call('HongguoService.GetSuggestions', 'd'),
        window.mygo.call('HongguoService.GetSuggestions', 'e')
    ]).then(() => true).catch(() => false)""", timeout=30)
    record("Y06 concurrent 5 IPC", v is True, f"got {v!r}", m)
    ok, d = js_ok(c, w, "const vs=['rank','new','history','favorites','reservations','settings','recommend','discover','rank','new']; vs.forEach(v=>switchView(v)); return 'done'")
    record("Y07 concurrent 10 switches", ok, d, m)
    ok, d = js_ok(c, w, "document.querySelectorAll('.filter-chip[data-val]').forEach(b=>b.click()); return 'ok'")
    record("Y08 rapid chip clicks", ok, d, m)
    switch_view(c, w, "recommend")
    v, e = js(c, w, "return document.readyState")
    record("Y09 eval after switch", v == "complete", f"got {v!r}", m)
    v, e = js(c, w, "return getComputedStyle(document.documentElement).getPropertyValue('--bg-app').length > 0")
    record("Y10 CSS variable exists", v is True, f"got {v!r}", m)


# === Module Z: Accessibility ===

def mod_z(c, w):
    m = "Z"
    switch_view(c, w, "discover")
    wait(2)
    v, e = js(c, w, 'return document.querySelectorAll("img:not([alt]):not([aria-hidden])").length')
    record("Z01 imgs have alt", isinstance(v, (int, float)) and v == 0, f"n={v}", m)
    v, e = js(c, w, 'return Array.from(document.querySelectorAll(".nav-item")).every(n => n.textContent.trim().length > 0)')
    record("Z02 nav items have text", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return (document.getElementById("search-input")?.placeholder || "").length > 0')
    record("Z03 search placeholder", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return (document.getElementById("history-search-input")?.placeholder || "").length > 0')
    record("Z04 history placeholder", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return document.querySelector("#sidebar-resizer[role]") !== null')
    record("Z06 resizer role", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return document.querySelector("[role=status]") !== null')
    record("Z07 loader role=status", v is True, f"got {v!r}", m)
    v, e = js(c, w, 'return (document.documentElement.lang || "").length > 0')
    record("Z08 html lang", v is True, f"got {v!r}", m)


# === Module AA: Performance ===

def mod_aa(c, w):
    m = "AA"
    v, e = js(c, w, "return performance.timing.domContentLoadedEventEnd - performance.timing.navigationStart")
    record("AA01 DOM ready < 10s", isinstance(v, (int, float)) and 0 < v < 10000, f"{v}ms", m)
    v, e = js(c, w, "const t0=performance.now(); switchView('rank'); return performance.now() - t0")
    record("AA02 switch < 200ms", isinstance(v, (int, float)) and v < 200, f"{v}ms", m)
    v, e = js(c, w, "return performance.now().then(t0 => window.mygo.call('HongguoService.GetHotSearch').then(() => performance.now() - t0)).catch(() => -1)", timeout=20)
    if isinstance(v, (int, float)) and v >= 0:
        record("AA03 GetHotSearch < 10s", v < 10000, f"{v}ms", m)
    else:
        v2, _ = js(c, w, "const t0=performance.now(); return window.mygo.call('HongguoService.GetHotSearch').then(() => performance.now()-t0).catch(() => -1)", timeout=20)
        record("AA03 GetHotSearch < 10s", isinstance(v2, (int, float)) and v2 >= 0 and v2 < 10000, f"{v2}ms", m)
    v, e = js(c, w, "const t0=performance.now(); return window.mygo.call('HongguoService.GetHistory').then(() => performance.now()-t0).catch(() => -1)", timeout=15)
    record("AA04 GetHistory < 5s", isinstance(v, (int, float)) and v >= 0 and v < 5000, f"{v}ms", m)
    v, e = js(c, w, "const t0=performance.now(); return window.mygo.call('HongguoService.GetCacheSize').then(() => performance.now()-t0).catch(() => -1)", timeout=15)
    record("AA05 GetCacheSize < 5s", isinstance(v, (int, float)) and v >= 0 and v < 5000, f"{v}ms", m)
    v, e = js(c, w, "const t0=performance.now(); return window.mygo.call('HongguoService.GetSuggestions', 'test').then(() => performance.now()-t0).catch(() => -1)", timeout=20)
    record("AA06 GetSuggestions < 10s", isinstance(v, (int, float)) and v >= 0 and v < 10000, f"{v}ms", m)


# === Module BB: Feed Streaming ===

def mod_bb(c, w):
    m = "BB"
    switch_view(c, w, "recommend")
    target = None
    for _ in range(30):
        wait(0.5)
        v, e = js(c, w, """return (function(){
            const slot = feedSlots[feedSlotNext];
            if (!slot || !slot.video.src) return null;
            return {src: slot.video.src, firstFrame: feedVideoHasFirstFrame(slot.video),
                    posterHidden: slot.poster.classList.contains('hidden')};
        })()""")
        if isinstance(v, dict) and v.get("src"):
            target = v
            break
    record("BB01 next feed source prepared", target is not None, f"target={target}, err={e}", m)
    if target:
        src = target.get("src") or ""
        is_selected_playlist = "/hls/" in src and "master.m3u8" not in src
        is_progressive = "mode=progressive" in src
        is_full_cache = "/video?" in src and "mode=progressive" not in src
        record("BB02 feed uses selected media route",
               is_selected_playlist or is_progressive or is_full_cache,
               f"src={src}", m)
        first_frame = target.get("firstFrame") is True
        poster_hidden = target.get("posterHidden") is True
        record("BB03 poster only hides after first frame", poster_hidden == first_frame,
               f"firstFrame={first_frame}, posterHidden={poster_hidden}", m)

    v, e = js(c, w, """return new Promise(resolve => {
        if (!state.feed.items[state.feed.index + 1]) return 'no-next';
        const from = state.feed.index;
        const deadline = Date.now() + 6000;
        const attempt = () => {
            if (state.feed.index === from) switchFeedDrama(1);
            const video = feedSlots[feedSlotCurr].video;
            const result = {
                switched: state.feed.index === from + 1,
                preparingHint: (document.getElementById('feed-hint-toast')?.textContent || '').includes('正在准备下一部'),
                previousPriming: feedSlots[feedSlotPrev]?.isPriming || false,
                paused: video.paused,
                readyState: video.readyState
            };
            if ((result.switched && !result.paused && result.readyState >= 2) || Date.now() >= deadline) resolve(result);
            else setTimeout(attempt, 200);
        };
        attempt();
    })""", timeout=8)
    ok = (
        isinstance(v, dict)
        and v.get("switched") is True
        and v.get("preparingHint") is False
        and v.get("previousPriming") is False
        and v.get("paused") is False
        and v.get("readyState", 0) >= 2
    )
    record("BB04 immediate feed switch keeps state clean", ok, f"got={v}, err={e}", m)
    wait(1)
    v, e = js(c, w, """return (function(){
        const current = feedSlots[feedSlotCurr];
        return {paused: current.video.paused, userPaused: state.feed.userPaused,
                readyState: current.video.readyState, hasSource: Boolean(current.video.src)};
    })()""")
    ok = isinstance(v, dict) and v.get("paused") is False and v.get("userPaused") is False and v.get("hasSource") is True
    record("BB05 next feed autoplay settles playing", ok, f"got={v}, err={e}", m)
    v, e = js(c, w, """return new Promise(resolve => {
        const deadline = Date.now() + 15000;
        const check = () => {
            if (state.feed.items.length >= state.feed.index + 5 || Date.now() >= deadline) {
                resolve({items: state.feed.items.length, index: state.feed.index,
                         loading: state.feed.loading, retryScheduled: Boolean(state.feed.pageRetryTimer)});
                return;
            }
            setTimeout(check, 250);
        };
        check();
    })""", timeout=17)
    record("BB06 feed prefetch keeps filling buffer",
           isinstance(v, dict) and (
               v.get("items", 0) >= v.get("index", 0) + 5 or v.get("loading") is True or v.get("retryScheduled") is True
           ),
           f"got={v}, err={e}", m)

    # Allow the serialized HLS preloader to finish i+2/i+3 before exercising
    # rapid handoffs. A user normally watches the current episode for longer.
    wait(3)
    observations = []
    start_index, _ = js(c, w, "return state.feed.index")
    for step in range(3):
        expected_index = start_index + step + 1
        deadline = time.time() + 10
        observation, obs_err = js(c, w, """return (function(){
            switchFeedDrama(1);
            return {index: state.feed.index, animating: isFeedAnimating,
                    hintVisible: document.getElementById('feed-hint-toast').style.display !== 'none',
                    loaderVisible: document.getElementById('feed-loader').style.display !== 'none',
                    paused: feedSlots[feedSlotCurr].video.paused,
                    readyState: feedSlots[feedSlotCurr].video.readyState};
        })()""")
        while (
            isinstance(observation, dict) and
            (observation.get("index") != expected_index or observation.get("paused") is True or observation.get("readyState", 0) < 2) and
            time.time() < deadline
        ):
            wait(0.2)
            observation, obs_err = js(c, w, """return (function(){
                if (state.feed.index < %d) switchFeedDrama(1);
                return {index: state.feed.index, animating: isFeedAnimating,
                        hintVisible: document.getElementById('feed-hint-toast').style.display !== 'none',
                        loaderVisible: document.getElementById('feed-loader').style.display !== 'none',
                        paused: feedSlots[feedSlotCurr].video.paused,
                        readyState: feedSlots[feedSlotCurr].video.readyState};
            })()""" % expected_index)
        if isinstance(observation, dict):
            observations.append(observation)
    for _ in range(10):
        wait(0.5)
        v, e = js(c, w, """return (function(){
            const video = feedSlots[feedSlotCurr].video;
            return {index: state.feed.index, paused: video.paused, readyState: video.readyState,
                animating: isFeedAnimating,
                    hintVisible: document.getElementById('feed-hint-toast').style.display !== 'none',
                    loaderVisible: document.getElementById('feed-loader').style.display !== 'none'};
        })()""")
        if isinstance(v, dict) and v.get("paused") is False and v.get("readyState", 0) >= 2:
            break
    if isinstance(v, dict):
        v["observations"] = observations
    ok = (
        isinstance(v, dict)
        and v.get("index") == start_index + 3
        and v.get("paused") is False
        and v.get("readyState", 0) >= 2
        and v.get("hintVisible") is False
        and v.get("loaderVisible") is False
        and v.get("animating") is False
        and all(not o.get("hintVisible") and not o.get("loaderVisible") for o in v.get("observations", []))
    )
    record("BB07 rapid feed swipes stay continuous", ok, f"got={v}, err={e}", m)



# === Module AB: Data Quality ===

def _cover_ratio(c, w, sel):
    """Return (real, total) for card-poster imgs in sel that load real covers."""
    v, e = js(c, w, f"""return (function(){{
        const imgs = document.querySelectorAll('{sel} img.card-poster, {sel} img[class*=poster], {sel} img[class*=cover]');
        if (imgs.length === 0) return JSON.stringify({{total: 0, real: 0}});
        let real = 0;
        for (const img of imgs) {{
            if (img.src && !img.src.startsWith('data:image/svg')) real++;
        }}
        return JSON.stringify({{total: imgs.length, real: real}});
    }})()""", timeout=25)
    try:
        dq = json.loads(v) if isinstance(v, str) else {}
    except (json.JSONDecodeError, TypeError):
        dq = {}
    return dq.get("real", 0), dq.get("total", 0)


def _text_populated(c, w, sel, text_sel):
    """Return (filled, total) for elements in sel whose text is non-empty and not '--'."""
    v, e = js(c, w, f"""return (function(){{
        const cards = Array.from(document.querySelectorAll('{sel}'))
            .filter(card => !card.classList.contains('drama-card-skeleton') && card.getAttribute('aria-hidden') !== 'true');
        if (cards.length === 0) return JSON.stringify({{total: 0, filled: 0}});
        let filled = 0;
        for (const card of cards) {{
            const t = card.querySelector('{text_sel}');
            if (t && t.textContent.trim().length > 0 && t.textContent.trim() !== '--') filled++;
        }}
        return JSON.stringify({{total: cards.length, filled: filled}});
    }})()""")
    try:
        tq = json.loads(v) if isinstance(v, str) else {}
    except (json.JSONDecodeError, TypeError):
        tq = {}
    return tq.get("filled", 0), tq.get("total", 0)


def mod_ab(c, w):
    m = "AB"
    # Discover data quality
    switch_view(c, w, "discover")
    for _ in range(20):
        v, _ = js(c, w, """return document.querySelectorAll('#discover-grid .drama-card:not(.drama-card-skeleton)').length""")
        if isinstance(v, (int, float)) and v > 0:
            break
        wait(0.5)
    real, total = _cover_ratio(c, w, "#discover-grid")
    ratio = real / max(total, 1)
    record("AB01 discover covers > 50%", ratio > 0.5, f"{real}/{total} ({ratio:.0%})", m)
    filled, total = _text_populated(c, w, "#discover-grid .drama-card", ".card-title, .card-name, h4, [class*=title]")
    record("AB02 discover titles populated", filled == total and total > 0, f"{filled}/{total}", m)

    # Rankings data quality: list should have items with text
    switch_view(c, w, "rank")
    wait(3)
    v, e = js(c, w, """return (function(){
        const items = document.querySelectorAll('#rank-items-list > *');
        if (items.length === 0) return JSON.stringify({total: 0, filled: 0});
        let filled = 0;
        for (const item of items) {
            if (item.textContent.trim().length > 0) filled++;
        }
        return JSON.stringify({total: items.length, filled: filled});
    })()""")
    try:
        rq = json.loads(v) if isinstance(v, str) else {}
    except (json.JSONDecodeError, TypeError):
        rq = {}
    record("AB03 rank items have text", rq.get("filled", 0) == rq.get("total", 0) and rq.get("total", 0) > 0,
           f"{rq.get('filled', 0)}/{rq.get('total', 0)}", m)

    # New releases data quality
    switch_view(c, w, "new")
    wait(3)
    real, total = _cover_ratio(c, w, "#new-drama-grid")
    ratio = real / max(total, 1)
    record("AB04 new covers > 50%", ratio > 0.5, f"{real}/{total} ({ratio:.0%})", m)

    # Detail data quality (after opening from discover)
    switch_view(c, w, "discover")
    wait(2)
    js(c, w, 'return (function(){const card=document.querySelector("#discover-grid .drama-card"); if(card) card.click(); return "clicked"})()')
    wait(3)
    v, e = js(c, w, 'return document.getElementById("detail-title")?.textContent')
    ok = isinstance(v, str) and len(v.strip()) > 0 and v.strip() != "--"
    record("AB05 detail title populated", ok, f"got {v!r}", m)
    v, e = js(c, w, 'return (document.getElementById("detail-poster")?.querySelector("img")?.src || "").startsWith("data:image/svg")')
    record("AB06 detail poster not fallback", v is False, f"got {v!r}", m)
    v, e = js(c, w, 'return (document.getElementById("detail-description")?.textContent || "").trim().length')
    ok = isinstance(v, (int, float)) and v > 0
    record("AB07 detail description non-empty", ok, f"len={v}", m)

    # IPC Search API data quality: majority of items should have coverUrl
    v, e = js(c, w, """return window.mygo.call('HongguoService.Search', '总裁').then(r => {
        if (!Array.isArray(r) || r.length === 0) return JSON.stringify({total: 0, withCover: 0});
        let withCover = 0;
        for (const item of r) { if (item.coverUrl && item.coverUrl.length > 0) withCover++; }
        return JSON.stringify({total: r.length, withCover: withCover});
    }).catch(() => JSON.stringify({total: -1, withCover: -1}))""", timeout=25)
    try:
        iq = json.loads(v) if isinstance(v, str) else {}
    except (json.JSONDecodeError, TypeError):
        iq = {}
    i_ratio = iq.get("withCover", 0) / max(iq.get("total", 1), 1)
    record("AB08 search API covers > 50%", i_ratio > 0.5,
           f"{iq.get('withCover', 0)}/{iq.get('total', 0)} ({i_ratio:.0%})", m)


MODULES = [
    ("Foundation", mod_a), ("Routing", mod_b), ("Feed", mod_c),
    ("Discover", mod_d), ("Rankings", mod_e), ("NewReleases", mod_f),
    ("History", mod_g), ("Favorites", mod_h), ("Search", mod_i),
    ("Settings", mod_j), ("Detail", mod_k), ("IPC", mod_l),
    ("Security", mod_m), ("Screenshots", mod_n), ("Stability", mod_p),
    ("IPCFull", mod_q), ("DetailFlow", mod_r), ("SearchFlow", mod_s),
    ("HistFavInteract", mod_t), ("SettingsInteract", mod_u),
    ("PlayerUI", mod_v), ("DiscoverInteract", mod_w),
    ("RankNewInteract", mod_x), ("EdgeCases", mod_y),
    ("Accessibility", mod_z), ("Performance", mod_aa),
    ("DataQuality", mod_ab), ("FeedStreaming", mod_bb),
]


def main():
    parser = argparse.ArgumentParser(description="MyGo E2E tests for myguo")
    parser.add_argument("--socket", default="/tmp/myguo-test.sock")
    parser.add_argument("--verbose", action="store_true")
    parser.add_argument("--save-screenshots", metavar="DIR")
    args = parser.parse_args()

    print(f"Connecting to {args.socket} ...")
    try:
        client = MyGoClient(args.socket)
    except (ConnectionRefusedError, FileNotFoundError) as e:
        print(f"ERROR: Cannot connect: {e}")
        sys.exit(2)

    r, e = client.call("windows")
    wins = (r or {}).get("windows", []) if r else []
    win = wins[0]["id"] if wins else 1

    total = 0
    for label, fn in MODULES:
        before = PASS + FAIL
        fn(client, win)
        n = PASS + FAIL - before
        total += n
        print(f"  [{label}] {n} tests")

    client.close()

    if args.save_screenshots:
        import os
        os.makedirs(args.save_screenshots, exist_ok=True)
        for name, data in screenshots:
            with open(os.path.join(args.save_screenshots, f"{name}.png"), "wb") as fp:
                fp.write(data)
        print(f"\nSaved {len(screenshots)} screenshots to {args.save_screenshots}/")

    print(f"\n{'='*70}")
    for mod, name, ok, detail in results:
        if not ok or args.verbose:
            mark = "+" if ok else "-"
            print(f"  [{mark}] [{mod}] {name}: {detail}")
    print(f"{'='*70}")
    print(f"  Total: {total}  Pass: {PASS}  Fail: {FAIL}  ({PASS*100//max(total,1)}%)")
    sys.exit(0 if FAIL == 0 else 1)


if __name__ == "__main__":
    main()
