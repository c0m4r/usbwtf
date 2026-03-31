// ================================================= //
// usbwtf - USB port inspector for Linux®            //
// https://github.com/c0m4r/usbwtf                   //
//                                                   //
// Linux® is the registered trademark of             //
// Linus Torvalds in the U.S. and other countries.   //
// ================================================= //

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const sysfsUSB = "/sys/bus/usb/devices"

// Version info — injected at build time via -ldflags.
var (
	version    = "dev"
	buildDate  = "unknown"
)

const projectURL = "https://github.com/c0m4r/usbwtf"

// ANSI color codes
type colors struct {
	Reset     string
	Bold      string
	Dim       string
	Underline string
	Red       string
	Green     string
	Yellow    string
	Blue      string
	Magenta   string
	Cyan      string
	White     string
	BrightRed string
	BrightGrn string
	BrightYlw string
	BrightBlu string
	BrightMag string
	BrightCyn string
	BrightWht string
	Gray      string
}

var c colors

func initColors() {
	if os.Getenv("NO_COLOR") != "" {
		return
	}
	// Check if stdout is a terminal
	stat, err := os.Stdout.Stat()
	if err != nil {
		return
	}
	// If it's a pipe/file, check for FORCE_COLOR
	if (stat.Mode()&os.ModeCharDevice) == 0 && os.Getenv("FORCE_COLOR") == "" {
		return
	}
	c = colors{
		Reset:     "\033[0m",
		Bold:      "\033[1m",
		Dim:       "\033[2m",
		Underline: "\033[4m",
		Red:       "\033[31m",
		Green:     "\033[38;5;65m",  // muted sage
		Yellow:    "\033[38;5;180m", // soft amber
		Blue:      "\033[38;5;67m",  // steel blue
		Magenta:   "\033[38;5;139m", // dusty mauve
		Cyan:      "\033[38;5;73m",  // muted teal
		White:     "\033[37m",
		BrightRed: "\033[91m",
		BrightGrn: "\033[38;5;108m", // soft green
		BrightYlw: "\033[38;5;223m", // warm cream
		BrightBlu: "\033[38;5;110m", // soft sky
		BrightMag: "\033[38;5;182m", // light lavender
		BrightCyn: "\033[38;5;116m", // soft aqua
		BrightWht: "\033[38;5;252m", // off-white
		Gray:      "\033[38;5;242m", // medium gray
	}
}

// USBDevice holds all the info we can gather from sysfs for a USB device or root hub.
type USBDevice struct {
	SysName      string
	BusNum       int
	DevNum       int
	DevPath      string
	Product      string
	Manufacturer string
	Serial       string
	VendorID     string
	ProductID    string
	Speed        string // Mbps string from sysfs
	MaxPower     string // e.g. "100mA"
	DeviceClass  string
	DeviceSubCl  string
	DeviceProto  string
	Version      string // USB spec version
	BCDDevice    string
	MaxChild     int
	Removable    string
	LTMCapable   string
	RxLanes      string
	TxLanes      string
	BMAttributes string
	Authorized   string
	URBNum       string // USB Request Block count — indicates transfer activity
	NumConfigs   string
	NumIfaces    string
	Quirks       string

	// Power management
	RuntimeStatus     string
	ActiveTime        string
	SuspendedTime     string
	ConnectedDuration string
	ActiveDuration    string
	Persist           string
	Control           string

	// Interfaces & drivers
	Interfaces []USBInterface
}

// USBInterface represents one interface of a USB device.
type USBInterface struct {
	SysName    string
	ClassCode  string
	SubClass   string
	Protocol   string
	Driver     string
	NumEndpts  string
	ClassName  string
	AltSetting string
}

