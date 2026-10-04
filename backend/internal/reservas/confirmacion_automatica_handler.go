package reservas

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"floridaAT/internal/httpx"
)

type confirmacionAutomaticaService interface {
	ListPendingConfirmations(
		ctx context.Context,
	) ([]ConfirmacionPendiente, error)

	RequestConfirmation(
		ctx context.Context,
		input SolicitarConfirmacionInput,
	) (ConfirmacionPendiente, error)
}

type ConfirmacionAutomaticaHandler struct {
	service confirmacionAutomaticaService
}

func NewConfirmacionAutomaticaHandler(
	service confirmacionAutomaticaService,
) *ConfirmacionAutomaticaHandler {
	return &ConfirmacionAutomaticaHandler{
		service: service,
	}
}

// HandlePending consulta las reservas sin anticipo que ya se
// encuentran dentro de la ventana para solicitar
// confirmación.
//
// Ruta:
//
//	GET /api/reservas/confirmaciones/pendientes
func (h *ConfirmacionAutomaticaHandler) HandlePending(
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

	pendientes, err :=
		h.service.ListPendingConfirmations(ctx)
	if err != nil {
		responderErrorConfirmacionAutomatica(
			w,
			err,
		)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": pendientes,
		},
	)
}

// HandleRequests registra que la solicitud de confirmación
// ya fue enviada correctamente al pasajero.
//
// Ruta:
//
//	POST /api/reservas/confirmaciones/solicitudes
func (h *ConfirmacionAutomaticaHandler) HandleRequests(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodPost:
		h.requestConfirmation(w, r)

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

func (h *ConfirmacionAutomaticaHandler) requestConfirmation(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input SolicitarConfirmacionInput

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

	resultado, err :=
		h.service.RequestConfirmation(
			ctx,
			input,
		)
	if err != nil {
		responderErrorConfirmacionAutomatica(
			w,
			err,
		)
		return
	}

	// Se actualiza una reserva existente; por eso se responde
	// con 200 y no con 201.
	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": resultado,
		},
	)
}

func responderErrorConfirmacionAutomatica(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		ErrReservaNoAceptaSolicitudConfirmacion,
	),
		errors.Is(
			err,
			ErrSolicitudConfirmacionFueraDeVentana,
		):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	default:
		// Reutiliza el mapeo existente para datos inválidos,
		// reserva inexistente, timeout y errores internos.
		responderErrorServicio(w, err)
	}
}
