package reservas

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

type reembolsoService interface {
	RegisterRefund(
		ctx context.Context,
		input RegistrarReembolsoInput,
	) (ReembolsosReserva, error)

	GetRefundsByReservation(
		ctx context.Context,
		reservaID int64,
	) (ReembolsosReserva, error)
}

type ReembolsoHandler struct {
	service reembolsoService
}

func NewReembolsoHandler(
	service reembolsoService,
) *ReembolsoHandler {
	return &ReembolsoHandler{
		service: service,
	}
}

// HandleCollection atiende el registro de movimientos.
//
// Ruta:
//
//	POST /api/reservas/reembolsos
func (h *ReembolsoHandler) HandleCollection(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodPost:
		h.register(w, r)

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

// HandleByReservation consulta los reembolsos de una reserva.
//
// Ruta:
//
//	GET /api/reservas/{reservaID}/reembolsos
func (h *ReembolsoHandler) HandleByReservation(
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

	reservaID, err :=
		leerReservaIDReembolsoRuta(r)
	if err != nil {
		responderErrorReembolso(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	resultado, err :=
		h.service.GetRefundsByReservation(
			ctx,
			reservaID,
		)
	if err != nil {
		responderErrorReembolso(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": resultado,
		},
	)
}

func (h *ReembolsoHandler) register(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input RegistrarReembolsoInput

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
		h.service.RegisterRefund(
			ctx,
			input,
		)
	if err != nil {
		responderErrorReembolso(w, err)
		return
	}

	// Se creó un nuevo movimiento financiero dentro de
	// reembolsos_reserva.
	httpx.WriteJSON(
		w,
		http.StatusCreated,
		map[string]any{
			"data": resultado,
		},
	)
}

// leerReservaIDReembolsoRuta obtiene el valor capturado por:
//
//	/api/reservas/{reservaID}/reembolsos
func leerReservaIDReembolsoRuta(
	r *http.Request,
) (int64, error) {
	textoID := strings.TrimSpace(
		r.PathValue("reservaID"),
	)

	reservaID, err := strconv.ParseInt(
		textoID,
		10,
		64,
	)
	if err != nil || reservaID <= 0 {
		return 0, fmt.Errorf(
			"%w: reserva_id debe ser un número entero positivo",
			ErrDatosInvalidos,
		)
	}

	return reservaID, nil
}

// responderErrorReembolso convierte los errores de dominio
// del reembolso en respuestas HTTP.
//
// Los errores generales del módulo se delegan al mapeo que ya
// existe en handler.go.
func responderErrorReembolso(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		ErrReservaNoAceptaReembolso,
	),
		errors.Is(
			err,
			ErrReservaNoReembolsable,
		),
		errors.Is(
			err,
			ErrReservaSinSaldoPorReembolsar,
		),
		errors.Is(
			err,
			ErrReembolsoExcedeSaldo,
		):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	default:
		// Reutiliza el mapeo existente para:
		//
		//   - datos inválidos;
		//   - reserva inexistente;
		//   - timeout;
		//   - errores internos.
		responderErrorServicio(w, err)
	}
}
