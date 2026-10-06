package reservas

import (
	"context"
	"errors"
	"testing"
)

type listaPasajerosStoreFake struct {
	resultado ListaPasajerosCorrida
	err       error

	corridaIDRecibido int64
	llamadas          int
}

func (f *listaPasajerosStoreFake) GetPassengerList(
	_ context.Context,
	corridaID int64,
) (ListaPasajerosCorrida, error) {
	f.llamadas++
	f.corridaIDRecibido = corridaID

	return f.resultado, f.err
}

func TestListaPasajerosServiceObtieneLista(
	t *testing.T,
) {
	store := &listaPasajerosStoreFake{
		resultado: ListaPasajerosCorrida{
			CorridaID:    4,
			CorridaFolio: "CORRIDA-004",
			Reservas: []ReservaListaPasajeros{
				{
					ReservaID:         6,
					ReservaFolio:      "RES-00000006",
					CantidadPasajeros: 1,
				},
			},
		},
	}

	service := NewListaPasajerosService(store)

	resultado, err := service.GetPassengerList(
		context.Background(),
		4,
	)
	if err != nil {
		t.Fatalf(
			"GetPassengerList devolvió error: %v",
			err,
		)
	}

	if store.llamadas != 1 {
		t.Fatalf(
			"llamadas al store = %d; se esperaba 1",
			store.llamadas,
		)
	}

	if store.corridaIDRecibido != 4 {
		t.Fatalf(
			"corridaID recibido = %d; se esperaba 4",
			store.corridaIDRecibido,
		)
	}

	if resultado.CorridaID != 4 {
		t.Fatalf(
			"CorridaID = %d; se esperaba 4",
			resultado.CorridaID,
		)
	}

	if len(resultado.Reservas) != 1 {
		t.Fatalf(
			"reservas = %d; se esperaba 1",
			len(resultado.Reservas),
		)
	}
}

func TestListaPasajerosServiceNormalizaReservasVacias(
	t *testing.T,
) {
	store := &listaPasajerosStoreFake{
		resultado: ListaPasajerosCorrida{
			CorridaID: 4,
			Reservas:  nil,
		},
	}

	service := NewListaPasajerosService(store)

	resultado, err := service.GetPassengerList(
		context.Background(),
		4,
	)
	if err != nil {
		t.Fatalf(
			"GetPassengerList devolvió error: %v",
			err,
		)
	}

	if resultado.Reservas == nil {
		t.Fatal(
			"Reservas es nil; se esperaba un arreglo vacío",
		)
	}

	if len(resultado.Reservas) != 0 {
		t.Fatalf(
			"reservas = %d; se esperaba 0",
			len(resultado.Reservas),
		)
	}
}

func TestListaPasajerosServiceRechazaIDInvalido(
	t *testing.T,
) {
	casos := []int64{
		0,
		-1,
	}

	for _, corridaID := range casos {
		t.Run(
			"corrida_id_invalido",
			func(t *testing.T) {
				store :=
					&listaPasajerosStoreFake{}

				service :=
					NewListaPasajerosService(store)

				_, err := service.GetPassengerList(
					context.Background(),
					corridaID,
				)

				if !errors.Is(
					err,
					ErrDatosInvalidos,
				) {
					t.Fatalf(
						"error = %v; se esperaba ErrDatosInvalidos",
						err,
					)
				}

				if store.llamadas != 0 {
					t.Fatalf(
						"el store recibió %d llamadas; se esperaba 0",
						store.llamadas,
					)
				}
			},
		)
	}
}

func TestListaPasajerosServicePropagaError(
	t *testing.T,
) {
	store := &listaPasajerosStoreFake{
		err: ErrCorridaNoEncontrada,
	}

	service := NewListaPasajerosService(store)

	_, err := service.GetPassengerList(
		context.Background(),
		999999,
	)

	if !errors.Is(
		err,
		ErrCorridaNoEncontrada,
	) {
		t.Fatalf(
			"error = %v; se esperaba ErrCorridaNoEncontrada",
			err,
		)
	}
}
