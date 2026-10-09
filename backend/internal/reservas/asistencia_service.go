package reservas

import (
	"context"
	"errors"
	"fmt"
)

const maxObservacionesAsistencia = 1000

var (
	ErrReservaNoAceptaAbordaje = errors.New(
		"la reserva no acepta el registro de abordajes en su estado actual",
	)
	ErrReservaNoAceptaCierreAsistencia = errors.New(
		"la reserva no acepta el cierre de asistencia en su estado actual",
	)
	ErrCorridaNoAceptaAsistencia = errors.New(
		"la corrida no acepta cambios de asistencia en su estado actual",
	)
	ErrAsistenciaCerrada = errors.New(
		"la asistencia de la reserva ya está cerrada",
	)
	ErrCantidadAbordadaExcedeReserva = errors.New(
		"la cantidad abordada no puede exceder los pasajeros de la reserva",
	)
	ErrCantidadAbordadaRetrocede = errors.New(
		"la cantidad abordada no puede ser menor a la ya registrada",
	)
	ErrAsistenciaHistoricaDesconocida = errors.New(
		"la reserva tiene asistencia histórica desconocida y requiere revisión manual",
	)
)

// AsistenciaStore separa la asistencia del servicio principal
// de reservas. Su implementación se agregará al Repository.
type AsistenciaStore interface {
	GetAttendance(
		ctx context.Context,
		reservaID int64,
	) (AsistenciaReserva, error)

	// Debe bloquear la reserva y validar su estado, la corrida,
	// la cantidad reservada y el cierre antes de actualizar.
	// Repetir el mismo total debe conservar el registro original.
	// Una solicitud con un total menor debe rechazarse.
	RegisterBoarding(
		ctx context.Context,
		input RegistrarAbordajeInput,
	) (AsistenciaReserva, error)

	// Debe cerrar la reserva bajo bloqueo. Repetir un cierre
	// devuelve el resultado existente, conservando fecha y notas.
	CloseAttendance(
		ctx context.Context,
		input CerrarAsistenciaInput,
	) (AsistenciaReserva, error)
}

type AsistenciaService struct {
	store AsistenciaStore
}

func NewAsistenciaService(store AsistenciaStore) *AsistenciaService {
	return &AsistenciaService{store: store}
}

func (s *AsistenciaService) GetAttendance(
	ctx context.Context,
	reservaID int64,
) (AsistenciaReserva, error) {
	if err := validarReservaIDAsistencia(reservaID); err != nil {
		return AsistenciaReserva{}, err
	}
	asistencia, err := s.store.GetAttendance(ctx, reservaID)
	if err != nil {
		return AsistenciaReserva{}, err
	}

	return completarResumenAsistencia(asistencia), nil
}

func (s *AsistenciaService) RegisterBoarding(
	ctx context.Context,
	input RegistrarAbordajeInput,
) (AsistenciaReserva, error) {
	if err := validarReservaIDAsistencia(input.ReservaID); err != nil {
		return AsistenciaReserva{}, err
	}

	if input.CantidadAbordada <= 0 {
		return AsistenciaReserva{}, fmt.Errorf(
			"%w: cantidad_abordada debe ser un entero positivo",
			ErrDatosInvalidos,
		)
	}

	// Reutilizamos el límite general de pasajeros del dominio.
	// El límite específico de esta reserva se valida en la BD.
	if input.CantidadAbordada > maxCantidadPasajeros {
		return AsistenciaReserva{}, fmt.Errorf(
			"%w: cantidad_abordada no puede exceder %d",
			ErrDatosInvalidos,
			maxCantidadPasajeros,
		)
	}

	observaciones, err := normalizarTextoOpcional(
		input.Observaciones,
		maxObservacionesAsistencia,
		"observaciones de asistencia",
	)
	if err != nil {
		return AsistenciaReserva{}, err
	}
	input.Observaciones = observaciones

	asistencia, err := s.store.RegisterBoarding(ctx, input)
	if err != nil {
		return AsistenciaReserva{}, err
	}

	return completarResumenAsistencia(asistencia), nil
}

func (s *AsistenciaService) CloseAttendance(
	ctx context.Context,
	input CerrarAsistenciaInput,
) (AsistenciaReserva, error) {
	if err := validarReservaIDAsistencia(input.ReservaID); err != nil {
		return AsistenciaReserva{}, err
	}

	observaciones, err := normalizarTextoOpcional(
		input.Observaciones,
		maxObservacionesAsistencia,
		"observaciones de asistencia",
	)
	if err != nil {
		return AsistenciaReserva{}, err
	}
	input.Observaciones = observaciones

	asistencia, err := s.store.CloseAttendance(ctx, input)
	if err != nil {
		return AsistenciaReserva{}, err
	}

	return completarResumenAsistencia(asistencia), nil
}

func validarReservaIDAsistencia(reservaID int64) error {
	if reservaID <= 0 {
		return fmt.Errorf(
			"%w: reserva_id debe ser un entero positivo",
			ErrDatosInvalidos,
		)
	}
	return nil
}

// completarResumenAsistencia diferencia pendientes y ausentes.
// Antes del cierre nadie se considera ausente definitivamente.
func completarResumenAsistencia(
	asistencia AsistenciaReserva,
) AsistenciaReserva {
	asistencia.AsistenciaConocida = asistencia.CantidadAbordada != nil
	asistencia.AsistenciaCerrada = asistencia.AsistenciaCerradaEn != nil
	asistencia.CantidadPendiente = nil
	asistencia.CantidadNoPresentada = nil

	if !asistencia.AsistenciaConocida {
		return asistencia
	}

	// Una reserva cancelada ya no espera pasajeros. Su cancelación
	// tampoco equivale a una ausencia registrada al cerrar asistencia.
	if asistencia.EstadoReserva == EstadoCancelada {
		cero := 0
		asistencia.CantidadPendiente = &cero
		return asistencia
	}

	restantes := asistencia.CantidadPasajeros - *asistencia.CantidadAbordada
	if asistencia.AsistenciaCerrada {
		cero := 0
		asistencia.CantidadPendiente = &cero
		asistencia.CantidadNoPresentada = &restantes
	} else {
		asistencia.CantidadPendiente = &restantes
	}

	return asistencia
}
