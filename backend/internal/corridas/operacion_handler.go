package corridas

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"floridaAT/internal/httpx"
)

type operacionHTTPService interface {
	GetByID(context.Context, int64) (Corrida, error)
	OpenBoarding(context.Context, OperacionInput) (Corrida, error)
	StartTrip(context.Context, OperacionInput) (Corrida, error)
	CompleteTrip(context.Context, OperacionInput) (Corrida, error)
}

type OperacionHandler struct{ service operacionHTTPService }

func NewOperacionHandler(service operacionHTTPService) *OperacionHandler {
	return &OperacionHandler{service: service}
}

func (h *OperacionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/corridas/{corridaID}", h.HandleByID)
	mux.HandleFunc("/api/corridas/abordajes/aperturas", h.HandleOpenBoarding)
	mux.HandleFunc("/api/corridas/salidas", h.HandleStartTrip)
	mux.HandleFunc("/api/corridas/finalizaciones", h.HandleCompleteTrip)
}

func (h *OperacionHandler) HandleByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		responderError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("corridaID"), 10, 64)
	if err != nil || id <= 0 {
		responderError(w, http.StatusBadRequest, "corrida_id debe ser un entero positivo")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	corrida, err := h.service.GetByID(ctx, id)
	responderOperacion(w, corrida, err)
}

func (h *OperacionHandler) HandleOpenBoarding(w http.ResponseWriter, r *http.Request) {
	h.handlePost(w, r, h.service.OpenBoarding)
}
func (h *OperacionHandler) HandleStartTrip(w http.ResponseWriter, r *http.Request) {
	h.handlePost(w, r, h.service.StartTrip)
}
func (h *OperacionHandler) HandleCompleteTrip(w http.ResponseWriter, r *http.Request) {
	h.handlePost(w, r, h.service.CompleteTrip)
}

func (h *OperacionHandler) handlePost(w http.ResponseWriter, r *http.Request, accion func(context.Context, OperacionInput) (Corrida, error)) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		responderError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var input OperacionInput
	if err := decodificarJSON(r, &input); err != nil {
		responderError(w, http.StatusBadRequest, fmt.Sprintf("JSON inválido: %v", err))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	corrida, err := accion(ctx, input)
	responderOperacion(w, corrida, err)
}

func responderOperacion(w http.ResponseWriter, corrida Corrida, err error) {
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": corrida})
	case errors.Is(err, ErrCorridaNoEncontrada):
		responderError(w, http.StatusNotFound, ErrCorridaNoEncontrada.Error())
	case errors.Is(err, ErrTransicionNoPermitida), errors.Is(err, ErrCorridaSinChofer),
		errors.Is(err, ErrCorridaSinTramo), errors.Is(err, ErrAsistenciasOrigenPendientes),
		errors.Is(err, ErrAsistenciasPendientes), errors.Is(err, ErrFechasOperacionInconsistentes):
		responderError(w, http.StatusConflict, err.Error())
	default:
		responderErrorServicio(w, err)
	}
}
