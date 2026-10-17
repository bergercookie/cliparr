#!/usr/bin/env python3
"""Renders packaging/cliparr.svg into the PNG icons the extension needs.

Chrome does not accept SVG icons in a manifest. Run this (through Playwright's
Chromium) only when the logo changes, and commit the PNGs:

    CHROMIUM=/path/to/chromium python3 scripts/make-extension-icons.py
"""
import os
from pathlib import Path

from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parent.parent
SIZES = (16, 32, 48, 128)


def main() -> None:
    svg = (ROOT / "packaging" / "cliparr.svg").read_text()
    out = ROOT / "extension" / "icons"
    out.mkdir(parents=True, exist_ok=True)
    chromium = os.environ.get("CHROMIUM")
    with sync_playwright() as p:
        browser = p.chromium.launch(executable_path=chromium) if chromium else p.chromium.launch()
        for size in SIZES:
            page = browser.new_page(viewport={"width": size, "height": size})
            page.set_content(
                f"<style>html,body{{margin:0;background:transparent}}svg{{width:{size}px;height:{size}px;display:block}}</style>{svg}"
            )
            page.screenshot(path=str(out / f"icon-{size}.png"), omit_background=True)
            page.close()
        browser.close()


if __name__ == "__main__":
    main()
