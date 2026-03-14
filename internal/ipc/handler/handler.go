package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc"
	cfghandler "github.com/ousiassllc/moleport/internal/ipc/handler/config"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

// DaemonInfo はデーモンの状態情報とシャットダウンを提供するインターフェース。
type DaemonInfo interface {
	Status() protocol.DaemonStatusResult
	Shutdown(purge bool) error
}

// NotificationSender はクライアントに通知を送信するインターフェース。
type NotificationSender interface {
	SendNotification(clientID string, notification protocol.Notification) error
}

// VersionChecker はバージョンチェック機能を提供するインターフェース。
type VersionChecker interface {
	LatestVersion(ctx context.Context) (*core.VersionCheckResult, error)
}

// Handler は JSON-RPC メソッドをコアマネージャーにルーティングする。
type Handler struct {
	sshMgr         core.SSHManager
	fwdMgr         core.ForwardManager
	cfgMgr         core.ConfigManager
	configH        *cfghandler.Handler
	broker         *ipc.EventBroker
	daemon         DaemonInfo
	sender         NotificationSender
	versionChecker VersionChecker

	credMu      sync.Mutex
	credPending map[string]chan protocol.CredentialResponseParams
	credNextID  atomic.Int64
}

// NewHandler は新しい Handler を生成する。
func NewHandler(
	sshMgr core.SSHManager,
	fwdMgr core.ForwardManager,
	cfgMgr core.ConfigManager,
	broker *ipc.EventBroker,
	daemon DaemonInfo,
	versionChecker VersionChecker,
) *Handler {
	return &Handler{
		sshMgr:         sshMgr,
		fwdMgr:         fwdMgr,
		cfgMgr:         cfgMgr,
		configH:        cfghandler.New(cfgMgr),
		broker:         broker,
		daemon:         daemon,
		versionChecker: versionChecker,
		credPending:    make(map[string]chan protocol.CredentialResponseParams),
	}
}

// SetSender は通知送信用のサーバー参照を設定する。
// IPCServer の生成後に呼び出す。
func (h *Handler) SetSender(sender NotificationSender) {
	h.sender = sender
}

// Handle は JSON-RPC メソッドをディスパッチする。HandlerFunc として使用する。
func (h *Handler) Handle(clientID string, method string, params json.RawMessage) (any, *protocol.RPCError) {
	switch method {
	case "host.list":
		return h.hostList()
	case "host.reload":
		return h.hostReload()
	case "ssh.connect":
		return h.sshConnect(clientID, params)
	case "ssh.disconnect":
		return h.sshDisconnect(params)
	case protocol.MethodCredentialResponse:
		return h.credentialResponse(params)
	case "forward.list":
		return h.forwardList(params)
	case "forward.add":
		return h.forwardAdd(params)
	case "forward.delete":
		return h.forwardDelete(params)
	case "forward.start":
		return h.forwardStart(clientID, params)
	case "forward.stop":
		return h.forwardStop(params)
	case "forward.stopAll":
		return h.forwardStopAll()
	case "session.list":
		return h.sessionList()
	case "session.get":
		return h.sessionGet(params)
	case "config.get":
		return h.configH.Get()
	case "config.update":
		return h.configH.Update(params)
	case "version.check":
		return h.versionCheck()
	case "daemon.status":
		return h.daemonStatus()
	case "daemon.shutdown":
		return h.daemonShutdown(params)
	case protocol.MethodEventsSubscribe:
		return h.eventsSubscribe(clientID, params)
	case protocol.MethodEventsUnsubscribe:
		return h.eventsUnsubscribe(params)
	default:
		return nil, &protocol.RPCError{Code: protocol.MethodNotFound, Message: "method not found: " + method}
	}
}

// parseParams は JSON-RPC パラメータをアンマーシャルする。
func parseParams(params json.RawMessage, target any) *protocol.RPCError {
	if len(params) == 0 {
		return &protocol.RPCError{Code: protocol.InvalidParams, Message: "params required"}
	}
	if err := json.Unmarshal(params, target); err != nil {
		return &protocol.RPCError{Code: protocol.InvalidParams, Message: "invalid params: " + err.Error()}
	}
	return nil
}

// credentialTimeout はクレデンシャル応答のタイムアウト。
const credentialTimeout = 30 * time.Second

type requiredField struct {
	name  string
	value string
}

func validateRequired(fields ...requiredField) *protocol.RPCError {
	for _, f := range fields {
		if f.value == "" {
			return &protocol.RPCError{
				Code:    protocol.InvalidParams,
				Message: fmt.Sprintf("%s is required", f.name),
			}
		}
	}
	return nil
}
