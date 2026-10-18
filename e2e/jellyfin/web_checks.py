#!/usr/bin/env python3
"""Checks the ways Cliparr hands a clip over to Jellyfin's real web client.

Opens Jellyfin's web interface in a browser as alice and checks that
  - the "open in Jellyfin web" link leads to the movie and Play continues at the clip start;
  - Cliparr sees the open web client as a device, and "play on a device" starts the clip there.

Usage: web_checks.py CLIPARR_URL JELLYFIN_URL
Set CHROMIUM=/path/to/chromium to use an installed browser.
"""
import os
import sys

from playwright.sync_api import sync_playwright

from common import (bearer, check, cliparr_login, find_test_movie, finish, jellyfin_login, jellyfin_token_header, request,
                    wait_for)

cliparr, jellyfin = sys.argv[1].rstrip("/"), sys.argv[2].rstrip("/")
START_MS = 120000
# Jellyfin transcodes for a browser without H.264, and a slow machine needs a while to start.
PLAYBACK_TIMEOUT = 150

alice = cliparr_login(cliparr, "alice", "pw")
item = find_test_movie(cliparr, alice)
_, clip = request(cliparr, "POST", "/api/clips", {"itemId": item["id"], "startMs": START_MS, "endMs": START_MS + 10000,
                                                  "quote": "", "note": "", "tags": []}, bearer(alice))
session = jellyfin_login(jellyfin, "alice", "pw")
jf_headers = jellyfin_token_header(session["AccessToken"])


def playing_position_ms(device_id=None):
    """Where Jellyfin says alice's web client is in the item, once it really plays it."""
    _, sessions = request(jellyfin, "GET", "/Sessions", headers=jf_headers)
    for s in sessions or []:
        if s.get("NowPlayingItem") and (device_id is None or s["Id"] == device_id):
            return s["PlayState"].get("PositionTicks", 0) // 10000
    return None


def sign_in(page) -> None:
    page.goto(f"{jellyfin}/web/index.html")
    page.fill("#txtManualName", "alice")
    page.fill("#txtManualPassword", "pw")
    page.click("button:has-text('Sign In')")
    page.wait_for_selector(".homePage, #indexPage", timeout=30000)


def stop_playback(page) -> None:
    page.goto(f"{jellyfin}/web/index.html#/home")
    wait_for("playback to stop", lambda: playing_position_ms() is None, timeout=30)


with sync_playwright() as p:
    browser = p.chromium.launch(executable_path=os.environ.get("CHROMIUM") or None,
                                args=["--autoplay-policy=no-user-gesture-required"])
    page = browser.new_page(viewport={"width": 1280, "height": 900})
    sign_in(page)

    print("== open in Jellyfin web")
    request(cliparr, "POST", f"/api/clips/{clip['id']}/jellyfin/resume", headers=bearer(alice))
    _, handoff = request(cliparr, "GET", f"/api/clips/{clip['id']}/jellyfin", headers=bearer(alice))
    page.goto(handoff["webUrl"])
    page.wait_for_selector("button.btnPlay", timeout=30000)
    check("the link leads to the movie's page", "Harbor Lights" in page.inner_text("body"))
    page.locator("button.btnPlay").first.click()
    position = wait_for("Jellyfin to report the web client playing", playing_position_ms, timeout=PLAYBACK_TIMEOUT, interval=2)
    check("Play continues at the clip start, not at the beginning", START_MS <= position <= START_MS + 20000, position)

    print("== play on a device")
    stop_playback(page)
    _, handoff = request(cliparr, "GET", f"/api/clips/{clip['id']}/jellyfin", headers=bearer(alice))
    web_clients = [d for d in handoff["devices"] if d["client"] == "Jellyfin Web"]
    check("Cliparr sees the open web client as a device", len(web_clients) >= 1, handoff["devices"])
    if web_clients:
        status, body = request(cliparr, "POST", f"/api/clips/{clip['id']}/jellyfin/play", {"deviceId": web_clients[0]["id"]}, bearer(alice))
        check("playing on it is accepted", status == 204, (status, body))
        position = wait_for("Jellyfin to report the web client playing", lambda: playing_position_ms(web_clients[0]["id"]),
                            timeout=PLAYBACK_TIMEOUT, interval=2)
        check("it starts at the clip start", START_MS <= position <= START_MS + 20000, position)
        status, body = request(cliparr, "POST", f"/api/clips/{clip['id']}/jellyfin/play", {"deviceId": "not-a-device"}, bearer(alice))
        check("a device that is not alice's is refused", status == 404, (status, body))
    browser.close()

finish()
