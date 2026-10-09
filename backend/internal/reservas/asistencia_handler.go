package reservas

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"floridaAT/internal/httpx"
)

type asistenciaHTTPService interface {
	GetAttendance(context.Context, int64) (AsistenciaReserva, error)
	RegisterBoarding(context.Context, RegistrarAbordajeInput) (AsistenciaReserva, error)
	CloseAttendance(context.Context, CerrarAsistenciaInput) (AsistenciaReserva, error)
}

var _ asistenciaHTTPService = (*AsistenciaService)(nil)

// AsistenciaHandler traduce HTTP a operaciones del Service.
// Las reglas de cantidades, estados y concurrencia pertenecen
// al Service y Repository, respectivamente.
type AsistenciaHandler struct {
	service asistenciaHTTPService
}

func NewAsistenciaHandler(service asistenciaHTTPService) *AsistenciaHandler {
	return &AsistenciaHandler{service: service}
}

// RegisterRoutes se utiliza tanto en main.go como en las pruebas HTTP.
// Registramos sin prefijo de método para devolver nuestros errores JSON
// y el encabezado Allow cuando se usa un método incorrecto.
func (h *AsistenciaHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/reservas/{reservaID}/asistencia", h.HandleByReservation)
	mux.HandleFunc("/api/reservas/abordajes", h.HandleBoardings)
	mux.HandleFunc("/api/reservas/asistencia/cierres", h.HandleClosures)
}

// HandleByReservation atiende GET /api/reservas/{reservaID}/asistencia.
func (h *AsistenciaHandler) HandleByReservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		responderError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	reservaID, err := strconv.ParseInt(r.PathValue("reservaID"), 10, 64)
	if err != nil || reservaID <= 0 {
		responderError(w, http.StatusBadRequest, "reserva_id debe ser un número entero positivo")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	asistencia, err := h.service.GetAttendance(ctx, reservaID)
	if err != nil {
		responderErrorAsistencia(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": asistencia})
}

// HandleBoardings atiende POST /api/reservas/abordajes.
// cantidad_abordada representa el TOTAL, no un incremento.
func (h *AsistenciaHandler) HandleBoardings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		responderError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var input RegistrarAbordajeInput
	if err := decodificarJSON(r, &input); err != nil {
		responderError(w, http.StatusBadRequest, fmt.Sprintf("JSON inválido: %v", err))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	asistencia, err := h.service.RegisterBoarding(ctx, input)
	if err != nil {
		responderErrorAsistencia(w, err)
		return
	}
	// Se actualiza una reserva existente, incluyendo los reintentos.
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": asistencia})
}

// HandleClosures atiende POST /api/reservas/asistencia/cierres.
// Cierra únicamente la reserva indicada, en su punto de abordaje.
func (h *AsistenciaHandler) HandleClosures(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		responderError(w, http.StatusMethodNotAllowed, "método no permitido")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var input CerrarAsistenciaInput
	if err := decodificarJSON(r, &input); err != nil {
		responderError(w, http.StatusBadRequest, fmt.Sprintf("JSON inválido: %v", err))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	asistencia, err := h.service.CloseAttendance(ctx, input)
	if err != nil {
		responderErrorAsistencia(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"data": asistencia})
}

func responderErrorAsistencia(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrReservaNoAceptaAbordaje),
		errors.Is(err, ErrReservaNoAceptaCierreAsistencia),
		errors.Is(err, ErrCorridaNoAceptaAsistencia),
		errors.Is(err, ErrAsistenciaCerrada),
		errors.Is(err, ErrCantidadAbordadaExcedeReserva),
		errors.Is(err, ErrCantidadAbordadaRetrocede),
		errors.Is(err, ErrAsistenciaHistoricaDesconocida):
		responderError(w, http.StatusConflict, err.Error())
	default:
		// Reutiliza datos inválidos (400), reserva inexistente (404),
		// timeout (504) y errores internos sin detalles técnicos (500).
		responderErrorServicio(w, err)
	}
}
