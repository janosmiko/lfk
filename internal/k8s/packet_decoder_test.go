package k8s

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

func TestDecodePacket_TCP(t *testing.T) {
	// Synthetic TCP packet: Ether/IPv4/TCP, srcPort=51234 dstPort=443 PSH+ACK
	eth := layers.Ethernet{
		SrcMAC:       []byte{1, 2, 3, 4, 5, 6},
		DstMAC:       []byte{6, 5, 4, 3, 2, 1},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		SrcIP:    []byte{10, 0, 0, 4},
		DstIP:    []byte{10, 0, 1, 5},
		Protocol: layers.IPProtocolTCP,
	}
	tcp := layers.TCP{
		SrcPort: 51234,
		DstPort: 443,
		PSH:     true,
		ACK:     true,
	}
	if err := tcp.SetNetworkLayerForChecksum(&ip); err != nil {
		t.Fatalf("set network layer: %v", err)
	}
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	if err := gopacket.SerializeLayers(buf, opts, &eth, &ip, &tcp); err != nil {
		t.Fatalf("serialize: %v", err)
	}

	now := time.Now()
	got := decodePacket(buf.Bytes(), gopacket.CaptureInfo{Timestamp: now, Length: len(buf.Bytes())}, layers.LayerTypeEthernet)

	if got.Protocol != "TCP" {
		t.Errorf("Protocol = %q, want TCP", got.Protocol)
	}
	if got.SrcIP != "10.0.0.4" || got.DstIP != "10.0.1.5" {
		t.Errorf("IPs = %s -> %s, want 10.0.0.4 -> 10.0.1.5", got.SrcIP, got.DstIP)
	}
	if got.SrcPort != "51234" || got.DstPort != "443" {
		t.Errorf("Ports = %s -> %s, want 51234 -> 443", got.SrcPort, got.DstPort)
	}
	if got.Flags != "PSH ACK" {
		t.Errorf("Flags = %q, want PSH ACK", got.Flags)
	}
	if !got.Time.Equal(now) {
		t.Errorf("Time = %v, want %v", got.Time, now)
	}
}

func TestDecodePacket_DNS(t *testing.T) {
	eth := layers.Ethernet{
		SrcMAC: []byte{1, 2, 3, 4, 5, 6}, DstMAC: []byte{6, 5, 4, 3, 2, 1},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := layers.IPv4{
		Version: 4, IHL: 5, TTL: 64,
		SrcIP: []byte{10, 0, 0, 4}, DstIP: []byte{10, 96, 0, 10},
		Protocol: layers.IPProtocolUDP,
	}
	udp := layers.UDP{SrcPort: 33445, DstPort: 53}
	if err := udp.SetNetworkLayerForChecksum(&ip); err != nil {
		t.Fatalf("checksum: %v", err)
	}
	dns := layers.DNS{
		ID:        0xabcd,
		QR:        false,
		Questions: []layers.DNSQuestion{{Name: []byte("kubernetes.default"), Type: layers.DNSTypeA, Class: layers.DNSClassIN}},
	}
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, &eth, &ip, &udp, &dns); err != nil {
		t.Fatalf("serialize: %v", err)
	}

	got := decodePacket(buf.Bytes(), gopacket.CaptureInfo{Timestamp: time.Now(), Length: len(buf.Bytes())}, layers.LayerTypeEthernet)
	if got.Protocol != "DNS" {
		t.Errorf("Protocol = %q, want DNS", got.Protocol)
	}
	if got.Extra != "Q kubernetes.default" {
		t.Errorf("Extra = %q, want Q kubernetes.default", got.Extra)
	}
}

