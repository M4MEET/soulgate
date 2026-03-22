# Desktop Control

You can control desktop applications on macOS using exec_command with AppleScript.
ALWAYS try to do it — never say "I can't control apps."

## Music (Spotify, Apple Music)

```bash
# Spotify
osascript -e 'tell application "Spotify" to play'
osascript -e 'tell application "Spotify" to pause'
osascript -e 'tell application "Spotify" to next track'
osascript -e 'tell application "Spotify" to previous track'
osascript -e 'tell application "Spotify" to set sound volume to 50'
osascript -e 'tell application "Spotify" to play track "spotify:track:TRACK_ID"'
osascript -e 'tell application "Spotify" to name of current track'

# Search and play (use Spotify URI)
open "spotify:search:lofi+beats"

# Apple Music
osascript -e 'tell application "Music" to play'
osascript -e 'tell application "Music" to pause'
```

## Browser

```bash
open "https://google.com"
open -a "Google Chrome" "https://example.com"
open -a Safari "https://example.com"
```

## System

```bash
# Volume
osascript -e 'set volume output volume 50'
osascript -e 'set volume output muted true'

# Notifications
osascript -e 'display notification "Hello" with title "SoulGate"'

# Clipboard
pbcopy <<< "text to copy"
pbpaste

# Screenshot
screencapture -x screenshot.png

# Lock screen
osascript -e 'tell application "System Events" to keystroke "q" using {control down, command down}'

# Dark mode toggle
osascript -e 'tell application "System Events" to tell appearance preferences to set dark mode to not dark mode'
```

## Apps

```bash
# Open any app
open -a "App Name"

# Quit app
osascript -e 'tell application "App Name" to quit'

# List running apps
osascript -e 'tell application "System Events" to name of every process whose background only is false'

# Finder
osascript -e 'tell application "Finder" to open folder "Downloads" of home'
```

## When something needs an API key or OAuth

If the user asks for something that needs an API (e.g., "play this song on Spotify by name"):
1. Try the simple approach first (open spotify:search:query)
2. If that's not enough, create a script in extensions/ that uses the API
3. Ask the user for credentials ONLY if needed, save to memory
4. Never say "I can't do this" — build the integration on the fly
