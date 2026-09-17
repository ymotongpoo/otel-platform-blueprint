# Build a custom OpenTelemetry Collector distribution with OCB.
#
# Chainguard images are used for both stages. Go keeps backward compatibility,
# so the builder always tracks :latest instead of pinning a minor version.
FROM cgr.dev/chainguard/go:latest AS build
# The Chainguard Go image does not put GOBIN (/root/go/bin) on PATH, unlike
# the Docker Hub golang image. Call the installed binary by its full path.
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.159.0
WORKDIR /work
COPY builder/manifest.yaml .
RUN CGO_ENABLED=0 /root/go/bin/builder --config manifest.yaml

# The binary is built with CGO_ENABLED=0, so the static image is enough.
# Use glibc-dynamic instead if a component needs cgo.
FROM cgr.dev/chainguard/static:latest
COPY --from=build /work/build/otelcol-internal /otelcol-internal
ENTRYPOINT ["/otelcol-internal"]