func main() {
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "%sError:%s This tool reads from Linux sysfs (%s) and only works on Linux.\n",
			c.Red, c.Reset, sysfsUSB)
		os.Exit(1)
	}

	initColors()

	if _, err := os.Stat(sysfsUSB); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "%sError:%s %s not found. Is sysfs mounted?\n", c.Red, c.Reset, sysfsUSB)
		os.Exit(1)
	}

	rootHubs, devices := scanDevices()

	printBanner()

	// Sort root hubs by bus number
	sort.Slice(rootHubs, func(i, j int) bool {
		return rootHubs[i].BusNum < rootHubs[j].BusNum
	})

	// Group root hubs by controller (serial field contains PCI address for root hubs)
	type controller struct {
		pciAddr string
		product string
		hubs    []*USBDevice
	}
	controllerMap := make(map[string]*controller)
	var controllerOrder []string
	for _, hub := range rootHubs {
		pci := hub.Serial
		if pci == "" {
			pci = fmt.Sprintf("bus%d", hub.BusNum)
		}
		ct, ok := controllerMap[pci]
		if !ok {
			ct = &controller{pciAddr: pci, product: hub.Product}
			controllerMap[pci] = ct
			controllerOrder = append(controllerOrder, pci)
		}
		ct.hubs = append(ct.hubs, hub)
	}

	for ci, pci := range controllerOrder {
		ctrl := controllerMap[pci]

		// Controller header
		ctrlType := identifyControllerType(ctrl.hubs)
		fmt.Printf("%s┌─%s %sController: %s%s", c.Dim, c.Reset, c.Bold, pci, c.Reset)
		if ctrlType != "" {
			fmt.Printf(" %s(%s)%s", c.Gray, ctrlType, c.Reset)
		}
		fmt.Println()

		// Show capabilities
		var caps []string
		for _, h := range ctrl.hubs {
			ver := guessUSBVersionShort(h.Speed, strings.TrimSpace(h.Version))
			caps = append(caps, ver)
		}
		fmt.Printf("%s│%s  Capabilities: %s%s%s\n", c.Dim, c.Reset, c.BrightYlw, strings.Join(unique(caps), ", "), c.Reset)
		fmt.Printf("%s│%s\n", c.Dim, c.Reset)

		for _, hub := range ctrl.hubs {
			usbVer := guessUSBVersionShort(hub.Speed, strings.TrimSpace(hub.Version))
			speedDesc := speedDescription(hub.Speed)
			speedColor := speedToColor(hub.Speed)

			fmt.Printf("%s│  ┌─%s %sRoot Hub:%s Bus %s%d%s (%s) — %s%s%s, %d port(s)\n",
				c.Dim, c.Reset, c.Bold, c.Reset,
				c.BrightWht, hub.BusNum, c.Reset, hub.SysName,
				speedColor, usbVer, c.Reset, hub.MaxChild)
			fmt.Printf("%s│  │%s  Max speed: %s%s%s\n",
				c.Dim, c.Reset, speedColor, speedDesc, c.Reset)

			// Find devices on this bus
			busDevices := filterByBus(devices, hub.BusNum)
			sort.Slice(busDevices, func(i, j int) bool {
				return naturalLess(busDevices[i].SysName, busDevices[j].SysName)
			})

			if len(busDevices) == 0 {
				fmt.Printf("%s│  │%s  %s(no devices connected)%s\n", c.Dim, c.Reset, c.Gray, c.Reset)
			}

			for _, dev := range busDevices {
				printDevice(dev, fmt.Sprintf("%s│  │%s  ", c.Dim, c.Reset))
			}
			fmt.Printf("%s│  └──%s\n", c.Dim, c.Reset)
		}
		fmt.Printf("%s└──%s\n", c.Dim, c.Reset)
		if ci < len(controllerOrder)-1 {
			fmt.Println()
		}
	}

	// Summary
	fmt.Println()
	printSummary(controllerOrder, rootHubs, devices)
}

func printBanner() {
	fmt.Printf("%s", c.Dim)
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║%s%s                          USB WTF — Port Inspector                            %s%s║%s\n",
		c.Reset, c.Bold, c.Reset, c.Dim, c.Reset)
	fmt.Printf("%s╚══════════════════════════════════════════════════════════════════════════════╝%s\n", c.Dim, c.Reset)
	fmt.Printf("%s  v%s (%s) — %s%s\n\n", c.Gray, version, buildDate, projectURL, c.Reset)
}

