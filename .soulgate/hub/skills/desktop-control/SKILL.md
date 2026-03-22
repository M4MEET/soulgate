# Skill: Desktop Control (macOS)

Control desktop apps via exec_command. Always use the correct command on the first try.

## App Control

```bash
# Open app
open -a "AppName"

# Open URL in default browser
open "https://example.com"

# Open URL in specific browser
open -a "Google Chrome" "https://example.com"

# Open file with default app
open file.pdf
```

## Chrome

```bash
# Open Chrome
open -a "Google Chrome"

# New tab with URL
osascript -e 'tell application "Google Chrome" to open location "https://example.com"'

# New empty tab (Cmd+T shortcut)
osascript -e 'tell application "Google Chrome" to activate' -e 'tell application "System Events" to keystroke "t" using command down'

# New incognito window (Cmd+Shift+N shortcut)
osascript -e 'tell application "Google Chrome" to activate' -e 'tell application "System Events" to keystroke "n" using {command down, shift down}'

# Close current tab (Cmd+W)
osascript -e 'tell application "Google Chrome" to activate' -e 'tell application "System Events" to keystroke "w" using command down'

# Get current URL
osascript -e 'tell application "Google Chrome" to get URL of active tab of front window'

# Get page title
osascript -e 'tell application "Google Chrome" to get title of active tab of front window'
```

IMPORTANT for Chrome:
- Do NOT use `open --args --incognito` — it doesn't work reliably.
- For incognito: use the Cmd+Shift+N keystroke method above.
- For new tabs: use the Cmd+T keystroke method above.
- For opening URLs: use `open location` — it's the only reliable method.

## Spotify

```bash
# Play/pause/next/previous
osascript -e 'tell application "Spotify" to play'
osascript -e 'tell application "Spotify" to pause'
osascript -e 'tell application "Spotify" to next track'
osascript -e 'tell application "Spotify" to previous track'

# Play specific URI
osascript -e 'tell application "Spotify" to play track "spotify:track:TRACK_ID"'

# Get current track
osascript -e 'tell application "Spotify" to get name of current track'
osascript -e 'tell application "Spotify" to get artist of current track'

# Set volume (0-100)
osascript -e 'tell application "Spotify" to set sound volume to 50'
```

## System

```bash
# Volume control
osascript -e 'set volume output volume 50'
osascript -e 'set volume output muted true'

# Notifications
osascript -e 'display notification "Body" with title "Title"'

# Screenshot
screencapture -x screenshot.png

# Lock screen
osascript -e 'tell application "System Events" to keystroke "q" using {command down, control down}'

# Sleep
pmset sleepnow
```

## Rules

- Use `open` for launching apps and URLs — simplest and always works.
- Use `osascript` only for app-specific control (play, pause, get info).
- Never chain multiple `-e` flags when a single command works.
- One command per exec_command call. Don't combine unrelated actions.
