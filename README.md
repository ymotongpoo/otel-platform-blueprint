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
$ docker compose build gateway
$ docker compose up -d --build
```

`gateway` を先にビルドするのは、Supervisor管理のagentイメージが
`gateway` のビルド成果物である `otelcol-internal:dev` を参照しているためです。
composeはビルドの順序を保証しないので、一括ビルドだけでは
`pull access denied` で失敗することがあります。

初回はCollectorのOCBビルドとサービスのビルドが走ります。4コアの環境で
10分ほどかかります。起動後、http://localhost:3000 でGrafanaが開きます
（匿名Adminでログイン済み）。

ホストで別のCollectorやGrafana Alloyが4317/4318を使っている場合、agentの
ポート公開が `address already in use` で失敗します。その場合は
`deploy/docker-compose.override.yaml` を置いて公開ポートをずらしてください。

```yaml
services:
  agent:
    ports: !override
      - "14317:4317"
      - "14318:4318"
```

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

live-checkはコンテナ内でOTLPを待ち受けるため、リスンアドレスを
`0.0.0.0` にしないとコンテナ外からのテレメトリーが届きません
（`weaver.sh` はこの指定を含んでいます）。

## ゼロコード計装の実験

```console
$ cd autoinstrument/otelc
$ go run go.opentelemetry.io/otelc/tool/cmd/otelc pin
$ go run go.opentelemetry.io/otelc/tool/cmd/otelc go build -o legacy-instrumented .
```

`otelc pin` は `otel.instrumentation.go` と `go.mod` の `require` / `replace` を
生成します。`replace` の宛先は作業ディレクトリ配下の絶対パスになるため、
**これらの生成物はコミットしません**（`.gitignore` で除外しています）。
生成物が残った状態で `pin` を実行すると
`package ... is not part of a module` で失敗します。

`services/uninstrumented` は `:8082` 固定なので、ビルドしたバイナリを
手元で動かすときは `docker compose stop uninstrumented` を先に実行します。

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
