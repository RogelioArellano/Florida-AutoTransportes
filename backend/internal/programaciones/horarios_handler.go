package programaciones

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"floridaAT/internal/httpx"
)

// horarioService contiene solamente las operaciones que
// necesita el handler de horarios.
type horarioService interface {
	Configure(
		ctx context.Context,
		input ConfigurarHorariosInput,
	) ([]HorarioParada, error)

	List(
		ctx context.Context,
		programacionID int64,
	) ([]HorarioParada, error)
}

type HorarioHandler struct {
	service horarioService
}

func NewHorarioHandler(
	service horarioService,
) *HorarioHandler {
	return &HorarioHandler{
		service: service,
	}
}

func (h *HorarioHandler) Handle(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)

	case http.MethodPost:
		h.configure(w, r)

	default:
		w.Header().Set(
			"Allow",
			"GET, POST",
		)

		responderError(
			w,
			http.StatusMethodNotAllowed,
			"método no permitido",
		)
	}
}

func (h *HorarioHandler) list(
	w http.ResponseWriter,
	r *http.Request,
) {
	programacionIDTexto := strings.TrimSpace(
		r.URL.Query().Get("programacion_id"),
	)

	programacionID, err := strconv.ParseInt(
		programacionIDTexto,
		10,
		64,
	)
	if err != nil || programacionID <= 0 {
		responderError(
			w,
			http.StatusBadRequest,
			"programacion_id debe ser un número entero positivo",
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	horarios, err := h.service.List(
		ctx,
		programacionID,
	)
	if err != nil {
		responderErrorHorarioServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": horarios,
		},
	)
}

func (h *HorarioHandler) configure(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input ConfigurarHorariosInput

	// Esta función ya existe en handler.go.
	// Como ambos archivos pertenecen al mismo paquete,
	// podemos reutilizarla.
	if err := decodificarJSON(r, &input); err != nil {
		responderError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf("JSON inválido: %v", err),
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	horarios, err := h.service.Configure(
		ctx,
		input,
	)
	if err != nil {
		responderErrorHorarioServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": horarios,
		},
	)
}

func responderErrorHorarioServicio(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(err, ErrDatosInvalidos):
		responderError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)

	case errors.Is(
		err,
		ErrProgramacionNoDisponible,
	):
		responderError(
			w,
			http.StatusNotFound,
			"la programación no existe o está inactiva",
		)

	case errors.Is(err, ErrParadaNoPertenece):
		responderError(
			w,
			http.StatusBadRequest,
			"alguna parada no pertenece a la ruta de la programación",
		)

	case errors.Is(
		err,
		ErrHorariosFueraDeOrden,
	):
		responderError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)

	case errors.Is(err, context.DeadlineExceeded):
		responderError(
			w,
			http.StatusGatewayTimeout,
			"la operación excedió el tiempo permitido",
		)

	default:
		responderError(
			w,
			http.StatusInternalServerError,
			"ocurrió un error interno",
		)
	}
}
