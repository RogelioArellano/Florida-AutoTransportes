package reservas

import "time"

// MetodoReembolso identifica la forma en que el negocio
// devolvió el dinero al pasajero.
//
// Aunque actualmente comparte los mismos valores que
// MetodoPago, se mantiene como un tipo independiente porque
// un ingreso y una devolución son movimientos distintos.
type MetodoReembolso string

const (
	MetodoReembolsoEfectivo      MetodoReembolso = "EFECTIVO"
	MetodoReembolsoTransferencia MetodoReembolso = "TRANSFERENCIA"
	MetodoReembolsoTarjeta       MetodoReembolso = "TARJETA"
	MetodoReembolsoOtro          MetodoReembolso = "OTRO"
)

// EstadoMovimientoReembolso representa el estado de un
// movimiento individual de devolución.
type EstadoMovimientoReembolso string

const (
	EstadoMovimientoReembolsoAplicado EstadoMovimientoReembolso = "APLICADO"
	EstadoMovimientoReembolsoAnulado  EstadoMovimientoReembolso = "ANULADO"
)

// EstadoReembolso representa el resultado acumulado de los
// reembolsos correspondientes a una reserva.
type EstadoReembolso string

const (
	EstadoReembolsoNoAplica    EstadoReembolso = "NO_APLICA"
	EstadoReembolsoPendiente   EstadoReembolso = "PENDIENTE"
	EstadoReembolsoParcial     EstadoReembolso = "PARCIAL"
	EstadoReembolsoReembolsado EstadoReembolso = "REEMBOLSADO"
	EstadoReembolsoExcedido    EstadoReembolso = "EXCEDIDO"
)

// Reembolso representa una salida real de dinero relacionada
// con una reserva cancelada.
//
// El pago original no se modifica ni se anula: ambos
// movimientos se conservan para mantener la auditoría.
type Reembolso struct {
	ID        int64  `json:"id"`
	Folio     string `json:"folio"`
	ReservaID int64  `json:"reserva_id"`

	Monto      Dinero          `json:"monto"`
	Metodo     MetodoReembolso `json:"metodo"`
	Referencia *string         `json:"referencia"`

	Estado EstadoMovimientoReembolso `json:"estado"`

	ReembolsadoEn   time.Time  `json:"reembolsado_en"`
	AnuladoEn       *time.Time `json:"anulado_en"`
	MotivoAnulacion *string    `json:"motivo_anulacion"`
	Notas           *string    `json:"notas"`
	CreadoEn        time.Time  `json:"creado_en"`
	ActualizadoEn   time.Time  `json:"actualizado_en"`
}

// RegistrarReembolsoInput contiene los datos recibidos para
// registrar una devolución total o parcial.
type RegistrarReembolsoInput struct {
	ReservaID  int64           `json:"reserva_id"`
	Monto      Dinero          `json:"monto"`
	Metodo     MetodoReembolso `json:"metodo"`
	Referencia *string         `json:"referencia"`
	Notas      *string         `json:"notas"`
}

// ReembolsosReserva contiene el resumen financiero y el
// historial completo de devoluciones de una reserva.
type ReembolsosReserva struct {
	ReservaID int64 `json:"reserva_id"`

	CancelacionReembolsable bool `json:"cancelacion_reembolsable"`

	MontoReembolsable  Dinero `json:"monto_reembolsable"`
	MontoReembolsado   Dinero `json:"monto_reembolsado"`
	SaldoPorReembolsar Dinero `json:"saldo_por_reembolsar"`

	Estado EstadoReembolso `json:"estado_reembolso"`

	Reembolsos []Reembolso `json:"reembolsos"`
}
