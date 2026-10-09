package corridas

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrCorridaNoEncontrada           = errors.New("la corrida no existe")
	ErrTransicionNoPermitida         = errors.New("la corrida no acepta esta operación")
	ErrCorridaSinChofer              = errors.New("la corrida necesita un chofer asignado para operar")
	ErrCorridaSinTramo               = errors.New("la corrida no tiene un tramo incluido válido")
	ErrAsistenciasOrigenPendientes   = errors.New("falta cerrar la asistencia del primer punto de abordaje")
	ErrAsistenciasPendientes         = errors.New("falta cerrar la asistencia de todas las reservas")
	ErrFechasOperacionInconsistentes = errors.New("las fechas reales de la corrida son inconsistentes")
)

// OperacionStore se mantiene separado del Store de generación y listado.
// Las transiciones se validan dentro de la transacción que bloquea la corrida.
type OperacionStore interface {
	GetOperation(context.Context, int64) (Corrida, error)
	TransitionOperation(context.Context, int64, Estado) (Corrida, error)
}

type OperacionService struct{ store OperacionStore }

func NewOperacionService(store OperacionStore) *OperacionService {
	return &OperacionService{store: store}
}

func validarIDOperacion(id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: corrida_id debe ser un entero positivo", ErrDatosInvalidos)
	}
	return nil
}

func (s *OperacionService) GetByID(ctx context.Context, id int64) (Corrida, error) {
	if err := validarIDOperacion(id); err != nil {
		return Corrida{}, err
	}
	return s.store.GetOperation(ctx, id)
}

func (s *OperacionService) OpenBoarding(ctx context.Context, input OperacionInput) (Corrida, error) {
	return s.transition(ctx, input, EstadoAbordando)
}

func (s *OperacionService) StartTrip(ctx context.Context, input OperacionInput) (Corrida, error) {
	return s.transition(ctx, input, EstadoEnCurso)
}

func (s *OperacionService) CompleteTrip(ctx context.Context, input OperacionInput) (Corrida, error) {
	return s.transition(ctx, input, EstadoCompletada)
}

func (s *OperacionService) transition(ctx context.Context, input OperacionInput, destino Estado) (Corrida, error) {
	if err := validarIDOperacion(input.CorridaID); err != nil {
		return Corrida{}, err
	}
	return s.store.TransitionOperation(ctx, input.CorridaID, destino)
}
