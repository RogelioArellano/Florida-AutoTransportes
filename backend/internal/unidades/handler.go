package unidades

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"floridaAT/internal/httpx"
)

// unidadService define únicamente las operaciones que necesita
// el handler.
//
// *Service implementa esta interfaz automáticamente porque
// contiene los métodos Create y List con las mismas firmas.
type unidadService interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Unidad, error)

	List(
		ctx context.Context,
	) ([]Unidad, error)
}

type Handler struct {
	service unidadService
}

func NewHandler(
	service unidadService,
) *Handler {
	return &Handler{
		service: service,
	}
}

// Handle recibe las peticiones enviadas a /api/unidades
// y selecciona la operación correspondiente.
func (h *Handler) Handle(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)

	case http.MethodPost:
		h.create(w, r)

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

// list obtiene todas las unidades registradas.
func (h *Handler) list(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	unidadesEncontradas, err := h.service.List(ctx)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": unidadesEncontradas,
		},
	)
}

// create registra una nueva unidad.
func (h *Handler) create(
	w http.ResponseWriter,
	r *http.Request,
) {
	// Limitamos el cuerpo de la petición a 1 MB.
	// Un registro de unidad debería ocupar mucho menos,
	// pero este límite protege al servidor.
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input CreateInput

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

	unidadCreada, err := h.service.Create(ctx, input)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusCreated,
		map[string]any{
			"data": unidadCreada,
		},
	)
}

// decodificarJSON transforma el cuerpo de la petición
// en una estructura de Go.
func decodificarJSON(
	r *http.Request,
	destino any,
) error {
	decoder := json.NewDecoder(r.Body)

	// Rechaza propiedades que no existen en CreateInput.
	//
	// Por ejemplo, rechazaría:
	// {"codigo":"UNIDAD-01","capacidad":"17"}
	//
	// porque el campo correcto es capacidad_total.
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destino); err != nil {
		return err
	}

	// Después del primer objeto JSON debe encontrarse EOF.
	// Esto impide recibir dos objetos JSON consecutivos.
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New(
			"el cuerpo debe contener un único objeto JSON",
		)
	}

	return nil
}

// responderErrorServicio traduce errores de dominio
// a códigos de estado HTTP.
func responderErrorServicio(
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

	case errors.Is(err, ErrCodigoDuplicado):
		responderError(
			w,
			http.StatusConflict,
			"ya existe una unidad con ese código",
		)

	case errors.Is(err, ErrPlacasDuplicadas):
		responderError(
			w,
			http.StatusConflict,
			"las placas ya están registradas en otra unidad",
		)

	case errors.Is(err, context.DeadlineExceeded):
		responderError(
			w,
			http.StatusGatewayTimeout,
			"la operación excedió el tiempo permitido",
		)

	default:
		// No mostramos el error técnico al cliente porque podría
		// contener información interna de PostgreSQL.
		responderError(
			w,
			http.StatusInternalServerError,
			"ocurrió un error interno",
		)
	}
}

func responderError(
	w http.ResponseWriter,
	status int,
	mensaje string,
) {
	httpx.WriteJSON(
		w,
		status,
		map[string]string{
			"error": mensaje,
		},
	)
}
