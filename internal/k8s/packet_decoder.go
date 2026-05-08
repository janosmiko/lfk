package k8s

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// PacketSummary is a presentation-friendly decoded packet summary.
type PacketSummary struct {
	Time     time.Time
	Protocol string // "TCP", "UDP", "DNS", "ICMP", "ICMPv6", "OTHER"
	SrcIP    string
	SrcPort  string
	DstIP    string
	DstPort  string
	Length   int
	Flags    string // for TCP: "PSH ACK"
	Extra    string // for DNS: "Q kubernetes.default"; for ICMP: "echo request"
}

func decodePacket(data []byte, ci gopacket.CaptureInfo, linkLayer gopacket.LayerType) PacketSummary {
	p := gopacket.NewPacket(data, linkLayer, gopacket.NoCopy)
	s := PacketSummary{Time: ci.Timestamp, Length: ci.Length}

	switch {
	case p.Layer(layers.LayerTypeIPv4) != nil:
		ip := p.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
		s.SrcIP, s.DstIP = ip.SrcIP.String(), ip.DstIP.String()
	case p.Layer(layers.LayerTypeIPv6) != nil:
		ip := p.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
		s.SrcIP, s.DstIP = ip.SrcIP.String(), ip.DstIP.String()
	}

	switch {
	case p.Layer(layers.LayerTypeTCP) != nil:
		t := p.Layer(layers.LayerTypeTCP).(*layers.TCP)
		s.Protocol = "TCP"
		s.SrcPort = strconv.Itoa(int(t.SrcPort))
		s.DstPort = strconv.Itoa(int(t.DstPort))
		s.Flags = tcpFlagsString(t)
	case p.Layer(layers.LayerTypeUDP) != nil:
		u := p.Layer(layers.LayerTypeUDP).(*layers.UDP)
		s.Protocol = "UDP"
		s.SrcPort = strconv.Itoa(int(u.SrcPort))
		s.DstPort = strconv.Itoa(int(u.DstPort))
		if u.SrcPort == 53 || u.DstPort == 53 {
			if dns := p.Layer(layers.LayerTypeDNS); dns != nil {
				s.Protocol = "DNS"
				s.Extra = dnsSummary(dns.(*layers.DNS))
			}
		}
	case p.Layer(layers.LayerTypeICMPv4) != nil:
		ic := p.Layer(layers.LayerTypeICMPv4).(*layers.ICMPv4)
		s.Protocol = "ICMP"
		s.Extra = ic.TypeCode.String()
	case p.Layer(layers.LayerTypeICMPv6) != nil:
		s.Protocol = "ICMPv6"
	default:
		s.Protocol = "OTHER"
	}
	return s
}

func tcpFlagsString(t *layers.TCP) string {
	var parts []string
	if t.SYN {
		parts = append(parts, "SYN")
	}
	if t.PSH {
		parts = append(parts, "PSH")
	}
	if t.ACK {
		parts = append(parts, "ACK")
	}
	if t.FIN {
		parts = append(parts, "FIN")
	}
	if t.RST {
		parts = append(parts, "RST")
	}
	if t.URG {
		parts = append(parts, "URG")
	}
	return strings.Join(parts, " ")
}

func dnsSummary(d *layers.DNS) string {
	if d.QR { // response
		if len(d.Answers) > 0 && len(d.Questions) > 0 {
			return "A " + string(d.Questions[0].Name)
		}
		if len(d.Questions) > 0 {
			return "R " + string(d.Questions[0].Name)
		}
		return ""
	}
	if len(d.Questions) > 0 {
		return "Q " + string(d.Questions[0].Name)
	}
	return ""
}

// packetDecoder reads a pcap stream from a backend process, tees raw bytes
// into an on-disk file for export, and decodes each packet for the live UI table.
type packetDecoder struct {
	file        io.WriteCloser
	packetCount *int64 // atomic counter shared with CaptureEntry
	byteCount   *int64 // atomic counter shared with CaptureEntry
	onPacket    func(PacketSummary)
}

// countingWriter wraps an io.Writer and atomically accumulates written byte totals.
type countingWriter struct {
	w io.Writer
	n *int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if c.n != nil {
		atomic.AddInt64(c.n, int64(n))
	}
	return n, err
}

