package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"floridaAT/internal/config"
	"floridaAT/internal/database"

	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type readinessResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func main() {
	appConfig, err := config.Load()
	if err != nil {
		log.Fatalf("Error de configuración: %v", err)
	}

	startupContext, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)

	pool, err := database.Open(
		startupContext,
		appConfig.DatabaseURL,
	)
	cancel()

	if err != nil {
		log.Fatalf("Error al conectar PostgreSQL: %v", err)
	}
	defer pool.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", healthHandler)
	mux.HandleFunc("/api/readyz", readyHandler(pool))

	server := &http.Server{
		Addr:              appConfig.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("PostgreSQL conectado correctamente")
	log.Printf("API disponible en http://%s", server.Addr)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("La API se detuvo: %v", err)
	}
}

func healthHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(
			w,
			"Método no permitido",
			http.StatusMethodNotAllowed,
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		healthResponse{
			Status:  "ok",
			Service: "florida-api",
		},
	)
}

func readyHandler(
	pool *pgxpool.Pool,
) http.HandlerFunc {
	return func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(
				w,
				"Método no permitido",
				http.StatusMethodNotAllowed,
			)
			return
		}

		ctx, cancel := context.WithTimeout(
			r.Context(),
			2*time.Second,
		)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			log.Printf(
				"PostgreSQL no está disponible: %v",
				err,
			)

			writeJSON(
				w,
				http.StatusServiceUnavailable,
				readinessResponse{
					Status:   "unavailable",
					Database: "error",
				},
			)
			return
		}

		writeJSON(
			w,
			http.StatusOK,
			readinessResponse{
				Status:   "ready",
				Database: "ok",
			},
		)
	}
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	payload any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json; charset=utf-8",
	)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Error al escribir respuesta JSON: %v", err)
	}
}
