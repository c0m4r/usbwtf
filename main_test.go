package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- naturalLess ---

func TestNaturalLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1-2", "1-10", true},
		{"1-10", "1-2", false},
		{"3-3", "3-3.1", true},
		{"3-3.1", "3-3", false},
		{"1-6", "1-6", false},
		{"usb1", "usb10", true},
		{"usb10", "usb2", false},
	}
	for _, tc := range cases {
		got := naturalLess(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("naturalLess(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// --- parsePowerMA ---

func TestParsePowerMA(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"500mA", 500},
		{"100mA", 100},
		{"0mA", 0},
		{"", 0},
		{"90mA", 90},
		{"2mA", 2},
	}
	for _, tc := range cases {
		got := parsePowerMA(tc.input)
		if got != tc.want {
			t.Errorf("parsePowerMA(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// --- unique ---

func TestUnique(t *testing.T) {
	got := unique([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("unique() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("unique()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUniqueEmpty(t *testing.T) {
	got := unique([]string{})
	if len(got) != 0 {
		t.Errorf("unique([]) = %v, want []", got)
	}
}

// --- formatNumber ---

func TestFormatNumber(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{100, "100"},
	}
	for _, tc := range cases {
		got := formatNumber(tc.n)
		if got != tc.want {
			t.Errorf("formatNumber(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// --- formatDuration ---

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{65 * time.Minute, "1h 5m"},
		{25 * time.Hour, "1d 1h 0m"},
		{48 * time.Hour, "2d 0h 0m"},
	}
	for _, tc := range cases {
		got := formatDuration(tc.d)
		if got != tc.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// --- guessUSBVersionShort ---

func TestGuessUSBVersionShort(t *testing.T) {
	cases := []struct {
		speed, spec string
		want        string
	}{
		{"5000", "3.00", "USB 3.0 (SuperSpeed)"},
		{"10000", "3.10", "USB 3.1 Gen 2 (SuperSpeed+)"},
		{"20000", "3.20", "USB 3.2 Gen 2x2"},
		{"480", "2.00", "USB 2.0 (High Speed)"},
		{"12", "1.10", "USB 1.1 (Full Speed)"},
		{"1.5", "1.00", "USB 1.0 (Low Speed)"},
		{"", "2.00", "USB 2.00"},
		{"", "", "Unknown"},
	}
	for _, tc := range cases {
		got := guessUSBVersionShort(tc.speed, tc.spec)
		if got != tc.want {
			t.Errorf("guessUSBVersionShort(%q, %q) = %q, want %q", tc.speed, tc.spec, got, tc.want)
		}
	}
}

// --- speedDescription ---

func TestSpeedDescription(t *testing.T) {
	cases := []struct {
		speed string
		want  string
	}{
		{"5000", "5 Gbps (SuperSpeed)"},
		{"10000", "10 Gbps (SuperSpeed+)"},
		{"20000", "20 Gbps (SuperSpeed+ 2x2)"},
		{"480", "480 Mbps (High Speed)"},
		{"12", "12 Mbps (Full Speed)"},
		{"1.5", "1.5 Mbps (Low Speed)"},
		{"", "Unknown"},
		{"9999", "9999 Mbps"},
	}
	for _, tc := range cases {
		got := speedDescription(tc.speed)
		if got != tc.want {
			t.Errorf("speedDescription(%q) = %q, want %q", tc.speed, got, tc.want)
		}
	}
}

// --- classifyDevice ---

func TestClassifyDevice(t *testing.T) {
	cases := []struct {
		cls  string
		want string
	}{
		{"09", "Hub"},
		{"08", "Mass Storage"},
		{"03", "Human Interface Device (HID)"},
		{"e0", "Bluetooth Adapter"},
		{"01", "Audio Device"},
		{"0e", "Video Device (Webcam/Capture)"},
	}
	for _, tc := range cases {
		dev := &USBDevice{DeviceClass: tc.cls, DeviceSubCl: "01"}
		got := classifyDevice(dev)
		if got != tc.want {
			t.Errorf("classifyDevice(class=%q) = %q, want %q", tc.cls, got, tc.want)
		}
	}
}

func TestClassifyDeviceFromInterface(t *testing.T) {
	// Class 00 means defined at interface level
	dev := &USBDevice{
		DeviceClass: "00",
		Interfaces: []USBInterface{
			{ClassCode: "03", SubClass: "01", Protocol: "01"},
		},
	}
	got := classifyDevice(dev)
	if got != "Keyboard (HID)" {
		t.Errorf("classifyDevice with interface class = %q, want %q", got, "Keyboard (HID)")
	}
}

// --- filterByBus ---

func TestFilterByBus(t *testing.T) {
	devices := []*USBDevice{
		{SysName: "1-1", BusNum: 1},
		{SysName: "2-1", BusNum: 2},
		{SysName: "1-2", BusNum: 1},
	}
	got := filterByBus(devices, 1)
	if len(got) != 2 {
		t.Fatalf("filterByBus(1) returned %d devices, want 2", len(got))
	}
	for _, d := range got {
		if d.BusNum != 1 {
			t.Errorf("unexpected BusNum %d in result", d.BusNum)
		}
	}
	got2 := filterByBus(devices, 99)
	if len(got2) != 0 {
		t.Errorf("filterByBus(99) = %v, want empty", got2)
	}
}

// --- readAttr via temp sysfs ---

func TestReadAttr(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "product"), []byte("Test Device\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readAttr(dir, "product")
	if got != "Test Device" {
		t.Errorf("readAttr() = %q, want %q", got, "Test Device")
	}
}

func TestReadAttrMissing(t *testing.T) {
	dir := t.TempDir()
	got := readAttr(dir, "nonexistent")
	if got != "" {
		t.Errorf("readAttr(missing) = %q, want %q", got, "")
	}
}

// --- describeAttributes ---

func TestDescribeAttributes(t *testing.T) {
	// 0xe0 = 11100000: bit7=reserved, bit6=Self-Powered, bit5=Remote Wakeup
	got := describeAttributes("e0")
	if !contains(got, "Self-Powered") {
		t.Errorf("describeAttributes(e0) missing Self-Powered, got %q", got)
	}
	if !contains(got, "Remote Wakeup") {
		t.Errorf("describeAttributes(e0) missing Remote Wakeup, got %q", got)
	}

	// 0x80 = 10000000: no self-power, no remote wakeup
	got2 := describeAttributes("80")
	if !contains(got2, "Bus-Powered") {
		t.Errorf("describeAttributes(80) missing Bus-Powered, got %q", got2)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- interfaceClassName ---

func TestInterfaceClassName(t *testing.T) {
	cases := []struct {
		class, sub, proto string
		want              string
	}{
		{"03", "01", "01", "Keyboard"},
		{"03", "01", "02", "Mouse"},
		{"03", "00", "00", "HID"},
		{"08", "06", "50", "Mass Storage (SCSI) [Bulk-Only]"},
		{"e0", "01", "01", "Bluetooth"},
		{"09", "00", "00", "Hub"},
		{"fe", "01", "00", "Device Firmware Upgrade (DFU)"},
	}
	for _, tc := range cases {
		got := interfaceClassName(tc.class, tc.sub, tc.proto)
		if got != tc.want {
			t.Errorf("interfaceClassName(%q,%q,%q) = %q, want %q",
				tc.class, tc.sub, tc.proto, got, tc.want)
		}
	}
}