func TestPacketDecoder_Run_CountsPacketsAndBytes(t *testing.T) {
	// Build a 2-packet pcap stream in-memory using pcapgo writer.
	var pcapBuf bytes.Buffer
	w := pcapgo.NewWriter(&pcapBuf)
	if err := w.WriteFileHeader(65535, layers.LinkTypeEthernet); err != nil {
		t.Fatalf("write header: %v", err)
	}
	for i := range 2 {
		eth := layers.Ethernet{
			SrcMAC: []byte{1, 2, 3, 4, 5, 6}, DstMAC: []byte{6, 5, 4, 3, 2, 1},
			EthernetType: layers.EthernetTypeIPv4,
		}
		ip := layers.IPv4{
			Version: 4, IHL: 5, TTL: 64,
			SrcIP: []byte{10, 0, 0, 4}, DstIP: []byte{10, 0, 1, 5},
			Protocol: layers.IPProtocolTCP,
		}
		tcp := layers.TCP{SrcPort: layers.TCPPort(50000 + i), DstPort: 443, ACK: true}
		_ = tcp.SetNetworkLayerForChecksum(&ip)
		buf := gopacket.NewSerializeBuffer()
		if err := gopacket.SerializeLayers(buf,
			gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true},
			&eth, &ip, &tcp); err != nil {
			t.Fatalf("serialize: %v", err)
		}
		if err := w.WritePacket(gopacket.CaptureInfo{
			Timestamp:     time.Now(),
			CaptureLength: len(buf.Bytes()),
			Length:        len(buf.Bytes()),
		}, buf.Bytes()); err != nil {
			t.Fatalf("write packet: %v", err)
		}
	}

	var fileSink bytes.Buffer
	var pkt, byt int64
	var got []PacketSummary
	d := &packetDecoder{
		file:        nopCloser{&fileSink},
		packetCount: &pkt,
		byteCount:   &byt,
		onPacket:    func(s PacketSummary) { got = append(got, s) },
	}
	if err := d.Run(context.Background(), &pcapBuf); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Run: %v", err)
	}

	if n := atomic.LoadInt64(&pkt); n != 2 {
		t.Errorf("packetCount = %d, want 2", n)
	}
	if atomic.LoadInt64(&byt) == 0 {
		t.Errorf("byteCount = 0, want > 0 (TeeReader should have piped to fileSink)")
	}
	if fileSink.Len() == 0 {
		t.Errorf("fileSink empty, want pcap bytes mirrored to file")
	}
	if len(got) != 2 {
		t.Errorf("onPacket called %d times, want 2", len(got))
	}
	if len(got) > 0 && got[0].Protocol != "TCP" {
		t.Errorf("packet[0].Protocol = %q, want TCP", got[0].Protocol)
	}
}

func TestPacketDecoder_LinuxSLL_LinkTypeDispatch(t *testing.T) {
	// Build a pcap stream with LinkTypeLinuxSLL and a single SLL+IPv4+UDP+DNS packet.
	// layers.LinuxSLL has no SerializeTo, so we hand-craft the 16-byte SLL header
	// and concatenate it with a gopacket-serialized IPv4/UDP/DNS payload.
	var pcapBuf bytes.Buffer
	w := pcapgo.NewWriter(&pcapBuf)
	if err := w.WriteFileHeader(65535, layers.LinkTypeLinuxSLL); err != nil {
		t.Fatalf("hdr: %v", err)
	}

	// Serialize IPv4/UDP/DNS payload.
	ip := layers.IPv4{
		Version: 4, IHL: 5, TTL: 64,
		SrcIP: []byte{10, 0, 0, 4}, DstIP: []byte{10, 96, 0, 10},
		Protocol: layers.IPProtocolUDP,
	}
	udp := layers.UDP{SrcPort: 33445, DstPort: 53}
	_ = udp.SetNetworkLayerForChecksum(&ip)
	dns := layers.DNS{
		ID: 1, QR: false,
		Questions: []layers.DNSQuestion{{Name: []byte("svc.cluster.local"), Type: layers.DNSTypeA, Class: layers.DNSClassIN}},
	}
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, &ip, &udp, &dns); err != nil {
		t.Fatalf("serialize ip/udp/dns: %v", err)
	}
	ipPayload := buf.Bytes()

	// Hand-craft the 16-byte Linux SLL header.
	// Format: PacketType(2) AddrType(2) AddrLen(2) Addr(8) EthernetType(2)
	sllHdr := [16]byte{}
	// PacketType = 0 (LinuxSLLPacketTypeHost, "to us")
	// AddrType   = 1 (ARPHRD_ETHER)
	sllHdr[2] = 0x00
	sllHdr[3] = 0x01
	// AddrLen    = 6 (MAC length)
	sllHdr[4] = 0x00
	sllHdr[5] = 0x06
	// Addr: 6 bytes MAC + 2 zero-pad bytes (positions 6..13)
	copy(sllHdr[6:12], []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF})
	// EthernetType = 0x0800 (IPv4)
	sllHdr[14] = 0x08
	sllHdr[15] = 0x00

	pktBytes := append(sllHdr[:], ipPayload...)
	if err := w.WritePacket(gopacket.CaptureInfo{
		Timestamp:     time.Now(),
		CaptureLength: len(pktBytes),
		Length:        len(pktBytes),
	}, pktBytes); err != nil {
		t.Fatalf("write packet: %v", err)
	}

	var fileSink bytes.Buffer
	var pkt, byt int64
	var got []PacketSummary
	d := &packetDecoder{
		file:        nopCloser{&fileSink},
		packetCount: &pkt,
		byteCount:   &byt,
		onPacket:    func(s PacketSummary) { got = append(got, s) },
	}
	if err := d.Run(context.Background(), &pcapBuf); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Run: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d packets, want 1", len(got))
	}
	if got[0].Protocol != "DNS" {
		t.Errorf("Protocol = %q, want DNS — link-type dispatch failed for LinuxSLL", got[0].Protocol)
	}
}

