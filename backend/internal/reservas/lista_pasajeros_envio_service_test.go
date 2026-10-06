package reservas

import (
	"context"
	"errors"
	"testing"
	"time"
)

type listaPasajerosEnvioStoreFake struct {
	pendientes []CorridaPendienteListaPasajeros
	envio      EnvioListaPasajeros
	errLista   error
	errEnvio   error

	politicaListaRecibida PoliticaEnvioListaPasajerosParams
	politicaEnvioRecibida PoliticaEnvioListaPasajerosParams
	inputRecibido         RegistrarEnvioListaPasajerosInput

	llamadasLista int
	llamadasEnvio int
}

func (f *listaPasajerosEnvioStoreFake) ListPendingPassengerLists(
	_ context.Context,
	politica PoliticaEnvioListaPasajerosParams,
) ([]CorridaPendienteListaPasajeros, error) {
	f.llamadasLista++
	f.politicaListaRecibida = politica

	return f.pendientes, f.errLista
}

func (f *listaPasajerosEnvioStoreFake) RegisterPassengerListDelivery(
	_ context.Context,
	input RegistrarEnvioListaPasajerosInput,
	politica PoliticaEnvioListaPasajerosParams,
) (EnvioListaPasajeros, error) {
	f.llamadasEnvio++
	f.inputRecibido = input
	f.politicaEnvioRecibida = politica

	return f.envio, f.errEnvio
}

func TestListaPasajerosEnvioServiceListaPendientes(
	t *testing.T,
) {
	store := &listaPasajerosEnvioStoreFake{
		pendientes: []CorridaPendienteListaPasajeros{
			{
				CorridaID:    4,
				CorridaFolio: "CORRIDA-004",
				ChoferID:     1,
			},
		},
	}

	service := NewListaPasajerosEnvioService(store)

	resultado, err :=
		service.ListPendingPassengerLists(
			context.Background(),
		)
	if err != nil {
		t.Fatalf(
			"ListPendingPassengerLists devolvió error: %v",
			err,
		)
	}

	if len(resultado) != 1 {
		t.Fatalf(
			"pendientes = %d; se esperaba 1",
			len(resultado),
		)
	}

	if store.llamadasLista != 1 {
		t.Fatalf(
			"llamadas al store = %d; se esperaba 1",
			store.llamadasLista,
		)
	}

	if store.politicaListaRecibida.MinutosAnticipacion != 60 {
		t.Fatalf(
			"anticipación = %d; se esperaba 60",
			store.politicaListaRecibida.MinutosAnticipacion,
		)
	}
}

func TestListaPasajerosEnvioServiceNormalizaListaVacia(
	t *testing.T,
) {
	store := &listaPasajerosEnvioStoreFake{
		pendientes: nil,
	}

	service := NewListaPasajerosEnvioService(store)

	resultado, err :=
		service.ListPendingPassengerLists(
			context.Background(),
		)
	if err != nil {
		t.Fatalf(
			"ListPendingPassengerLists devolvió error: %v",
			err,
		)
	}

	if resultado == nil {
		t.Fatal(
			"resultado es nil; se esperaba un arreglo vacío",
		)
	}

	if len(resultado) != 0 {
		t.Fatalf(
			"pendientes = %d; se esperaba 0",
			len(resultado),
		)
	}
}

func TestListaPasajerosEnvioServiceRegistraEnvio(
	t *testing.T,
) {
	enviadaEn := time.Date(
		2026,
		time.October,
		5,
		18,
		30,
		0,
		0,
		time.FixedZone("CST", -6*60*60),
	)

	store := &listaPasajerosEnvioStoreFake{
		envio: EnvioListaPasajeros{
			CorridaID:    4,
			CorridaFolio: "CORRIDA-004",
			ChoferID:     1,
			EnviadaEn:    enviadaEn,
		},
	}

	service := NewListaPasajerosEnvioService(store)

	input := RegistrarEnvioListaPasajerosInput{
		CorridaID: 4,
		ChoferID:  1,
	}

	resultado, err :=
		service.RegisterPassengerListDelivery(
			context.Background(),
			input,
		)
	if err != nil {
		t.Fatalf(
			"RegisterPassengerListDelivery devolvió error: %v",
			err,
		)
	}

	if store.llamadasEnvio != 1 {
		t.Fatalf(
			"llamadas al store = %d; se esperaba 1",
			store.llamadasEnvio,
		)
	}

	if store.inputRecibido != input {
		t.Fatalf(
			"input recibido = %+v; se esperaba %+v",
			store.inputRecibido,
			input,
		)
	}

	if store.politicaEnvioRecibida.MinutosAnticipacion != 60 {
		t.Fatalf(
			"anticipación = %d; se esperaba 60",
			store.politicaEnvioRecibida.MinutosAnticipacion,
		)
	}

	if resultado.CorridaID != 4 {
		t.Fatalf(
			"CorridaID = %d; se esperaba 4",
			resultado.CorridaID,
		)
	}
}

func TestListaPasajerosEnvioServiceRechazaIDsInvalidos(
	t *testing.T,
) {
	casos := []struct {
		nombre string
		input  RegistrarEnvioListaPasajerosInput
	}{
		{
			nombre: "corrida inválida",
			input: RegistrarEnvioListaPasajerosInput{
				CorridaID: 0,
				ChoferID:  1,
			},
		},
		{
			nombre: "chofer inválido",
			input: RegistrarEnvioListaPasajerosInput{
				CorridaID: 4,
				ChoferID:  0,
			},
		},
	}

	for _, caso := range casos {
		t.Run(
			caso.nombre,
			func(t *testing.T) {
				store :=
					&listaPasajerosEnvioStoreFake{}

				service :=
					NewListaPasajerosEnvioService(
						store,
					)

				_, err :=
					service.RegisterPassengerListDelivery(
						context.Background(),
						caso.input,
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

				if store.llamadasEnvio != 0 {
					t.Fatalf(
						"llamadas al store = %d; se esperaba 0",
						store.llamadasEnvio,
					)
				}
			},
		)
	}
}

func TestListaPasajerosEnvioServicePropagaError(
	t *testing.T,
) {
	store := &listaPasajerosEnvioStoreFake{
		errEnvio: ErrChoferListaPasajerosCambio,
	}

	service := NewListaPasajerosEnvioService(store)

	_, err :=
		service.RegisterPassengerListDelivery(
			context.Background(),
			RegistrarEnvioListaPasajerosInput{
				CorridaID: 4,
				ChoferID:  1,
			},
		)

	if !errors.Is(
		err,
		ErrChoferListaPasajerosCambio,
	) {
		t.Fatalf(
			"error = %v; se esperaba ErrChoferListaPasajerosCambio",
			err,
		)
	}
}
