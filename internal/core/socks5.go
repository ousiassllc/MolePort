package core

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
)

// SOCKS5 プロトコル定数
const (
	Socks5Version      = 0x05
	Socks5AuthNone     = 0x00
	Socks5NoAcceptable = 0xFF
	Socks5CmdConnect   = 0x01
	Socks5AddrIPv4     = 0x01
	Socks5AddrDomain   = 0x03
	Socks5AddrIPv6     = 0x04

	// SOCKS5 reply codes
	Socks5ReplySuccess              = 0x00
	Socks5ReplyCommandNotSupported  = 0x07
	Socks5ReplyAddrTypeNotSupported = 0x08
	Socks5ReplyConnectionRefused    = 0x05
)

// Socks5Negotiate は SOCKS5 の挨拶・認証ネゴシエーションを処理する。
func Socks5Negotiate(conn net.Conn) error {
	// Client greeting: VER + NMETHODS (2 bytes)
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}

	if header[0] != Socks5Version {
		return fmt.Errorf("unsupported SOCKS version: %d", header[0]) //nolint:gosec // io.ReadFull guarantees len(header)==2
	}

	nmethods := int(header[1]) //nolint:gosec // io.ReadFull guarantees len(header)==2
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}

	// クライアントが No Authentication をサポートしているか確認
	noAuthSupported := false
	for _, method := range methods {
		if method == Socks5AuthNone {
			noAuthSupported = true
			break
		}
	}
	if !noAuthSupported {
		// No acceptable methods
		_, _ = conn.Write([]byte{Socks5Version, Socks5NoAcceptable})
		return fmt.Errorf("no acceptable auth methods")
	}

	// No authentication
	if _, err := conn.Write([]byte{Socks5Version, Socks5AuthNone}); err != nil {
		return err
	}
	return nil
}

// Socks5ParseRequest は SOCKS5 の CONNECT リクエストを解析し、接続先アドレスを返す。
func Socks5ParseRequest(conn net.Conn) (string, error) {
	// SOCKS5 request: VER, CMD, RSV, ATYP (4 bytes)
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, reqHeader); err != nil {
		return "", err
	}

	if reqHeader[0] != Socks5Version || reqHeader[1] != Socks5CmdConnect { //nolint:gosec // io.ReadFull guarantees len==4
		// Command not supported
		_, _ = conn.Write([]byte{Socks5Version, Socks5ReplyCommandNotSupported, 0x00, Socks5AddrIPv4, 0, 0, 0, 0, 0, 0})
		return "", fmt.Errorf("unsupported SOCKS5 command: %d", reqHeader[1]) //nolint:gosec // io.ReadFull guarantees len(reqHeader)==4
	}

	switch reqHeader[3] { // Address type
	case Socks5AddrIPv4:
		addrPort := make([]byte, 4+2)
		if _, err := io.ReadFull(conn, addrPort); err != nil {
			return "", err
		}
		ip := net.IP(addrPort[:4])
		port := binary.BigEndian.Uint16(addrPort[4:6])
		return net.JoinHostPort(ip.String(), strconv.Itoa(int(port))), nil
	case Socks5AddrDomain:
		domainLenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, domainLenBuf); err != nil {
			return "", err
		}
		domainLen := int(domainLenBuf[0])
		domainPort := make([]byte, domainLen+2)
		if _, err := io.ReadFull(conn, domainPort); err != nil {
			return "", err
		}
		domain := string(domainPort[:domainLen])
		port := binary.BigEndian.Uint16(domainPort[domainLen : domainLen+2])
		return net.JoinHostPort(domain, strconv.Itoa(int(port))), nil
	case Socks5AddrIPv6:
		addrPort := make([]byte, 16+2)
		if _, err := io.ReadFull(conn, addrPort); err != nil {
			return "", err
		}
		ip := net.IP(addrPort[:16])
		port := binary.BigEndian.Uint16(addrPort[16:18])
		return net.JoinHostPort(ip.String(), strconv.Itoa(int(port))), nil
	default:
		// Address type not supported
		_, _ = conn.Write([]byte{Socks5Version, Socks5ReplyAddrTypeNotSupported, 0x00, Socks5AddrIPv4, 0, 0, 0, 0, 0, 0})
		return "", fmt.Errorf("unsupported address type: %d", reqHeader[3]) //nolint:gosec // io.ReadFull guarantees len(reqHeader)==4
	}
}
