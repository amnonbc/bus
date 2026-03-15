# bus

<img width="434" alt="Screenshot 2023-12-13 at 06 45 12" src="https://github.com/amnonbc/bus/assets/7283149/7755bdeb-f894-4c08-a951-1003bbcb1666">

A full-screen bus arrival display that runs on a Raspberry Pi 2, showing the next buses due at a nearby stop using the [TFL Countdown](http://countdown.tfl.gov.uk/) API, along with the current weather and time.

## How it works

- **Linux (Pi):** renders directly to `/dev/fb0` — no X11, Wayland, or GPU driver required.
- **macOS (dev):** serves a live preview at `http://localhost:8080`, rendering to PNG on each request.

The display shows up to three upcoming buses (route number + minutes until arrival), weather conditions at the bottom left, and a clock at the bottom right.

## Building

```
go build
```

No CGO, no system dependencies beyond a standard Go toolchain.

## Usage

```
./bus [flags]

  -stop int          TFL bus stop code (default 74640)
  -rotate            rotate display 180 degrees (default true)
  -weather-key str   weatherapi.com API key
  -location str      location for weather — postcode or city (default "N2")
```

## Configuration

Weather is fetched from [weatherapi.com](https://www.weatherapi.com/). Sign up for a free API key and pass it via `-weather-key`.

## History

Originally written in Python a decade ago, it stopped working when TFL dropped support for old TLS ciphers and Python 2.7 was no longer maintained on the Pi. It was rewritten in Go, and later migrated from Fyne (which required CGO and a GPU driver) to a pure-Go framebuffer renderer. The old Python code lives in the [pybus](https://github.com/amnonbc/bus/tree/master/pybus) directory.
