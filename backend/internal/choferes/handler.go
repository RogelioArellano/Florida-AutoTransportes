package choferes

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

// choferService define únicamente las operaciones que
// necesita el Handler.
//
// *Service implementa automáticamente esta interfaz.
type choferService interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Chofer, error)

	List(
		ctx context.Context,
	) ([]Chofer, error)
}

type Handler struct {
	service choferService
}

func NewHandler(
	service choferService,
) *Handler {
	return &Handler{
		service: service,
	}
}

// Handle recibe las peticiones dirigidas a /api/choferes
// y selecciona la operación según el método HTTP.
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

	choferesEncontrados, err := h.service.List(ctx)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": choferesEncontrados,
		},
	)
}

func (h *Handler) create(
	w http.ResponseWriter,
	r *http.Request,
) {
	// Limitamos el cuerpo de la petición a 1 MB.
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

	choferCreado, err := h.service.Create(
		ctx,
		input,
	)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusCreated,
		map[string]any{
			"data": choferCreado,
		},
	)
}

// decodificarJSON convierte un único objeto JSON en una
// estructura de Go.
//
// DisallowUnknownFields evita aceptar propiedades que no
// pertenecen al contrato de CreateInput.
func decodificarJSON(
	r *http.Request,
	destino any,
) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destino); err != nil {
		return err
	}

	// Después del primer objeto debe encontrarse EOF.
	// Así evitamos recibir dos objetos JSON consecutivos.
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New(
			"el cuerpo debe contener un único objeto JSON",
		)
	}

	return nil
}

// responderErrorServicio convierte los errores del dominio
// en respuestas HTTP.
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
