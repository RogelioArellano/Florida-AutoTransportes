package reservas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"floridaAT/internal/httpx"
)

type reservaService interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Reserva, error)

	List(
		ctx context.Context,
		filter ListFilter,
	) ([]Reserva, error)

	GetByID(
		ctx context.Context,
		reservaID int64,
	) (Reserva, error)

	RegisterPayment(
		ctx context.Context,
		input RegistrarPagoInput,
	) (Reserva, error)

	Confirm(
		ctx context.Context,
		input ConfirmarInput,
	) (Reserva, error)
}

type Handler struct {
	service reservaService
}

func NewHandler(
	service reservaService,
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

// HandlePayments atiende las operaciones relacionadas
// con los pagos posteriores de una reserva.
//
// Se mantiene separado de Handle porque utiliza una ruta
// diferente:
//
//	POST /api/reservas/pagos
func (h *Handler) HandlePayments(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodPost:
		h.registerPayment(w, r)

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

// HandleDetail atiende la consulta individual de una reserva.
//
// Ejemplo:
//
//	GET /api/reservas/2
//
// Se mantiene separado de Handle porque /api/reservas
// representa la colección, mientras que /api/reservas/2
// representa un recurso específico.
func (h *Handler) HandleDetail(
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
		leerReservaIDRuta(r)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	reserva, err := h.service.GetByID(
		ctx,
		reservaID,
	)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": reserva,
		},
	)
}

