// ai-app is a demo agent-style application. It produces gen_ai.* spans
// following the OpenTelemetry GenAI semantic conventions (Development stage)
// with a stubbed LLM client, so no external API is required.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ymotongpoo/otel-platform-blueprint/sdk/otelinit"
)

// GenAI semantic convention keys (Development stage, pinned by commit SHA in
// registry/manifest.yaml). Hand-written until code generation covers them.
var (
	genAIOperationName    = attribute.Key("gen_ai.operation.name")
	genAIProviderName     = attribute.Key("gen_ai.provider.name")
	genAIRequestModel     = attribute.Key("gen_ai.request.model")
	genAIResponseModel    = attribute.Key("gen_ai.response.model")
	genAIUsageInput       = attribute.Key("gen_ai.usage.input_tokens")
	genAIUsageOutput      = attribute.Key("gen_ai.usage.output_tokens")
	genAIConversationID   = attribute.Key("gen_ai.conversation.id")
	genAIAgentName        = attribute.Key("gen_ai.agent.name")
	genAIToolName         = attribute.Key("gen_ai.tool.name")
)

const model = "stub-model-1"

type stubLLM struct{ tracer trace.Tracer }

func (s *stubLLM) chat(ctx context.Context, purpose string) (string, error) {
	_, span := s.tracer.Start(ctx, "chat "+model,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			genAIOperationName.String("chat"),
			genAIProviderName.String("stub"),
			genAIRequestModel.String(model),
		),
	)
	defer span.End()
	time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
	span.SetAttributes(
		genAIResponseModel.String(model),
		genAIUsageInput.Int(120+rand.Intn(200)),
		genAIUsageOutput.Int(40+rand.Intn(80)),
	)
	return "stub answer for " + purpose, nil
}

func main() {
	ctx := context.Background()
	shutdown, err := otelinit.Setup(ctx)
	if err != nil {
		log.Fatalf("initialize telemetry: %v", err)
	}
	defer func() {
		_ = shutdown(context.Background())
	}()

	tracer := otel.Tracer("ai-app")
	llm := &stubLLM{tracer: tracer}

	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		backendURL = "http://localhost:8081"
	}
	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	handler := func(w http.ResponseWriter, r *http.Request) {
		ctx, agentSpan := tracer.Start(r.Context(), "invoke_agent support-agent",
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				genAIOperationName.String("invoke_agent"),
				genAIAgentName.String("support-agent"),
				genAIConversationID.String(fmt.Sprintf("conv-%d", rand.Intn(1000))),
			),
		)
		defer agentSpan.End()

		// Reasoning step: decide which tool to call.
		if _, err := llm.chat(ctx, "plan"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Tool execution: call an internal API. From here on this is an
		// ordinary distributed trace.
		toolCtx, toolSpan := tracer.Start(ctx, "execute_tool search_orders",
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				genAIOperationName.String("execute_tool"),
				genAIToolName.String("search_orders"),
			),
		)
		req, _ := http.NewRequestWithContext(toolCtx, http.MethodGet, backendURL+"/inventory", nil)
		resp, err := client.Do(req)
		if err != nil {
			toolSpan.End()
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		toolSpan.End()

		// Final answer generation.
		answer, err := llm.chat(ctx, "answer")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "%s (tool result: %s)", answer, body)
	}

	mux := http.NewServeMux()
	mux.Handle("/ask", otelhttp.NewHandler(http.HandlerFunc(handler), "ask"))
	addr := ":8083"
	log.Printf("ai-app listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
