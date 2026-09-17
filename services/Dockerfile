# Multi-target build for the demo services.
# Build with: docker build --build-arg SERVICE=frontend -t blueprint-frontend ..
# (context must be the repository root so ../sdk is reachable)
#
# Chainguard images are used for both stages. Go keeps backward compatibility,
# so the builder always tracks :latest instead of pinning a minor version.
FROM cgr.dev/chainguard/go:latest AS build
ARG SERVICE=frontend
WORKDIR /src
COPY sdk/ sdk/
COPY services/ services/
WORKDIR /src/services
RUN CGO_ENABLED=0 go build -o /out/app ./${SERVICE}

# The binary is built with CGO_ENABLED=0, so the static image is enough.
# Use glibc-dynamic instead if a service needs cgo.
FROM cgr.dev/chainguard/static:latest
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
