package reservas

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"floridaAT/internal/httpx"
)

type confirmacionVencimientoService interface {
	ListExpiredConfirmations(
		ctx context.Context,
	) ([]ConfirmacionPendiente, error)

	ExpireConfirmation(
		ctx context.Context,
		input VencerConfirmacionInput,
	) (Reserva, error)
}

type ConfirmacionVencimientoHandler struct {
	service confirmacionVencimientoService
}

func NewConfirmacionVencimientoHandler(
	service confirmacionVencimientoService,
) *ConfirmacionVencimientoHandler {
	return &ConfirmacionVencimientoHandler{
		service: service,
	}
}

// HandleExpired consulta solicitudes de confirmación cuyo
// límite de respuesta ya fue alcanzado.
//
// Ruta:
//
//	GET /api/reservas/confirmaciones/vencidas
func (h *ConfirmacionVencimientoHandler) HandleExpired(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodGet {
		w.Header().Set(
			"Allow",
			"GET",
		)

		responderError(
			w,
			http.StatusMethodNotAllowed,
			"método no permitido",
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	vencidas, err :=
		h.service.ListExpiredConfirmations(ctx)
	if err != nil {
		responderErrorVencimientoConfirmacion(
			w,
			err,
		)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": vencidas,
		},
	)
}

// HandleExpirations cancela una reserva cuya solicitud de
// confirmación ya venció.
//
// Ruta:
//
//	POST /api/reservas/confirmaciones/vencimientos
func (h *ConfirmacionVencimientoHandler) HandleExpirations(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodPost:
		h.expireConfirmation(w, r)

	default:
		w.Header().Set(
			"Allow",
			"POST",
		)

		responderError(
			w,
			http.StatusMethodNotAllowed,
			"método no permitido",
		)
	}
}

func (h *ConfirmacionVencimientoHandler) expireConfirmation(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input VencerConfirmacionInput

	if err := decodificarJSON(
		r,
		&input,
	); err != nil {
		responderError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf(
				"JSON inválido: %v",
				err,
			),
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		10*time.Second,
	)
	defer cancel()

	reserva, err :=
		h.service.ExpireConfirmation(
			ctx,
			input,
		)
	if err != nil {
		responderErrorVencimientoConfirmacion(
			w,
			err,
		)
		return
	}

	// Se modifica una reserva existente; por eso la respuesta
	// es 200 y no 201.
	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": reserva,
		},
	)
}

func responderErrorVencimientoConfirmacion(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		ErrSolicitudConfirmacionNoVencida,
	),
		errors.Is(
			err,
			ErrReservaNoAceptaVencimientoConfirmacion,
		):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	default:
		// Reutiliza el mapeo general para datos inválidos,
		// reserva inexistente, timeout y errores internos.
		responderErrorServicio(w, err)
	}
}
