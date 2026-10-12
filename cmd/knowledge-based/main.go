package main

import (
	"log"
	"net/http"
	"os"

	kb "github.com/kaulie/knowledge-base/src"
)

func main() {
	addr := os.Getenv("KNOWLEDGE_BASE_ADDR")
	if addr == "" {
		addr = ":4360"
	}
	srv := kb.NewServer(kb.NewService(kb.NewMemoryStore()))
	log.Printf("knowledge-base listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
