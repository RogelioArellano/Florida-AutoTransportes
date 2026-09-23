package rutas

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	// Estos errores podrán ser utilizados posteriormente
	// por el Handler para seleccionar el código HTTP adecuado.
	ErrDatosInvalidos  = errors.New("datos de ruta inválidos")
	ErrCodigoDuplicado = errors.New("el código de la ruta ya existe")
	ErrPuntoNoExiste   = errors.New("uno de los puntos de abordaje no existe")
)

// Store define las operaciones de almacenamiento requeridas
// por el servicio.
//
// El servicio no necesita saber si la implementación utiliza
// PostgreSQL, memoria o un objeto simulado para pruebas.
type Store interface {
	Create(
		ctx context.Context,
		input CreateInput,
	) (Ruta, error)

	List(ctx context.Context) ([]Ruta, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
	}
}

// Create valida las reglas de negocio antes de solicitar
// al repositorio que guarde la ruta.
func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Ruta, error) {
	input.Codigo = strings.ToUpper(
		strings.TrimSpace(input.Codigo),
	)

	input.Nombre = strings.TrimSpace(input.Nombre)

	if input.Codigo == "" {
		return Ruta{}, fmt.Errorf(
			"%w: el código es obligatorio",
			ErrDatosInvalidos,
		)
	}

	if len(input.Codigo) > 30 {
		return Ruta{}, fmt.Errorf(
			"%w: el código no puede exceder 30 caracteres",
			ErrDatosInvalidos,
		)
	}

	if input.Nombre == "" {
		return Ruta{}, fmt.Errorf(
			"%w: el nombre es obligatorio",
			ErrDatosInvalidos,
		)
	}

	if len(input.Nombre) > 150 {
		return Ruta{}, fmt.Errorf(
			"%w: el nombre no puede exceder 150 caracteres",
			ErrDatosInvalidos,
		)
	}

	if len(input.Paradas) < 2 {
		return Ruta{}, fmt.Errorf(
			"%w: una ruta necesita al menos dos paradas",
			ErrDatosInvalidos,
		)
	}

	puntosUtilizados := make(map[int64]struct{})

	for indice, parada := range input.Paradas {
		numeroParada := indice + 1

		if parada.PuntoAbordajeID <= 0 {
			return Ruta{}, fmt.Errorf(
				"%w: la parada %d no tiene un punto válido",
				ErrDatosInvalidos,
				numeroParada,
			)
		}

		if _, existe := puntosUtilizados[parada.PuntoAbordajeID]; existe {
			return Ruta{}, fmt.Errorf(
				"%w: el punto de abordaje de la parada %d está repetido",
				ErrDatosInvalidos,
				numeroParada,
			)
		}

		puntosUtilizados[parada.PuntoAbordajeID] = struct{}{}

		if !parada.PermiteSubir && !parada.PermiteBajar {
			return Ruta{}, fmt.Errorf(
				"%w: la parada %d debe permitir subir, bajar o ambas operaciones",
				ErrDatosInvalidos,
				numeroParada,
			)
		}
	}

	primeraParada := &input.Paradas[0]
	ultimaParada := &input.Paradas[len(input.Paradas)-1]

	if !primeraParada.PermiteSubir {
		return Ruta{}, fmt.Errorf(
			"%w: la primera parada debe permitir abordar",
			ErrDatosInvalidos,
		)
	}

	if !ultimaParada.PermiteBajar {
		return Ruta{}, fmt.Errorf(
			"%w: la última parada debe permitir descender",
			ErrDatosInvalidos,
		)
	}

	// El origen y el destino forman parte esencial del recorrido.
	// Por eso se marcan como obligatorios sin depender del valor
	// recibido desde la interfaz.
	primeraParada.EsObligatoria = true
	ultimaParada.EsObligatoria = true

	return s.store.Create(ctx, input)
}

func (s *Service) List(
	ctx context.Context,
) ([]Ruta, error) {
	return s.store.List(ctx)
}
