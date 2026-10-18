#!/usr/bin/env python3
"""Checks the browser extension in a real Chromium on a real Jellyfin web player.

The extension is configured through its options page, then used like a person
would: play a movie in Jellyfin, press the bookmark button twice, and press
Shift+B after seeking. The test then reads the bookmarks back from Cliparr.

Usage: extension_checks.py EXTENSION_DIR CLIPARR_URL JELLYFIN_URL
EXTENSION_DIR is the output of `scripts/build-extension.sh 0.0.0 --test-hosts "http://127.0.0.1/*"`.
Set CHROMIUM=/path/to/chromium to use an installed browser.
"""
import json
import os
import sys
import tempfile

from playwright.sync_api import expect, sync_playwright

from common import (bearer, check, cliparr_login, find_test_movie, finish, jellyfin_login, jellyfin_token_header, request,
                    wait_for)

extension, cliparr, jellyfin = os.path.abspath(sys.argv[1]), sys.argv[2].rstrip("/"), sys.argv[3].rstrip("/")
PLAYBACK_TIMEOUT = 150

alice = cliparr_login(cliparr, "alice", "pw")
item = find_test_movie(cliparr, alice)
jf_headers = jellyfin_token_header(jellyfin_login(jellyfin, "alice", "pw")["AccessToken"])


def clips() -> list:
    return sorted(request(cliparr, "GET", "/api/clips", headers=bearer(alice))[1], key=lambda c: c["id"])


def jellyfin_knows_playback() -> bool:
    _, sessions = request(jellyfin, "GET", "/Sessions", headers=jf_headers)
    return any(s.get("NowPlayingItem") for s in sessions or [])


with sync_playwright() as p, tempfile.TemporaryDirectory() as profile:
    context = p.chromium.launch_persistent_context(
        profile,
        executable_path=os.environ.get("CHROMIUM") or None,
        headless=False,  # extensions need the "new" headless mode, selected below
        args=[f"--disable-extensions-except={extension}", f"--load-extension={extension}", "--headless=new",
              "--autoplay-policy=no-user-gesture-required"],
    )
    worker = context.service_workers[0] if context.service_workers else context.wait_for_event("serviceworker")
    extension_id = worker.url.split("/")[2]

    print("== options page")
    options = context.new_page()
    options.goto(f"chrome-extension://{extension_id}/options.html")
    options.fill("#jellyfin", jellyfin)
    options.fill("#cliparr", cliparr)
    options.click("button[type=submit]")
    expect(options.locator("#status")).to_contain_text("Saved")
    options.click("#test")
    expect(options.locator("#status")).to_contain_text("Connected to Cliparr")
    check("settings are saved and the connection test reaches Cliparr", True)

    print("== bookmarking in Jellyfin's own player")
    page = context.new_page()
    page.goto(f"{jellyfin}/web/index.html")
    page.fill("#txtManualName", "alice")
    page.fill("#txtManualPassword", "pw")
    page.click("button:has-text('Sign In')")
    page.wait_for_selector(".homePage, #indexPage", timeout=30000)

    stored = json.loads(page.evaluate("localStorage.getItem('jellyfin_credentials')") or "{}")
    server = (stored.get("Servers") or [{}])[0]
    check("Jellyfin keeps the login where the extension looks for it",
          bool(server.get("AccessToken") and server.get("UserId") and server.get("ManualAddress")), sorted(server))
    check("and the device id too", bool(page.evaluate("localStorage.getItem('_deviceId2')")))

    page.goto(f"{jellyfin}/web/index.html#/details?id={item['id']}")
    page.wait_for_selector("button.btnPlay", timeout=30000)
    page.locator("button.btnPlay").first.click()
    page.wait_for_function(
        "(() => { const v = document.querySelector('video'); return !!v && v.readyState >= 2 && v.currentTime > 0; })()",
        timeout=PLAYBACK_TIMEOUT * 1000,
    )
    button = page.locator("#cliparr-bookmark button.main")
    toast = page.locator("#cliparr-bookmark .toast")
    button.wait_for(state="visible", timeout=15000)
    check("the bookmark button appears on the player", True)
    wait_for("Jellyfin to hear about the playback", jellyfin_knows_playback, timeout=60, interval=1)

    before = {c["id"] for c in clips()}
    button.click()
    expect(toast).to_contain_text("Start set")
    started_at = page.evaluate("document.querySelector('video').currentTime")
    page.wait_for_timeout(5000)
    button.click()
    expect(toast).to_contain_text("Saved")
    new = [c for c in clips() if c["id"] not in before]
    check("one bookmark reached Cliparr", len(new) == 1, new)
    if new:
        clip = new[0]
        check("it is for the movie being played", clip["itemId"] == item["id"], clip["itemId"])
        check("it starts where the player was at the first click", abs(clip["startMs"] / 1000 - started_at) < 3, (clip["startMs"], started_at))
        check("it lasts about the five seconds in between", 4000 <= clip["endMs"] - clip["startMs"] <= 9000, clip["endMs"] - clip["startMs"])

    print("== a quick bookmark after seeking")
    page.evaluate("document.querySelector('video').currentTime = 300")
    page.wait_for_function("(() => { const v = document.querySelector('video'); return v.currentTime >= 299 && v.readyState >= 2; })()",
                           timeout=PLAYBACK_TIMEOUT * 1000)
    page.evaluate("document.querySelector('video').pause()")
    page.wait_for_timeout(4000)  # the web client reports a pause to Jellyfin straight away
    paused_at = page.evaluate("document.querySelector('video').currentTime")
    known = {c["id"] for c in clips()}
    page.locator("body").click(position={"x": 5, "y": 5})  # give the page keyboard focus
    earlier_toast = toast.inner_text()  # the last bookmark's message may still be showing
    page.keyboard.press("Shift+B")
    expect(toast).not_to_have_text(earlier_toast)
    expect(toast).to_contain_text("Saved")
    check("the position is exact, with nothing approximate about it", "approximate" not in toast.inner_text(), toast.inner_text())
    quick = [c for c in clips() if c["id"] not in known]
    check("Shift+B saved one bookmark", len(quick) == 1, quick)
    if quick:
        check("it ends where the video was paused", abs(quick[0]["endMs"] / 1000 - paused_at) < 1.5, (quick[0]["endMs"], paused_at))
        check("and covers the ten seconds before", quick[0]["endMs"] - quick[0]["startMs"] == 10000, quick[0])

    context.close()

finish()
