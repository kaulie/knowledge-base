// Command knowledge-based serves the knowledge-base HTTP API.
//
// The deployment platform starts this binary: build.sh writes it to
// bin/knowledge-based, and scripts/start.sh runs it with KNOWLEDGE_BASE_ADDR
// taken from SERVICE_PORT (platform PORT mapped, default 4360).
//
//	go run ./cmd/knowledge-based
//	KNOWLEDGE_BASE_ADDR=127.0.0.1:4360 go run ./cmd/knowledge-based
//
// General API Info for swaggo/swag — scripts/register-contract.sh reads this
// block (`swag init -g cmd/knowledge-based/main.go`) and registers the result
// in the service registry. Handler annotations live next to the routes in
// src/http.go. Nothing at runtime imports swaggo.
//
// @title        knowledge-base
// @version      1.0.0
// @description  统一知识库：按 kind+provider 登记源，按 CONTEXT entity {type,id} 绑定与检索。project 只是一种 entity type。
// @BasePath     /
// @schemes      http
// @host         127.0.0.1:4360
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	kb "github.com/kaulie/knowledge-base/src"
)

// version is stamped by build.sh (-X main.version=$APP_VERSION).
var version = "dev"

const defaultPort = "4360"

func main() {
	addr := listenAddr()
	srv := kb.NewServer(kb.NewService(kb.NewMemoryStore()))
	log.Printf("knowledge-base %s listening on %s", version, addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func listenAddr() string {
	if addr := strings.TrimSpace(os.Getenv("KNOWLEDGE_BASE_ADDR")); addr != "" {
		return addr
	}
	port := strings.TrimSpace(os.Getenv("SERVICE_PORT"))
	if port == "" {
		port = defaultPort
	}
	return "127.0.0.1:" + port
}
