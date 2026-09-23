package rutas

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

// routeService define exclusivamente las operaciones que
// necesita el Handler.
//
// *Service cumple automáticamente esta interfaz.
type routeService interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Ruta, error)

	List(ctx context.Context) ([]Ruta, error)
}

type Handler struct {
	service routeService
}

func NewHandler(service routeService) *Handler {
	return &Handler{
		service: service,
	}
}

// Handle recibe todas las peticiones dirigidas a /api/rutas
// y decide qué función ejecutar según el método HTTP.
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

	rutasEncontradas, err := h.service.List(ctx)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": rutasEncontradas,
		},
	)
}

func (h *Handler) create(
	w http.ResponseWriter,
	r *http.Request,
) {
	// Impide recibir un cuerpo excesivamente grande.
	// Una ruta no debería necesitar más de 1 MB de JSON.
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

	rutaCreada, err := h.service.Create(ctx, input)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusCreated,
		map[string]any{
			"data": rutaCreada,
		},
	)
}

// decodificarJSON convierte el cuerpo de la petición
// en una estructura de Go.
//
// DisallowUnknownFields evita aceptar campos escritos
// incorrectamente o que no pertenezcan al contrato.
func decodificarJSON(
	r *http.Request,
	destino any,
) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destino); err != nil {
		return err
	}

	// Después del primer objeto solo debe existir EOF.
	// Esto evita aceptar dos objetos JSON consecutivos.
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
			"ya existe una ruta con ese código",
		)

	case errors.Is(err, ErrPuntoNoExiste):
		responderError(
			w,
			http.StatusBadRequest,
			"algún punto de abordaje no existe o está inactivo",
		)

	case errors.Is(err, context.DeadlineExceeded):
		responderError(
			w,
			http.StatusGatewayTimeout,
			"la operación excedió el tiempo permitido",
		)

	default:
		// No enviamos el error técnico al cliente.
		// Podría revelar información interna de PostgreSQL.
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
