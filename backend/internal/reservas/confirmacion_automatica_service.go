package reservas

import (
	"context"
	"errors"
	"fmt"
)

const (
	// La solicitud debe comenzar tres horas antes de la salida.
	horasAnticipacionConfirmacion = 3

	// El pasajero tendrá 90 minutos para responder.
	minutosRespuestaConfirmacion = 90
)

var (
	ErrReservaNoAceptaSolicitudConfirmacion = errors.New(
		"la reserva no acepta una solicitud de confirmación",
	)

	ErrSolicitudConfirmacionFueraDeVentana = errors.New(
		"la reserva se encuentra fuera de la ventana de confirmación",
	)
)

// ConfirmacionAutomaticaStore define las operaciones de
// persistencia necesarias para administrar solicitudes de
// confirmación de reservas sin anticipo.
type ConfirmacionAutomaticaStore interface {
	ListPendingConfirmations(
		ctx context.Context,
		politica PoliticaConfirmacionParams,
	) ([]ConfirmacionPendiente, error)

	RequestConfirmation(
		ctx context.Context,
		params SolicitarConfirmacionParams,
	) (ConfirmacionPendiente, error)
}

type ConfirmacionAutomaticaService struct {
	store ConfirmacionAutomaticaStore
}

func NewConfirmacionAutomaticaService(
	store ConfirmacionAutomaticaStore,
) *ConfirmacionAutomaticaService {
	return &ConfirmacionAutomaticaService{
		store: store,
	}
}

// ListPendingConfirmations obtiene las reservas que ya se
// encuentran dentro de la ventana para solicitar confirmación.
func (s *ConfirmacionAutomaticaService) ListPendingConfirmations(
	ctx context.Context,
) ([]ConfirmacionPendiente, error) {
	return s.store.ListPendingConfirmations(
		ctx,
		politicaConfirmacionActual(),
	)
}

// RequestConfirmation registra que el mensaje fue enviado
// correctamente y calcula el límite de respuesta.
//
// El Repository comprobará el estado real de la reserva y
// utilizará el reloj de PostgreSQL dentro de una transacción.
func (s *ConfirmacionAutomaticaService) RequestConfirmation(
	ctx context.Context,
	input SolicitarConfirmacionInput,
) (ConfirmacionPendiente, error) {
	if input.ReservaID <= 0 {
		return ConfirmacionPendiente{}, fmt.Errorf(
			"%w: reserva_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	return s.store.RequestConfirmation(
		ctx,
		SolicitarConfirmacionParams{
			Input:                      input,
			PoliticaConfirmacionParams: politicaConfirmacionActual(),
		},
	)
}

func politicaConfirmacionActual() PoliticaConfirmacionParams {
	return PoliticaConfirmacionParams{
		HorasAnticipacion: horasAnticipacionConfirmacion,
		MinutosRespuesta:  minutosRespuestaConfirmacion,
	}
}
