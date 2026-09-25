package corridas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// acumulado de errores en corridas
var (
	ErrDatosInvalidos = errors.New(
		"datos de corrida inválidos",
	)

	ErrProgramacionNoDisponible = errors.New(
		"la programación no existe o está inactiva",
	)

	ErrProgramacionNoOperaFecha = errors.New(
		"la programación no opera en la fecha seleccionada",
	)

	ErrCorridaDuplicada = errors.New(
		"la corrida ya fue generada para esa fecha",
	)

	ErrProgramacionSinParadas = errors.New(
		"la programación no contiene paradas",
	)

	ErrUnidadNoDisponible = errors.New(
		"la unidad no existe o está inactiva",
	)

	ErrChoferNoDisponible = errors.New(
		"el chofer no existe o está inactivo",
	)

	ErrLicenciaNoVigente = errors.New(
		"la licencia del chofer no está vigente",
	)
)

// Store define las operaciones de persistencia necesarias
// para el servicio de corridas.
type Store interface {
	Generate(
		ctx context.Context,
		input GenerarInput,
	) (Corrida, error)

	List(
		ctx context.Context,
		filter ListFilter,
	) ([]Corrida, error)
}

type Service struct {
	store Store
}

func NewService(
	store Store,
) *Service {
	return &Service{
		store: store,
	}
}

// Generate valida los datos básicos antes de solicitar
// al repositorio la generación transaccional de la corrida.
func (s *Service) Generate(
	ctx context.Context,
	input GenerarInput,
) (Corrida, error) {
	if input.ProgramacionID <= 0 {
		return Corrida{}, fmt.Errorf(
			"%w: se debe seleccionar una programación válida",
			ErrDatosInvalidos,
		)
	}

	input.FechaServicio = strings.TrimSpace(
		input.FechaServicio,
	)

	if input.FechaServicio == "" {
		return Corrida{}, fmt.Errorf(
			"%w: la fecha de servicio es obligatoria",
			ErrDatosInvalidos,
		)
	}

	if _, err := convertirFechaExacta(
		input.FechaServicio,
	); err != nil {
		return Corrida{}, fmt.Errorf(
			"%w: fecha_servicio debe utilizar el formato AAAA-MM-DD",
			ErrDatosInvalidos,
		)
	}

	input.Observaciones = normalizarTextoOpcional(
		input.Observaciones,
	)

	if input.Observaciones != nil &&
		utf8.RuneCountInString(
			*input.Observaciones,
		) > 1000 {
		return Corrida{}, fmt.Errorf(
			"%w: las observaciones no pueden exceder 1000 caracteres",
			ErrDatosInvalidos,
		)
	}

	return s.store.Generate(ctx, input)
}

// List normaliza y valida los filtros antes de consultar
// el repositorio.
func (s *Service) List(
	ctx context.Context,
	filter ListFilter,
) ([]Corrida, error) {
	filter.FechaDesde = normalizarTextoOpcional(
		filter.FechaDesde,
	)

	filter.FechaHasta = normalizarTextoOpcional(
		filter.FechaHasta,
	)

	var fechaDesde time.Time
	var fechaHasta time.Time
	var err error

	if filter.FechaDesde != nil {
		fechaDesde, err = convertirFechaExacta(
			*filter.FechaDesde,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: fecha_desde debe utilizar el formato AAAA-MM-DD",
				ErrDatosInvalidos,
			)
		}
	}

	if filter.FechaHasta != nil {
		fechaHasta, err = convertirFechaExacta(
			*filter.FechaHasta,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: fecha_hasta debe utilizar el formato AAAA-MM-DD",
				ErrDatosInvalidos,
			)
		}
	}

	if filter.FechaDesde != nil &&
		filter.FechaHasta != nil &&
		fechaHasta.Before(fechaDesde) {
		return nil, fmt.Errorf(
			"%w: fecha_hasta no puede ser anterior a fecha_desde",
			ErrDatosInvalidos,
		)
	}

	if filter.Estado != nil {
		estadoNormalizado := Estado(
			strings.ToUpper(
				strings.TrimSpace(
					string(*filter.Estado),
				),
			),
		)

		if !esEstadoValido(estadoNormalizado) {
			return nil, fmt.Errorf(
				"%w: el estado de corrida no es válido",
				ErrDatosInvalidos,
			)
		}

		filter.Estado = &estadoNormalizado
	}

	return s.store.List(ctx, filter)
}

func normalizarTextoOpcional(
	valor *string,
) *string {
	if valor == nil {
		return nil
	}

	texto := strings.TrimSpace(*valor)

	if texto == "" {
		return nil
	}

	return &texto
}

// convertirFechaExacta valida tanto la existencia de la fecha
// como su representación canónica AAAA-MM-DD.
func convertirFechaExacta(
	fecha string,
) (time.Time, error) {
	const formatoFecha = "2006-01-02"

	fechaConvertida, err := time.Parse(
		formatoFecha,
		fecha,
	)
	if err != nil {
		return time.Time{}, err
	}

	if fechaConvertida.Format(formatoFecha) != fecha {
		return time.Time{}, errors.New(
			"la fecha no utiliza el formato canónico",
		)
	}

	return fechaConvertida, nil
}

func esEstadoValido(
	estado Estado,
) bool {
	switch estado {
	case EstadoProgramada,
		EstadoAbordando,
		EstadoEnCurso,
		EstadoCompletada,
		EstadoCancelada:
		return true

	default:
		return false
	}
}
