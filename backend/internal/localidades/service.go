package localidades

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var ErrLocalidadDuplicada = errors.New(
	"La localidad ya esta registrada en ese estado",
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
	) (Localidad, error)

	List(
		ctx context.Context,
	) ([]Localidad, error)
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
) (Localidad, error) {

	input.Nombre = strings.TrimSpace(input.Nombre)
	input.Estado = strings.TrimSpace(input.Estado)

	if input.Nombre == "" {
		return Localidad{}, &ValidationError{
			Field:   "nombre",
			Message: "es obligatorio",
		}
	}

	if utf8.RuneCountInString(input.Nombre) > 100 {
		return Localidad{}, &ValidationError{
			Field:   "nombre",
			Message: "no puede superar 100 caracteres",
		}
	}

	if input.Estado == "" {
		return Localidad{}, &ValidationError{
			Field:   "estado",
			Message: "es obligatorio",
		}
	}

	if utf8.RuneCountInString(input.Estado) > 100 {
		return Localidad{}, &ValidationError{
			Field:   "estado",
			Message: "no puede superar 100 caracteres",
		}
	}

	return s.store.Create(ctx, input)
}

// List retorna el acumulado de localidades
func (s *Service) List(
	ctx context.Context,
) ([]Localidad, error) {
	return s.store.List(ctx)
}
