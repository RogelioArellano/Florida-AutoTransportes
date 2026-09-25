package programaciones

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"
)

var (
	ErrProgramacionNoDisponible = errors.New(
		"la programación no existe o está inactiva",
	)

	ErrParadaNoPertenece = errors.New(
		"una parada no pertenece a la ruta de la programación",
	)

	ErrHorariosFueraDeOrden = errors.New(
		"los horarios no respetan el orden de la ruta",
	)
)

// HorarioStore define las operaciones de almacenamiento
// necesarias para configurar horarios de paradas.
type HorarioStore interface {
	Replace(
		ctx context.Context,
		input ConfigurarHorariosInput,
	) ([]HorarioParada, error)

	ListByProgramacion(
		ctx context.Context,
		programacionID int64,
	) ([]HorarioParada, error)
}

type HorarioService struct {
	store HorarioStore
}

func NewHorarioService(
	store HorarioStore,
) *HorarioService {
	return &HorarioService{
		store: store,
	}
}

// Configure valida la configuración completa antes de solicitar
// al repositorio que reemplace los horarios anteriores.
func (s *HorarioService) Configure(
	ctx context.Context,
	input ConfigurarHorariosInput,
) ([]HorarioParada, error) {
	if input.ProgramacionID <= 0 {
		return nil, fmt.Errorf(
			"%w: se debe seleccionar una programación válida",
			ErrDatosInvalidos,
		)
	}

	if len(input.Paradas) == 0 {
		return nil, fmt.Errorf(
			"%w: se debe configurar al menos una parada",
			ErrDatosInvalidos,
		)
	}

	paradasNormalizadas := make(
		[]ConfigurarHorarioParadaInput,
		len(input.Paradas),
	)

	paradasUtilizadas := make(map[int64]struct{})

	for indice, parada := range input.Paradas {
		numeroParada := indice + 1

		if parada.RutaParadaID <= 0 {
			return nil, fmt.Errorf(
				"%w: la parada %d no tiene un identificador válido",
				ErrDatosInvalidos,
				numeroParada,
			)
		}

		if _, existe := paradasUtilizadas[parada.RutaParadaID]; existe {
			return nil, fmt.Errorf(
				"%w: la parada %d está repetida",
				ErrDatosInvalidos,
				numeroParada,
			)
		}

		paradasUtilizadas[parada.RutaParadaID] =
			struct{}{}

		if parada.MinutosDesdeSalidaInicio < 0 {
			return nil, fmt.Errorf(
				"%w: los minutos iniciales de la parada %d no pueden ser negativos",
				ErrDatosInvalidos,
				numeroParada,
			)
		}

		if parada.MinutosDesdeSalidaFin <
			parada.MinutosDesdeSalidaInicio {
			return nil, fmt.Errorf(
				"%w: los minutos finales de la parada %d no pueden ser menores que los iniciales",
				ErrDatosInvalidos,
				numeroParada,
			)
		}

		if parada.MinutosDesdeSalidaInicio > 32767 ||
			parada.MinutosDesdeSalidaFin > 32767 {
			return nil, fmt.Errorf(
				"%w: los minutos de la parada %d exceden el límite permitido",
				ErrDatosInvalidos,
				numeroParada,
			)
		}

		parada.Notas = normalizarTextoOpcional(
			parada.Notas,
		)

		if parada.Notas != nil &&
			utf8.RuneCountInString(
				*parada.Notas,
			) > 250 {
			return nil, fmt.Errorf(
				"%w: las notas de la parada %d no pueden exceder 250 caracteres",
				ErrDatosInvalidos,
				numeroParada,
			)
		}

		paradasNormalizadas[indice] = parada
	}

	input.Paradas = paradasNormalizadas

	return s.store.Replace(ctx, input)
}

func (s *HorarioService) List(
	ctx context.Context,
	programacionID int64,
) ([]HorarioParada, error) {
	if programacionID <= 0 {
		return nil, fmt.Errorf(
			"%w: se debe seleccionar una programación válida",
			ErrDatosInvalidos,
		)
	}

	return s.store.ListByProgramacion(
		ctx,
		programacionID,
	)
}
