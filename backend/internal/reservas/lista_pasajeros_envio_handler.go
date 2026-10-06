package reservas

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"floridaAT/internal/httpx"
)

type listaPasajerosEnvioService interface {
	ListPendingPassengerLists(
		ctx context.Context,
	) ([]CorridaPendienteListaPasajeros, error)

	RegisterPassengerListDelivery(
		ctx context.Context,
		input RegistrarEnvioListaPasajerosInput,
	) (EnvioListaPasajeros, error)
}

// ListaPasajerosEnvioHandler atiende la automatización de
// envíos de listas al chofer.
type ListaPasajerosEnvioHandler struct {
	service listaPasajerosEnvioService
}

func NewListaPasajerosEnvioHandler(
	service listaPasajerosEnvioService,
) *ListaPasajerosEnvioHandler {
	return &ListaPasajerosEnvioHandler{
		service: service,
	}
}

// HandlePending consulta las corridas cuya lista debe enviarse.
//
// Ruta:
//
//	GET /api/corridas/listas-pasajeros/pendientes
func (h *ListaPasajerosEnvioHandler) HandlePending(
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
		h.service.ListPendingPassengerLists(ctx)
	if err != nil {
		responderErrorEnvioListaPasajeros(
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

// HandleDeliveries registra un envío confirmado por el
// proveedor de mensajería.
//
// Ruta:
//
//	POST /api/corridas/listas-pasajeros/envios
func (h *ListaPasajerosEnvioHandler) HandleDeliveries(
	w http.ResponseWriter,
	r *http.Request,
) {
	if r.Method != http.MethodPost {
		w.Header().Set(
			"Allow",
			"POST",
		)

		responderError(
			w,
			http.StatusMethodNotAllowed,
			"método no permitido",
		)
		return
	}

	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input RegistrarEnvioListaPasajerosInput

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
		h.service.RegisterPassengerListDelivery(
			ctx,
			input,
		)
	if err != nil {
		responderErrorEnvioListaPasajeros(
			w,
			err,
		)
		return
	}

	// Se actualiza una corrida existente, por lo que
	// respondemos 200 y no 201.
	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": resultado,
		},
	)
}

func responderErrorEnvioListaPasajeros(
	w http.ResponseWriter,
	err error,
) {
	switch {
	case errors.Is(
		err,
		ErrCorridaNoEncontrada,
	):
		responderError(
			w,
			http.StatusNotFound,
			ErrCorridaNoEncontrada.Error(),
		)

	case errors.Is(
		err,
		ErrCorridaNoAceptaEnvioLista,
	),
		errors.Is(
			err,
			ErrEnvioListaFueraDeVentana,
		),
		errors.Is(
			err,
			ErrCorridaSinChoferLista,
		),
		errors.Is(
			err,
			ErrCorridaSinReservasLista,
		),
		errors.Is(
			err,
			ErrChoferListaPasajerosCambio,
		):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	default:
		// Reutiliza el mapeo existente para datos inválidos,
		// timeout y errores internos.
		responderErrorServicio(w, err)
	}
}
