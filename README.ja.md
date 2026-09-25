# otel-platform-blueprint

日本語 | [English](README.md)

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
$ go tool otelc go build -o legacy-instrumented .
```

`otelc go build` は、ビルドの間だけ計装の構成を生成します。`otelc pin` で
`otel.instrumentation.go` を作る手順は使いません。upstream が
[#585](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation/issues/585)
で「pinが生成したファイルをコミットする使い方はまだ未対応」と明記しており、
計装パッケージは擬似バージョンでotelc実行ファイルの内側でしか解決できないため、
ローカル以外では `package ... is not part of a module` で失敗します。

手元で `otelc pin` を試す場合は、生成物（`otel.instrumentation.go` と
`go.mod` の `replace`）をコミットしないでください（`.gitignore` で除外して
います）。

`services/uninstrumented` は `:8082` 固定なので、ビルドしたバイナリを
手元で動かすときは `docker compose stop uninstrumented` を先に実行します。

## フリート管理の実験

`opamp/remote-configs/remote.yaml` を編集すると、opamp-serverが数秒で
Supervisor管理のagentへ配布します。状態は http://localhost:4321/status で
確認できます。実験の記録は `docs/measurements.md` を参照してください。

## 書籍との対応

章番号は[Zenn本](https://zenn.dev/ymotongpoo/books/observability-platform-with-otel)の章番号です。

| 章 | ディレクトリ |
|---|---|
| 2章 SDKディストリビューションの設計 | sdk/ |
| 3章 ゼロコード計装の配布 | autoinstrument/、services/uninstrumented/ |
| 4章 Collector層の設計とカスタムビルド | collector/、deploy/ |
| 5章 OpAMPによるCollectorフリート管理 | opamp/ |
| 6章 セマンティック規約のガバナンスとWeaver | registry/、sdk/semconv/ |
| 7章 AIワークロードのテレメトリー | services/ai-app/ |
| 8章 AIがテレメトリーを読む | ai-ops/ |
| 9章 リファレンス実装で動かす | リポジトリ全体 |

## 動かすときの注意

### ポートの衝突

手元で別のCollectorやGrafana Alloyが動いている場合、エージェントのポート公開が `address already in use` で失敗します。`deploy/docker-compose.override.yaml` で公開ポートをずらしてください。

```bash
# 使用中のポートを確認する
ss -ltnp | grep -E '4317|4318'
```

### Tempoの起動待ち

Tempoは起動後15秒から20秒ほど `/ready` を返さず、その間に届いたテレメトリーを保存しません。起動直後に送ったトレースは保存されないため、準備完了を確認してからリクエストを送ってください。

```bash
until curl -sf http://localhost:3200/ready > /dev/null; do sleep 2; done
```

### live-checkのリッスンアドレス

`weaver registry live-check` をコンテナで動かす場合は、リッスンアドレスとポートを明示します。`registry/weaver.sh` では `--otlp-grpc-address 0.0.0.0 --otlp-grpc-port 4317` を指定しています。Weaver v0.26.0からデフォルトが `127.0.0.1` になったため、この指定がないとコンテナ外から届きません。
