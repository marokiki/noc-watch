# NOC Watch - WiFi Quality Monitor

WiFi品質を監視し、DHCP更新時間、接続性、レイテンシーを定期的にテストするツールです。

## 機能

- **DHCPテスト**: 5分ごとにWiFiインターフェース（デフォルト: wlan0）のDHCP更新時間を測定
- **接続性テスト**: 1分ごとにIPv4/IPv6接続性とレイテンシーを測定
- **ログ出力**: 1分ごとに結果をテキストファイルに保存
- **systemd管理**: systemdのunitファイルでサービスとして管理
- **ヘッドレスモード**: systemdサービスとして実行時にTUIなしで動作

## 動作モード

### TUIモード（対話的実行）
- ターミナルで直接実行
- リアルタイムでUI表示
- テスト結果を画面上で確認

### ヘッドレスモード（systemdサービス）
- systemdサービスとして実行
- TUIなしでバックグラウンド動作
- 結果はログファイルにのみ出力

## インストール

### 1. ビルドとインストール

```bash
# バイナリをビルドしてインストール
make install
```

### 2. サービスを有効化して開始

```bash
# サービスを有効化して開始
make enable
```

## 使用方法

### サービスの管理

```bash
# サービス開始
make start

# サービス停止
make stop

# サービス状態確認
make status

# サービスログ表示
make logs

# ログファイル内容表示
make logfile
```

### 設定

環境変数で設定をカスタマイズできます：

```bash
# WiFiインターフェースを指定
export WIFI_INTERFACE=wlan0

# ログファイルパスを指定
export LOG_FILE=/var/log/noc-watch/noc-watch.log

# ヘッドレスモードを有効化（systemdサービス用）
export HEADLESS=true
```

または、systemdのunitファイルで設定：

```bash
sudo systemctl edit noc-watch.service
```

### ローカルでの実行（TUIモード）

```bash
# ビルド
make build

# TUIモードで実行
./noc-watch
```

### アンインストール

```bash
make uninstall
```

## ログファイル

ログファイルは `/var/log/noc-watch/noc-watch.log` に保存され、以下の形式で出力されます：

```
=== WiFi Quality Test Results - 2024-01-15 10:30:00 ===
DHCP Test: Success=true, Time=2.5s
Ping Test: Success=true, IPv4=true, IPv6=true, Latency=15ms
Total Tests: 10, Success: 9, Success Rate: 90.00%
==========================================
```

## システム要件

- Linux (systemd対応)
- Go 1.16以上
- sudo権限（DHCP操作のため）
- WiFiインターフェース（wlan0など）

## トラブルシューティング

### サービスが起動しない場合

```bash
# サービスログを確認
sudo journalctl -u noc-watch -f

# サービス状態を確認
sudo systemctl status noc-watch
```

### TUIエラーが発生する場合

systemdサービスとして実行する際は、自動的にヘッドレスモードになります。環境変数`HEADLESS=true`が設定されていることを確認してください。

## 開発

### ローカルでの実行

```bash
# ビルド
make build

# TUIモードで実行
./noc-watch

# ヘッドレスモードで実行
HEADLESS=true ./noc-watch
```

### テスト

```bash
go test ./...
```
