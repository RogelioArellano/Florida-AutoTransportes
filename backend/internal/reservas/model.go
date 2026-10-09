package reservas

import "time"

/*
Dinero representa un importe decimal exacto.
Se maneja como string para evitar los errores de precisión
que puede producir float64 en operaciones financieras.

Ejemplos válidos:

	"450.00"
	"1200.50"
	"0.00"
*/
type Dinero string

// Estado representa la situación operativa de una reserva.
type Estado string

const (
	// EstadoApartada representa una reserva sin confirmar.
	EstadoApartada Estado = "APARTADA"

	// EstadoConfirmada representa una reserva confirmada
	// manualmente o mediante un pago.
	EstadoConfirmada Estado = "CONFIRMADA"

	// EstadoCancelada representa una reserva que ya no ocupa
	// lugares dentro de la corrida.
	EstadoCancelada Estado = "CANCELADA"

	// EstadoAbordada indica que el pasajero subió a la unidad.
	EstadoAbordada Estado = "ABORDADA"

	// EstadoNoPresentada indica que el pasajero no abordó.
	EstadoNoPresentada Estado = "NO_PRESENTADA"
)

// TipoDescuento indica cómo se calcula un descuento.
type TipoDescuento string

const (
	TipoDescuentoPorcentaje TipoDescuento = "PORCENTAJE"
	TipoDescuentoMontoFijo  TipoDescuento = "MONTO_FIJO"
)

// MetodoPago identifica la forma en que se recibió dinero.
type MetodoPago string

const (
	MetodoPagoEfectivo      MetodoPago = "EFECTIVO"
	MetodoPagoTransferencia MetodoPago = "TRANSFERENCIA"
	MetodoPagoTarjeta       MetodoPago = "TARJETA"
	MetodoPagoOtro          MetodoPago = "OTRO"
)

// EstadoMovimientoPago representa el estado individual
// de un pago registrado.
type EstadoMovimientoPago string

const (
	EstadoMovimientoAplicado EstadoMovimientoPago = "APLICADO"
	EstadoMovimientoAnulado  EstadoMovimientoPago = "ANULADO"
)

// EstadoPago representa el resultado financiero acumulado
// de una reserva.
type EstadoPago string

const (
	EstadoPagoSinPago     EstadoPago = "SIN_PAGO"
	EstadoPagoParcial     EstadoPago = "PAGO_PARCIAL"
	EstadoPagoPagada      EstadoPago = "PAGADA"
	EstadoPagoSaldoAFavor EstadoPago = "SALDO_A_FAVOR"
)

// Pasajero representa a la persona responsable de la reserva.
//
// Una reserva puede incluir varios pasajes, pero siempre tendrá
// un pasajero responsable o contacto principal.
type Pasajero struct {
	ID             int64     `json:"id"`
	NombreCompleto string    `json:"nombre_completo"`
	Telefono       string    `json:"telefono"`
	Correo         *string   `json:"correo"`
	Notas          *string   `json:"notas"`
	Activo         bool      `json:"activo"`
	CreadoEn       time.Time `json:"creado_en"`
	ActualizadoEn  time.Time `json:"actualizado_en"`
}

// Pago representa un ingreso relacionado con una reserva.
//
// Los pagos no se eliminan físicamente. Si existe un error,
// deben marcarse como ANULADOS para conservar la auditoría.
type Pago struct {
	ID              int64                `json:"id"`
	ReservaID       int64                `json:"reserva_id"`
	Monto           Dinero               `json:"monto"`
	Metodo          MetodoPago           `json:"metodo"`
	Referencia      *string              `json:"referencia"`
	Estado          EstadoMovimientoPago `json:"estado"`
	PagadoEn        time.Time            `json:"pagado_en"`
	AnuladoEn       *time.Time           `json:"anulado_en"`
	MotivoAnulacion *string              `json:"motivo_anulacion"`
	Notas           *string              `json:"notas"`
	CreadoEn        time.Time            `json:"creado_en"`
	ActualizadoEn   time.Time            `json:"actualizado_en"`
}

