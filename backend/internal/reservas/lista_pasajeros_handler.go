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

// listaPasajerosService define únicamente la operación que
// necesita el handler.
type listaPasajerosService interface {
	GetPassengerList(
		ctx context.Context,
		corridaID int64,
	) (ListaPasajerosCorrida, error)
}

// ListaPasajerosHandler atiende las consultas de pasajeros
// asociados a una corrida.
type ListaPasajerosHandler struct {
	service listaPasajerosService
}

// NewListaPasajerosHandler construye el handler.
func NewListaPasajerosHandler(
	service listaPasajerosService,
) *ListaPasajerosHandler {
	return &ListaPasajerosHandler{
		service: service,
	}
}

// Handle consulta la lista operativa y financiera.
//
// Ruta:
//
//	GET /api/corridas/{corridaID}/pasajeros
func (h *ListaPasajerosHandler) Handle(
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

	corridaID, err :=
		leerCorridaIDListaPasajerosRuta(r)
	if err != nil {
		responderErrorListaPasajeros(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	lista, err :=
		h.service.GetPassengerList(
			ctx,
			corridaID,
		)
	if err != nil {
		responderErrorListaPasajeros(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": lista,
		},
	)
}

// leerCorridaIDListaPasajerosRuta obtiene el identificador
// capturado por:
//
//	/api/corridas/{corridaID}/pasajeros
func leerCorridaIDListaPasajerosRuta(
	r *http.Request,
) (int64, error) {
	textoID := strings.TrimSpace(
		r.PathValue("corridaID"),
	)

	corridaID, err := strconv.ParseInt(
		textoID,
		10,
		64,
	)
	if err != nil || corridaID <= 0 {
		return 0, fmt.Errorf(
			"%w: corrida_id debe ser un número entero positivo",
			ErrDatosInvalidos,
		)
	}

	return corridaID, nil
}

// responderErrorListaPasajeros convierte los errores del
// dominio en respuestas HTTP.
func responderErrorListaPasajeros(
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

	default:
		// Reutiliza el mapeo existente para:
		//
		//   - datos inválidos;
		//   - tiempo de espera agotado;
		//   - errores internos.
		responderErrorServicio(w, err)
	}
}
