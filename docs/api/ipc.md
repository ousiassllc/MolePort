# MolePort IPC API 仕様

## 概要

デーモンとクライアント（CLI/TUI）間の通信プロトコルを定義する。
Unix ドメインソケット上で JSON-RPC 2.0 を使用する。

## 接続情報

| 項目 | 値 |
|------|-----|
| **ソケットパス** | `~/.config/moleport/moleport.sock` |
| **プロトコル** | JSON-RPC 2.0 |
| **メッセージ区切り** | 改行（`\n`）— NDJSON 形式 |
| **エンコーディング** | UTF-8 |

## 通信パターン

### 同期リクエスト/レスポンス

CLI が使用する基本パターン。1回のリクエストに対して1回のレスポンスを返す。

```
クライアント → デーモン:  {"jsonrpc":"2.0","id":1,"method":"...","params":{...}}
デーモン → クライアント:  {"jsonrpc":"2.0","id":1,"result":{...}}
```

### イベントサブスクリプション

TUI が使用するパターン。`events.subscribe` 後、デーモンから通知が非同期に送信される。

```
クライアント → デーモン:  {"jsonrpc":"2.0","id":1,"method":"events.subscribe","params":{...}}
デーモン → クライアント:  {"jsonrpc":"2.0","id":1,"result":{"subscription_id":"sub-1"}}
デーモン → クライアント:  {"jsonrpc":"2.0","method":"event.ssh","params":{...}}      ← 通知（id なし）
デーモン → クライアント:  {"jsonrpc":"2.0","method":"event.forward","params":{...}}  ← 通知（id なし）
デーモン → クライアント:  {"jsonrpc":"2.0","method":"event.metrics","params":{...}}  ← 通知（id なし）
```

## API メソッド

---

### host.list

SSH config から読み込んだホスト一覧と接続状態を返す。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "host.list",
  "params": {}
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "hosts": [
      {
        "name": "prod-server",
        "hostname": "192.168.1.10",
        "port": 22,
        "user": "deploy",
        "state": "connected",
        "active_forward_count": 2
      },
      {
        "name": "staging",
        "hostname": "10.0.0.5",
        "port": 22,
        "user": "deploy",
        "state": "disconnected",
        "active_forward_count": 0
      }
    ]
  }
}
```

---

### host.reload

SSH config を再読み込みし、ホスト一覧を更新する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "host.reload",
  "params": {}
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "total": 4,
    "added": ["new-server"],
    "removed": []
  }
}
```

---

### ssh.connect

指定ホストに SSH 接続を確立する。auto_connect ルールがあれば自動的にフォワーディングも開始する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "ssh.connect",
  "params": {
    "host": "prod-server"
  }
}
```

**レスポンス（成功）**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "host": "prod-server",
    "status": "connected"
  }
}
```

**レスポンス（エラー）**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": 1001,
    "message": "host not found: unknown-host"
  }
}
```

---

### ssh.disconnect

指定ホストの SSH 接続を切断する。当該ホストの全ポートフォワーディングも停止する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "ssh.disconnect",
  "params": {
    "host": "prod-server"
  }
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "host": "prod-server",
    "status": "disconnected"
  }
}
```

---

### forward.list

転送ルールの一覧を返す。`host` パラメータで特定ホストに絞り込み可能。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "forward.list",
  "params": {
    "host": "prod-server"
  }
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "forwards": [
      {
        "name": "prod-web",
        "host": "prod-server",
        "type": "local",
        "local_port": 8080,
        "remote_host": "localhost",
        "remote_port": 80,
        "auto_connect": true
      },
      {
        "name": "prod-db",
        "host": "prod-server",
        "type": "local",
        "local_port": 5432,
        "remote_host": "localhost",
        "remote_port": 5432,
        "auto_connect": true
      }
    ]
  }
}
```

---

### forward.add

新しい転送ルールを追加し、config.yaml に永続化する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "forward.add",
  "params": {
    "name": "prod-web",
    "host": "prod-server",
    "type": "local",
    "local_port": 8080,
    "remote_host": "localhost",
    "remote_port": 80,
    "auto_connect": true
  }
}
```

**レスポンス（成功）**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "name": "prod-web"
  }
}
```

**レスポンス（エラー — ルール名重複）**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": 1005,
    "message": "rule already exists: prod-web"
  }
}
```

**レスポンス（エラー — ポート競合）**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": 1006,
    "message": "port 8080 is already in use"
  }
}
```

---

### forward.delete

転送ルールを削除する。アクティブな場合は先にフォワーディングを停止する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "forward.delete",
  "params": {
    "name": "prod-web"
  }
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "ok": true
  }
}
```

---

### forward.start

転送ルールのポートフォワーディングを開始する。SSH 未接続の場合は自動的に接続する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "forward.start",
  "params": {
    "name": "prod-web"
  }
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "name": "prod-web",
    "status": "active"
  }
}
```

---

### forward.stop

転送ルールのポートフォワーディングを停止する。SSH 接続は維持する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "forward.stop",
  "params": {
    "name": "prod-web"
  }
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "name": "prod-web",
    "status": "stopped"
  }
}
```

---

### forward.stopAll

全てのアクティブなポートフォワーディングを一括停止する。SSH 接続は維持する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "forward.stopAll",
  "params": {}
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "stopped": 3
  }
}
```

---

### session.list

全アクティブセッションの状態とメトリクスを返す。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "session.list",
  "params": {}
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "sessions": [
      {
        "id": "prod-server-local-8080",
        "name": "prod-web",
        "host": "prod-server",
        "type": "local",
        "local_port": 8080,
        "remote_host": "localhost",
        "remote_port": 80,
        "status": "active",
        "connected_at": "2026-02-11T10:00:00+09:00",
        "bytes_sent": 1258291,
        "bytes_received": 348160,
        "reconnect_count": 0,
        "last_error": ""
      }
    ]
  }
}
```

