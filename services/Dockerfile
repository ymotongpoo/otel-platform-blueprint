# Multi-target build for the demo services.
# Build with: docker build --build-arg SERVICE=frontend -t blueprint-frontend ..
# (context must be the repository root so ../sdk is reachable)
FROM golang:1.26 AS build
ARG SERVICE=frontend
WORKDIR /src
COPY sdk/ sdk/
COPY services/ services/
WORKDIR /src/services
RUN CGO_ENABLED=0 go build -o /out/app ./${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]
