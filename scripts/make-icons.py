#!/usr/bin/env python3
"""Renders the logo, packaging/cliparr.svg, into every PNG icon in the repository.

Chrome, Android and web app manifests do not accept SVG icons. Run `just icons`
only when the logo changes, and commit the PNGs it writes. Or run it by hand:

    CHROMIUM=/path/to/chromium python3 scripts/make-icons.py

Writes: the browser extension icons, the web app's favicon and install icons
(plus the "maskable" ones, which are full-bleed with the artwork pulled into
the middle so Android can crop them to any shape), and the Android launcher icons.
"""
import os
from pathlib import Path

from playwright.sync_api import sync_playwright

ROOT = Path(__file__).resolve().parent.parent
RES = ROOT / "app" / "android" / "app" / "src" / "main" / "res"

# (output file, size in pixels)
ICONS = [
    *((ROOT / "extension" / "icons" / f"icon-{size}.png", size) for size in (16, 32, 48, 128)),
    (ROOT / "app" / "web" / "favicon.png", 32),
    (ROOT / "app" / "web" / "icons" / "Icon-192.png", 192),
    (ROOT / "app" / "web" / "icons" / "Icon-512.png", 512),
    *((RES / f"mipmap-{density}" / "ic_launcher.png", size)
      for density, size in (("mdpi", 48), ("hdpi", 72), ("xhdpi", 96), ("xxhdpi", 144), ("xxxhdpi", 192))),
]
MASKABLE_ICONS = [
    (ROOT / "app" / "web" / "icons" / "Icon-maskable-192.png", 192),
    (ROOT / "app" / "web" / "icons" / "Icon-maskable-512.png", 512),
]
# Android keeps the middle 80% of a maskable icon; leave a little more room than that.
MASKABLE_SCALE = 0.78


def maskable(svg: str) -> str:
    """The logo without rounded corners or clipping, with the artwork shrunk into the safe zone."""
    edits = [
        ('<rect id="tile-bg" width="256" height="256" rx="56"', '<rect id="tile-bg" width="256" height="256" rx="0"'),
        ('<g clip-path="url(#tile)">', "<g>"),
        ('<g id="art">', f'<g id="art" transform="translate(128 128) scale({MASKABLE_SCALE}) translate(-128 -128)">'),
    ]
    for old, new in edits:
        if old not in svg:
            raise SystemExit(f"cliparr.svg no longer contains {old!r}; update maskable() to match")
        svg = svg.replace(old, new)
    return svg


def render(browser, svg: str, path: Path, size: int) -> None:
    page = browser.new_page(viewport={"width": size, "height": size})
    page.set_content(
        f"<style>html,body{{margin:0;background:transparent}}svg{{width:{size}px;height:{size}px;display:block}}</style>{svg}"
    )
    path.parent.mkdir(parents=True, exist_ok=True)
    page.screenshot(path=str(path), omit_background=True)
    page.close()


def main() -> None:
    svg = (ROOT / "packaging" / "cliparr.svg").read_text()
    full_bleed = maskable(svg)
    chromium = os.environ.get("CHROMIUM")
    with sync_playwright() as p:
        browser = p.chromium.launch(executable_path=chromium) if chromium else p.chromium.launch()
        for path, size in ICONS:
            render(browser, svg, path, size)
        for path, size in MASKABLE_ICONS:
            render(browser, full_bleed, path, size)
        browser.close()
    for path, _ in ICONS + MASKABLE_ICONS:
        print("wrote", path.relative_to(ROOT))


if __name__ == "__main__":
    main()
