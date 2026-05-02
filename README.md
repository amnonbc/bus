# bus

<img width="800" height="480" alt="bus" src="https://github.com/user-attachments/assets/fb81db42-0186-4042-8ce2-9c69c23f6730" />

A full-screen bus arrival display that runs on a Raspberry Pi, showing the next buses due at a nearby stop using the [TFL Countdown](http://countdown.tfl.gov.uk/) API, along with the current weather and time. Tested on Pi 1 (ARMv6, fbdev 16 bpp) and Pi 2 (ARMv7, DRM/KMS).

## How it works

- **Linux (Pi):** renders to the display via DRM/KMS if available, falling back to `/dev/fb0` — no X11, Wayland, or GPU driver required.
- **macOS (dev):** serves a live preview at `http://localhost:8080`, rendering to PNG on each request.

The display shows the stop name and direction at the top, up to three upcoming buses (route number + minutes until arrival), weather conditions at the bottom left, and a clock at the bottom right.

If a second bus stop is configured, tapping the Raspberry Pi touchscreen toggles between the two stops.

## Display output

### DRM/KMS (preferred)

On Linux the program tries `/dev/dri/card0`, `card1`, `card2` in order and uses the first that supports modesetting. On Pi 4/5 with the KMS driver, `card0` is a render-only V3D node; the display controller is typically `card1`. It finds the first connected connector, picks its preferred mode, allocates a 32 bpp dumb buffer in XRGB8888 format, and sets the CRTC directly. The pixel conversion from the internal RGBA image is a simple R/B channel swap — no dithering or bit-shifting — which is significantly faster than the fbdev RGB565 path.

This requires the process to be DRM master (i.e. running on the console with no display server). On a Pi running Pi OS, enable the KMS driver by adding `dtoverlay=vc4-kms-v3d` to `/boot/config.txt`.

If DRM setup fails for any reason (wrong permissions, no device, display server already active), the program logs a message and falls back to the fbdev path automatically.

**Portability notes:**
- Works on Pi 2/3/4/5 with `vc4-kms-v3d` overlay enabled. Pi 1 (ARMv6, kernel 3.x) has no DRM support and uses fbdev automatically.
- Falls back silently on desktop Linux where a display server holds DRM master.
- On a Pi 4 with both HDMI ports connected, HDMI0 is always used (first connected connector).
- The encoder must already be bound to the connector at startup; if the Pi booted headless with no display attached, fbdev fallback is used instead.

### fbdev fallback

If DRM is unavailable, the program writes directly to `/dev/fb0`. It supports both 16 bpp (RGB565 with Bayer 4×4 ordered dithering to reduce colour banding on anti-aliased text) and 32 bpp (XRGB8888).

### Blit performance (ARMv7, 800×480, Pi 2)

CPU profiling showed `fbDevice.blit` consuming ~15% of total CPU. The blit was optimised in three stages:

**Stage 1 — fbdev inner loop (fbdev path)**
1. **Hoist the bpp switch outside the x loop** — bpp is constant for the device lifetime, so each colour depth now has its own tight loop.
2. **Replace per-pixel multiplies with stepping offsets** — `srcOff` and `dstOff` advance by addition rather than recomputing `x*4` and `dstX*bytesPerPixel` each iteration.
3. **Cache the Bayer dither row** — `bayer4x4[y&3]` is constant across the entire x loop for a given row.

**Stage 2 — DRM/KMS path**

The DRM path avoids RGB565 dithering entirely, replacing it with a plain R/B channel swap (one `uint32` load and store per pixel).

**Stage 3 — eliminate bounds checks (DRM path)**

`binary.LittleEndian.Uint32/PutUint32` performs a bounds check on every pixel (384,000 per frame) that the compiler cannot eliminate when using stepped byte offsets. Replacing them with `unsafe.Slice` `[]uint32` row views lets the compiler prove the bounds statically, removing all per-pixel checks. The rotate branch was also hoisted outside both loops.

**Stage 4 — ABGR8888 format + hardware rotation**

Querying the DRM plane's supported fourcc formats revealed ABGR8888 (`AB24`), which matches Go's `image.RGBA` memory layout exactly (R at byte[0], G at byte[1], B at byte[2], A at byte[3]). Switching from XRGB8888 to ABGR8888 via `DRM_IOCTL_MODE_ADDFB2` eliminates the per-pixel R/B channel swap entirely, replacing the inner loop with a plain `copy` of each row. The DRM plane's `rotation` property (`DRM_MODE_ROTATE_180`) is set at startup via `DRM_IOCTL_MODE_OBJ_SETPROPERTY`, offloading 180° rotation to the display controller and making the rotated and non-rotated blit paths identical.

| Path | Time/frame | vs original |
|---|---|---|
| fbdev 16bpp (original) | 53.2 ms | baseline |
| fbdev 16bpp (stage 1) | 39.6 ms | −26% |
| fbdev 32bpp (stage 1) | 34.6 ms | −35% |
| DRM XRGB8888 (stage 2) | 28.7 ms | −46% |
| DRM XRGB8888 (stage 3) | 8.6 ms | −84% |
| DRM ABGR8888, sw rotate (stage 3) | 4.93 ms | −91% |
| DRM ABGR8888 + hw rotate (stage 4) | **1.66 ms** | **−97%** |

## Building

```
go build
```

No CGO, no system dependencies beyond a standard Go toolchain. The DRM/KMS and fbdev paths use raw Linux ioctls via the Go `syscall` package.

### Cross-compilation

To build for a Raspberry Pi from macOS or Linux:

| Target | Command |
|---|---|
| Pi 2/3 (ARMv7) | `GOOS=linux GOARCH=arm GOARM=7 go build -o bus .` |
| Pi 1 (ARMv6) | `GOOS=linux GOARCH=arm GOARM=6 go build -o bus .` |
| Pi 4/5 (ARM64) | `GOOS=linux GOARCH=arm64 go build -o bus .` |

Copy the binary to the Pi with `scp bus pi@raspberrypi:bus/bus`.

## Usage

```
./bus [flags]

  -stop int          TFL bus stop code; repeat for multiple stops (touch cycles through stops then shows clock)
  -touch str         touch input device path (auto-detected if empty)
  -debounce dur      minimum interval between touch-triggered stop switches (default 100ms)
  -rotate            rotate display 180 degrees (default true)
  -white             white background: render black text on white instead of white on black
  -font str          path to TTF font file for bus numbers and times (default: Go Bold)
  -points int        font height in points for bus numbers and times (default 100)
  -color str         text color as X11 color name (e.g. white, orange, darkred, cornflowerblue; default: white)
  -fb                force framebuffer (/dev/fb0) rendering, skipping DRM even if available (useful for testing the fbdev path on a machine that supports both)
  -debug             log DRM device information and other diagnostic output
  -weather-key str   weatherapi.com API key
```

## Configuration

Weather is fetched from [weatherapi.com](https://www.weatherapi.com/). Sign up for a free API key and pass it via `-weather-key`. The location is derived automatically from the GPS coordinates returned by the TFL API for the configured bus stop.

## History

Originally written in Python a decade ago, it stopped working when TFL dropped support for old TLS ciphers and Python 2.7 was no longer maintained on the Pi. It was rewritten in Go, and later migrated from Fyne (which required CGO and a GPU driver) to a pure-Go framebuffer renderer. The old Python code lives in the [pybus](https://github.com/amnonbc/bus/tree/master/pybus) directory.
