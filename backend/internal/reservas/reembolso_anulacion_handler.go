package reservas

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"floridaAT/internal/httpx"
)

// HandleVoids atiende la anulación de movimientos de
// reembolso capturados por error.
//
// Ruta:
//
//	POST /api/reservas/reembolsos/anulaciones
func (h *ReembolsoHandler) HandleVoids(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodPost:
		h.voidRefund(w, r)

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

func (h *ReembolsoHandler) voidRefund(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input AnularReembolsoInput

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
		h.service.VoidRefund(
			ctx,
			input,
		)
	if err != nil {
		responderErrorAnulacionReembolso(
			w,
			err,
		)
		return
	}

	// Se modifica un movimiento existente; por eso la
	// respuesta es 200 y no 201.
	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": resultado,
		},
	)
}

func responderErrorAnulacionReembolso(
	w http.ResponseWriter,
	err error,
) {
	if errors.Is(
		err,
		ErrReembolsoNoEncontrado,
	) {
		responderError(
			w,
			http.StatusNotFound,
			err.Error(),
		)
		return
	}

	responderErrorReembolso(w, err)
}
