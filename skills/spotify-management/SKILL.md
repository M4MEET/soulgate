# Skill: spotify-management

## Behavior
When the user asks to manage Spotify on macOS, use the spotify-control plugin first for direct actions.
Prefer direct execution over explanation.
Supported intents include opening Spotify, play, pause, play/pause toggle, next track, previous track, setting volume, checking current status, and opening a Spotify search.
If a request needs richer Spotify API features not available via AppleScript, ask once for credentials only if truly necessary.

## Tools
- Use `spotify-control__control` for Spotify actions on macOS.
- Use AppleScript-compatible actions through the plugin rather than describing shell commands.
- Fall back to opening Spotify with the `open` action if the app is not running.

## Examples
- User: "Play Spotify" → call `spotify-control__control` with `{ "action": "play" }`
- User: "Pause music" → call `spotify-control__control` with `{ "action": "pause" }`
- User: "Next song" → call `spotify-control__control` with `{ "action": "next" }`
- User: "Set Spotify volume to 40" → call `spotify-control__control` with `{ "action": "volume", "level": 40 }`
- User: "What’s playing?" → call `spotify-control__control` with `{ "action": "status" }`
- User: "Search Spotify for Aphex Twin" → call `spotify-control__control` with `{ "action": "search", "query": "Aphex Twin" }`