func printDevice(dev *USBDevice, prefix string) {
	devType := classifyDevice(dev)
	usbVer := guessUSBVersionShort(dev.Speed, strings.TrimSpace(dev.Version))
	speedDesc := speedDescription(dev.Speed)
	speedCol := speedToColor(dev.Speed)
	typeIcon := deviceIcon(dev)

	// Device header
	name := dev.Product
	if name == "" {
		name = fmt.Sprintf("Unknown Device [%s:%s]", dev.VendorID, dev.ProductID)
	}

	// Unauthorized device warning
	unauthorized := dev.Authorized == "0"

	// Tree line helper
	tl := c.Dim // tree lines use dim

	fmt.Printf("%s\n", prefix)
	if unauthorized {
		fmt.Printf("%s%s┌─%s %s%s%s  %s(UNAUTHORIZED)%s\n", prefix, tl, c.Reset, c.Bold, name, c.Reset, c.BrightRed, c.Reset)
	} else {
		fmt.Printf("%s%s┌─%s %s%s%s\n", prefix, tl, c.Reset, c.Bold, name, c.Reset)
	}

	if dev.Manufacturer != "" {
		fmt.Printf("%s%s│%s  Manufacturer:  %s\n", prefix, tl, c.Reset, dev.Manufacturer)
	}
	fmt.Printf("%s%s│%s  ID:            %s%s:%s%s\n", prefix, tl, c.Reset, c.Yellow, dev.VendorID, dev.ProductID, c.Reset)
	fmt.Printf("%s%s│%s  Port:          Bus %d, Port %s %s(sysfs: %s)%s\n",
		prefix, tl, c.Reset, dev.BusNum, dev.DevPath, c.Gray, dev.SysName, c.Reset)
	fmt.Printf("%s%s│%s  Type:          %s%s%s%s\n", prefix, tl, c.Reset, c.Magenta, typeIcon, devType, c.Reset)
	fmt.Printf("%s%s│%s  USB Version:   %s%s%s\n", prefix, tl, c.Reset, speedCol, usbVer, c.Reset)
	fmt.Printf("%s%s│%s  Speed:         %s%s%s\n", prefix, tl, c.Reset, speedCol, speedDesc, c.Reset)

	if dev.RxLanes != "" && dev.RxLanes != "0" && dev.RxLanes != "1" {
		fmt.Printf("%s%s│%s  Lanes:         %s rx / %s tx\n", prefix, tl, c.Reset, dev.RxLanes, dev.TxLanes)
	}

	// Power draw with color coding
	mA := parsePowerMA(dev.MaxPower)
	if dev.MaxPower != "" {
		powerCol := c.Gray
		if mA >= 900 {
			powerCol = c.BrightRed
		} else if mA >= 500 {
			powerCol = c.Yellow
		}
		fmt.Printf("%s%s│%s  Max Power:     %s%s%s\n", prefix, tl, c.Reset, powerCol, dev.MaxPower, c.Reset)
	}

	// Connection status / activity
	printStatus(dev, prefix)

	// Data activity (URB count)
	if dev.URBNum != "" && dev.URBNum != "0" {
		urbCount, _ := strconv.ParseInt(dev.URBNum, 10, 64)
		fmt.Printf("%s%s│%s  Transfers:     %s%s URBs%s\n",
			prefix, tl, c.Reset, c.Gray, formatNumber(urbCount), c.Reset)
	}

	// Uptime / connected duration
	if dev.ConnectedDuration != "" {
		connMs, _ := strconv.ParseInt(dev.ConnectedDuration, 10, 64)
		if connMs > 0 {
			fmt.Printf("%s%s│%s  Connected for: %s%s%s\n",
				prefix, tl, c.Reset, c.Gray, formatDuration(time.Duration(connMs)*time.Millisecond), c.Reset)
		}
	}

	if dev.Serial != "" {
		fmt.Printf("%s%s│%s  Serial:        %s%s%s\n", prefix, tl, c.Reset, c.Gray, dev.Serial, c.Reset)
	}
	if dev.Removable != "" && dev.Removable != "unknown" {
		label := dev.Removable
		fmt.Printf("%s%s│%s  Removable:     %s%s%s\n", prefix, tl, c.Reset, c.Gray, label, c.Reset)
	}
	if dev.BMAttributes != "" {
		fmt.Printf("%s%s│%s  Attributes:    %s\n", prefix, tl, c.Reset, describeAttributes(dev.BMAttributes))
	}
	if dev.LTMCapable == "yes" {
		fmt.Printf("%s%s│%s  LTM:           %sLatency Tolerance Messaging supported%s\n",
			prefix, tl, c.Reset, c.Gray, c.Reset)
	}
	if dev.Quirks != "" && dev.Quirks != "0x0" {
		fmt.Printf("%s%s│%s  Quirks:        %s%s%s\n", prefix, tl, c.Reset, c.BrightRed, dev.Quirks, c.Reset)
	}

	// Interfaces
	if len(dev.Interfaces) > 0 {
		fmt.Printf("%s%s│%s  Interfaces:\n", prefix, tl, c.Reset)
		for _, iface := range dev.Interfaces {
			driverStr := iface.Driver
			driverCol := c.BrightGrn
			if driverStr == "" {
				driverStr = "(no driver bound)"
				driverCol = c.BrightRed
			}
			fmt.Printf("%s%s│%s    %s•%s %-28s %s[%s:%s:%s]%s — driver: %s%s%s, endpoints: %s\n",
				prefix, tl, c.Reset, c.Blue, c.Reset,
				iface.ClassName,
				c.Gray, iface.ClassCode, iface.SubClass, iface.Protocol, c.Reset,
				driverCol, driverStr, c.Reset, iface.NumEndpts)
		}
	}

	// If it's a hub, show port count
	if dev.MaxChild > 0 {
		fmt.Printf("%s%s│%s  Hub Ports:     %d\n", prefix, tl, c.Reset, dev.MaxChild)
	}

	fmt.Printf("%s%s└──%s\n", prefix, tl, c.Reset)
}

