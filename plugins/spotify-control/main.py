#!/usr/bin/env python3
import json, subprocess, sys, urllib.parse


def run(cmd):
    p = subprocess.run(cmd, capture_output=True, text=True)
    return p.returncode, p.stdout.strip(), p.stderr.strip()


def ok(**kwargs):
    print(json.dumps({"ok": True, **kwargs}))
    sys.exit(0)


def fail(message, **kwargs):
    print(json.dumps({"ok": False, "error": message, **kwargs}))
    sys.exit(0)


def osa(script):
    return run(["osascript", "-e", script])


def spotify_running():
    code, out, err = osa('tell application "System Events" to (name of processes) contains "Spotify"')
    return out.lower() == 'true'


def ensure_spotify():
    if not spotify_running():
        run(["open", "-a", "Spotify"])


def main():
    try:
        data = json.load(sys.stdin)
    except Exception as e:
        fail(f"Invalid JSON input: {e}")

    action = data.get("action")
    if not action:
        fail("Missing 'action'")

    if action == "open":
        code, out, err = run(["open", "-a", "Spotify"])
        if code != 0:
            fail("Failed to open Spotify", details=err)
        ok(message="Spotify opened")

    ensure_spotify()

    if action == "play":
        code, out, err = osa('tell application "Spotify" to play')
        if code != 0:
            fail("Failed to play", details=err)
        ok(message="Playback started")

    elif action == "pause":
        code, out, err = osa('tell application "Spotify" to pause')
        if code != 0:
            fail("Failed to pause", details=err)
        ok(message="Playback paused")

    elif action == "playpause":
        code, state, err = osa('tell application "Spotify" to player state as string')
        if code != 0:
            fail("Failed to get player state", details=err)
        target = 'pause' if state.strip().lower() == 'playing' else 'play'
        code, out, err = osa(f'tell application "Spotify" to {target}')
        if code != 0:
            fail("Failed to toggle playback", details=err)
        ok(message=f"Playback {target}d", state_before=state.strip())

    elif action == "next":
        code, out, err = osa('tell application "Spotify" to next track')
        if code != 0:
            fail("Failed to skip to next track", details=err)
        ok(message="Skipped to next track")

    elif action == "previous":
        code, out, err = osa('tell application "Spotify" to previous track')
        if code != 0:
            fail("Failed to go to previous track", details=err)
        ok(message="Went to previous track")

    elif action == "volume":
        level = data.get("level")
        if not isinstance(level, int) or not (0 <= level <= 100):
            fail("'level' must be an integer between 0 and 100")
        code, out, err = osa(f'tell application "Spotify" to set sound volume to {level}')
        if code != 0:
            fail("Failed to set volume", details=err)
        ok(message="Volume set", level=level)

    elif action == "status":
        code, state, err = osa('tell application "Spotify" to player state as string')
        if code != 0:
            fail("Failed to get player state", details=err)
        code, name, err = osa('tell application "Spotify" to name of current track')
        if code != 0:
            fail("Failed to get current track name", details=err)
        code, artist, err = osa('tell application "Spotify" to artist of current track')
        if code != 0:
            fail("Failed to get current track artist", details=err)
        code, album, err = osa('tell application "Spotify" to album of current track')
        if code != 0:
            fail("Failed to get current track album", details=err)
        code, volume, err = osa('tell application "Spotify" to sound volume')
        if code != 0:
            fail("Failed to get volume", details=err)
        ok(state=state.strip(), track=name.strip(), artist=artist.strip(), album=album.strip(), volume=volume.strip())

    elif action == "search":
        query = data.get("query")
        if not query or not isinstance(query, str):
            fail("'query' must be a non-empty string")
        uri = 'spotify:search:' + urllib.parse.quote_plus(query)
        code, out, err = run(["open", uri])
        if code != 0:
            fail("Failed to open Spotify search", details=err)
        ok(message="Opened Spotify search", query=query, uri=uri)

    else:
        fail("Unsupported action", supported=["open","play","pause","playpause","next","previous","volume","status","search"])

if __name__ == "__main__":
    main()
