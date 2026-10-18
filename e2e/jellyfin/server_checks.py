#!/usr/bin/env python3
"""Checks Cliparr's server against a real Jellyfin: everything that needs no browser.

Login, search, streaming, thumbnails, who may see which clip, and the resume point.

Usage: server_checks.py CLIPARR_URL JELLYFIN_URL
"""
import json
import subprocess
import sys
import tempfile

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

print("== audio browsers cannot decode")
status, plain = request(cliparr, "GET", f"/api/items/{item['id']}", headers=bearer(alice))
check("AAC audio plays as it is", status == 200 and plain.get("audioPlayable") is True, (status, plain))
status, found = request(cliparr, "GET", "/api/items?q=dolby", headers=bearer(alice))
dolby = found[0] if status == 200 and len(found) == 1 else {"id": "missing"}
status, detail = request(cliparr, "GET", f"/api/items/{dolby['id']}", headers=bearer(alice))
check("Dolby Digital audio does not", status == 200 and detail.get("audioPlayable") is False, (status, detail))


def probe(data: bytes) -> dict:
    """What ffprobe makes of a downloaded stream."""
    with tempfile.NamedTemporaryFile(suffix=".mp3") as f:
        f.write(data)
        f.flush()
        out = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "stream=codec_type,codec_name,channels:format=duration",
                              "-of", "json", f.name], capture_output=True, text=True, check=True).stdout
    return json.loads(out)


def audio_from(start_ms: int):
    return request(cliparr, "GET", f"/api/items/{dolby['id']}/audio?start={start_ms}", headers=bearer(alice), raw=True)


status, headers, data = audio_from(0)
check("the sound comes as an audio file", status == 200 and len(data) > 1000, (status, len(data)))
info = probe(data) if status == 200 else {"streams": [], "format": {}}
audio = info["streams"][0] if info["streams"] else {}
check("it is MP3", audio.get("codec_name") == "mp3" and len(info["streams"]) == 1, info["streams"])
check("its 5.1 sound is mixed down to stereo", audio.get("channels") == 2, audio)
check("it covers the whole minute", 59 <= float(info["format"].get("duration", 0)) <= 62, info["format"])

status, headers, data = audio_from(40000)
duration = float(probe(data)["format"].get("duration", 0)) if status == 200 else 0
check("starting 40 s in leaves 20 s of sound", 19 <= duration <= 22, duration)
status, body = request(cliparr, "GET", f"/api/items/{dolby['id']}/audio?start=oops", headers=bearer(alice))
check("a start that is no number is refused", status == 400, (status, body))
status, body = request(cliparr, "GET", f"/api/items/{dolby['id']}/audio")
check("the sound needs a login", status == 401, status)

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
