<div align="center">

# usbwtf

![Linux](https://img.shields.io/badge/made%20for-linux-yellow?logo=linux&logoColor=ffffff)
![Go](https://img.shields.io/badge/go%20go%20go-blue?logo=go&logoColor=ffffff)
[![License: MIT](https://img.shields.io/badge/License-MIT-black.svg)](https://opensource.org/license/mit)

USB port inspector for Linux®.

Reads from sysfs — no root required, no external dependencies, pure Go.

</div>

```
╔══════════════════════════════════════════════════════════════════════════════╗
║                          USB WTF — Port Inspector                            ║
╚══════════════════════════════════════════════════════════════════════════════╝
  v0.1.0 (2026-03-31) — https://github.com/c0m4r/usbwtf

┌─ Controller: 0000:00:00.0 (xHCI)
│  Capabilities: USB 2.0 (High Speed), USB 3.1 Gen 2 (SuperSpeed+)
│
│  ┌─ Root Hub: Bus 3 (usb3) — USB 2.0 (High Speed), 12 port(s)
│  │  Max speed: 480 Mbps (High Speed)
│  │  
│  │  ┌─ Razer DeathAdder
│  │  │  Manufacturer:  Razer
│  │  │  ID:            0000:0000
│  │  │  Port:          Bus 3, Port 6 (sysfs: 3-6)
│  │  │  Type:          [MOU] Mouse (HID)
│  │  │  USB Version:   USB 1.1 (Full Speed)
│  │  │  Speed:         12 Mbps (Full Speed)
│  │  │  Max Power:     500mA
│  │  │  Status:        Active  (active 100.0% of uptime)
│  │  │  PM Control:    on (always active, no autosuspend)
│  │  │  Transfers:     3,021,895 URBs
│  │  │  Connected for: 0d 3h 27m
│  │  │  Removable:     removable
│  │  │  Attributes:    Bus-Powered, Remote Wakeup
│  │  │  Interfaces:
│  │  │    • Mouse                        [03:01:02] — driver: usbhid, endpoints: 01
│  │  │    • HID                          [03:00:01] — driver: usbhid, endpoints: 01
│  │  │    • HID                          [03:00:01] — driver: usbhid, endpoints: 01
│  │  └──
│  └──
└──

─── Summary ──────────────────────────────────────────────
  Controllers:       6
  Root Hubs:         11
  Connected Devices: 5
  Ports in Use:      4
  Total Power Draw:  890 mA (max declared)
  Speed Breakdown:   4 x USB 1.1, 1 x USB 1.0
──────────────────────────────────────────────────────────
```

## What it shows

For each connected device:
- **Identity** — product name, manufacturer, vendor:product ID, serial number
- **Location** — controller, bus, port path, sysfs name
- **Type** — auto-detected from USB class codes (Keyboard, Mouse, Bluetooth, Mass Storage, Hub, Webcam, etc.)
- **USB version & speed** — negotiated speed from USB 1.0 (1.5 Mbps) through USB 3.2 Gen 2x2 (20 Gbps)
- **Power draw** — max declared current with high-power warnings
- **Activity status** — active vs suspended, percentage of uptime active
- **Transfer count** — total USB Request Blocks processed (data activity indicator)
- **Connected duration** — how long the device has been plugged in
- **Power management** — autosuspend mode (auto / always-on)
- **Interfaces** — each USB interface with class, bound driver, and endpoint count
- **Attributes** — self-powered vs bus-powered, remote wakeup capability
- **Quirks** — kernel workaround flags if any are applied
- **Unauthorized devices** — flagged in red in the summary

For each controller/hub:
- PCI address and controller type (xHCI / EHCI / OHCI)
- USB capabilities (all speed tiers present)

Summary section shows connected device count, ports in use, total power draw, speed breakdown, and any warnings (unauthorized devices, interfaces without drivers).

## Requirements

- Linux with sysfs mounted (standard on all distributions)
- Go 1.21+ to build

## Build

```bash
go build -o usbwtf .
```

Cross-compile for amd64, arm64, and riscv64 with version injection and size optimization:

```bash
chmod +x build.sh
./build.sh
```

Binaries are placed in `./dist/`. Built with `-trimpath -ldflags "-s -w"` for minimal size and no debug symbols or path information.

## Run

```bash
./usbwtf
```

Color output is automatic when running in a terminal. Disable with `NO_COLOR=1`, force with `FORCE_COLOR=1`.

## Test

```bash
./test.sh
```

## Check for dependency updates

```bash
chmod +x check-updates.sh
./check-updates.sh
```

## License

MIT

Linux® is the registered trademark of Linus Torvalds in the U.S. and other countries.
