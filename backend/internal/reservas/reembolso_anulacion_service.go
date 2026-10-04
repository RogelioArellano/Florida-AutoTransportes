package reservas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrReembolsoNoEncontrado = errors.New(
		"el reembolso no existe",
	)
)

// VoidRefund valida la solicitud de anulación antes de
// enviarla al Repository.
//
// El Repository determinará la reserva relacionada, bloqueará
// sus movimientos y conservará la auditoría original.
func (s *ReembolsoService) VoidRefund(
	ctx context.Context,
	input AnularReembolsoInput,
) (ReembolsosReserva, error) {
	if input.ReembolsoID <= 0 {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: reembolso_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	input.Motivo = strings.TrimSpace(
		input.Motivo,
	)

	if input.Motivo == "" {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: el motivo de anulación es obligatorio",
			ErrDatosInvalidos,
		)
	}

	if utf8.RuneCountInString(
		input.Motivo,
	) > 500 {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: el motivo de anulación no puede exceder 500 caracteres",
			ErrDatosInvalidos,
		)
	}

	return s.store.VoidRefund(
		ctx,
		input,
	)
}
