#!/usr/bin/env python3
"""End-to-end test of the browser extension in a real Chromium.

A fake Jellyfin web player (with a real video) and a real Cliparr server run
locally. The test configures the extension through its options page, plays the
video, uses the bookmark button and checks what ended up in Cliparr.

Usage: e2e.py EXTENSION_DIR CLIPARR_URL JELLYFIN_URL
Set CHROMIUM=/path/to/chromium to use an installed browser, and
SCREENSHOT_DIR=docs/screenshots to also write a picture of the button for the docs.
"""
import json
import os
import sys
import tempfile
import urllib.request

from playwright.sync_api import Page, expect, sync_playwright

failures = 0


def check(description: str, expected, actual) -> None:
    global failures
    if expected == actual:
        print(f"ok   - {description}")
    else:
        print(f"FAIL - {description} (expected {expected!r}, got {actual!r})")
        failures += 1


def cliparr_clips(cliparr: str) -> list:
    """Alice's clips, read through Cliparr's own API like any other client."""

    def call(path: str, body: dict | None = None, token: str | None = None):
        headers = {"Content-Type": "application/json"}
        if token:
            headers["Authorization"] = f"Bearer {token}"
        request = urllib.request.Request(
            cliparr + path, data=json.dumps(body).encode() if body is not None else None, headers=headers
        )
        with urllib.request.urlopen(request) as response:
            return json.load(response)

    token = call("/api/auth/login", {"username": "alice", "password": "pw"})["token"]
    return sorted(call("/api/clips", token=token), key=lambda clip: clip["id"])


def configure(context, extension_id: str, cliparr: str, jellyfin: str) -> Page:
    """Fills in the options page and saves, as a user would."""
    page = context.new_page()
    page.goto(f"chrome-extension://{extension_id}/options.html")
    page.fill("#jellyfin", jellyfin)
    page.fill("#cliparr", cliparr)
    page.click("button[type=submit]")
    expect(page.locator("#status")).to_contain_text("Saved")
    return page


def open_player(context, jellyfin: str) -> Page:
    page = context.new_page()
    page.on("pageerror", lambda e: print(f"  [player page error] {e}"))
    page.goto(f"{jellyfin}/web/index.html")
    page.wait_for_function("document.querySelector('video').readyState >= 1")
    return page


def seek(page: Page, seconds: float) -> None:
    """Pauses at an exact time and waits until the fake player has reported it to Jellyfin."""
    page.evaluate("t => { const v = document.querySelector('video'); v.pause(); v.currentTime = t; }", seconds)
    page.wait_for_timeout(1500)


def button(page: Page):
    return page.locator("#cliparr-bookmark button.main")


def toast(page: Page):
    return page.locator("#cliparr-bookmark .toast")


def main() -> int:
    extension, cliparr, jellyfin = (arg.rstrip("/") for arg in sys.argv[1:4])
    extension = os.path.abspath(extension)

    with sync_playwright() as p, tempfile.TemporaryDirectory() as profile:
        context = p.chromium.launch_persistent_context(
            profile,
            executable_path=os.environ.get("CHROMIUM") or None,
            headless=False,  # extensions need the "new" headless mode, selected below
            args=[f"--disable-extensions-except={extension}", f"--load-extension={extension}", "--headless=new"],
        )
        worker = context.service_workers[0] if context.service_workers else context.wait_for_event("serviceworker")
        extension_id = worker.url.split("/")[2]

        print("== options page ==")
        options = configure(context, extension_id, cliparr, jellyfin)
        options.click("#test")
        expect(options.locator("#status")).to_contain_text("Connected to Cliparr")
        print("ok   - settings saved and the connection test succeeds")
        options.fill("#cliparr", "not an address")
        options.click("button[type=submit]")
        expect(options.locator("#status")).to_contain_text("Enter both addresses")
        print("ok   - an invalid address is refused")

        print("== bookmarking in the Jellyfin player ==")
        player = open_player(context, jellyfin)
        button(player).wait_for(state="visible")
        print("ok   - the bookmark button appears on the player page")

        seek(player, 10)
        button(player).click()
        expect(player.locator("#cliparr-bookmark .label")).to_have_text("Set end · from 0:10")
        print("ok   - the first click marks the start")

        seek(player, 15)
        button(player).click()
        expect(toast(player)).to_contain_text("Saved 0:10–0:15")
        print("ok   - the second click saves the clip")
        if os.environ.get("SCREENSHOT_DIR"):
            player.locator("#cliparr-bookmark").screenshot(path=os.path.join(os.environ["SCREENSHOT_DIR"], "extension-button.png"))

        clips = cliparr_clips(cliparr)
        check("one bookmark reached Cliparr", 1, len(clips))
        if clips:
            first = clips[0]
            check("it is for the movie being played", "movie-rocky", first["itemId"])
            check("it starts at the exact marked time", 10000, first["startMs"])
            check("it ends at the exact marked time", 15000, first["endMs"])
            check("Cliparr knows the title", "Rocky (1976)", first["title"])

        seek(player, 30)
        player.locator("body").click(position={"x": 5, "y": 5})  # give the page keyboard focus
        player.keyboard.press("Shift+B")
        expect(toast(player)).to_contain_text("Saved 0:20–0:30")
        clips = cliparr_clips(cliparr)
        check("Shift+B saved the last ten seconds", [(20000, 30000)], [(c["startMs"], c["endMs"]) for c in clips[1:]])

        print("== problems are explained ==")
        seek(player, 40)
        button(player).click()
        seek(player, 40.2)
        button(player).click()
        expect(toast(player)).to_contain_text("under a second")
        player.keyboard.press("Escape")
        expect(toast(player)).to_contain_text("Cancelled")
        print("ok   - a clip under a second is refused and Escape cancels")

        options.fill("#cliparr", "http://127.0.0.1:9")  # nothing listens there
        options.fill("#jellyfin", jellyfin)
        options.click("button[type=submit]")
        expect(options.locator("#status")).to_contain_text("Saved")
        seek(player, 50)
        player.keyboard.press("Shift+B")
        expect(toast(player)).to_contain_text("Cliparr cannot be reached")
        print("ok   - an unreachable Cliparr gives a clear message")

        context.close()

    print()
    if failures:
        print(f"{failures} check(s) failed")
        return 1
    print("all extension e2e checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