// HandleConfirmations atiende la confirmación manual
// de una reserva.
//
// Ruta:
//
//	POST /api/reservas/confirmaciones
func (h *Handler) HandleConfirmations(
	w http.ResponseWriter,
	r *http.Request,
) {
	switch r.Method {
	case http.MethodPost:
		h.confirm(w, r)

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

// confirm decodifica la solicitud y delega al Service
// la confirmación manual de la reserva.
func (h *Handler) confirm(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input ConfirmarInput

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

	reserva, err := h.service.Confirm(
		ctx,
		input,
	)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	// La confirmación modifica una reserva existente,
	// por lo que respondemos 200 y no 201.
	httpx.WriteJSON(
		w,
		http.StatusOK,
		map[string]any{
			"data": reserva,
		},
	)
}

func (h *Handler) create(
	w http.ResponseWriter,
	r *http.Request,
) {
	// Limita el tamaño del cuerpo a 1 MiB.
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input CreateInput

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

	// La creación realiza varias consultas, bloqueos e
	// inserciones dentro de una transacción.
	ctx, cancel := context.WithTimeout(
		r.Context(),
		10*time.Second,
	)
	defer cancel()

	reserva, err := h.service.Create(
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
			"data": reserva,
		},
	)
}

// registerPayment decodifica la solicitud HTTP y delega
// las reglas de negocio al Service.
func (h *Handler) registerPayment(
	w http.ResponseWriter,
	r *http.Request,
) {
	r.Body = http.MaxBytesReader(
		w,
		r.Body,
		1<<20,
	)

	var input RegistrarPagoInput

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

	// El registro realiza un bloqueo y varias operaciones
	// dentro de una transacción.
	ctx, cancel := context.WithTimeout(
		r.Context(),
		10*time.Second,
	)
	defer cancel()

	reserva, err :=
		h.service.RegisterPayment(
			ctx,
			input,
		)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	// Respondemos 201 porque se creó un nuevo movimiento
	// dentro de pagos_reserva.
	httpx.WriteJSON(
		w,
		http.StatusCreated,
		map[string]any{
			"data": reserva,
		},
	)
}

func (h *Handler) list(
	w http.ResponseWriter,
	r *http.Request,
) {
	filter, err := construirFiltro(r)
	if err != nil {
		responderErrorServicio(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(
		r.Context(),
		5*time.Second,
	)
	defer cancel()

	reservasEncontradas, err :=
		h.service.List(
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
			"data": reservasEncontradas,
		},
	)
}

// construirFiltro convierte los parámetros de la URL en
// un ListFilter.
//
// El Handler valida que los identificadores realmente sean
// números. Las demás reglas pertenecen al Service.
func construirFiltro(
	r *http.Request,
) (ListFilter, error) {
	var filter ListFilter

	corridaID, err := leerIDQueryOpcional(
		r,
		"corrida_id",
	)
	if err != nil {
		return ListFilter{}, err
	}

	filter.CorridaID = corridaID

	pasajeroID, err := leerIDQueryOpcional(
		r,
		"pasajero_id",
	)
	if err != nil {
		return ListFilter{}, err
	}

	filter.PasajeroID = pasajeroID

	if valor := leerTextoQueryOpcional(
		r,
		"estado",
	); valor != nil {
		estado := Estado(*valor)
		filter.Estado = &estado
	}

	if valor := leerTextoQueryOpcional(
		r,
		"estado_pago",
	); valor != nil {
		estadoPago := EstadoPago(*valor)
		filter.EstadoPago = &estadoPago
	}

	filter.FechaServicioDesde =
		leerTextoQueryOpcional(
			r,
			"fecha_servicio_desde",
		)

	filter.FechaServicioHasta =
		leerTextoQueryOpcional(
			r,
			"fecha_servicio_hasta",
		)

	filter.Busqueda =
		leerTextoQueryOpcional(
			r,
			"busqueda",
		)

	return filter, nil
}

// leerReservaIDRuta extrae el identificador ubicado después
// de /api/reservas/.
//
// Ejemplos:
//
//	/api/reservas/2       -> 2
//	/api/reservas/abc     -> error
//	/api/reservas/2/otro  -> error
func leerReservaIDRuta(
	r *http.Request,
) (int64, error) {
	const prefijo = "/api/reservas/"

	if !strings.HasPrefix(
		r.URL.Path,
		prefijo,
	) {
		return 0, fmt.Errorf(
			"%w: la ruta de la reserva no es válida",
			ErrDatosInvalidos,
		)
	}

	textoID := strings.TrimSpace(
		strings.TrimPrefix(
			r.URL.Path,
			prefijo,
		),
	)

	if textoID == "" ||
		strings.Contains(textoID, "/") {
		return 0, fmt.Errorf(
			"%w: se debe proporcionar un único reserva_id",
			ErrDatosInvalidos,
		)
	}

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

func leerIDQueryOpcional(
	r *http.Request,
	nombre string,
) (*int64, error) {
	texto := strings.TrimSpace(
		r.URL.Query().Get(nombre),
	)

	if texto == "" {
		return nil, nil
	}

	id, err := strconv.ParseInt(
		texto,
		10,
		64,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: %s debe ser un número entero",
			ErrDatosInvalidos,
			nombre,
		)
	}

	return &id, nil
}

func leerTextoQueryOpcional(
	r *http.Request,
	nombre string,
) *string {
	texto := strings.TrimSpace(
		r.URL.Query().Get(nombre),
	)

	if texto == "" {
		return nil
	}

	return &texto
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

	// Intenta leer un segundo valor. Solamente EOF significa
	// que el cuerpo contenía exactamente un objeto JSON.
	if err := decoder.Decode(
		&struct{}{},
	); err != io.EOF {
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
		ErrCorridaNoDisponible,
	):
		responderError(
			w,
			http.StatusConflict,
			"la corrida no existe o no acepta reservas",
		)

	case errors.Is(
		err,
		ErrPasajeroNoDisponible,
	):
		responderError(
			w,
			http.StatusNotFound,
			"el pasajero no existe o está inactivo",
		)

	case errors.Is(err, ErrParadasInvalidas):
		responderError(
			w,
			http.StatusConflict,
			"el origen o destino no son válidos para la corrida",
		)

	case errors.Is(err, ErrCupoInsuficiente):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	case errors.Is(err, ErrAnticipoRequerido):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	case errors.Is(
		err,
		ErrAnticipoInsuficiente,
	):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	case errors.Is(err, ErrPagoExcedeTotal):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	case errors.Is(
		err,
		ErrReservaNoEncontrada,
	):
		responderError(
			w,
			http.StatusNotFound,
			err.Error(),
		)

	case errors.Is(
		err,
		ErrReservaNoAceptaPagos,
	):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	case errors.Is(
		err,
		ErrReservaSinSaldo,
	):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	case errors.Is(
		err,
		ErrPagoExcedeSaldo,
	):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)
	case errors.Is(
		err,
		ErrReservaNoAceptaConfirmacion,
	):
		responderError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	case errors.Is(
		err,
		context.DeadlineExceeded,
	):
		responderError(
			w,
			http.StatusGatewayTimeout,
			"la operación excedió el tiempo permitido",
		)

	default:
		// El detalle técnico solamente se escribe en la
		// terminal del servidor. No se expone al cliente.
		log.Printf(
			"error interno en el módulo reservas: %v",
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
