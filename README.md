# bus

<img width="800" height="480" alt="bus" src="https://github.com/user-attachments/assets/fb81db42-0186-4042-8ce2-9c69c23f6730" />

A full-screen bus arrival display that runs on a Raspberry Pi 2, showing the next buses due at a nearby stop using the [TFL Countdown](http://countdown.tfl.gov.uk/) API, along with the current weather and time.

## How it works

- **Linux (Pi):** renders to the display via DRM/KMS if available, falling back to `/dev/fb0` — no X11, Wayland, or GPU driver required.
- **macOS (dev):** serves a live preview at `http://localhost:8080`, rendering to PNG on each request.

The display shows the stop name and direction at the top, up to three upcoming buses (route number + minutes until arrival), weather conditions at the bottom left, and a clock at the bottom right.

If a second bus stop is configured, tapping the Raspberry Pi touchscreen toggles between the two stops.

## Display output

### DRM/KMS (preferred)

On Linux the program first tries to open `/dev/dri/card0` and use the kernel's DRM/KMS subsystem. It finds the first connected connector, picks its preferred mode, allocates a 32 bpp dumb buffer in XRGB8888 format, and sets the CRTC directly. The pixel conversion from the internal RGBA image is a simple R/B channel swap — no dithering or bit-shifting — which is significantly faster than the fbdev RGB565 path.

This requires the process to be DRM master (i.e. running on the console with no display server). On a Pi running Pi OS, enable the KMS driver by adding `dtoverlay=vc4-kms-v3d` to `/boot/config.txt`.

If DRM setup fails for any reason (wrong permissions, no device, display server already active), the program logs a message and falls back to the fbdev path automatically.

**Portability notes:**
- Works on Pi 2/3/4/5 with `vc4-kms-v3d` overlay enabled.
- Falls back silently on desktop Linux where a display server holds DRM master.
- On a Pi 4 with both HDMI ports connected, HDMI0 is always used (first connected connector).
- The encoder must already be bound to the connector at startup; if the Pi booted headless with no display attached, fbdev fallback is used instead.

### fbdev fallback

If DRM is unavailable, the program writes directly to `/dev/fb0`. It supports both 16 bpp (RGB565 with Bayer 4×4 ordered dithering to reduce colour banding on anti-aliased text) and 32 bpp (XRGB8888).

### Blit performance (ARMv7, 800×480, Pi 2)

CPU profiling showed `fbDevice.blit` consuming ~15% of total CPU. Three optimisations were applied to the inner pixel loop:

1. **Hoist the bpp switch outside the x loop** — bpp is constant for the device lifetime, so each colour depth now has its own tight loop.
2. **Replace per-pixel multiplies with stepping offsets** — `srcOff` and `dstOff` advance by addition rather than recomputing `x*4` and `dstX*bytesPerPixel` each iteration.
3. **Cache the Bayer dither row** — `bayer4x4[y&3]` is constant across the entire x loop for a given row.

The DRM path avoids the RGB565 dithering entirely, replacing it with a plain R/B channel swap.

| Path | Time/frame | vs fbdev 16bpp |
|---|---|---|
| fbdev 16bpp (before optimisation) | 53.2 ms | baseline |
| fbdev 16bpp (after optimisation) | 39.6 ms | −26% |
| fbdev 32bpp (after optimisation) | 34.7 ms | −35% |
| DRM XRGB8888 | **24.7 ms** | **−54%** |

## Building

```
go build
```

No CGO, no system dependencies beyond a standard Go toolchain. The DRM/KMS and fbdev paths use raw Linux ioctls via the Go `syscall` package.

## Usage

```
./bus [flags]

  -stop int          TFL bus stop code (default 74640)
  -stop2 int         secondary bus stop code; touch screen toggles between the two
  -touch str         touch input device path (auto-detected if empty)
  -debounce dur      minimum interval between touch-triggered stop switches (default 100ms)
  -rotate            rotate display 180 degrees (default true)
  -weather-key str   weatherapi.com API key
```

## Configuration

Weather is fetched from [weatherapi.com](https://www.weatherapi.com/). Sign up for a free API key and pass it via `-weather-key`. The location is derived automatically from the GPS coordinates returned by the TFL API for the configured bus stop.

## History

Originally written in Python a decade ago, it stopped working when TFL dropped support for old TLS ciphers and Python 2.7 was no longer maintained on the Pi. It was rewritten in Go, and later migrated from Fyne (which required CGO and a GPU driver) to a pure-Go framebuffer renderer. The old Python code lives in the [pybus](https://github.com/amnonbc/bus/tree/master/pybus) directory.
