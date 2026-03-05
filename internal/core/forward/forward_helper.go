package forward

import (
	"context"
	"fmt"
	"net"

	"github.com/ousiassllc/moleport/internal/core"
)

// openListener はルールの種類に応じてフォワーディング用リスナーを作成する。
func openListener(
	ctx context.Context, sshConn core.SSHConnection, rule core.ForwardRule,
) (net.Listener, error) {
	switch rule.Type {
	case core.Local:
		remoteAddr := fmt.Sprintf("%s:%d", rule.RemoteHost, rule.RemotePort)
		return sshConn.LocalForward(ctx, rule.LocalPort, remoteAddr)
	case core.Remote:
		localAddr := fmt.Sprintf("127.0.0.1:%d", rule.LocalPort)
		return sshConn.RemoteForward(ctx, rule.RemotePort, localAddr)
	case core.Dynamic:
		return sshConn.DynamicForward(ctx, rule.LocalPort)
	default:
		return nil, fmt.Errorf("unsupported forward type: %v", rule.Type)
	}
}
