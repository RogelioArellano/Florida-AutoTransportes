package localidades

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"floridaAT/internal/httpx"
)

const maxRequestBodySize = 1 << 20

type LocalidadService interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Localidad, error)

	List(
		ctx context.Context,
	) ([]Localidad, error)
}

type Handler struct {
	service LocalidadService
}

func NewHandler(
	service LocalidadService,
) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) ServeHTTP(
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
		httpx.WriteError(
			w,
			http.StatusMethodNotAllowed,
			"Método no permitido.",
		)
	}
}

func (h *Handler) create(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		maxRequestBodySize,
	)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var input CreateInput

	if err := decoder.Decode(&input); err != nil {
		httpx.WriteError(
			w,
			http.StatusBadRequest,
			"El cuerpo JSON no es válido.",
		)
		return
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		httpx.WriteError(
			w,
			http.StatusBadRequest,
			"Solo se permite un objeto JSON.",
		)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		3*time.Second,
	)
	defer cancel()

	localidad, err := h.service.Create(ctx, input)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusCreated,
		localidad,
	)
}

func (h *Handler) list(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx, cancel := context.WithTimeout(
		r.Context(),
		3*time.Second,
	)
	defer cancel()

	result, err := h.service.List(ctx)
	if err != nil {
		h.handleError(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		result,
	)
}

func (h *Handler) handleError(
	w http.ResponseWriter,
	err error,
) {
	var validationError *ValidationError

	switch {
	case errors.As(err, &validationError):
		httpx.WriteError(
			w,
			http.StatusBadRequest,
			validationError.Error(),
		)

	case errors.Is(err, ErrLocalidadDuplicada):
		httpx.WriteError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	default:
		httpx.WriteError(
			w,
			http.StatusInternalServerError,
			"No fue posible completar la operación.",
		)
	}
}