func printStatus(dev *USBDevice, prefix string) {
	tl := c.Dim

	// Runtime power status
	var statusCol string
	statusText := ""
	switch dev.RuntimeStatus {
	case "active":
		statusCol = c.BrightGrn
		statusText = "Active"
	case "suspended":
		statusCol = c.Yellow
		statusText = "Suspended (idle)"
	case "unsupported":
		statusCol = c.Gray
		statusText = "PM unsupported"
	default:
		if dev.RuntimeStatus != "" {
			statusText = dev.RuntimeStatus
			statusCol = c.Gray
		} else {
			statusText = "Connected"
			statusCol = c.Gray
		}
	}

	fmt.Printf("%s%s│%s  Status:        %s%s%s", prefix, tl, c.Reset, statusCol, statusText, c.Reset)

	// Active/suspended time percentage
	activeMs, _ := strconv.ParseInt(dev.ActiveTime, 10, 64)
	suspendMs, _ := strconv.ParseInt(dev.SuspendedTime, 10, 64)
	total := activeMs + suspendMs
	if total > 0 {
		activePct := float64(activeMs) / float64(total) * 100
		fmt.Printf("  %s(active %.1f%% of uptime)%s", c.Gray, activePct, c.Reset)
	}
	fmt.Println()

	// Power management control setting
	if dev.Control != "" {
		label := dev.Control
		switch dev.Control {
		case "auto":
			label = "auto (kernel manages suspend)"
		case "on":
			label = "on (always active, no autosuspend)"
		}
		fmt.Printf("%s%s│%s  PM Control:    %s%s%s\n", prefix, tl, c.Reset, c.Gray, label, c.Reset)
	}
}

func printSummary(controllerOrder []string, rootHubs []*USBDevice, devices []*USBDevice) {
	fmt.Printf("%s─── Summary ──────────────────────────────────────────────%s\n", c.Dim, c.Reset)

	fmt.Printf("  Controllers:       %d\n", len(controllerOrder))
	fmt.Printf("  Root Hubs:         %d\n", len(rootHubs))
	fmt.Printf("  Connected Devices: %d\n", len(devices))

	// Count ports that have (or had) a device connected — these are real physical ports.
	// We do this by looking at which root hub port numbers are actually referenced by devices.
	occupiedPorts := make(map[string]bool)
	for _, d := range devices {
		// Top-level port: first segment of sysfs name, e.g. "3-6" from "3-6" or "3-3" from "3-3.1"
		parts := strings.SplitN(d.SysName, ".", 2)
		occupiedPorts[parts[0]] = true
	}
	fmt.Printf("  Ports in Use:      %d\n", len(occupiedPorts))

	// Power summary
	var totalPower int
	for _, d := range devices {
		totalPower += parsePowerMA(d.MaxPower)
	}
	powerCol := ""
	if totalPower >= 2000 {
		powerCol = c.BrightRed
	} else if totalPower >= 1000 {
		powerCol = c.Yellow
	}
	fmt.Printf("  Total Power Draw:  %s%d mA%s %s(max declared)%s\n",
		powerCol, totalPower, c.Reset, c.Gray, c.Reset)

	// Speed breakdown
	speedBuckets := map[string]int{}
	for _, d := range devices {
		speedBuckets[d.Speed]++
	}
	if len(speedBuckets) > 0 {
		fmt.Printf("  Speed Breakdown:   ")
		first := true
		for _, sp := range []string{"20000", "10000", "5000", "480", "12", "1.5"} {
			cnt, ok := speedBuckets[sp]
			if !ok {
				continue
			}
			if !first {
				fmt.Printf(", ")
			}
			col := speedToColor(sp)
			label := speedLabel(sp)
			fmt.Printf("%s%d x %s%s", col, cnt, label, c.Reset)
			first = false
		}
		fmt.Println()
	}

	// Unauthorized device warning
	var unauth []string
	for _, d := range devices {
		if d.Authorized == "0" {
			unauth = append(unauth, fmt.Sprintf("%s:%s (%s)", d.VendorID, d.ProductID, d.SysName))
		}
	}
	if len(unauth) > 0 {
		fmt.Printf("\n  %s%sWARNING: %d unauthorized device(s):%s\n", c.Bold, c.BrightRed, len(unauth), c.Reset)
		for _, u := range unauth {
			fmt.Printf("    %s• %s%s\n", c.BrightRed, u, c.Reset)
		}
	}

	// Driverless interfaces
	var driverless []string
	for _, d := range devices {
		for _, iface := range d.Interfaces {
			if iface.Driver == "" {
				driverless = append(driverless, fmt.Sprintf("%s %s (%s)", d.SysName, iface.ClassName, iface.SysName))
			}
		}
	}
	if len(driverless) > 0 {
		fmt.Printf("\n  %sInterfaces without drivers:%s\n", c.Yellow, c.Reset)
		for _, dl := range driverless {
			fmt.Printf("    %s• %s%s\n", c.Yellow, dl, c.Reset)
		}
	}

	fmt.Printf("%s──────────────────────────────────────────────────────────%s\n", c.Dim, c.Reset)
}

