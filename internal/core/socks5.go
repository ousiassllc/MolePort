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
	socks5Version      = 0x05
	socks5NoAuth       = 0x00
	socks5NoAcceptable = 0xFF
	socks5CmdConnect   = 0x01
	socks5AddrIPv4     = 0x01
	socks5AddrDomain   = 0x03
	socks5AddrIPv6     = 0x04

	// SOCKS5 reply codes
	socks5ReplySuccess              = 0x00
	socks5ReplyCommandNotSupported  = 0x07
	socks5ReplyAddrTypeNotSupported = 0x08
	socks5ReplyConnectionRefused    = 0x05
)

// socks5Negotiate は SOCKS5 の挨拶・認証ネゴシエーションを処理する。
func socks5Negotiate(conn net.Conn) error {
	// Client greeting: VER + NMETHODS (2 bytes)
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return err
	}

	if header[0] != socks5Version {
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
		if method == socks5NoAuth {
			noAuthSupported = true
			break
		}
	}
	if !noAuthSupported {
		// No acceptable methods
		_, _ = conn.Write([]byte{socks5Version, socks5NoAcceptable})
		return fmt.Errorf("no acceptable auth methods")
	}

	// No authentication
	if _, err := conn.Write([]byte{socks5Version, socks5NoAuth}); err != nil {
		return err
	}
	return nil
}

// socks5ParseRequest は SOCKS5 の CONNECT リクエストを解析し、接続先アドレスを返す。
func socks5ParseRequest(conn net.Conn) (string, error) {
	// SOCKS5 request: VER, CMD, RSV, ATYP (4 bytes)
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, reqHeader); err != nil {
		return "", err
	}

	if reqHeader[0] != socks5Version || reqHeader[1] != socks5CmdConnect {
		// Command not supported
		_, _ = conn.Write([]byte{socks5Version, socks5ReplyCommandNotSupported, 0x00, socks5AddrIPv4, 0, 0, 0, 0, 0, 0})
		return "", fmt.Errorf("unsupported SOCKS5 command: %d", reqHeader[1]) //nolint:gosec // io.ReadFull guarantees len(reqHeader)==4
	}

	switch reqHeader[3] { // Address type
	case socks5AddrIPv4:
		addrPort := make([]byte, 4+2)
		if _, err := io.ReadFull(conn, addrPort); err != nil {
			return "", err
		}
		ip := net.IP(addrPort[:4])
		port := binary.BigEndian.Uint16(addrPort[4:6])
		return net.JoinHostPort(ip.String(), strconv.Itoa(int(port))), nil
	case socks5AddrDomain:
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
	case socks5AddrIPv6:
		addrPort := make([]byte, 16+2)
		if _, err := io.ReadFull(conn, addrPort); err != nil {
			return "", err
		}
		ip := net.IP(addrPort[:16])
		port := binary.BigEndian.Uint16(addrPort[16:18])
		return net.JoinHostPort(ip.String(), strconv.Itoa(int(port))), nil
	default:
		// Address type not supported
		_, _ = conn.Write([]byte{socks5Version, socks5ReplyAddrTypeNotSupported, 0x00, socks5AddrIPv4, 0, 0, 0, 0, 0, 0})
		return "", fmt.Errorf("unsupported address type: %d", reqHeader[3]) //nolint:gosec // io.ReadFull guarantees len(reqHeader)==4
	}
}
