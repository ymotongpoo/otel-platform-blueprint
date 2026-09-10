# 実測記録

書籍80章の記述の根拠となる実測の記録。

## 2026-09-10（Linux/amd64）

Ubuntu (x86_64, 4 CPU / 15GiB RAM)、Docker Engine 29.0.0、Docker Compose v2.40.3、
Go 1.26.0 の環境で実施した。以下、本文の数値はこの回のものを使う。

### 使用バージョン

| コンポーネント | バージョン |
|---|---|
| OpenTelemetry Collector（OCBビルド） | v0.159.0 |
| OCB (cmd/builder) | v0.159.0 |
| opentelemetry-go | v1.45.0（logのみ v0.21.0） |
| opentelemetry-go-contrib（autoexport、autoprop、otelhttp、otelslog） | v0.70.0系 |
| opamp-go | v0.23.0 |
| OpAMP Supervisor | 0.159.0（stability: alpha） |
| Weaver | v0.25.1 |
| otelc | v1.1.0 |
| Tempo / Loki / Mimir / Grafana | 2.9.0 / 3.5.0 / 2.17.1 / 12.2.0 |

### 環境依存の注意（この回で判明）

- **`docker compose up -d --build` は一度では通らない。** `agent` イメージの
  Dockerfile が `FROM otelcol-internal:dev` を参照しているが、これは compose の
  `gateway` サービスがビルドする成果物である。ビルドの順序が保証されないため、
  未ビルドの状態では `pull access denied` で失敗する。**先に
  `docker compose build gateway` を実行してから全体をビルドする。**
- ホストで別のCollectorやAlloyが4317/4318を使っている場合、`agent` の
  ポート公開が `address already in use` で失敗する。`docker-compose.override.yaml`
  で公開ポートをずらす。
- `services/uninstrumented` は `:8082` 固定である。otelcでビルドしたバイナリを
  手元で動かすときは、compose の同サービスを止めてから実行する。

### シナリオ1: 3本柱を通す

- 10コンテナが起動。Tempoは起動から15〜20秒ほど `/ready` が200を返さず
  （`Ingester not ready: waiting for 15s after being ready`）、その間の
  テレメトリーを受け付けない
- frontend（`otelinit.Setup` のみ、計装設定コードなし）へ30リクエスト →
  Tempoに **7トレース** 到達。tail sampling（正常系10%）が機能。
  **保存されるトレース数は確率的に決まるため、回ごとに変動する**
  （同条件の別の回では30リクエストで6トレース）
- frontendとbackendのスパンが同一trace IDで連結し、1トレースあたり
  frontend 2スパン + backend 1スパンになる。**propagationは
  ディストリビューションの既定（autoprop）で成立**
- スパンのresource属性に `host.name`（agentのresource detectionが付与）、
  `deployment.environment.name`、`team.name`（OTEL_RESOURCE_ATTRIBUTES経由）を確認
- スパン属性に `com.example.delivery.id` と `com.example.delivery.carrier`
  （Weaver生成定数で記述）を確認

### シナリオ1b: gatewayのredaction

- frontendはスパンに `user.email` を付けて送信する
- Tempoに保存されたトレースの属性一覧に `user.email` は存在せず、
  `com.example.delivery.id` は存在。**gatewayのtransform/redactが機能**

### シナリオ1c: メトリクスとログ

- Mimirに `http_server_request_duration`、`http_client_request_duration`、
  `http_server_response_body_size` 等のotelhttp由来メトリクスが到達
  （OTLPインジェスト経由、Prometheus命名に変換される）
- backendのotelslogによるログがLokiに到達。**ログのラベルに `trace_id` と
  `span_id` が付き、トレースとの相関が張られている**ことを確認

### シナリオ2: レジストリとWeaver

- `weaver registry check`: 依存（公式semconv、Git URLで解決）込みで成功し、
  `✔ No after_resolution policy violation` を出力。実行時間は約3秒。
  レジストリのルートは manifest.yaml を含むディレクトリ（`registry/model/`）に
  する必要がある
- 負のテスト: `myteam.custom.flag` を定義したファイルを追加すると、Regoポリシーが
  `Violation: semconv_attribute / id=internal_namespace_only` で拒否。削除で成功に戻る
- `weaver registry generate`: minijinjaテンプレートから `sdk/semconv/semconv.go` を
  生成。`com.example.delivery.id` → `ComExampleDeliveryId` のPascalCase変換を確認。
  **生成結果は既存のコミット済みファイルと一致し、差分が出ない**（再現性がある）
- `weaver registry live-check`: **コンテナで動かす場合は
  `--otlp-grpc-address 0.0.0.0` が要る。** 既定のリスンアドレスではコンテナ外からの
  OTLPが届かず、`total seen: 0.0%` のまま何も検査されない
- live-checkにtelemetrygenからスパンを送ると、レジストリ未登録の
  `myteam.rogue.attr` を violation として報告。`com.example.delivery.id` は
  violation にならないが、**`[improvement] stability = development` の指摘は付く**
- **注意**: 依存先（公式レジストリ）の属性（`service.name`、`network.peer.address` 等）も
  violation 扱いになった。live-checkの依存解決の範囲は要追加調査

### シナリオ3: OpAMPによるフリート管理

前提: agentはSupervisor（alpha）管理。カスタムCollectorには
**opampextensionとnopreceiver/nopexporterを含める必要がある**
（ないとSupervisorのbootstrapが失敗する）。