// TestPacketDecoder_LinuxSLL2_DecodesIPv4 guards the SLL2 strip path.
// libpcap >= 1.10 emits link type 276 (DLT_LINUX_SLL2) for `tcpdump -i any`,
// which gopacket v1.1.19 doesn't know about. Without the strip every packet
// surfaces as Protocol="OTHER" with empty addresses (the bug the user
// reported in the live overlay).
func TestPacketDecoder_LinuxSLL2_DecodesIPv4(t *testing.T) {
	// gopacket v1.1.19's layers.LinkType is uint8, so we can't ask
	// pcapgo.NewWriter to emit linktype 276. Hand-craft the 24-byte pcap
	// file header directly with the SLL2 link type in the network field.
	var pcapBuf bytes.Buffer
	pcapBuf.Write([]byte{
		0xd4, 0xc3, 0xb2, 0xa1, // magic (LE microsecond)
		0x02, 0x00, 0x04, 0x00, // version 2.4
		0x00, 0x00, 0x00, 0x00, // thiszone
		0x00, 0x00, 0x00, 0x00, // sigfigs
		0xff, 0xff, 0x00, 0x00, // snaplen 65535
		0x14, 0x01, 0x00, 0x00, // linktype 276 (LE) — DLT_LINUX_SLL2
	})

	// IPv4 + TCP payload (the inner protocol after the SLL2 header).
	ip := layers.IPv4{
		Version: 4, IHL: 5, TTL: 64,
		SrcIP: []byte{10, 0, 0, 4}, DstIP: []byte{10, 0, 1, 5},
		Protocol: layers.IPProtocolTCP,
	}
	tcp := layers.TCP{SrcPort: 51234, DstPort: 443, ACK: true, PSH: true}
	_ = tcp.SetNetworkLayerForChecksum(&ip)
	sbuf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(sbuf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, &ip, &tcp, gopacket.Payload([]byte("hello"))); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	ipPayload := sbuf.Bytes()

	// Hand-craft 20-byte SLL2 header. Layout per stripSLL2 docstring.
	sll2 := [20]byte{}
	// protocol = 0x0800 (IPv4), big-endian
	sll2[0] = 0x08
	sll2[1] = 0x00
	// reserved (2-3): zero
	// if_index (4-7): zero
	// hatype (8-9): 0x0001 ARPHRD_ETHER
	sll2[9] = 0x01
	// pkttype (10): 0x00
	// halen (11): 0x06
	sll2[11] = 0x06
	// addr (12-19): MAC + 2 zero-pad
	copy(sll2[12:18], []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF})

	pktBytes := append(sll2[:], ipPayload...)

	// Hand-craft a 16-byte pcap record header.
	now := time.Now()
	tsSec := uint32(now.Unix())
	tsUsec := uint32(now.Nanosecond() / 1000)
	pcapLen := uint32(len(pktBytes))
	pcapBuf.Write([]byte{
		byte(tsSec), byte(tsSec >> 8), byte(tsSec >> 16), byte(tsSec >> 24),
		byte(tsUsec), byte(tsUsec >> 8), byte(tsUsec >> 16), byte(tsUsec >> 24),
		byte(pcapLen), byte(pcapLen >> 8), byte(pcapLen >> 16), byte(pcapLen >> 24),
		byte(pcapLen), byte(pcapLen >> 8), byte(pcapLen >> 16), byte(pcapLen >> 24),
	})
	pcapBuf.Write(pktBytes)

	var fileSink bytes.Buffer
	var pkt, byt int64
	var got []PacketSummary
	d := &packetDecoder{
		file:        nopCloser{&fileSink},
		packetCount: &pkt,
		byteCount:   &byt,
		onPacket:    func(s PacketSummary) { got = append(got, s) },
	}
	if err := d.Run(context.Background(), &pcapBuf); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Run: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d packets, want 1", len(got))
	}
	if got[0].Protocol != "TCP" {
		t.Errorf("Protocol = %q, want TCP — SLL2 strip path failed", got[0].Protocol)
	}
	if got[0].SrcIP != "10.0.0.4" || got[0].DstIP != "10.0.1.5" {
		t.Errorf("addresses = %s -> %s, want 10.0.0.4 -> 10.0.1.5", got[0].SrcIP, got[0].DstIP)
	}
	if got[0].SrcPort != "51234" || got[0].DstPort != "443" {
		t.Errorf("ports = %s -> %s, want 51234 -> 443", got[0].SrcPort, got[0].DstPort)
	}
}

