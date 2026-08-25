// uninstrumented is a demo service with no instrumentation code at all.
// It exists to demonstrate zero-code instrumentation (build-time otelc, eBPF).
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/legacy", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "legacy service response")
	})
	addr := ":8082"
	log.Printf("uninstrumented listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
