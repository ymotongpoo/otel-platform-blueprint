FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 go build -o /out/opamp-server .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/opamp-server /opamp-server
ENTRYPOINT ["/opamp-server"]