// TestPacketDecoder_LinuxSLL2_BigEndianMagic guards the endianness branch
// in Run: a pcap captured on a big-endian host has magic 0xa1b2c3d4 and the
// linktype is also big-endian. We must NOT read it as little-endian (the
// pre-fix code did, which would silently produce rawLinkType=0x14010000
// = ~335M, not 276, and SLL2 detection would miss).
func TestPacketDecoder_LinuxSLL2_BigEndianMagic(t *testing.T) {
	var pcapBuf bytes.Buffer
	pcapBuf.Write([]byte{
		0xa1, 0xb2, 0xc3, 0xd4, // BE magic
		0x00, 0x02, 0x00, 0x04, // version 2.4 (BE)
		0x00, 0x00, 0x00, 0x00, // thiszone
		0x00, 0x00, 0x00, 0x00, // sigfigs
		0x00, 0x00, 0xff, 0xff, // snaplen 65535 (BE)
		0x00, 0x00, 0x01, 0x14, // linktype 276 SLL2 (BE)
	})

	ip := layers.IPv4{
		Version: 4, IHL: 5, TTL: 64,
		SrcIP: []byte{10, 0, 0, 4}, DstIP: []byte{10, 0, 1, 5},
		Protocol: layers.IPProtocolTCP,
	}
	tcp := layers.TCP{SrcPort: 12345, DstPort: 443}
	_ = tcp.SetNetworkLayerForChecksum(&ip)
	sbuf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(sbuf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, &ip, &tcp); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	ipPayload := sbuf.Bytes()
	sll2 := [20]byte{0x08, 0x00, 0, 0, 0, 0, 0, 0, 0, 0x01, 0, 6}
	pktBytes := append(sll2[:], ipPayload...)

	now := time.Now()
	tsSec := uint32(now.Unix())
	tsUsec := uint32(now.Nanosecond() / 1000)
	pcapLen := uint32(len(pktBytes))
	// BE record header.
	pcapBuf.Write([]byte{
		byte(tsSec >> 24), byte(tsSec >> 16), byte(tsSec >> 8), byte(tsSec),
		byte(tsUsec >> 24), byte(tsUsec >> 16), byte(tsUsec >> 8), byte(tsUsec),
		byte(pcapLen >> 24), byte(pcapLen >> 16), byte(pcapLen >> 8), byte(pcapLen),
		byte(pcapLen >> 24), byte(pcapLen >> 16), byte(pcapLen >> 8), byte(pcapLen),
	})
	pcapBuf.Write(pktBytes)

	var fileSink bytes.Buffer
	var pkt, byt int64
	var got []PacketSummary
	d := &packetDecoder{
		file:        nopCloser{&fileSink},
		packetCount: &pkt,
		byteCount:   &byt,
		onPacket:    func(s PacketSummary) { got = append(got, s) },
	}
	if err := d.Run(context.Background(), &pcapBuf); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d packets, want 1 (BE pcap parsed?)", len(got))
	}
	if got[0].Protocol != "TCP" {
		t.Errorf("Protocol = %q, want TCP — endianness branch missed SLL2 in BE pcap", got[0].Protocol)
	}
}