#### 実験A: 正常な設定変更

- `remote-configs/remote.yaml` の `fleet.config.version` を変更して保存
- **保存から2秒以内にサーバーが変更を検知して配布**し、ステータスは APPLIED、
  healthy: true。以降のスパンに新しい `fleet.config.version` が付与された
- Collectorの再デプロイなし（Supervisorが子プロセスを再起動）
- サーバーは設定ファイルを2秒間隔でポーリングする実装なので、検知の遅延は
  この間隔に依存する

#### 実験B: 起動できない設定（存在しないprocessor参照）

- 配布から約1秒でSupervisorが「Agent crashed during config application,
  reporting FAILED status」を報告
- **サーバー側の注意**: 素朴な「ハッシュ不一致なら再送」ロジックだと、
  FAILED報告後もサーバーが壊れた設定を再プッシュし続けてロールバックと
  押し合いになる。**FAILEDが報告されたハッシュは再送しない制御が必須**
  （opamp/server/main.go に実装）
- 再送を止めた上でも、**Supervisor 0.159.0は壊れた設定を
  `last_working_remote_config.dat` として永続化し、`automatic_config_rollback: true`
  でも前の設定への自己復旧は起きなかった**。FAILEDのまま約30秒観測して変化なし。
  この挙動は macOS での前回の実測でも再現しており、環境依存ではない
- 復旧は、サーバー側で修正済み設定を配り直すことで完了

#### 実験C: 起動するが有害な設定（filterで全スパンdrop）

- 配布後、ステータスは **APPLIED、healthy: true のまま**
- 50リクエスト送信してもTempo到達は0件。**テレメトリーだけが止まり、
  ロールバックは発生しない**（設計どおり検出対象外）
- 正常設定を再配布すると復旧し、25リクエストで6トレースが到達した

#### 実験の含意

自動ロールバックは（機能したとしても）起動失敗しか守らない。実測では
alpha実装がその起動失敗からも自己復旧できなかった。フリートの安全は
canary、テレメトリー到達の監視、サーバー側の再配布手順で作る必要がある。

### シナリオ4: otelcによるゼロコード計装

- インストール: `go install go.opentelemetry.io/otelc/tool/cmd/otelc@v1.1.0`
  （モジュールルートではなく tool/cmd/otelc がコマンド）
- **`otelc pin` は、生成済みの `otel.instrumentation.go` と、`replace` 行を
  含む `go.mod` が残っている状態では失敗する**
  （`package ... is not part of a module`）。`replace` の宛先が
  `.otelc-build/` 配下の絶対パスであり、別のマシンでは解決できないためである。
  **`otel.instrumentation.go`、`go.sum`、`go.mod` の `require`/`replace` を
  除いた状態から `otelc pin` を実行する**と、依存が解決されて再生成される。
  したがって**これらの生成物はリポジトリにコミットしない**
- 計装コードゼロのHTTPサービスに対して `otelc pin` → `otelc go build`。
  バイナリは 26,026,623 バイト（約25MB、計装ランタイム込み）
- 実行ログに「trace provider initialized with auto-export」「runtime metrics enabled」。
  100リクエスト → Tempoに **11トレース**到達（tail sampling通過後）
- 環境変数はディストリビューションと同じ `OTEL_SERVICE_NAME` と
  `OTEL_EXPORTER_OTLP_ENDPOINT` がそのまま機能

### シナリオ5: AIワークロードのテレメトリー

- `services/ai-app` の `/ask` へリクエストすると、次の構造のトレースが記録される。
  `invoke_agent` の下に `chat` 2本と `execute_tool` が並び、ツールから呼んだ
  backendの `GET /inventory` が同じトレースにつながる

```text
GET /ask
└── invoke_agent support-agent      gen_ai.operation.name, gen_ai.agent.name, gen_ai.conversation.id
    ├── chat stub-model-1           gen_ai.provider.name, gen_ai.request.model, gen_ai.usage.input_tokens ...
    ├── execute_tool search_orders   gen_ai.tool.name
    │   └── HTTP GET
    │       └── GET /inventory      （backendの通常のスパン）
    └── chat stub-model-1
```

- **トークン数はリクエストごとに乱数で決まるため、特定の値を本文に書かない**
  （観測例: input 129 / output 45、input 223 / output 117）
- ai-appも tail sampling の対象なので、30リクエストで3トレースの到達だった

### 未実施・今後

- Kubernetes（kind + Operator）でのInstrumentation CRD検証
- GenAI規約リポジトリのSHA固定依存をレジストリに追加する検証
- live-checkの依存解決範囲の調査
- MCP経由のAI調査の通し実験（ai-ops/README.md の構成での実走）

## 2026-08-25（macOS/arm64、参考）

macOS (arm64) + colima、Docker Engine 29.2.1、Docker Compose 5.5.0、Go 1.26.5 で
実施した初回の記録。使用バージョンは上と同じ。実験Bの「壊れた設定を
`last_working_remote_config.dat` として永続化し、自動ロールバックが働かない」挙動を
2回再現しており、2026-09-10 のLinuxでの実測でも同じ結果になった。
シナリオ1では30リクエストで2トレース、シナリオ4では100リクエストで7トレースだった
（いずれも tail sampling による変動の範囲内）。