func scanDevices() (rootHubs []*USBDevice, devices []*USBDevice) {
	entries, err := os.ReadDir(sysfsUSB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError:%s Cannot read %s: %v\n", c.Red, c.Reset, sysfsUSB, err)
		fmt.Fprintf(os.Stderr, "Make sure sysfs is mounted and you have read permissions.\n")
		os.Exit(1)
	}

	for _, entry := range entries {
		name := entry.Name()

		// Root hubs: "usb1", "usb2", etc.
		if strings.HasPrefix(name, "usb") {
			dev := readDevice(name)
			if dev != nil {
				rootHubs = append(rootHubs, dev)
			}
			continue
		}

		// Actual devices: "1-6", "3-3.1", etc. (no colon = not an interface)
		if !strings.Contains(name, ":") && len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
			dev := readDevice(name)
			if dev != nil {
				devices = append(devices, dev)
			}
		}
	}
	return
}

func readDevice(sysName string) *USBDevice {
	base := filepath.Join(sysfsUSB, sysName)

	dev := &USBDevice{
		SysName:      sysName,
		Product:      readAttr(base, "product"),
		Manufacturer: readAttr(base, "manufacturer"),
		Serial:       readAttr(base, "serial"),
		VendorID:     readAttr(base, "idVendor"),
		ProductID:    readAttr(base, "idProduct"),
		Speed:        readAttr(base, "speed"),
		MaxPower:     readAttr(base, "bMaxPower"),
		DeviceClass:  readAttr(base, "bDeviceClass"),
		DeviceSubCl:  readAttr(base, "bDeviceSubClass"),
		DeviceProto:  readAttr(base, "bDeviceProtocol"),
		Version:      readAttr(base, "version"),
		BCDDevice:    readAttr(base, "bcdDevice"),
		DevPath:      readAttr(base, "devpath"),
		Removable:    readAttr(base, "removable"),
		LTMCapable:   readAttr(base, "ltm_capable"),
		RxLanes:      readAttr(base, "rx_lanes"),
		TxLanes:      readAttr(base, "tx_lanes"),
		BMAttributes: readAttr(base, "bmAttributes"),
		Authorized:   readAttr(base, "authorized"),
		URBNum:       readAttr(base, "urbnum"),
		NumConfigs:   readAttr(base, "bNumConfigurations"),
		NumIfaces:    readAttr(base, "bNumInterfaces"),
		Quirks:       readAttr(base, "quirks"),

		// Power
		RuntimeStatus:     readAttr(filepath.Join(base, "power"), "runtime_status"),
		ActiveTime:        readAttr(filepath.Join(base, "power"), "runtime_active_time"),
		SuspendedTime:     readAttr(filepath.Join(base, "power"), "runtime_suspended_time"),
		ConnectedDuration: readAttr(filepath.Join(base, "power"), "connected_duration"),
		ActiveDuration:    readAttr(filepath.Join(base, "power"), "active_duration"),
		Persist:           readAttr(filepath.Join(base, "power"), "persist"),
		Control:           readAttr(filepath.Join(base, "power"), "control"),
	}

	dev.BusNum, _ = strconv.Atoi(readAttr(base, "busnum"))
	dev.DevNum, _ = strconv.Atoi(readAttr(base, "devnum"))
	dev.MaxChild, _ = strconv.Atoi(readAttr(base, "maxchild"))

	// Read interfaces
	entries, _ := os.ReadDir(base)
	for _, e := range entries {
		iName := e.Name()
		if !strings.Contains(iName, ":") {
			continue
		}
		iBase := filepath.Join(base, iName)
		classCode := readAttr(iBase, "bInterfaceClass")
		if classCode == "" {
			continue
		}
		iface := USBInterface{
			SysName:    iName,
			ClassCode:  classCode,
			SubClass:   readAttr(iBase, "bInterfaceSubClass"),
			Protocol:   readAttr(iBase, "bInterfaceProtocol"),
			NumEndpts:  readAttr(iBase, "bNumEndpoints"),
			AltSetting: readAttr(iBase, "bAlternateSetting"),
		}
		// Get driver
		driverLink, err := os.Readlink(filepath.Join(iBase, "driver"))
		if err == nil {
			iface.Driver = filepath.Base(driverLink)
		}
		iface.ClassName = interfaceClassName(classCode, iface.SubClass, iface.Protocol)
		dev.Interfaces = append(dev.Interfaces, iface)
	}

	return dev
}