// Run reads packets from src until EOF, context cancellation, or a read error.
// Raw pcap bytes are teed to d.file via io.TeeReader; each decoded packet is
// forwarded to d.onPacket. Atomic counters are updated per packet/byte.
func (d *packetDecoder) Run(ctx context.Context, src io.Reader) error {
	counting := &countingWriter{w: d.file, n: d.byteCount}
	teed := io.TeeReader(src, counting)

	// Read the 24-byte pcap file header ourselves so we get the full 32-bit
	// linktype value. gopacket v1.1.19's layers.LinkType is `uint8`, so a
	// linktype of 276 (DLT_LINUX_SLL2 — what `tcpdump -i any` writes on
	// libpcap >= 1.10) truncates to 20 by the time pcapgo.Reader.LinkType()
	// returns it. Parsing here lets stripSLL2 actually fire.
	//
	// pcap is endian-tagged: a magic of 0xa1b2c3d4 / 0xa1b23c4d means the
	// rest of the header is big-endian; 0xd4c3b2a1 / 0x4d3cb2a1 means
	// little-endian. tcpdump on Linux/macOS produces little-endian by
	// default, but a pcap captured on a big-endian host (some MIPS / older
	// SPARC) and shipped to lfk would silently parse the linktype wrong
	// without this branch.
	var fileHdr [24]byte
	if _, err := io.ReadFull(teed, fileHdr[:]); err != nil {
		return fmt.Errorf("opening pcap stream: %w", err)
	}
	magicBE := binary.BigEndian.Uint32(fileHdr[0:4])
	var rawLinkType uint32
	if magicBE == 0xa1b2c3d4 || magicBE == 0xa1b23c4d {
		rawLinkType = binary.BigEndian.Uint32(fileHdr[20:24])
	} else {
		rawLinkType = binary.LittleEndian.Uint32(fileHdr[20:24])
	}
	isSLL2 := rawLinkType == linkTypeLinuxSLL2

	// Prepend the header back so pcapgo's NewReader can read it normally.
	// MultiReader's first source isn't teed — that's fine, the header bytes
	// have already been written to disk and counted via the initial
	// ReadFull above.
	rd, err := pcapgo.NewReader(io.MultiReader(bytes.NewReader(fileHdr[:]), teed))
	if err != nil {
		return fmt.Errorf("opening pcap stream: %w", err)
	}
	decodeAs := layerTypeForLinkType(rd.LinkType())

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		data, ci, err := rd.ReadPacketData()
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading packet: %w", err)
		}
		// nil-guard mirrors countingWriter.Write so callers can pass a
		// decoder without a counter without crashing the read loop.
		if d.packetCount != nil {
			atomic.AddInt64(d.packetCount, 1)
		}
		// SLL2 (linktype 276) is what tcpdump uses for `-i any` on libpcap
		// >= 1.10. gopacket v1.1.19 has no SLL2 decoder, so without this
		// strip every SLL2 packet decodes as "OTHER" with empty addresses.
		// stripSLL2 returns the inner-protocol payload + the gopacket entry
		// layer (IPv4 / IPv6 / etc.) so decodePacket can decode normally.
		entry := decodeAs
		payload := data
		if isSLL2 {
			payload, entry = stripSLL2(data)
		}
		d.onPacket(decodePacket(payload, ci, entry))
	}
}

// linkTypeLinuxSLL2 (DLT_LINUX_SLL2 = 276) is libpcap's link type for
// "Linux cooked v2", produced by tcpdump for `-i any` on libpcap >= 1.10.
// Defined here as a numeric constant because gopacket v1.1.19 has no
// LinkTypeLinuxSLL2 enum value.
const linkTypeLinuxSLL2 = 276

// stripSLL2 strips the 20-byte SLL2 header and returns (inner payload,
// gopacket entry layer for that payload).
//
// SLL2 header layout (20 bytes):
//
//	00-01  protocol (ethertype, big-endian)
//	02-03  reserved (mbz)
//	04-07  if_index
//	08-09  hatype (ARPHRD_*)
//	10     pkttype (PACKET_HOST etc.)
//	11     halen (link-layer address length)
//	12-19  addr (8 bytes; padded to 8 regardless of halen)
//
// Returns the original buffer and Ethernet on a too-short buffer or unknown
// ethertype so we still surface *something* in the live table.
func stripSLL2(data []byte) ([]byte, gopacket.LayerType) {
	const sll2HeaderLen = 20
	if len(data) < sll2HeaderLen {
		return data, layers.LayerTypeEthernet
	}
	ethertype := binary.BigEndian.Uint16(data[:2])
	payload := data[sll2HeaderLen:]
	switch ethertype {
	case 0x0800:
		return payload, layers.LayerTypeIPv4
	case 0x86dd:
		return payload, layers.LayerTypeIPv6
	case 0x0806:
		return payload, layers.LayerTypeARP
	default:
		return payload, layers.LayerTypeEthernet
	}
}

// layerTypeForLinkType maps a pcap link-type to the gopacket layer type used
// as the decoding entry point. gopacket v1.1.19 does not include LinuxSLL2
// (DLT 276); SLL2 is handled by stripSLL2 in the decoder loop, so other
// unknown link types fall back to Ethernet as best-effort.
func layerTypeForLinkType(lt layers.LinkType) gopacket.LayerType {
	switch lt {
	case layers.LinkTypeEthernet:
		return layers.LayerTypeEthernet
	case layers.LinkTypeLinuxSLL:
		return layers.LayerTypeLinuxSLL
	default:
		return layers.LayerTypeEthernet // best-effort for unknown link types
	}
}
