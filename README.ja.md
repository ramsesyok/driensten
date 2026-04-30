# driensten

![logo](docs/description_image.png)

driensten は、HTTP サーバ・MQTT ブローカー・UDP-MQTT ブリッジの3つのサービスを提供します。

# 機能

1. **HTTP サーバ**
   静的 HTML ファイルを配信します。

2. **MQTT ブローカー**
   MQTT ブローカーとして動作し、MQTT 通信を提供します。

3. **UDP–MQTT ブリッジ**
   1. UDP で受信したメッセージを MQTT へパブリッシュします。
   2. MQTT でサブスクライブしたメッセージを UDP で送信します。

# 設定

すべての設定は、実行ファイルと同じディレクトリに置いた `driensten.yaml` で定義します。

```yaml
HTTP:
    listen: 127.0.0.1:8080
    root: dist
    tls:
        enable: false
        cert: server.crt
        key: server.key
MQTT:
  tcp: 127.0.0.1:1883
  websocket: 127.0.0.1:9090
  auth:
    mode: none  # none | password | cert
  tls:
    enable: false
    cert: server.crt
    key: server.key
UDP:
  listen: 127.0.0.1:6565
  allow_unauthenticated_forwards: false
  forwards:
    127.0.0.1:6566:
      - topicA
      - topicB
    127.0.0.1:6567:
      - topicC
      - topicD
```

## HTTP サーバ設定

1. **待受アドレス**
   HTTP サーバのホストとポートを設定します。

   - 例: ポート 8080 で待受
       ```yaml
       HTTP:
           listen: :8080
       ```
   - 例: ローカルホストのみで待受
       ```yaml
       HTTP:
           listen: 127.0.0.1:8080
       ```

2. **ドキュメントルート**
   配信するディレクトリを設定します。絶対パス、または実行ファイルからの相対パスを指定できます。
   ```yaml
   HTTP:
       root: dist
   ```

## MQTT ブローカー設定

TCP および WebSocket の待受アドレスを設定します。

```yaml
MQTT:
  tcp: 127.0.0.1:1883
  websocket: 127.0.0.1:9090
```

### 認証

認証モードは `MQTT.auth.mode` で制御します。3つのモードが利用できます。

| モード | 説明 |
|--------|------|
| `none` | 認証なし。起動時に警告ログを出力します。開発・PoC 環境のみで使用してください。 |
| `password` | ユーザ名とパスワードによる認証。`MQTT.auth.users` にユーザを定義します。 |
| `cert` | TLS クライアント証明書による認証。`MQTT.tls.enable: true` が必要です。 |

**`none` モード（開発・PoC 用）:**
```yaml
MQTT:
  auth:
    mode: none
```

**`password` モード:**
```yaml
MQTT:
  auth:
    mode: password
    users:
      - username: alice
        password: secret
      - username: bob
        password: pass123
```

**`cert` モード:**
```yaml
MQTT:
  auth:
    mode: cert
  tls:
    enable: true
    cert: server.crt
    key: server.key
```

`cert` モードでは、クライアントは TLS ハンドシェイク時に有効なクライアント証明書を提示する必要があります。サーバ側の TLS を有効にし、クライアント証明書を要求する設定が必要です。

## UDP ブリッジ設定

UDP ブリッジは次の2方向の転送をサポートします。

1. **UDP → MQTT**
   設定したポートで UDP パケットを受信し、MQTT へパブリッシュします。
   ```yaml
   UDP:
       listen: 127.0.0.1:6565
   ```

   送信先トピックを指定するには、UDP ペイロードの先頭にトピック名と改行を付与します。
   ```
   <トピック名>\n<ペイロード>
   ```

   > **注意:** `UDP.forwards` に定義されたフィルタにマッチするトピックのみ受け付けます。未登録または空のトピックは破棄されます。トピックフィルタは MQTT ワイルドカード（`+` で単一レベル、`#` で残り全レベル）に対応しています。

2. **MQTT → UDP**
   設定したトピックの MQTT メッセージを、対応する UDP アドレスへ転送します。
   ```yaml
   UDP:
     forwards:
       127.0.0.1:5653:
         - topicA
         - topicB
       127.0.0.1:5656:
         - topicC
         - topicD
   ```

### MQTT → UDP 転送と認証

MQTT → UDP 転送は、MQTT ブローカーの認証が有効（`MQTT.auth.mode: password` または `cert`）な場合のみ動作します。これにより、認証されていないクライアントがバックエンドの UDP サービスに任意のデータを注入することを防ぎます。

| `MQTT.auth.mode` | `UDP.allow_unauthenticated_forwards` | MQTT → UDP 転送 |
|---|---|---|
| `password` または `cert` | 任意 | **有効**（認証済みクライアントのみ） |
| `none` | `false`（デフォルト） | **無効** — 認証なし構成での安全なデフォルト |
| `none` | `true` | **有効** — 未認証クライアントによるデータ注入が可能 |

**開発・PoC 環境** — 認証なしで MQTT → UDP 転送を使用する場合は、明示的にオプトインします。

```yaml
MQTT:
  auth:
    mode: none
UDP:
  allow_unauthenticated_forwards: true  # 誰でも UDP ストリームにデータを注入できる
```

> **警告:** `auth.mode: none` の状態で `allow_unauthenticated_forwards: true` を設定すると、MQTT ポートに到達できるプロセスが設定済みの UDP 送信先に任意のデータを送信できます。隔離された開発環境のみで使用してください。
