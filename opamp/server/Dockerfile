FROM cgr.dev/chainguard/go:latest AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 go build -o /out/opamp-server .

# The binary is built with CGO_ENABLED=0, so the static image is enough.
FROM cgr.dev/chainguard/static:latest
COPY --from=build /out/opamp-server /opamp-server
ENTRYPOINT ["/opamp-server"]