func readAttr(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func filterByBus(devices []*USBDevice, busNum int) []*USBDevice {
	var result []*USBDevice
	for _, d := range devices {
		if d.BusNum == busNum {
			result = append(result, d)
		}
	}
	return result
}

// identifyControllerType returns a short description like "xHCI" based on root hub info.
func identifyControllerType(hubs []*USBDevice) string {
	if len(hubs) == 0 {
		return ""
	}
	prod := hubs[0].Product
	switch {
	case strings.Contains(strings.ToLower(prod), "xhci"):
		return "xHCI"
	case strings.Contains(strings.ToLower(prod), "ehci"):
		return "EHCI — USB 2.0 only"
	case strings.Contains(strings.ToLower(prod), "ohci"):
		return "OHCI — USB 1.1"
	case strings.Contains(strings.ToLower(prod), "uhci"):
		return "UHCI — USB 1.1"
	}
	return ""
}

func guessUSBVersionShort(speedMbps, specVersion string) string {
	switch speedMbps {
	case "5000":
		return "USB 3.0 (SuperSpeed)"
	case "10000":
		return "USB 3.1 Gen 2 (SuperSpeed+)"
	case "20000":
		return "USB 3.2 Gen 2x2"
	case "480":
		return "USB 2.0 (High Speed)"
	case "12":
		return "USB 1.1 (Full Speed)"
	case "1.5":
		return "USB 1.0 (Low Speed)"
	}
	if specVersion != "" {
		return "USB " + specVersion
	}
	return "Unknown"
}

func speedDescription(speedMbps string) string {
	switch speedMbps {
	case "5000":
		return "5 Gbps (SuperSpeed)"
	case "10000":
		return "10 Gbps (SuperSpeed+)"
	case "20000":
		return "20 Gbps (SuperSpeed+ 2x2)"
	case "480":
		return "480 Mbps (High Speed)"
	case "12":
		return "12 Mbps (Full Speed)"
	case "1.5":
		return "1.5 Mbps (Low Speed)"
	default:
		if speedMbps != "" {
			return speedMbps + " Mbps"
		}
		return "Unknown"
	}
}

func speedLabel(speedMbps string) string {
	switch speedMbps {
	case "20000":
		return "USB 3.2"
	case "10000":
		return "USB 3.1"
	case "5000":
		return "USB 3.0"
	case "480":
		return "USB 2.0"
	case "12":
		return "USB 1.1"
	case "1.5":
		return "USB 1.0"
	default:
		return speedMbps + "M"
	}
}

func speedToColor(speedMbps string) string {
	switch speedMbps {
	case "20000":
		return c.BrightCyn
	case "10000":
		return c.BrightBlu
	case "5000":
		return c.Blue
	case "480":
		return c.BrightGrn
	case "12":
		return c.Yellow
	case "1.5":
		return c.Gray
	default:
		return c.White
	}
}

func classifyDevice(dev *USBDevice) string {
	cls := dev.DeviceClass

	// If device class is 00 (defined at interface level), look at interface classes
	if cls == "00" && len(dev.Interfaces) > 0 {
		cls = dev.Interfaces[0].ClassCode
	}

	switch cls {
	case "01":
		return "Audio Device"
	case "02":
		// Refine CDC: could be serial/modem, ethernet, etc.
		if len(dev.Interfaces) > 0 {
			for _, iface := range dev.Interfaces {
				switch iface.SubClass {
				case "06":
					return "Ethernet Adapter (CDC ECM)"
				case "0d":
					return "Network Adapter (CDC NCM)"
				case "0e":
					return "Network Adapter (CDC MBIM)"
				case "02":
					return "Modem / Serial Port (CDC ACM)"
				case "01":
					return "Direct Line Modem"
				}
			}
		}
		return "Communication Device (CDC)"
	case "03":
		// Refine HID: keyboard, mouse, gamepad...
		for _, iface := range dev.Interfaces {
			if iface.SubClass == "01" {
				switch iface.Protocol {
				case "01":
					return "Keyboard (HID)"
				case "02":
					return "Mouse (HID)"
				}
			}
		}
		return "Human Interface Device (HID)"
	case "05":
		return "Physical Device"
	case "06":
		return "Imaging Device (Scanner/Camera)"
	case "07":
		return "Printer"
	case "08":
		// Refine mass storage
		for _, iface := range dev.Interfaces {
			switch iface.SubClass {
			case "06":
				return "Mass Storage (SCSI/USB Flash Drive)"
			case "04":
				return "Mass Storage (Floppy)"
			case "02":
				return "Mass Storage (ATAPI/CD-ROM)"
			case "01":
				return "Mass Storage (RBC)"
			}
		}
		return "Mass Storage"
	case "09":
		return "Hub"
	case "0a":
		return "CDC Data"
	case "0b":
		return "Smart Card Reader"
	case "0d":
		return "Content Security"
	case "0e":
		return "Video Device (Webcam/Capture)"
	case "0f":
		return "Personal Healthcare Device"
	case "10":
		return "Audio/Video Device"
	case "11":
		return "Billboard Device"
	case "12":
		return "USB Type-C Bridge"
	case "dc":
		return "Diagnostic Device"
	case "e0":
		sub := dev.DeviceSubCl
		if sub == "" && len(dev.Interfaces) > 0 {
			sub = dev.Interfaces[0].SubClass
		}
		switch sub {
		case "01":
			return "Bluetooth Adapter"
		case "02":
			return "Wireless USB Adapter"
		default:
			return "Wireless Controller"
		}
	case "ef":
		// Many composite devices use this
		return "Miscellaneous / Composite Device"
	case "fe":
		for _, iface := range dev.Interfaces {
			if iface.SubClass == "01" {
				return "Device Firmware Upgrade (DFU)"
			}
		}
		return "Application-Specific"
	case "ff":
		return "Vendor-Specific"
	}
	if cls == "" {
		return "Unknown"
	}
	return "Unknown (class 0x" + cls + ")"
}

func deviceIcon(dev *USBDevice) string {
	cls := dev.DeviceClass
	if cls == "00" && len(dev.Interfaces) > 0 {
		cls = dev.Interfaces[0].ClassCode
	}

	// Return a simple text icon to help visually distinguish device types
	switch cls {
	case "01":
		return "[AUD] "
	case "02":
		return "[NET] "
	case "03":
		for _, iface := range dev.Interfaces {
			if iface.SubClass == "01" {
				switch iface.Protocol {
				case "01":
					return "[KBD] "
				case "02":
					return "[MOU] "
				}
			}
		}
		return "[HID] "
	case "06":
		return "[IMG] "
	case "07":
		return "[PRN] "
	case "08":
		return "[DSK] "
	case "09":
		return "[HUB] "
	case "0b":
		return "[CRD] "
	case "0e":
		return "[CAM] "
	case "e0":
		sub := dev.DeviceSubCl
		if sub == "" && len(dev.Interfaces) > 0 {
			sub = dev.Interfaces[0].SubClass
		}
		if sub == "01" {
			return "[BT]  "
		}
		return "[WLN] "
	case "ef":
		return "[MIX] "
	case "ff":
		return "[VND] "
	}
	return "[???] "
}

func parsePowerMA(s string) int {
	s = strings.TrimSuffix(s, "mA")
	s = strings.TrimSpace(s)
	v, _ := strconv.Atoi(s)
	return v
}

func interfaceClassName(classCode, subClass, protocol string) string {
	switch classCode {
	case "01":
		switch subClass {
		case "01":
			return "Audio Control"
		case "02":
			return "Audio Streaming"
		case "03":
			return "MIDI Streaming"
		}
		return "Audio"
	case "02":
		switch subClass {
		case "01":
			return "Direct Line Modem"
		case "02":
			return "ACM (Modem/Serial)"
		case "03":
			return "Telephone"
		case "04":
			return "Multi-Channel"
		case "05":
			return "CAPI Control"
		case "06":
			return "Ethernet Networking (ECM)"
		case "07":
			return "ATM Networking"
		case "08":
			return "Wireless Handset"
		case "09":
			return "Device Management"
		case "0a":
			return "Mobile Direct Line"
		case "0b":
			return "OBEX"
		case "0c":
			return "Ethernet Emulation (EEM)"
		case "0d":
			return "NCM Networking"
		case "0e":
			return "MBIM Networking"
		}
		return "Communications"
	case "03":
		sub := "HID"
		if subClass == "01" {
			sub = "Boot Interface HID"
			switch protocol {
			case "01":
				return "Keyboard"
			case "02":
				return "Mouse"
			}
		}
		return sub
	case "05":
		return "Physical"
	case "06":
		switch subClass {
		case "01":
			return "Still Imaging"
		}
		return "Imaging"
	case "07":
		switch subClass {
		case "01":
			switch protocol {
			case "01":
				return "Printer (Unidirectional)"
			case "02":
				return "Printer (Bidirectional)"
			case "03":
				return "Printer (IEEE 1284.4)"
			}
		}
		return "Printer"
	case "08":
		proto := ""
		switch protocol {
		case "50":
			proto = " [Bulk-Only]"
		case "62":
			proto = " [UAS]"
		}
		switch subClass {
		case "01":
			return "Mass Storage (RBC)" + proto
		case "02":
			return "Mass Storage (ATAPI)" + proto
		case "03":
			return "Mass Storage (QIC-157)" + proto
		case "04":
			return "Mass Storage (UFI/Floppy)" + proto
		case "05":
			return "Mass Storage (SFF-8070i)" + proto
		case "06":
			return "Mass Storage (SCSI)" + proto
		}
		return "Mass Storage" + proto
	case "09":
		return "Hub"
	case "0a":
		return "CDC Data"
	case "0b":
		return "Smart Card"
	case "0d":
		return "Content Security"
	case "0e":
		switch subClass {
		case "01":
			return "Video Control"
		case "02":
			return "Video Streaming"
		case "03":
			return "Video Interface Collection"
		}
		return "Video"
	case "10":
		switch subClass {
		case "01":
			return "AV Control"
		case "02":
			return "AV Data Video Streaming"
		case "03":
			return "AV Data Audio Streaming"
		}
		return "Audio/Video"
	case "e0":
		switch subClass {
		case "01":
			switch protocol {
			case "01":
				return "Bluetooth"
			case "02":
				return "Bluetooth UWB"
			case "03":
				return "Bluetooth RNDIS"
			case "04":
				return "Bluetooth AMP"
			}
			return "Bluetooth"
		case "02":
			switch protocol {
			case "01":
				return "Host Wire Adapter"
			case "02":
				return "Device Wire Adapter"
			}
			return "Wireless USB"
		}
		return "Wireless"
	case "ef":
		switch subClass {
		case "01":
			return "Active Sync"
		case "02":
			switch protocol {
			case "01":
				return "Interface Association"
			case "02":
				return "Wire Adapter Multifunction"
			}
			return "Common Class"
		case "05":
			return "USB Type-C Alt Mode"
		}
		return "Miscellaneous"
	case "fe":
		switch subClass {
		case "01":
			return "Device Firmware Upgrade (DFU)"
		case "02":
			return "IrDA Bridge"
		case "03":
			switch protocol {
			case "01":
				return "TMC (Test & Measurement)"
			case "02":
				return "TMC USB488"
			}
			return "Test & Measurement"
		}
		return "Application Specific"
	case "ff":
		return "Vendor Specific"
	}
	return "Unknown (0x" + classCode + ")"
}

func describeAttributes(bmAttr string) string {
	val, err := strconv.ParseUint(bmAttr, 16, 8)
	if err != nil {
		return bmAttr
	}

	var attrs []string
	if val&0x40 != 0 {
		attrs = append(attrs, fmt.Sprintf("%sSelf-Powered%s", c.BrightGrn, c.Reset))
	} else {
		attrs = append(attrs, fmt.Sprintf("%sBus-Powered%s", c.Gray, c.Reset))
	}
	if val&0x20 != 0 {
		attrs = append(attrs, fmt.Sprintf("%sRemote Wakeup%s", c.Gray, c.Reset))
	}
	if len(attrs) == 0 {
		return bmAttr
	}
	return strings.Join(attrs, ", ")
}

func unique(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// naturalLess sorts sysfs device names in natural order (1-2 before 1-10).
func naturalLess(a, b string) bool {
	partsA := splitNumeric(a)
	partsB := splitNumeric(b)
	for i := range partsA {
		if i >= len(partsB) {
			return false
		}
		if partsA[i] != partsB[i] {
			na, errA := strconv.Atoi(partsA[i])
			nb, errB := strconv.Atoi(partsB[i])
			if errA == nil && errB == nil {
				return na < nb
			}
			return partsA[i] < partsB[i]
		}
	}
	return len(partsA) < len(partsB)
}

func splitNumeric(s string) []string {
	var parts []string
	current := ""
	wasDigit := false
	for _, ch := range s {
		isDigit := ch >= '0' && ch <= '9'
		if current != "" && isDigit != wasDigit {
			parts = append(parts, current)
			current = ""
		}
		current += string(ch)
		wasDigit = isDigit
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func formatNumber(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	// Insert commas
	var result []byte
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(ch))
	}
	return string(result)
}
