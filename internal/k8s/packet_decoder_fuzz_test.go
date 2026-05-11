package k8s

import (
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// FuzzDecodePacket pushes arbitrary byte slices through decodePacket /
// gopacket for each plausible link-layer entry point. gopacket parses
// dozens of protocol headers; this fuzzer is panic discovery on attacker-
// controlled bytes (the live-capture stream comes from `kubectl debug` and
// reaches us un-validated).
func FuzzDecodePacket(f *testing.F) {
	// Minimal seed: an empty Ethernet frame and a couple of plausible
	// shapes. The fuzzer will grow these.
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x00})
	f.Add([]byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x86\xdd"))
	f.Add([]byte{0x45, 0x00, 0x00, 0x14, 0x00, 0x00, 0x00, 0x00, 0x40, 0x06})

	ci := gopacket.CaptureInfo{Timestamp: time.Unix(0, 0)}
	entries := []gopacket.LayerType{
		layers.LayerTypeEthernet,
		layers.LayerTypeIPv4,
		layers.LayerTypeIPv6,
		layers.LayerTypeLinuxSLL,
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		ci.Length = len(data)
		for _, entry := range entries {
			_ = decodePacket(data, ci, entry)
		}
	})
}

// FuzzStripSLL2 drives the 20-byte SLL2 header stripper. The function
// reads big-endian uint16s and slices off the header; arbitrary input
// must not panic.
func FuzzStripSLL2(f *testing.F) {
	f.Add([]byte{})
	f.Add(make([]byte, 19)) // one byte short of the 20-byte header
	f.Add(make([]byte, 20)) // exactly header-sized, zero ethertype
	f.Add([]byte{0x08, 0x00, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x45})

	f.Fuzz(func(t *testing.T, data []byte) {
		payload, layer := stripSLL2(data)
		// Payload must always be a suffix (or the original) of the input —
		// never longer.
		if len(payload) > len(data) {
			t.Fatalf("stripSLL2: payload len %d exceeds input len %d", len(payload), len(data))
		}
		_ = layer
	})
}
