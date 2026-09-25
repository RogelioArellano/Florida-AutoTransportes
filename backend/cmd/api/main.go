package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"floridaAT/internal/choferes"
	"floridaAT/internal/config"
	"floridaAT/internal/corridas"
	"floridaAT/internal/database"
	"floridaAT/internal/httpx"
	"floridaAT/internal/localidades"
	"floridaAT/internal/programaciones"
	"floridaAT/internal/puntosabordaje"
	"floridaAT/internal/rutas"
	"floridaAT/internal/unidades"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
		log.Fatalf(
			"Error al conectar PostgreSQL: %v",
			err,
		)
	}
	defer pool.Close()

	localidadesRepository :=
		localidades.NewPostgresRepository(pool)

	localidadesService :=
		localidades.NewService(localidadesRepository)

	localidadesHandler :=
		localidades.NewHandler(localidadesService)

	puntosRepository :=
		puntosabordaje.NewPostgresRepository(pool)

	puntosService :=
		puntosabordaje.NewService(puntosRepository)

	puntosHandler :=
		puntosabordaje.NewHandler(puntosService)

	rutasRepository := rutas.NewRepository(pool)
	rutasService := rutas.NewService(rutasRepository)
	rutasHandler := rutas.NewHandler(rutasService)

	unidadesRepository :=
		unidades.NewPostgresRepository(pool)

	unidadesService :=
		unidades.NewService(unidadesRepository)

	unidadesHandler :=
		unidades.NewHandler(unidadesService)

	choferesRepository :=
		choferes.NewPostgresRepository(pool)

	choferesService :=
		choferes.NewService(choferesRepository)

	choferesHandler :=
		choferes.NewHandler(choferesService)

	programacionesRepository :=
		programaciones.NewPostgresRepository(pool)

	programacionesService :=
		programaciones.NewService(
			programacionesRepository,
		)

	programacionesHandler :=
		programaciones.NewHandler(
			programacionesService,
		)

	horariosService :=
		programaciones.NewHorarioService(
			programacionesRepository,
		)

	horariosHandler :=
		programaciones.NewHorarioHandler(
			horariosService,
		)

	corridasRepository :=
		corridas.NewPostgresRepository(pool)

	corridasService :=
		corridas.NewService(corridasRepository)

	corridasHandler :=
		corridas.NewHandler(corridasService)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", healthHandler)
	mux.HandleFunc("/api/readyz", readyHandler(pool))
	mux.Handle("/api/localidades", localidadesHandler)
	mux.Handle(
		"/api/puntos-abordaje",
		puntosHandler,
	)

	mux.HandleFunc(
		"/api/rutas",
		rutasHandler.Handle,
	)

	mux.HandleFunc(
		"/api/unidades",
		unidadesHandler.Handle,
	)

	mux.HandleFunc(
		"/api/choferes",
		choferesHandler.Handle,
	)

	mux.HandleFunc(
		"/api/programaciones",
		programacionesHandler.Handle,
	)

	mux.HandleFunc(
		"/api/programaciones/horarios",
		horariosHandler.Handle,
	)

	mux.HandleFunc(
		"/api/corridas",
		corridasHandler.Handle,
	)

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
		httpx.WriteError(
			w,
			http.StatusMethodNotAllowed,
			"Método no permitido.",
		)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]string{
			"status":  "ok",
			"service": "florida-api",
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
			httpx.WriteError(
				w,
				http.StatusMethodNotAllowed,
				"Método no permitido.",
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

			httpx.WriteJSON(
				w,
				http.StatusServiceUnavailable,
				map[string]string{
					"status":   "unavailable",
					"database": "error",
				},
			)
			return
		}

		httpx.WriteJSON(
			w,
			http.StatusOK,
			map[string]string{
				"status":   "ready",
				"database": "ok",
			},
		)
	}
}
