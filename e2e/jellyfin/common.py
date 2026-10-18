"""Helpers shared by the end-to-end scripts that run against a real Jellyfin server."""
import json
import sys
import time
import urllib.error
import urllib.request

failures = 0

# How Jellyfin wants every client to introduce itself.
JELLYFIN_CLIENT = 'MediaBrowser Client="cliparr-e2e", Device="e2e", DeviceId="cliparr-e2e", Version="1"'


def check(description: str, ok: bool, detail=None) -> None:
    global failures
    if ok:
        print(f"ok   - {description}")
    else:
        print(f"FAIL - {description}" + (f" ({detail!r})" if detail is not None else ""))
        failures += 1


def finish() -> None:
    print()
    if failures:
        print(f"{failures} check(s) failed")
        sys.exit(1)
    print("all checks passed")


def request(base: str, method: str, path: str, body=None, headers=None, raw: bool = False):
    """Returns (status, JSON or text), or (status, headers, bytes) with raw=True. Never raises on HTTP errors."""
    all_headers = {"Content-Type": "application/json", **(headers or {})}
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(base + path, method=method, headers=all_headers, data=data)
    try:
        with urllib.request.urlopen(req, timeout=60) as response:
            status, response_headers, payload = response.status, dict(response.headers), response.read()
    except urllib.error.HTTPError as error:
        status, response_headers, payload = error.code, dict(error.headers), error.read()
    if raw:
        return status, response_headers, payload
    try:
        return status, json.loads(payload) if payload else None
    except ValueError:
        return status, payload.decode(errors="replace")[:300]


def jellyfin_login(jellyfin: str, username: str, password: str) -> dict:
    status, session = request(
        jellyfin, "POST", "/Users/AuthenticateByName",
        {"Username": username, "Pw": password}, {"Authorization": JELLYFIN_CLIENT},
    )
    if status != 200:
        raise SystemExit(f"Jellyfin refused the login of {username}: {status} {session}")
    return session


def jellyfin_token_header(token: str) -> dict:
    return {"Authorization": f'MediaBrowser Token="{token}"'}


def cliparr_login(cliparr: str, username: str, password: str) -> str:
    status, body = request(cliparr, "POST", "/api/auth/login", {"username": username, "password": password})
    if status != 200:
        raise SystemExit(f"Cliparr refused the login of {username}: {status} {body}")
    return body["token"]


def bearer(token: str) -> dict:
    return {"Authorization": f"Bearer {token}"}


def wait_for(description: str, produce, timeout: float, interval: float = 1.0):
    """Calls produce() until it returns something truthy, and returns that. Fails the run on timeout."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = produce()
        if value:
            return value
        time.sleep(interval)
    raise SystemExit(f"Timed out after {timeout:.0f}s waiting for: {description}")


MOVIE_QUERY = "harbor"
MOVIE_TITLE = "Harbor Lights (2020)"


def find_test_movie(cliparr: str, token: str) -> dict:
    """The generated test movie, as Cliparr's search returns it."""
    status, found = request(cliparr, "GET", f"/api/items?q={MOVIE_QUERY}", headers=bearer(token))
    if status != 200 or len(found) != 1:
        raise SystemExit(f"Expected Cliparr's search for {MOVIE_QUERY!r} to find the test movie once, got {status}: {found}")
    return found[0]
