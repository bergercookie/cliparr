#!/usr/bin/env python3
"""Takes screenshots of the Cliparr web app for the documentation.

Usage: screenshots.py BASE_URL OUTPUT_DIR

BASE_URL must be a Cliparr server (with the web app bundled) connected to the
fake Jellyfin, where alice and bob exist with the password "pw".
`just screenshots` starts that setup and calls this script.
"""
import json
import os
import sys
import urllib.request
from pathlib import Path

from playwright.sync_api import Page, sync_playwright

# Path of a Chromium binary to use instead of the one Playwright downloads.
CHROMIUM = os.environ.get("CHROMIUM")

# Invented quotes: the screenshots must not attribute real lines to real films.
# The fake Jellyfin streams a one-minute video, so every clip lies inside it.
SAMPLE_CLIPS = [
    {
        "itemId": "movie-rocky", "startMs": 15_000, "endMs": 25_000,
        "quote": "Hard things are only hard until they are done.",
        "note": "The pep talk before the final round. Watch this when I want to quit.",
        "tags": ["motivation", "speech"],
    },
    {
        "itemId": "episode-pilot", "startMs": 30_000, "endMs": 40_000,
        "quote": "You miss every shot you do not take, so take two.",
        "note": "", "tags": ["motivation"],
    },
    {
        "itemId": "movie-rocky", "startMs": 42_000, "endMs": 50_000,
        "quote": "That is not a bug, that is a surprise feature.",
        "note": "Quote for the team chat.", "tags": ["funny"],
    },
]


def api(base: str, path: str, body: dict | None = None, token: str | None = None) -> dict:
    request = urllib.request.Request(
        base + path,
        data=json.dumps(body).encode() if body is not None else None,
        headers={"Content-Type": "application/json", **({"Authorization": f"Bearer {token}"} if token else {})},
    )
    with urllib.request.urlopen(request) as response:
        return json.load(response)


def seed(base: str) -> None:
    """Gives alice a few clips, unless she already has some."""
    token = api(base, "/api/auth/login", {"username": "alice", "password": "pw"})["token"]
    if api(base, "/api/clips", token=token):
        return
    for clip in SAMPLE_CLIPS:
        api(base, "/api/clips", clip, token)


def open_app(page: Page, base: str) -> None:
    page.goto(base)
    page.wait_for_selector("flt-glass-pane", state="attached", timeout=30_000)
    page.wait_for_timeout(2000)


def enable_semantics(page: Page) -> None:
    """Flutter draws on a canvas; this makes it expose buttons we can click by name."""
    page.evaluate("document.querySelector('flt-semantics-placeholder')?.click()")
    page.wait_for_timeout(500)


def log_in(page: Page, user: str = "alice") -> None:
    # The username field has focus when the page opens.
    page.keyboard.type(user)
    page.keyboard.press("Tab")
    page.keyboard.type("pw")
    page.keyboard.press("Enter")
    page.wait_for_timeout(2500)
    enable_semantics(page)


def shoot(page: Page, out: Path, name: str) -> None:
    page.wait_for_timeout(800)
    page.screenshot(path=str(out / f"{name}.png"))
    print(f"wrote {out / name}.png")


def take(browser, base: str, out: Path, *, prefix: str, dark: bool, viewport: dict, all_screens: bool) -> None:
    """Logs in and photographs the clips page; with all_screens, the other pages too."""
    context = browser.new_context(viewport=viewport, color_scheme="dark" if dark else "light")
    page = context.new_page()
    open_app(page, base)
    if all_screens:
        shoot(page, out, f"{prefix}login")
    log_in(page)
    shoot(page, out, f"{prefix}clips")
    if not all_screens:
        context.close()
        return

    # The first card (newest clip) is the one these screens are taken from.
    page.get_by_role("button", name="Open in Jellyfin").first.click()
    page.wait_for_timeout(1500)
    shoot(page, out, f"{prefix}open-in-jellyfin")
    page.keyboard.press("Escape")
    page.wait_for_timeout(500)

    page.get_by_role("button", name="Settings").click()
    shoot(page, out, f"{prefix}settings")
    page.mouse.move(500, 400)
    page.mouse.wheel(0, 2000)  # down to the panel for other devices
    shoot(page, out, f"{prefix}settings-other-devices")

    # Reloading returns to the clips page: the login cookie survives.
    open_app(page, base)
    enable_semantics(page)
    page.get_by_role("button", name="Play clip").first.click()
    page.wait_for_timeout(4000)  # let the video load and start looping
    shoot(page, out, f"{prefix}clip-player")

    open_app(page, base)
    enable_semantics(page)
    page.get_by_role("button", name="New clip").click()
    page.wait_for_timeout(800)
    page.keyboard.type("rock")
    page.keyboard.press("Enter")
    page.wait_for_timeout(1500)
    shoot(page, out, f"{prefix}new-clip-search")

    # Pick the first result, mark a few seconds with the button on the video.
    page.get_by_role("button", name="Rocky").first.click()
    page.wait_for_timeout(3000)  # the video loads and starts by itself
    # A full-screen invisible node covers the video's accessibility layer, so a real click
    # on the button's node is intercepted: send the click to the node itself.
    page.get_by_role("button", name="Start clip").dispatch_event("click")
    page.wait_for_timeout(3000)
    page.get_by_role("button", name="End clip").dispatch_event("click")
    page.wait_for_timeout(500)
    page.keyboard.press("Space")  # pause, so the play button shows on the picture
    shoot(page, out, f"{prefix}clip-editor")
    context.close()


def take_first_clip(browser, base: str, out: Path) -> None:
    """What a user without any clips sees first: bob has none."""
    context = browser.new_context(viewport={"width": 1000, "height": 720})
    page = context.new_page()
    open_app(page, base)
    log_in(page, "bob")
    shoot(page, out, "first-clip")
    context.close()


def main() -> None:
    base, out = sys.argv[1].rstrip("/"), Path(sys.argv[2])
    out.mkdir(parents=True, exist_ok=True)
    seed(base)
    desktop = {"width": 1000, "height": 720}
    with sync_playwright() as p:
        browser = p.chromium.launch(executable_path=CHROMIUM) if CHROMIUM else p.chromium.launch()
        take(browser, base, out, prefix="", dark=False, viewport=desktop, all_screens=True)
        take_first_clip(browser, base, out)
        take(browser, base, out, prefix="dark-", dark=True, viewport=desktop, all_screens=False)
        take(browser, base, out, prefix="mobile-", dark=False, viewport={"width": 390, "height": 760}, all_screens=False)
        browser.close()


if __name__ == "__main__":
    main()
