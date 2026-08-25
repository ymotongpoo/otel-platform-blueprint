# Build a custom OpenTelemetry Collector distribution with OCB.
FROM golang:1.25 AS build
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.159.0
WORKDIR /work
COPY builder/manifest.yaml .
RUN CGO_ENABLED=0 builder --config manifest.yaml

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /work/build/otelcol-internal /otelcol-internal
ENTRYPOINT ["/otelcol-internal"]
