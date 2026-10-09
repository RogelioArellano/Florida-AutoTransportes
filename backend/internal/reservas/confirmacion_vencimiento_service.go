package reservas

import (
	"context"
	"errors"
	"fmt"
)

const motivoCancelacionConfirmacionVencida = "Cancelación automática: el pasajero no confirmó la reserva dentro del tiempo permitido"

var (
	ErrSolicitudConfirmacionNoVencida = errors.New(
		"el plazo de confirmación de la reserva aún no ha vencido",
	)

	ErrReservaNoAceptaVencimientoConfirmacion = errors.New(
		"la reserva no acepta cancelación por falta de confirmación",
	)
)

// ConfirmacionVencimientoStore define las operaciones de
// persistencia necesarias para consultar y procesar
// solicitudes de confirmación vencidas.
type ConfirmacionVencimientoStore interface {
	ListExpiredConfirmations(
		ctx context.Context,
	) ([]ConfirmacionPendiente, error)

	ExpireConfirmation(
		ctx context.Context,
		params VencerConfirmacionParams,
	) (Reserva, error)
}

type ConfirmacionVencimientoService struct {
	store ConfirmacionVencimientoStore
}

func NewConfirmacionVencimientoService(
	store ConfirmacionVencimientoStore,
) *ConfirmacionVencimientoService {
	return &ConfirmacionVencimientoService{
		store: store,
	}
}

// ListExpiredConfirmations obtiene las reservas cuyo límite
// de respuesta ya fue alcanzado y que todavía permanecen
// apartadas.
func (s *ConfirmacionVencimientoService) ListExpiredConfirmations(
	ctx context.Context,
) ([]ConfirmacionPendiente, error) {
	return s.store.ListExpiredConfirmations(ctx)
}

// ExpireConfirmation cancela una reserva que no fue
// confirmada dentro del plazo permitido.
//
// El Repository volverá a comprobar el estado y el límite
// bajo bloqueo antes de modificar la reserva.
func (s *ConfirmacionVencimientoService) ExpireConfirmation(
	ctx context.Context,
	input VencerConfirmacionInput,
) (Reserva, error) {
	if input.ReservaID <= 0 {
		return Reserva{}, fmt.Errorf(
			"%w: reserva_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	return completarReservaConAsistencia(s.store.ExpireConfirmation(
		ctx,
		VencerConfirmacionParams{
			Input:             input,
			MotivoCancelacion: motivoCancelacionConfirmacionVencida,
		},
	))
}