---

### session.get

指定ルール名のセッション詳細を返す。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "session.get",
  "params": {
    "name": "prod-web"
  }
}
```

**レスポンス**: `session.list` の1要素と同形式。

---

### config.get

現在の設定を返す。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "config.get",
  "params": {}
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "ssh_config_path": "~/.ssh/config",
    "reconnect": {
      "enabled": true,
      "max_retries": 10,
      "initial_delay": "1s",
      "max_delay": "60s"
    },
    "session": {
      "auto_restore": true
    },
    "log": {
      "level": "info",
      "file": "~/.config/moleport/moleport.log"
    }
  }
}
```

---

### config.update

設定を部分的に更新する。指定したフィールドのみ変更される。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "config.update",
  "params": {
    "reconnect": {
      "max_retries": 20
    }
  }
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "ok": true
  }
}
```

---

### daemon.status

デーモンの稼働状態を返す。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "daemon.status",
  "params": {}
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "pid": 12345,
    "started_at": "2026-02-11T09:00:00+09:00",
    "uptime": "3h 30m",
    "connected_clients": 1,
    "active_ssh_connections": 2,
    "active_forwards": 3
  }
}
```

---

### daemon.shutdown

デーモンを停止する。全接続をグレースフルに切断し、状態を保存する。`purge` を指定すると状態ファイルも削除する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "daemon.shutdown",
  "params": {
    "purge": false
  }
}
```

| パラメータ | 型 | 必須 | デフォルト | 説明 |
|-----------|------|------|-----------|------|
| purge | boolean | no | false | `true` の場合、停止時に状態ファイルを削除する |

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "ok": true
  }
}
```

---

### events.subscribe

イベントストリームを開始する。サブスクライブ後、デーモンから通知が非同期に送信される。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "events.subscribe",
  "params": {
    "types": ["ssh", "forward", "metrics"]
  }
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "subscription_id": "sub-abc123"
  }
}
```

**イベントタイプ**:

| タイプ | 説明 |
|-------|------|
| `ssh` | SSH 接続状態の変化（接続/切断/再接続/エラー） |
| `forward` | ポートフォワーディングの状態変化（開始/停止/エラー） |
| `metrics` | メトリクスの定期更新（1秒間隔） |

---

### events.unsubscribe

イベントストリームを停止する。

**リクエスト**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "events.unsubscribe",
  "params": {
    "subscription_id": "sub-abc123"
  }
}
```

**レスポンス**:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "ok": true
  }
}
```

---

## イベント通知

サブスクリプション中にデーモンからクライアントへ送信される通知。`id` フィールドを持たない。

### event.ssh

SSH 接続状態の変化。

```json
{
  "jsonrpc": "2.0",
  "method": "event.ssh",
  "params": {
    "type": "disconnected",
    "host": "prod-server",
    "error": "connection reset by peer"
  }
}
```

| フィールド | 型 | 説明 |
|-----------|------|------|
| type | string | `"connected"` / `"disconnected"` / `"reconnecting"` / `"error"` |
| host | string | ホスト名 |
| error | string | エラーメッセージ（エラー時のみ） |

### event.forward

ポートフォワーディングの状態変化。

```json
{
  "jsonrpc": "2.0",
  "method": "event.forward",
  "params": {
    "type": "started",
    "name": "prod-web",
    "host": "prod-server"
  }
}
```

| フィールド | 型 | 説明 |
|-----------|------|------|
| type | string | `"started"` / `"stopped"` / `"error"` |
| name | string | ルール名 |
| host | string | ホスト名 |
| error | string | エラーメッセージ（エラー時のみ） |

### event.metrics

メトリクスの定期更新（1秒間隔）。

```json
{
  "jsonrpc": "2.0",
  "method": "event.metrics",
  "params": {
    "sessions": [
      {
        "name": "prod-web",
        "status": "active",
        "bytes_sent": 1258291,
        "bytes_received": 348160,
        "uptime": "2h 15m"
      }
    ]
  }
}
```

## エラーコード

### JSON-RPC 標準エラー

| コード | 名前 | 説明 |
|-------|------|------|
| -32700 | ParseError | JSON パースエラー |
| -32600 | InvalidRequest | 不正なリクエスト |
| -32601 | MethodNotFound | 存在しないメソッド |
| -32602 | InvalidParams | 不正なパラメータ |
| -32603 | InternalError | 内部エラー |

### アプリケーション固有エラー

| コード | 名前 | 説明 |
|-------|------|------|
| 1001 | HostNotFound | 指定ホストが SSH config に存在しない |
| 1002 | AlreadyConnected | 指定ホストに既に接続済み |
| 1003 | NotConnected | 指定ホストに未接続 |
| 1004 | RuleNotFound | 指定転送ルールが存在しない |
| 1005 | RuleAlreadyExists | ルール名が重複している |
| 1006 | PortConflict | ポートが他のルールまたはシステムで使用中 |
| 1007 | AuthenticationFailed | SSH 認証に失敗（鍵不正、パスフレーズ誤り等） |

## 改訂履歴

| 版 | 日付 | 変更内容 | 変更理由 |
|---|------|---------|---------|
| 1.0 | 2026-02-11 | 初版作成 | デーモン化対応 |
