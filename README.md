# otel-platform-blueprint

セルフサービスのオブザーバビリティ基盤をOpenTelemetryで作るための
リファレンス実装です。書籍「セルフサービスのオブザーバビリティ基盤を
OpenTelemetryで作る」（Platform Engineering Kaigi 2026 登壇の解説資料）の
サンプルコードとして、次の部品を動く形で組み合わせています。

- 社内SDKディストリビューション（`sdk/otelinit`）
- セマンティック規約レジストリとWeaverによる検査・コード生成（`registry/`）
- OCBでカスタムビルドしたCollectorのagent/gateway 2段構成（`collector/`）
- OpAMP Supervisorによるフリート管理と最小OpAMPサーバー（`opamp/`）
- ビルド時計装otelcによるゼロコード計装（`autoinstrument/`）
- gen_ai属性を出すエージェント風デモアプリ（`services/ai-app`）
- AIエージェント向けMCP接続の構成例（`ai-ops/`）

検証バックエンドはOSSのGrafanaスタック（Grafana、Tempo、Loki、Mimir）ですが、
計装からOTLP送信までの設計はバックエンドに依存しません。

## 必要なもの

- Docker（compose plugin付き）
- Go 1.26以上（otelcのシナリオのみ）

## 起動

```console
$ cd deploy
$ docker compose up -d --build
```

初回はCollectorのOCBビルドとサービスのビルドが走ります。起動後、
http://localhost:3000 でGrafanaが開きます（匿名Adminでログイン済み）。

```console
$ curl localhost:8080/checkout   # frontend → backend の分散トレース
$ curl localhost:8083/ask        # ai-app のエージェント風トレース
```

tail samplingでエラーなしトレースは10%だけ保存されるため、
Tempoで確認する際は数十回リクエストを送ってください。

## レジストリの操作

```console
$ ./registry/weaver.sh check      # 構文・参照・Regoポリシー検査
$ ./registry/weaver.sh generate   # Go定数を sdk/semconv/ へ生成
$ ./registry/weaver.sh live-check # OTLPを受けて実測を検査
```

## フリート管理の実験

`opamp/remote-configs/remote.yaml` を編集すると、opamp-serverが数秒で
Supervisor管理のagentへ配布します。状態は http://localhost:4321/status で
確認できます。実験の記録は `docs/measurements.md` を参照してください。

## 書籍との対応

| 章 | ディレクトリ |
|---|---|
| 10章 SDKディストリビューション | sdk/ |
| 20章 ゼロコード計装 | autoinstrument/、services/uninstrumented/ |
| 30章 Collector層 | collector/、deploy/ |
| 40章 フリート管理 | opamp/ |
| 50章 レジストリとWeaver | registry/、sdk/semconv/ |
| 60章 AIワークロードの観測 | services/ai-app/ |
| 70章 AIによる読み取り | ai-ops/ |
