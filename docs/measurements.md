# 実測記録（2026-08-25）

書籍80章の記述の根拠となる実測の記録。すべて2026年8月25日に、
macOS (arm64) + colima、Docker Engine 29.2.1、Docker Compose 5.5.0、
Go 1.26.5 の環境で実施した。

## 使用バージョン

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

## シナリオ1: 3本柱を通す

- `docker compose up -d --build` で10コンテナが起動。Tempoは起動から
  約15〜20秒は `/ready` が200を返さず、その間のテレメトリーは受け付けない
- frontend（`otelinit.Setup` のみ、計装設定コードなし）へ30リクエスト →
  Tempoに2トレース到達。**tail sampling（正常系10%）が機能**
- frontendとbackendのスパンが同一trace IDで連結。**propagationは
  ディストリビューションの既定（autoprop）で成立**
- スパンのresource属性に `host.name`（agentのresource_detectionが付与）、
  `deployment.environment.name`、`team.name`（OTEL_RESOURCE_ATTRIBUTES経由）を確認
- スパン属性に `com.example.delivery.id`（Weaver生成定数で記述）を確認

## シナリオ1b: gatewayのredaction

- frontendのスパンに意図的に `user.email` を付与して送信
- Tempoに保存されたトレースでは `user.email` が存在せず、
  `com.example.delivery.id` は存在。**gatewayのtransform/redactが機能**

## シナリオ1c: メトリクスとログ

- Mimirに `http_server_request_duration` 等のotelhttp由来メトリクスが到達
  （OTLPインジェスト経由、Prometheus命名に変換される）
- backendのotelslogによるログがLokiに到達。**ログエントリに trace_id と
  span_id が付き、トレースとの相関が張られている**ことを確認

## シナリオ2: レジストリとWeaver

- `weaver registry check`: 依存（公式semconv v1.44.0、Git URLで解決）込みで成功。
  レジストリのルートは manifest.yaml を含むディレクトリ（`registry/model/`）に
  する必要がある（テンプレート等のYAMLを同居させると検査対象になり失敗する）
- 負のテスト: `myteam.custom.flag` を定義したファイルを追加すると、
  Regoポリシーが `internal_namespace_only` violationで拒否。削除で成功に戻る
- `weaver registry generate`: minijinjaテンプレートから `sdk/semconv/semconv.go` を
  生成。`com.example.delivery.id` → `ComExampleDeliveryId` のPascalCase変換を確認
- `weaver registry live-check`: OTLP gRPCで受信し、レジストリ未登録の
  `myteam.rogue.attr` をviolationとして報告。`com.example.delivery.id` は違反なし。
  **注意**: 依存先（公式レジストリ）の属性（`service.name` 等）もviolation扱いに
  なった。live-checkの依存解決の範囲は要追加調査

## シナリオ3: OpAMPによるフリート管理

前提: agentはSupervisor（alpha）管理。カスタムCollectorには
**opampextensionとnopreceiver/nopexporterを含める必要がある**
（ないとSupervisorのbootstrapが「collector's OpAMP client never connected」で失敗）。

### 実験A: 正常な設定変更

- `remote-configs/remote.yaml` に attributes processorを追加して保存
- **保存から約8秒でAPPLIED**になり、以降のスパンに `fleet.config.version="2"` が付与
- Collectorの再デプロイなし（Supervisorが子プロセスを再起動）

### 実験B: 起動できない設定（存在しないprocessor参照）

- 配布から約1秒でSupervisorが「Agent crashed during config application,
  reporting FAILED status」を報告
- **サーバー側の注意**: 素朴な「ハッシュ不一致なら再送」ロジックだと、
  FAILED報告後もサーバーが壊れた設定を再プッシュし続けてロールバックと
  押し合いになる。**FAILEDが報告されたハッシュは再送しない制御が必須**
  （opamp/server/main.go に実装）
- 再送を止めた上でも、**Supervisor v0.159.0は壊れた設定を
  `last_working_remote_config.dat` として永続化し、`automatic_config_rollback: true`
  でも前の設定への自己復旧は起きなかった**（2回再現）。フリートは停止したまま
- 復旧は、サーバー側で修正済み設定を配り直すことで完了（配布から約10秒で
  Collectorが再起動しテレメトリー再開）

### 実験C: 起動するが有害な設定（filterで全スパンdrop）

- 配布後、ステータスは **APPLIED、healthy: true のまま**
- 50リクエスト送信してもTempo到達は0件。**テレメトリーだけが止まり、
  ロールバックは発生しない**（設計どおり検出対象外）
- 復旧は正常設定の再配布で完了

### 実験の含意

自動ロールバックは（機能したとしても）起動失敗しか守らない。実測では
alpha実装がその起動失敗からも自己復旧できなかった。フリートの安全は
canary、テレメトリー到達の監視、サーバー側の再配布手順で作る必要がある。

## シナリオ4: otelcによるゼロコード計装

- インストール: `go install go.opentelemetry.io/otelc/tool/cmd/otelc@v1.1.0`
  （モジュールルートではなく tool/cmd/otelc がコマンド）
- 計装コードゼロのHTTPサービスに対して `otelc pin` → `otelc go build`。
  バイナリは約25MB（計装ランタイム込み）
- 実行ログに「HTTP server instrumentation initialized」。
  100リクエスト → Tempoに7トレース到達（tail sampling通過後）
- 環境変数はディストリビューションと同じ `OTEL_SERVICE_NAME` と
  `OTEL_EXPORTER_OTLP_ENDPOINT` がそのまま機能

## 未実施・今後

- Kubernetes（kind + Operator）でのInstrumentation CRD検証
- GenAI規約リポジトリのSHA固定依存をレジストリに追加する検証
- live-checkの依存解決範囲の調査
- MCP経由のAI調査の通し実験（ai-ops/README.md の構成での実走）
