package icmp

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ICMPReturn contains the info for a returned ICMP
type ICMPReturn struct {
	Success bool
	Addr    string
	Elapsed time.Duration
}

// SendDiscoverICMP sends a ICMP to a given destination with a TTL to discover hops
func SendDiscoverICMP(localAddr string, dst net.Addr, ttl, pid int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	return SendICMP(localAddr, dst, "", ttl, pid, timeout, seq)
}

// SendICMP sends a ICMP to a given destination which requires a  reply from that specific destination
func SendICMP(localAddr string, dst net.Addr, target string, ttl, pid int, timeout time.Duration, seq int) (hop ICMPReturn, err error) {
	hop.Success = false
	start := time.Now()
	c, err := icmp.ListenPacket("ip4:icmp", localAddr)
	if err != nil {
		log.Panicf("Failed to listen to address %v. Msg: %v.", localAddr, err.Error())
		return hop, err
	}
	defer c.Close()
	err = c.IPv4PacketConn().SetTTL(ttl)
	if err != nil {
		return hop, err
	}
	err = c.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return hop, err
	}

	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, uint32(seq))
	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: pid, Seq: seq,
			Data: append(bs, 'x'),
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		return hop, err
	}

	if _, err := c.WriteTo(wb, dst); err != nil {
		return hop, err
	}

	peer, _, err := listenForSpecific(c, time.Now().Add(timeout), target, append(bs, 'x'), seq, wb)
	if err != nil {
		return hop, err
	}

	elapsed := time.Since(start)
	hop.Elapsed = elapsed
	hop.Addr = peer
	hop.Success = true
	return hop, err
}

// listenForSpecific listens for a reply from a specific destination with a specifi body and returns the body if returned
func listenForSpecific(conn *icmp.PacketConn, deadline time.Time, neededPeer string, neededBody []byte, needSeq int, sent []byte) (string, []byte, error) {
	for {
		b := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(b)
		if err != nil {
			if neterr, ok := err.(*net.OpError); ok {
				return "", []byte{}, neterr
			}
		}
		if n == 0 {
			continue
		}

		if neededPeer != "" && peer.String() != neededPeer {
			continue
		}

		x, err := icmp.ParseMessage(1, b[:n])
		if err != nil {
			continue
		}

		if x.Type.(ipv4.ICMPType).String() == "time exceeded" {
			body := x.Body.(*icmp.TimeExceeded).Data

			index := bytes.Index(body, sent[:4])
			if index > 0 {
				x, _ := icmp.ParseMessage(1, body[index:])
				switch x.Body.(type) {
				case *icmp.Echo:
					seq := x.Body.(*icmp.Echo).Seq
					if seq == needSeq {
						return peer.String(), []byte{}, nil
					}
				default:
					// ignore
				}
			}
		}

		if x.Type.(ipv4.ICMPType).String() == "echo reply" {
			b, _ := x.Body.Marshal(1)
			if string(b[4:]) != string(neededBody) {
				continue
			}
			return peer.String(), b[4:], nil
		}
	}
}
