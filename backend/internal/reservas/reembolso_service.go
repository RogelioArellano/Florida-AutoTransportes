package reservas

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrReservaNoAceptaReembolso = errors.New(
		"la reserva no acepta reembolsos en su estado actual",
	)

	ErrReservaNoReembolsable = errors.New(
		"la cancelación de la reserva no es reembolsable",
	)

	ErrReservaSinSaldoPorReembolsar = errors.New(
		"la reserva no tiene saldo por reembolsar",
	)

	ErrReembolsoExcedeSaldo = errors.New(
		"el reembolso no puede exceder el saldo por reembolsar",
	)
)

// ReembolsoStore define únicamente las operaciones de
// persistencia requeridas por el servicio de reembolsos.
//
// Se mantiene separado de Store para no aumentar las
// responsabilidades del servicio principal de reservas.
type ReembolsoStore interface {
	RegisterRefund(
		ctx context.Context,
		input RegistrarReembolsoInput,
	) (ReembolsosReserva, error)

	GetRefundsByReservation(
		ctx context.Context,
		reservaID int64,
	) (ReembolsosReserva, error)

	VoidRefund(
		ctx context.Context,
		input AnularReembolsoInput,
	) (ReembolsosReserva, error)
}

// ReembolsoService contiene las reglas independientes de
// PostgreSQL.
//
// El Repository validará posteriormente:
//
//   - estado operativo de la reserva;
//   - derecho al reembolso;
//   - monto autorizado;
//   - reembolsos aplicados anteriormente;
//   - saldo disponible para devolver.
type ReembolsoService struct {
	store ReembolsoStore
}

func NewReembolsoService(
	store ReembolsoStore,
) *ReembolsoService {
	return &ReembolsoService{
		store: store,
	}
}

// RegisterRefund valida y normaliza una solicitud antes de
// enviarla al Repository.
func (s *ReembolsoService) RegisterRefund(
	ctx context.Context,
	input RegistrarReembolsoInput,
) (ReembolsosReserva, error) {
	if input.ReservaID <= 0 {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: reserva_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	montoCentavos, err := parsearDecimal(
		string(input.Monto),
	)
	if err != nil || montoCentavos <= 0 {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: el monto del reembolso debe ser positivo y tener máximo dos decimales",
			ErrDatosInvalidos,
		)
	}

	if montoCentavos > maxImporteCentavos {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: el monto del reembolso excede el importe permitido",
			ErrDatosInvalidos,
		)
	}

	input.Monto = Dinero(
		formatearDecimal(montoCentavos),
	)

	input.Metodo = MetodoReembolso(
		strings.ToUpper(
			strings.TrimSpace(
				string(input.Metodo),
			),
		),
	)

	if !esMetodoReembolsoValido(
		input.Metodo,
	) {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: el método de reembolso no es válido",
			ErrDatosInvalidos,
		)
	}

	input.Referencia, err =
		normalizarTextoOpcional(
			input.Referencia,
			150,
			"referencia del reembolso",
		)
	if err != nil {
		return ReembolsosReserva{}, err
	}

	input.Notas, err =
		normalizarTextoOpcional(
			input.Notas,
			1000,
			"notas del reembolso",
		)
	if err != nil {
		return ReembolsosReserva{}, err
	}

	return s.store.RegisterRefund(
		ctx,
		input,
	)
}

// GetRefundsByReservation obtiene el resumen y todos los
// movimientos de reembolso de una reserva.
func (s *ReembolsoService) GetRefundsByReservation(
	ctx context.Context,
	reservaID int64,
) (ReembolsosReserva, error) {
	if reservaID <= 0 {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: reserva_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	return s.store.GetRefundsByReservation(
		ctx,
		reservaID,
	)
}

func esMetodoReembolsoValido(
	metodo MetodoReembolso,
) bool {
	switch metodo {
	case MetodoReembolsoEfectivo,
		MetodoReembolsoTransferencia,
		MetodoReembolsoTarjeta,
		MetodoReembolsoOtro:
		return true

	default:
		return false
	}
}
