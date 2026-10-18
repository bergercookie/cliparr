#!/usr/bin/env python3
"""Checks Cliparr's server against a real Jellyfin: everything that needs no browser.

Login, search, streaming, thumbnails, who may see which clip, and the resume point.

Usage: server_checks.py CLIPARR_URL JELLYFIN_URL
"""
import sys

from common import (MOVIE_QUERY, MOVIE_TITLE, bearer, check, cliparr_login, finish, jellyfin_login, jellyfin_token_header,
                    request)

cliparr, jellyfin = sys.argv[1].rstrip("/"), sys.argv[2].rstrip("/")

print("== login")
status, info = request(cliparr, "GET", "/api/info")
check("the server answers", status == 200, info)
status, body = request(cliparr, "POST", "/api/auth/login", {"username": "alice", "password": "wrong"})
check("a wrong password is refused", status == 401, (status, body))
alice = cliparr_login(cliparr, "alice", "pw")
bob = cliparr_login(cliparr, "bob", "pw")
status, me = request(cliparr, "GET", "/api/me", headers=bearer(alice))
check("a Jellyfin account logs in", status == 200 and me.get("name") == "alice", me)

print("== search and playback")
status, found = request(cliparr, "GET", f"/api/items?q={MOVIE_QUERY}", headers=bearer(alice))
check("search finds the movie", status == 200 and len(found) == 1, found)
item = found[0] if found else {"id": "missing", "title": "", "runtimeMs": 0}
check("the title has the year once", item["title"] == MOVIE_TITLE, item["title"])
check("the runtime is ten minutes", item["runtimeMs"] == 600000, item["runtimeMs"])
status, headers, data = request(cliparr, "GET", f"/api/items/{item['id']}/stream", headers={**bearer(alice), "Range": "bytes=0-99"}, raw=True)
check("the original file streams with range support", status == 206 and len(data) == 100, (status, len(data)))
status, headers, data = request(cliparr, "GET", f"/api/items/{item['id']}/image", headers=bearer(alice), raw=True)
# This library has no metadata, so Jellyfin may have no poster at all; then a clean 404 is right.
check("the poster comes through, or is cleanly missing",
      (status == 200 and headers.get("Content-Type", "").startswith("image/")) or status == 404, (status, headers.get("Content-Type")))
status, body = request(cliparr, "GET", f"/api/items/{item['id']}/stream")
check("streaming needs a login", status == 401, status)

print("== clips")
status, clip = request(cliparr, "POST", "/api/clips", {"itemId": item["id"], "startMs": 120000, "endMs": 130000,
                                                      "quote": "A line", "note": "why", "tags": ["a", "b"]}, bearer(alice))
check("a clip is created", status in (200, 201) and "id" in clip, (status, clip))
clip_id = clip["id"]
status, thumb_headers, thumb = request(cliparr, "GET", f"/api/clips/{clip_id}/thumbnail", headers=bearer(alice), raw=True)
check("the thumbnail is a JPEG cut from Jellyfin's stream", status == 200 and thumb[:2] == b"\xff\xd8", (status, thumb[:40]))
status, body = request(cliparr, "GET", f"/api/clips/{clip_id}", headers=bearer(bob))
check("bob cannot read alice's clip", status == 404, (status, body))
status, listing = request(cliparr, "GET", "/api/clips", headers=bearer(bob))
check("bob's list is empty", status == 200 and listing == [], listing)
status, body = request(cliparr, "DELETE", f"/api/clips/{clip_id}", headers=bearer(bob))
check("bob cannot delete alice's clip", status == 404, (status, body))

print("== opening the clip in Jellyfin")
status, handoff = request(cliparr, "GET", f"/api/clips/{clip_id}/jellyfin", headers=bearer(alice))
check("the hand-off answers", status == 200, (status, handoff))
check("it links to the item in Jellyfin's web interface", "/web/index.html#!/details?id=" + item["id"] in handoff["webUrl"], handoff["webUrl"])
check("nobody is watching, so no devices", handoff["devices"] == [], handoff["devices"])
status, body = request(cliparr, "POST", f"/api/clips/{clip_id}/jellyfin/resume", headers=bearer(alice))
check("the resume point is set", status == 204, (status, body))
session = jellyfin_login(jellyfin, "alice", "pw")
status, from_jellyfin = request(jellyfin, "GET", f"/Items/{item['id']}?userId={session['User']['Id']}", headers=jellyfin_token_header(session["AccessToken"]))
check("Jellyfin itself reports the resume position", status == 200 and from_jellyfin["UserData"]["PlaybackPositionTicks"] == 120000 * 10000,
      from_jellyfin.get("UserData") if status == 200 else status)
status, body = request(cliparr, "POST", f"/api/clips/{clip_id}/jellyfin/resume", headers=bearer(bob))
check("bob cannot move his resume point to alice's clip", status == 404, (status, body))

finish()
