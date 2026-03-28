// main is the entry point for the demo application.
// It wires up the application components and starts the HTTP server.
package main

import (
	"demo/internal/config"
	"demo/internal/handler"
	"demo/internal/repo"
	"demo/internal/service"
	"fmt"
	"net/http"
)

func main() {
	cfg := config.NewDefaultConfig()

	userRepo := repo.NewUserRepo()

	// Clean wiring: service depends on repo via interface.
	svc := service.NewUserService(userRepo)
	_ = svc // service is available but handler bypasses it (see handler/api.go)

	// VIOLATION: handler is wired directly to repo, bypassing service.
	h := handler.NewUserHandler(userRepo, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("/users", h.CreateUser)
	mux.HandleFunc("/users/", h.GetUser)

	addr := fmt.Sprintf("%s:%d", cfg.GetHost(), cfg.GetPort())
	fmt.Printf("demo server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Printf("server error: %v\n", err)
	}
}
