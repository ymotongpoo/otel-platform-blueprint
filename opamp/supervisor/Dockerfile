# Supervisor-managed agent: OpAMP Supervisor + the internal collector binary.
FROM otelcol-internal:dev AS collector

FROM ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-opampsupervisor:0.159.0 AS supervisor

# Create the Supervisor's storage directory in a builder stage. The runtime
# image has no shell, so `RUN mkdir` is not available there.
#
# The static image runs as UID 65532 (non-root), so the directory must be
# owned by that UID. Leaving it root-owned makes the Supervisor fail at
# startup with "open /var/lib/otelcol/supervisor/persistent_state.yaml:
# permission denied".
FROM cgr.dev/chainguard/wolfi-base:latest AS layout
RUN mkdir -p /var/lib/otelcol/supervisor && chown -R 65532:65532 /var/lib/otelcol

# Both binaries are statically linked (verified with `file`), so the static
# image is enough. It ships no shell, which is why the directory above is
# prepared in the builder stage and copied in.
FROM cgr.dev/chainguard/static:latest
COPY --from=layout --chown=65532:65532 /var/lib/otelcol /var/lib/otelcol
COPY --from=collector /otelcol-internal /otelcol-internal
COPY --from=supervisor /usr/local/bin/opampsupervisor /usr/local/bin/opampsupervisor
COPY supervisor.yaml /etc/opampsupervisor/supervisor.yaml
ENTRYPOINT ["/usr/local/bin/opampsupervisor", "--config", "/etc/opampsupervisor/supervisor.yaml"]
