// frontend is a demo service instrumented with the internal SDK distribution.
// It receives HTTP requests and calls the backend service.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ymotongpoo/otel-platform-blueprint/sdk/otelinit"
	"github.com/ymotongpoo/otel-platform-blueprint/sdk/semconv"
)

func main() {
	ctx := context.Background()
	shutdown, err := otelinit.Setup(ctx)
	if err != nil {
		log.Fatalf("initialize telemetry: %v", err)
	}
	defer func() {
		_ = shutdown(context.Background())
	}()

	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8081"
	}
	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	handler := func(w http.ResponseWriter, r *http.Request) {
		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(
			semconv.ComExampleDeliveryId.String("dlv-2026-000123"),
			semconv.ComExampleDeliveryCarrier.String("carrier-a"),
			// Deliberately attach PII to demonstrate that the gateway's
			// transform/redact processor strips it before any backend.
			attribute.String("user.email", "taro@example.com"),
		)
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, backendURL+"/inventory", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(w, "frontend ok, backend says: %s", body)
	}

	mux := http.NewServeMux()
	mux.Handle("/checkout", otelhttp.NewHandler(http.HandlerFunc(handler), "checkout"))
	addr := ":8080"
	log.Printf("frontend listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
	_ = otel.GetTracerProvider()
}
