package reservas

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrCorridaNoEncontrada indica que no existe una corrida
	// con el identificador solicitado.
	//
	// Es diferente de ErrCorridaNoDisponible, porque consultar
	// una lista de pasajeros no depende de que la corrida
	// todavía acepte nuevas reservas.
	ErrCorridaNoEncontrada = errors.New(
		"la corrida no existe",
	)
)

// ListaPasajerosStore define únicamente la operación de
// persistencia necesaria para consultar la lista operativa
// de pasajeros de una corrida.
//
// Se mantiene separado de Store para no aumentar las
// responsabilidades del servicio principal de reservas.
type ListaPasajerosStore interface {
	GetPassengerList(
		ctx context.Context,
		corridaID int64,
	) (ListaPasajerosCorrida, error)
}

// ListaPasajerosService contiene las reglas para consultar
// la lista operativa y financiera de una corrida.
type ListaPasajerosService struct {
	store ListaPasajerosStore
}

// NewListaPasajerosService construye el servicio utilizando
// una implementación de ListaPasajerosStore.
func NewListaPasajerosService(
	store ListaPasajerosStore,
) *ListaPasajerosService {
	return &ListaPasajerosService{
		store: store,
	}
}

// GetPassengerList obtiene las reservas activas asociadas a
// una corrida.
//
// El Repository será responsable de:
//
//   - comprobar que la corrida exista;
//   - excluir las reservas canceladas;
//   - calcular la ocupación;
//   - calcular los importes financieros;
//   - ordenar las reservas por punto de origen y pasajero.
func (s *ListaPasajerosService) GetPassengerList(
	ctx context.Context,
	corridaID int64,
) (ListaPasajerosCorrida, error) {
	if corridaID <= 0 {
		return ListaPasajerosCorrida{}, fmt.Errorf(
			"%w: corrida_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	lista, err := s.store.GetPassengerList(
		ctx,
		corridaID,
	)
	if err != nil {
		return ListaPasajerosCorrida{}, err
	}

	// Una colección vacía debe representarse como [] en JSON,
	// no como null. Esto simplifica el consumo desde React,
	// n8n y otros clientes.
	if lista.Reservas == nil {
		lista.Reservas = make(
			[]ReservaListaPasajeros,
			0,
		)
	}

	return lista, nil
}
