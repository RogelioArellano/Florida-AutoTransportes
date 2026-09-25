package programaciones

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

// programacionService define solamente las operaciones
// que necesita el Handler.
type programacionService interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Programacion, error)

	List(
		ctx context.Context,
	) ([]Programacion, error)
}

type Handler struct {
	service programacionService
}

func NewHandler(
	service programacionService,
) *Handler {
	return &Handler{
		service: service,
	}
}

// Handle recibe las peticiones dirigidas a
// /api/programaciones.
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

func (h *Handler) list(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	programacionesEncontradas, err :=
		h.service.List(ctx)

	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": programacionesEncontradas,
		},
	)
}

func (h *Handler) create(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	programacionCreada, err :=
		h.service.Create(ctx, input)

	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusCreated,
		map[string]any{
			"data": programacionCreada,
		},
	)
}

func decodificarJSON(
	r *http.Request,
	destino any,
) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destino); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New(
			"el cuerpo debe contener un único objeto JSON",
		)
	}

	return nil
}

// responderErrorServicio traduce errores del dominio
// a códigos HTTP.
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
			"ya existe una programación con ese código",
		)

	case errors.Is(err, ErrRutaNoDisponible):
		responderError(
			w,
			http.StatusBadRequest,
			"la ruta no existe o está inactiva",
		)

	case errors.Is(err, ErrUnidadNoDisponible):
		responderError(
			w,
			http.StatusBadRequest,
			"la unidad no existe o está inactiva",
		)

	case errors.Is(err, ErrChoferNoDisponible):
		responderError(
			w,
			http.StatusBadRequest,
			"el chofer no existe o está inactivo",
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
