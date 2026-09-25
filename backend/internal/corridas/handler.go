package corridas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"floridaAT/internal/httpx"
)

type corridaService interface {
	Generate(
		ctx context.Context,
		input GenerarInput,
	) (Corrida, error)

	List(
		ctx context.Context,
		filter ListFilter,
	) ([]Corrida, error)
}

type Handler struct {
	service corridaService
}

func NewHandler(
	service corridaService,
) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) Handle(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)

	case http.MethodPost:
		h.generate(w, r)

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

func (h *Handler) generate(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input GenerarInput

	if err := decodificarJSON(r, &input); err != nil {
		responderError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf("JSON inválido: %v", err),
		)
		return
	}

	// Generar una corrida implica varias operaciones SQL,
	// por lo que concedemos hasta 10 segundos.
	ctx, cancel := context.WithTimeout(
		r.Context(),
		10*time.Second,
	)
	defer cancel()

	corrida, err := h.service.Generate(
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
			"data": corrida,
		},
	)
}

func (h *Handler) list(
	w http.ResponseWriter,
	r *http.Request,
) {
	filter := construirFiltro(r)

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	corridasEncontradas, err := h.service.List(
		ctx,
		filter,
	)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": corridasEncontradas,
		},
	)
}

// construirFiltro obtiene los parámetros de la URL.
//
// No valida las fechas ni el estado; esa responsabilidad
// permanece en el Service.
func construirFiltro(
	r *http.Request,
) ListFilter {
	var filter ListFilter

	fechaDesde := strings.TrimSpace(
		r.URL.Query().Get("fecha_desde"),
	)

	if fechaDesde != "" {
		filter.FechaDesde = &fechaDesde
	}

	fechaHasta := strings.TrimSpace(
		r.URL.Query().Get("fecha_hasta"),
	)

	if fechaHasta != "" {
		filter.FechaHasta = &fechaHasta
	}

	estadoTexto := strings.TrimSpace(
		r.URL.Query().Get("estado"),
	)

	if estadoTexto != "" {
		estado := Estado(estadoTexto)
		filter.Estado = &estado
	}

	return filter
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

	case errors.Is(
		err,
		ErrProgramacionNoDisponible,
	):
		responderError(
			w,
			http.StatusNotFound,
			"la programación no existe o está inactiva",
		)

	case errors.Is(
		err,
		ErrProgramacionNoOperaFecha,
	):
		responderError(
			w,
			http.StatusConflict,
			"la programación no opera en la fecha seleccionada",
		)

	case errors.Is(err, ErrCorridaDuplicada):
		responderError(
			w,
			http.StatusConflict,
			"la corrida ya fue generada para esa fecha",
		)

	case errors.Is(
		err,
		ErrProgramacionSinParadas,
	):
		responderError(
			w,
			http.StatusConflict,
			"la programación no contiene paradas",
		)

	case errors.Is(err, ErrUnidadNoDisponible):
		responderError(
			w,
			http.StatusConflict,
			"la unidad no existe o está inactiva",
		)

	case errors.Is(err, ErrChoferNoDisponible):
		responderError(
			w,
			http.StatusConflict,
			"el chofer no existe o está inactivo",
		)

	case errors.Is(err, ErrLicenciaNoVigente):
		responderError(
			w,
			http.StatusConflict,
			"la licencia del chofer no está registrada o no está vigente",
		)

	case errors.Is(err, context.DeadlineExceeded):
		responderError(
			w,
			http.StatusGatewayTimeout,
			"la operación excedió el tiempo permitido",
		)

	default:
		log.Printf(
			"error interno en el módulo corridas: %v",
			err,
		)

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
