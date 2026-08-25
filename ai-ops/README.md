# ai-ops: AIエージェントにテレメトリーを読ませる構成例

AI調査エージェントに渡す接続は二種類に分かれる。

- **registry MCP**: スキーマ（属性の意味と規約）を読むための接続。
  `weaver registry mcp` が社内レジストリをMCPサーバーとして公開する
- **telemetry MCP**: 実データを読むための接続。観測バックエンド側のMCPサーバーを使う
  （Tempo / Loki / Mimir に対しては [grafana/mcp-grafana](https://github.com/grafana/mcp-grafana) など）

認可は別々に設計する。レジストリは組織内で広く読ませてよい定義情報だが、
本番テレメトリーの読み取りはテナント分離と監査の統制下に置く。

## registry MCP の設定例

MCP対応クライアント（Claude Code等）の設定に次を追加する。
stdio 上の JSON-RPC で動くため、ポート公開は不要。

```json
{
  "mcpServers": {
    "semconv-registry": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-v", "/path/to/otel-platform-blueprint:/work", "-w", "/work",
        "otel/weaver:v0.25.1",
        "registry", "mcp", "-r", "registry/model"
      ]
    }
  }
}
```

これでエージェントは「com.example.delivery.id はどんな意味の属性か」
「配送ドメインにはどんな属性があるか」をレジストリから直接引ける。

## telemetry MCP の設定例

検証環境のGrafanaスタックに対しては mcp-grafana を使う。

```json
{
  "mcpServers": {
    "telemetry": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-e", "GRAFANA_URL=http://host.docker.internal:3000",
        "mcp/grafana"
      ]
    }
  }
}
```

エージェントへの依頼の例: 「ai-appのトレースを検索して、LLM呼び出しの
トークン使用量が多いリクエストを調べて。属性の意味はsemconv-registryで確認して」
