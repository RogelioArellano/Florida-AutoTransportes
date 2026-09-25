package unidades

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// acumulado de errores especificos para la gestión de unidades
var (
	ErrDatosInvalidos = errors.New(
		"datos de unidad invalidos",
	)

	ErrCodigoDuplicado = errors.New(
		"el código de la unidad ya existe",
	)

	ErrPlacasDuplicadas = errors.New(
		"las placas ya están registradas",
	)
)

/*
Store define exclusivamente las operaciones de almacenamiento
que se requiere en este servicio.

La implementación real será con BD, pero las pruebas usarán
un objeto simulado
*/

type Store interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Unidad, error)

	List(
		ctx context.Context,
	) ([]Unidad, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
	}
}

// create normaliza y valida los datos antes de enviarlos al repo
func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Unidad, error) {

	input.Codigo = strings.ToUpper(
		strings.TrimSpace(input.Codigo),
	)

	input.Placas = normalizarTextoOpcional(
		input.Placas,
		true,
	)

	input.Marca = normalizarTextoOpcional(
		input.Marca,
		false,
	)

	input.Modelo = normalizarTextoOpcional(
		input.Modelo,
		false,
	)

	if input.Codigo == "" {
		return Unidad{}, fmt.Errorf(
			"%w: el código es obligatorio",
			ErrDatosInvalidos,
		)
	}

	if utf8.RuneCountInString(input.Codigo) > 30 {
		return Unidad{}, fmt.Errorf(
			"%w: el código no puede exceder 30 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.Placas != nil &&
		utf8.RuneCountInString(*input.Placas) > 20 {
		return Unidad{}, fmt.Errorf(
			"%w: las placas no pueden exceder 20 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.Marca != nil &&
		utf8.RuneCountInString(*input.Marca) > 80 {
		return Unidad{}, fmt.Errorf(
			"%w: la marca no puede exceder 80 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.Modelo != nil &&
		utf8.RuneCountInString(*input.Modelo) > 80 {
		return Unidad{}, fmt.Errorf(
			"%w: el modelo no puede exceder 80 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.Anio != nil &&
		(*input.Anio < 1950 || *input.Anio > 2100) {
		return Unidad{}, fmt.Errorf(
			"%w: el año debe estar entre 1950 y 2100",
			ErrDatosInvalidos,
		)
	}

	if input.CapacidadTotal < 2 {
		return Unidad{}, fmt.Errorf(
			"%w: la capacidad total debe ser al menos 2",
			ErrDatosInvalidos,
		)
	}

	if input.CapacidadPasajeros <= 0 {
		return Unidad{}, fmt.Errorf(
			"%w: la capacidad de pasajeros debe ser mayor que cero",
			ErrDatosInvalidos,
		)
	}

	if input.CapacidadPasajeros >= input.CapacidadTotal {
		return Unidad{}, fmt.Errorf(
			"%w: la capacidad de pasajeros debe ser menor que la capacidad total",
			ErrDatosInvalidos,
		)
	}

	return s.store.Create(ctx, input)
}

func (s *Service) List(
	ctx context.Context,
) ([]Unidad, error) {
	return s.store.List(ctx)
}

/*
	normalizarTextoOpcional limpia un valor que puede ser nulo.

Si el puntero es nil o el texto contiene solamente espacios,
devuelve nil.

Si convertirMayusculas es true, también convierte el texto
a mayúsculas. Esto se utiliza para código y placas.
*/
func normalizarTextoOpcional(
	valor *string,
	convertirMayusculas bool,
) *string {
	if valor == nil {
		return nil
	}

	texto := strings.TrimSpace(*valor)

	if texto == "" {
		return nil
	}

	if convertirMayusculas {
		texto = strings.ToUpper(texto)
	}

	return &texto
}
