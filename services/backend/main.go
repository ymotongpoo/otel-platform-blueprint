// backend is a demo service instrumented with the internal SDK distribution.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/ymotongpoo/otel-platform-blueprint/sdk/otelinit"
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

	logger := otelslog.NewLogger("backend")

	handler := func(w http.ResponseWriter, r *http.Request) {
		logger.InfoContext(r.Context(), "inventory checked", "items", 42)
		fmt.Fprint(w, "inventory: 42 items")
	}
	mux := http.NewServeMux()
	mux.Handle("/inventory", otelhttp.NewHandler(http.HandlerFunc(handler), "inventory"))
	addr := ":8081"
	log.Printf("backend listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
