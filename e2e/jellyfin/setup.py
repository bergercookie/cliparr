#!/usr/bin/env python3
"""Sets up a brand-new Jellyfin server for the end-to-end tests.

Completes the first-run wizard, creates the users alice and bob (password pw),
adds a movie library and waits until Jellyfin has scanned the sample movie.

Usage: setup.py JELLYFIN_URL MEDIA_DIR
"""
import sys

from common import JELLYFIN_CLIENT, jellyfin_login, jellyfin_token_header, request, wait_for

jellyfin, media = sys.argv[1].rstrip("/"), sys.argv[2]
auth = {"Authorization": JELLYFIN_CLIENT}


def expect(what: str, answer) -> None:
    status = answer[0]
    if status >= 300:
        raise SystemExit(f"Jellyfin setup step '{what}' failed: {answer}")


# The server answers on its public address a moment before it has finished loading;
# until then the wizard's endpoints reply 503.
wait_for("Jellyfin to finish loading", lambda: request(jellyfin, "GET", "/Startup/Configuration", headers=auth)[0] == 200, timeout=120)

expect("configuration", request(jellyfin, "POST", "/Startup/Configuration",
                                {"UICulture": "en-US", "MetadataCountryCode": "US", "PreferredMetadataLanguage": "en"}, auth))
request(jellyfin, "GET", "/Startup/User", headers=auth)  # Jellyfin creates its first user here
expect("first user", request(jellyfin, "POST", "/Startup/User", {"Name": "alice", "Password": "pw"}, auth))
expect("remote access", request(jellyfin, "POST", "/Startup/RemoteAccess",
                                {"EnableRemoteAccess": True, "EnableAutomaticPortMapping": False}, auth))
expect("complete", request(jellyfin, "POST", "/Startup/Complete", headers=auth))

token = jellyfin_login(jellyfin, "alice", "pw")["AccessToken"]
headers = jellyfin_token_header(token)
expect("second user", request(jellyfin, "POST", "/Users/New", {"Name": "bob", "Password": "pw"}, headers))

# No online metadata: the test must not depend on what a metadata provider thinks the movie is.
library = {"LibraryOptions": {
    "EnableInternetProviders": False, "EnableRealtimeMonitor": False, "MetadataSavers": [],
    "PathInfos": [{"Path": media}], "AutomaticallyAddToCollection": False,
    "TypeOptions": [{"Type": "Movie", "MetadataFetchers": [], "ImageFetchers": []}],
}}
expect("library", request(jellyfin, "POST", "/Library/VirtualFolders?name=Movies&collectionType=movies&refreshLibrary=true",
                          library, headers))


attempts = 0


def scanned():
    global attempts
    status, items = request(jellyfin, "GET", "/Items?Recursive=true&IncludeItemTypes=Movie", headers=headers)
    found = items["Items"] if status == 200 else []
    if found and all(item.get("RunTimeTicks") for item in found):
        return found
    attempts += 1
    if attempts % 15 == 0:  # the first scan can be missed while Jellyfin is still starting up
        request(jellyfin, "POST", "/Library/Refresh", headers=headers)
    return None


def what_jellyfin_sees() -> str:
    status, items = request(jellyfin, "GET", "/Items?Recursive=true&Fields=Path", headers=headers)
    return f"{status}: {items}"


try:
    movies = wait_for("Jellyfin to scan and probe the sample movie", scanned, timeout=180)
except SystemExit:
    print("Jellyfin's items at that point:", what_jellyfin_sees(), file=sys.stderr)
    raise
print("Jellyfin is ready:", ", ".join(f"{m['Name']} ({m['RunTimeTicks'] // 10**7} s)" for m in movies))
