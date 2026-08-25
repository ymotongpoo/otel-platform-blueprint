# Supervisor-managed agent: OpAMP Supervisor + the internal collector binary.
FROM otelcol-internal:dev AS collector

FROM ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-opampsupervisor:0.159.0 AS supervisor

FROM debian:stable-slim
COPY --from=collector /otelcol-internal /otelcol-internal
COPY --from=supervisor /usr/local/bin/opampsupervisor /usr/local/bin/opampsupervisor
COPY supervisor.yaml /etc/opampsupervisor/supervisor.yaml
RUN mkdir -p /var/lib/otelcol/supervisor
ENTRYPOINT ["opampsupervisor", "--config", "/etc/opampsupervisor/supervisor.yaml"]
