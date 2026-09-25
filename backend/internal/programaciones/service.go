package programaciones

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrDatosInvalidos = errors.New(
		"datos de programación inválidos",
	)

	ErrCodigoDuplicado = errors.New(
		"el código de la programación ya existe",
	)

	ErrRutaNoDisponible = errors.New(
		"la ruta no existe o está inactiva",
	)

	ErrUnidadNoDisponible = errors.New(
		"la unidad no existe o está inactiva",
	)

	ErrChoferNoDisponible = errors.New(
		"el chofer no existe o está inactivo",
	)
)

type Store interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Programacion, error)

	List(
		ctx context.Context,
	) ([]Programacion, error)
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

// Create normaliza y valida una programación antes de
// enviarla al repositorio.
func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Programacion, error) {
	input.Codigo = strings.ToUpper(
		strings.TrimSpace(input.Codigo),
	)

	input.Nombre = strings.TrimSpace(
		input.Nombre,
	)

	input.HoraSalida = strings.TrimSpace(
		input.HoraSalida,
	)

	input.VigenciaDesde = strings.TrimSpace(
		input.VigenciaDesde,
	)

	input.VigenciaHasta = normalizarTextoOpcional(
		input.VigenciaHasta,
	)

	if input.Codigo == "" {
		return Programacion{}, fmt.Errorf(
			"%w: el código es obligatorio",
			ErrDatosInvalidos,
		)
	}

	if utf8.RuneCountInString(input.Codigo) > 30 {
		return Programacion{}, fmt.Errorf(
			"%w: el código no puede exceder 30 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.Nombre == "" {
		return Programacion{}, fmt.Errorf(
			"%w: el nombre es obligatorio",
			ErrDatosInvalidos,
		)
	}

	if utf8.RuneCountInString(input.Nombre) > 150 {
		return Programacion{}, fmt.Errorf(
			"%w: el nombre no puede exceder 150 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.RutaID <= 0 {
		return Programacion{}, fmt.Errorf(
			"%w: se debe seleccionar una ruta válida",
			ErrDatosInvalidos,
		)
	}

	if input.UnidadID <= 0 {
		return Programacion{}, fmt.Errorf(
			"%w: se debe seleccionar una unidad válida",
			ErrDatosInvalidos,
		)
	}

	if input.ChoferID != nil &&
		*input.ChoferID <= 0 {
		return Programacion{}, fmt.Errorf(
			"%w: el chofer seleccionado no es válido",
			ErrDatosInvalidos,
		)
	}

	if input.HoraSalida == "" {
		return Programacion{}, fmt.Errorf(
			"%w: la hora de salida es obligatoria",
			ErrDatosInvalidos,
		)
	}

	if !esHoraValida(input.HoraSalida) {
		return Programacion{}, fmt.Errorf(
			"%w: la hora de salida debe utilizar el formato HH:MM",
			ErrDatosInvalidos,
		)
	}

	if input.DuracionEstimadaMinutos != nil {
		if *input.DuracionEstimadaMinutos <= 0 {
			return Programacion{}, fmt.Errorf(
				"%w: la duración estimada debe ser mayor que cero",
				ErrDatosInvalidos,
			)
		}

		// SMALLINT admite como máximo 32767.
		if *input.DuracionEstimadaMinutos > 32767 {
			return Programacion{}, fmt.Errorf(
				"%w: la duración estimada excede el límite permitido",
				ErrDatosInvalidos,
			)
		}
	}

	if input.VigenciaDesde == "" {
		return Programacion{}, fmt.Errorf(
			"%w: la fecha inicial de vigencia es obligatoria",
			ErrDatosInvalidos,
		)
	}

	fechaDesde, err := convertirFecha(
		input.VigenciaDesde,
	)
	if err != nil {
		return Programacion{}, fmt.Errorf(
			"%w: vigencia_desde debe utilizar el formato AAAA-MM-DD",
			ErrDatosInvalidos,
		)
	}

	if input.VigenciaHasta != nil {
		fechaHasta, err := convertirFecha(
			*input.VigenciaHasta,
		)
		if err != nil {
			return Programacion{}, fmt.Errorf(
				"%w: vigencia_hasta debe utilizar el formato AAAA-MM-DD",
				ErrDatosInvalidos,
			)
		}

		if fechaHasta.Before(fechaDesde) {
			return Programacion{}, fmt.Errorf(
				"%w: vigencia_hasta no puede ser anterior a vigencia_desde",
				ErrDatosInvalidos,
			)
		}
	}

	if len(input.DiasSemana) == 0 {
		return Programacion{}, fmt.Errorf(
			"%w: se debe seleccionar al menos un día de operación",
			ErrDatosInvalidos,
		)
	}

	diasUtilizados := make(map[int]struct{})

	for _, dia := range input.DiasSemana {
		if dia < 1 || dia > 7 {
			return Programacion{}, fmt.Errorf(
				"%w: el día %d está fuera del rango 1 a 7",
				ErrDatosInvalidos,
				dia,
			)
		}

		if _, existe := diasUtilizados[dia]; existe {
			return Programacion{}, fmt.Errorf(
				"%w: el día %d está repetido",
				ErrDatosInvalidos,
				dia,
			)
		}

		diasUtilizados[dia] = struct{}{}
	}

	// Copiamos el slice antes de ordenarlo para evitar modificar
	// directamente el arreglo recibido por quien llamó al servicio.
	diasOrdenados := append(
		[]int(nil),
		input.DiasSemana...,
	)

	sort.Ints(diasOrdenados)

	input.DiasSemana = diasOrdenados

	return s.store.Create(ctx, input)
}

func (s *Service) List(
	ctx context.Context,
) ([]Programacion, error) {
	return s.store.List(ctx)
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

func esHoraValida(
	hora string,
) bool {
	const formatoHora = "15:04"

	horaConvertida, err := time.Parse(
		formatoHora,
		hora,
	)
	if err != nil {
		return false
	}

	// time.Parse puede aceptar algunos formatos flexibles,
	// como "6:00". Al volver a formatear el resultado,
	// exigimos que el texto original coincida exactamente
	// con el formato HH:MM.
	return horaConvertida.Format(
		formatoHora,
	) == hora
}

func convertirFecha(
	fecha string,
) (time.Time, error) {
	const formatoFecha = "2006-01-02"

	return time.Parse(
		formatoFecha,
		fecha,
	)
}