// Reserva representa uno o varios pasajes vendidos dentro
// de un tramo específico de una corrida.
type Reserva struct {
	ID    int64  `json:"id"`
	Folio string `json:"folio"`

	// Información de la corrida.
	CorridaID        int64     `json:"corrida_id"`
	CorridaFolio     string    `json:"corrida_folio"`
	FechaServicio    string    `json:"fecha_servicio"`
	SalidaProgramada time.Time `json:"salida_programada"`

	// Pasajero responsable.
	PasajeroID       int64  `json:"pasajero_id"`
	PasajeroNombre   string `json:"pasajero_nombre"`
	PasajeroTelefono string `json:"pasajero_telefono"`

	// Tramo reservado.
	ParadaOrigenID     int64  `json:"corrida_parada_origen_id"`
	ParadaOrigenNombre string `json:"parada_origen_nombre"`
	OrdenOrigen        int    `json:"orden_origen"`

	ParadaDestinoID     int64  `json:"corrida_parada_destino_id"`
	ParadaDestinoNombre string `json:"parada_destino_nombre"`
	OrdenDestino        int    `json:"orden_destino"`

	// Información de los pasajes.
	CantidadPasajeros int    `json:"cantidad_pasajeros"`
	PrecioUnitario    Dinero `json:"precio_unitario"`
	Subtotal          Dinero `json:"subtotal"`

	// Descuento opcional.
	TipoDescuento            *TipoDescuento `json:"tipo_descuento"`
	ValorDescuento           Dinero         `json:"valor_descuento"`
	CantidadPasajesDescuento int            `json:"cantidad_pasajes_descuento"`
	MontoDescuento           Dinero         `json:"monto_descuento"`
	DescripcionDescuento     *string        `json:"descripcion_descuento"`

	Total Dinero `json:"total"`

	// Resumen de pagos obtenido desde vw_reservas_saldos.
	MontoPagado    Dinero     `json:"monto_pagado"`
	SaldoPendiente Dinero     `json:"saldo_pendiente"`
	EstadoPago     EstadoPago `json:"estado_pago"`

	// Estado operativo y confirmación.
	Estado                   Estado     `json:"estado"`
	RequiereConfirmacion     bool       `json:"requiere_confirmacion"`
	ConfirmadaEn             *time.Time `json:"confirmada_en"`
	ConfirmacionSolicitadaEn *time.Time `json:"confirmacion_solicitada_en"`
	ConfirmacionLimiteEn     *time.Time `json:"confirmacion_limite_en"`

	// Se serializa junto al estado operativo, sin modificar los pagos.
	DetalleAsistencia

	// Cancelación.
	CanceladaEn       *time.Time `json:"cancelada_en"`
	MotivoCancelacion *string    `json:"motivo_cancelacion"`

	// Política de reembolso aplicada al cancelar.
	//
	// CancelacionReembolsable solamente indica que existe
	// derecho a solicitar el reembolso. No significa que el
	// dinero ya fue devuelto.
	CancelacionReembolsable bool       `json:"cancelacion_reembolsable"`
	MontoReembolsable       Dinero     `json:"monto_reembolsable"`
	LimiteReembolsoEn       *time.Time `json:"limite_reembolso_en"`

	Observaciones *string `json:"observaciones"`

	// Los pagos se cargarán al consultar el detalle.
	Pagos []Pago `json:"pagos"`

	CreadoEn      time.Time `json:"creado_en"`
	ActualizadoEn time.Time `json:"actualizado_en"`
}

// NuevoPasajeroInput contiene la información necesaria para
// registrar un pasajero durante la creación de una reserva.
type NuevoPasajeroInput struct {
	NombreCompleto string  `json:"nombre_completo"`
	Telefono       string  `json:"telefono"`
	Correo         *string `json:"correo"`
	Notas          *string `json:"notas"`
}

// DescuentoInput representa un descuento opcional.
//
// Valor utiliza significados diferentes según Tipo:
//
//	PORCENTAJE:
//	  "10.00" representa un descuento del 10%.
//
//	MONTO_FIJO:
//	  "100.00" representa un descuento de $100.00.
type DescuentoInput struct {
	Tipo            TipoDescuento `json:"tipo"`
	Valor           string        `json:"valor"`
	CantidadPasajes int           `json:"cantidad_pasajes"`
	Descripcion     string        `json:"descripcion"`
}

// PagoInput representa un pago que puede registrarse junto
// con la reserva o posteriormente.
type PagoInput struct {
	Monto      Dinero     `json:"monto"`
	Metodo     MetodoPago `json:"metodo"`
	Referencia *string    `json:"referencia"`
	Notas      *string    `json:"notas"`
}

// CreateInput contiene los datos para crear una reserva.
//
// Debe enviarse exactamente una de estas opciones:
//
//  1. PasajeroID, cuando el pasajero ya existe.
//  2. NuevoPasajero, cuando se registrará uno nuevo.
//
// El Service validará que no lleguen ambas ni que falten ambas.
type CreateInput struct {
	CorridaID int64 `json:"corrida_id"`

	PasajeroID    *int64              `json:"pasajero_id"`
	NuevoPasajero *NuevoPasajeroInput `json:"nuevo_pasajero"`

	ParadaOrigenID  int64 `json:"corrida_parada_origen_id"`
	ParadaDestinoID int64 `json:"corrida_parada_destino_id"`

	CantidadPasajeros int    `json:"cantidad_pasajeros"`
	PrecioUnitario    Dinero `json:"precio_unitario"`

	Descuento   *DescuentoInput `json:"descuento"`
	PagoInicial *PagoInput      `json:"pago_inicial"`

	Observaciones *string `json:"observaciones"`
}

// RegistrarPagoInput se utilizará para agregar un pago
// posteriormente a una reserva ya existente.
type RegistrarPagoInput struct {
	ReservaID int64 `json:"reserva_id"`
	PagoInput
}

// CancelarInput contiene la información necesaria para
// cancelar una reserva sin eliminar su historial.
type CancelarInput struct {
	ReservaID int64  `json:"reserva_id"`
	Motivo    string `json:"motivo"`
}

// ConfirmarInput permite confirmar manualmente una reserva
// que no tiene anticipo.
type ConfirmarInput struct {
	ReservaID int64 `json:"reserva_id"`
}

// ListFilter contiene los filtros opcionales para consultar
// reservas.
type ListFilter struct {
	CorridaID  *int64
	PasajeroID *int64

	Estado     *Estado
	EstadoPago *EstadoPago

	FechaServicioDesde *string
	FechaServicioHasta *string

	// Busqueda podrá contener nombre, teléfono o folio.
	Busqueda *string
}
