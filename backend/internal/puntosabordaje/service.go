package puntosabordaje

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrPuntoDuplicado = errors.New(
		"el punto de abordaje ya está registrado en esa localidad",
	)

	ErrLocalidadNoExiste = errors.New(
		"la localidad seleccionada no existe",
	)
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type Store interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (PuntoAbordaje, error)

	List(
		ctx context.Context,
	) ([]PuntoAbordaje, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
	}
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (PuntoAbordaje, error) {
	input.Nombre = strings.TrimSpace(input.Nombre)
	input.Referencia = strings.TrimSpace(
		input.Referencia,
	)

	if input.LocalidadID <= 0 {
		return PuntoAbordaje{}, &ValidationError{
			Field:   "localidad_id",
			Message: "debe seleccionar una localidad",
		}
	}

	if input.Nombre == "" {
		return PuntoAbordaje{}, &ValidationError{
			Field:   "nombre",
			Message: "es obligatorio",
		}
	}

	if utf8.RuneCountInString(input.Nombre) > 150 {
		return PuntoAbordaje{}, &ValidationError{
			Field:   "nombre",
			Message: "no puede superar 150 caracteres",
		}
	}

	if utf8.RuneCountInString(
		input.Referencia,
	) > 500 {
		return PuntoAbordaje{}, &ValidationError{
			Field: "referencia",
			Message: "no puede superar " +
				"500 caracteres",
		}
	}

	hasLatitud := input.Latitud != nil
	hasLongitud := input.Longitud != nil

	if hasLatitud != hasLongitud {
		return PuntoAbordaje{}, &ValidationError{
			Field: "coordenadas",
			Message: "latitud y longitud deben " +
				"capturarse juntas",
		}
	}

	if input.Latitud != nil &&
		(*input.Latitud < -90 ||
			*input.Latitud > 90) {
		return PuntoAbordaje{}, &ValidationError{
			Field: "latitud",
			Message: "debe encontrarse entre " +
				"-90 y 90",
		}
	}

	if input.Longitud != nil &&
		(*input.Longitud < -180 ||
			*input.Longitud > 180) {
		return PuntoAbordaje{}, &ValidationError{
			Field: "longitud",
			Message: "debe encontrarse entre " +
				"-180 y 180",
		}
	}

	return s.store.Create(ctx, input)
}

func (s *Service) List(
	ctx context.Context,
) ([]PuntoAbordaje, error) {
	return s.store.List(ctx)
}
