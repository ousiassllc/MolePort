package core

import (
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
)

func TestSocks5Negotiate(t *testing.T) {
	tests := []struct {
		name       string
		clientData []byte
		wantResp   []byte
		wantErr    string
	}{
		{
			name:       "success with single NoAuth method",
			clientData: []byte{socks5Version, 0x01, socks5NoAuth},
			wantResp:   []byte{socks5Version, socks5NoAuth},
		},
		{
			name:       "success with multiple methods including NoAuth",
			clientData: []byte{socks5Version, 0x03, 0x01, 0x02, socks5NoAuth},
			wantResp:   []byte{socks5Version, socks5NoAuth},
		},
		{
			name:       "no acceptable methods",
			clientData: []byte{socks5Version, 0x01, 0x01},
			wantResp:   []byte{socks5Version, socks5NoAcceptable},
			wantErr:    "no acceptable auth methods",
		},
		{
			name:       "wrong SOCKS version",
			clientData: []byte{0x04, 0x01, socks5NoAuth},
			wantErr:    "unsupported SOCKS version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer clientConn.Close()
			defer serverConn.Close()

			errCh := make(chan error, 1)
			go func() {
				errCh <- socks5Negotiate(serverConn)
			}()

			// net.Pipe はバッファなしのため、サーバーが全バイトを読まない
			// エラーケースでは書き込みがブロックされる可能性がある。
			// 書き込みをゴルーチンで行い、デッドロックを防ぐ。
			go func() {
				clientConn.Write(tt.clientData)
			}()

			if tt.wantResp != nil {
				resp := make([]byte, len(tt.wantResp))
				if _, err := io.ReadFull(clientConn, resp); err != nil {
					t.Fatalf("read response: %v", err)
				}
				for i, b := range tt.wantResp {
					if resp[i] != b {
						t.Errorf("resp[%d] = 0x%02X, want 0x%02X", i, resp[i], b)
					}
				}
			}

			err := <-errCh
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestSocks5ParseRequest(t *testing.T) {
	// ポートをビッグエンディアンでエンコードするヘルパー
	portBytes := func(port uint16) []byte {
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, port)
		return buf
	}

	tests := []struct {
		name       string
		clientData []byte
		wantAddr   string
		wantErr    string
	}{
		{
			name: "IPv4 CONNECT",
			clientData: func() []byte {
				req := []byte{socks5Version, socks5CmdConnect, 0x00, socks5AddrIPv4}
				req = append(req, 127, 0, 0, 1)
				req = append(req, portBytes(8080)...)
				return req
			}(),
			wantAddr: "127.0.0.1:8080",
		},
		{
			name: "domain CONNECT",
			clientData: func() []byte {
				domain := "example.com"
				req := []byte{socks5Version, socks5CmdConnect, 0x00, socks5AddrDomain, byte(len(domain))}
				req = append(req, []byte(domain)...)
				req = append(req, portBytes(443)...)
				return req
			}(),
			wantAddr: "example.com:443",
		},
		{
			name: "IPv6 CONNECT",
			clientData: func() []byte {
				req := []byte{socks5Version, socks5CmdConnect, 0x00, socks5AddrIPv6}
				req = append(req, net.ParseIP("::1").To16()...)
				req = append(req, portBytes(80)...)
				return req
			}(),
			wantAddr: "[::1]:80",
		},
		{
			name:       "unsupported command BIND",
			clientData: []byte{socks5Version, 0x02, 0x00, socks5AddrIPv4, 127, 0, 0, 1, 0x00, 0x50},
			wantErr:    "unsupported SOCKS5 command",
		},
		{
			name:       "unsupported address type",
			clientData: []byte{socks5Version, socks5CmdConnect, 0x00, 0x06, 0, 0, 0, 0, 0, 0},
			wantErr:    "unsupported address type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientConn, serverConn := net.Pipe()
			defer clientConn.Close()
			defer serverConn.Close()

			type result struct {
				addr string
				err  error
			}
			resCh := make(chan result, 1)
			go func() {
				addr, err := socks5ParseRequest(serverConn)
				resCh <- result{addr, err}
			}()

			// net.Pipe はバッファなしのため、エラーケースでは以下のデッドロックが起きる:
			//   - サーバーはヘッダー(4バイト)だけ読み、エラー応答(10バイト)を書こうとする
			//   - クライアントはデータ(10バイト)を書こうとするが、サーバーが残りを読まないためブロック
			//   - サーバーもクライアントが応答を読まないためブロック
			// 書き込みと応答読み取りを別々のゴルーチンで行い、デッドロックを防ぐ。
			go func() {
				clientConn.Write(tt.clientData)
			}()
			// エラーケースではサーバーが応答を書くため、それを読んでブロックを解除する
			if tt.wantErr != "" {
				go func() {
					io.Copy(io.Discard, clientConn)
				}()
			}

			res := <-resCh
			if tt.wantErr == "" {
				if res.err != nil {
					t.Errorf("unexpected error: %v", res.err)
				}
				if res.addr != tt.wantAddr {
					t.Errorf("addr = %q, want %q", res.addr, tt.wantAddr)
				}
			} else {
				if res.err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(res.err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want containing %q", res.err.Error(), tt.wantErr)
				}
			}
		})
	}
}
