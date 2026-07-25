# Surfista

A minimalist terminal user interface (TUI) for finding, tracking, and comparing surf spots. Explore ten days of Surfline forecasts, including surf height, ratings, swell, wind, tides, temperature, and daylight, without leaving your terminal.

Surfista was built to compare your favorite surf breaks at a glance. Sort spots by current or future conditions, move through the ten-day outlook, and open detailed forecasts when you want a closer look.

*For surfers and terminal enthusiasts who want forecasts in a fast, keyboard-driven interface without leaving their workflow.*

## Installation & Updates

### Homebrew

```bash
# Install
brew install defcarlito/tap/surfista

# Update
brew upgrade defcarlito/tap/surfista
```

### Build from source

```bash
git clone https://github.com/defcarlito/surfista
cd surfista
go build
./surfista
```

## Usage

Run Surfista:

```bash
surfista
```

**Controls**

| Key | Action |
|---|---|
| `←` / `→` or `h` / `l` | Move between forecast days |
| `↑` / `↓` or `k` / `j` | Navigate locations and forecast details |
| `Enter` | Open the selected location’s detailed forecast |
| `/` | Search for and track a surf spot |
| `s` | Switch sorting modes |
| `v` | Cycle through surf, wind, and swell views |
| `r` | Refresh forecasts |
| `u` | Open the selected spot on Surfline |
| `x` | Remove the selected spot |
| `Esc` | Clear the selection, close a view, or cancel |
| `q` or `Ctrl+C` | Quit Surfista |

When searching, type to find a spot, press `Enter` to select the results, use `↑` / `↓` or `k` / `j` to navigate, and press `Enter` again to track the selected spot.