// TestPacketDecoder_Run_NilCountersDoNotPanic guards the symmetry with
// countingWriter.Write — both packetCount and byteCount may be nil on a
// decoder used in non-counting contexts, and the Run loop must not nil-
// dereference. Without the guard at the AddInt64 call site, a single
// packet pcap stream would crash here.
func TestPacketDecoder_Run_NilCountersDoNotPanic(t *testing.T) {
	var pcapBuf bytes.Buffer
	w := pcapgo.NewWriter(&pcapBuf)
	if err := w.WriteFileHeader(65535, layers.LinkTypeEthernet); err != nil {
		t.Fatalf("hdr: %v", err)
	}
	eth := layers.Ethernet{
		SrcMAC: []byte{1, 2, 3, 4, 5, 6}, DstMAC: []byte{7, 8, 9, 10, 11, 12},
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: []byte{1, 1, 1, 1}, DstIP: []byte{2, 2, 2, 2}, Protocol: layers.IPProtocolTCP}
	tcp := layers.TCP{SrcPort: 1, DstPort: 2}
	_ = tcp.SetNetworkLayerForChecksum(&ip)
	sbuf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(sbuf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, &eth, &ip, &tcp); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	if err := w.WritePacket(gopacket.CaptureInfo{Timestamp: time.Now(), CaptureLength: len(sbuf.Bytes()), Length: len(sbuf.Bytes())}, sbuf.Bytes()); err != nil {
		t.Fatalf("write packet: %v", err)
	}

	var fileSink bytes.Buffer
	d := &packetDecoder{
		file:        nopCloser{&fileSink},
		packetCount: nil, // both counters nil — Run must not panic
		byteCount:   nil,
		onPacket:    func(PacketSummary) {},
	}
	if err := d.Run(context.Background(), &pcapBuf); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Run: %v", err)
	}
}

func TestStripSLL2_TooShort(t *testing.T) {
	short := []byte{0x08, 0x00, 0x00}
	out, lt := stripSLL2(short)
	if len(out) != len(short) {
		t.Errorf("too-short input must be returned unchanged so we can still log it; got %d bytes, want %d", len(out), len(short))
	}
	if lt != layers.LayerTypeEthernet {
		t.Errorf("too-short SLL2 should fall back to Ethernet entry layer; got %v", lt)
	}
}

func TestStripSLL2_EthertypeDispatch(t *testing.T) {
	tests := []struct {
		name      string
		ethertype uint16
		wantLayer gopacket.LayerType
	}{
		{"ipv4", 0x0800, layers.LayerTypeIPv4},
		{"ipv6", 0x86dd, layers.LayerTypeIPv6},
		{"arp", 0x0806, layers.LayerTypeARP},
		{"unknown", 0x9999, layers.LayerTypeEthernet},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, 20+8)
			data[0] = byte(tt.ethertype >> 8)
			data[1] = byte(tt.ethertype)
			payload, lt := stripSLL2(data)
			if len(payload) != 8 {
				t.Errorf("payload len = %d, want 8 (input - 20)", len(payload))
			}
			if lt != tt.wantLayer {
				t.Errorf("layer = %v, want %v", lt, tt.wantLayer)
			}
		})
	}
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
