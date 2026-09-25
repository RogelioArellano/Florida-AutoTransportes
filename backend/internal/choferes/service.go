package choferes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Error especifico de la creacion de un chofer
var ErrDatosInvalidos = errors.New(
	"datos de chofer invalidos",
)

type Store interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Chofer, error)

	List(
		ctx context.Context,
	) ([]Chofer, error)
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

// Create normaliza y valida los datos antes de solicitar
// al repositorio que registre el chofer.
func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Chofer, error) {
	input.NombreCompleto = strings.TrimSpace(
		input.NombreCompleto,
	)

	input.Telefono = strings.TrimSpace(
		input.Telefono,
	)

	input.LicenciaNumero = normalizarTextoOpcional(
		input.LicenciaNumero,
	)

	input.LicenciaVigencia = normalizarTextoOpcional(
		input.LicenciaVigencia,
	)

	if input.NombreCompleto == "" {
		return Chofer{}, fmt.Errorf(
			"%w: el nombre completo es obligatorio",
			ErrDatosInvalidos,
		)
	}

	if utf8.RuneCountInString(
		input.NombreCompleto,
	) > 150 {
		return Chofer{}, fmt.Errorf(
			"%w: el nombre completo no puede exceder 150 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.Telefono == "" {
		return Chofer{}, fmt.Errorf(
			"%w: el teléfono es obligatorio",
			ErrDatosInvalidos,
		)
	}

	if utf8.RuneCountInString(input.Telefono) > 20 {
		return Chofer{}, fmt.Errorf(
			"%w: el teléfono no puede exceder 20 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.LicenciaNumero != nil &&
		utf8.RuneCountInString(
			*input.LicenciaNumero,
		) > 50 {
		return Chofer{}, fmt.Errorf(
			"%w: el número de licencia no puede exceder 50 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.LicenciaVigencia != nil {
		if input.LicenciaNumero == nil {
			return Chofer{}, fmt.Errorf(
				"%w: no se puede registrar la vigencia sin el número de licencia",
				ErrDatosInvalidos,
			)
		}

		if !esFechaValida(
			*input.LicenciaVigencia,
		) {
			return Chofer{}, fmt.Errorf(
				"%w: la vigencia debe utilizar el formato AAAA-MM-DD",
				ErrDatosInvalidos,
			)
		}
	}

	return s.store.Create(ctx, input)
}

func (s *Service) List(
	ctx context.Context,
) ([]Chofer, error) {
	return s.store.List(ctx)
}

// normalizarTextoOpcional elimina los espacios al principio
// y al final.
//
// Un valor nulo o vacío se transforma en nil.
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

// esFechaValida comprueba que la fecha exista realmente
// y utilice exactamente el formato AAAA-MM-DD.
func esFechaValida(
	fecha string,
) bool {
	const formatoFecha = "2006-01-02"

	_, err := time.Parse(
		formatoFecha,
		fecha,
	)

	return err == nil
}
