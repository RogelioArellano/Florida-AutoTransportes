package reservas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type listaPasajerosEnvioServiceFake struct {
	pendientes []CorridaPendienteListaPasajeros
	envio      EnvioListaPasajeros
	errLista   error
	errEnvio   error

	inputRecibido RegistrarEnvioListaPasajerosInput
	llamadasLista int
	llamadasEnvio int
}

func (f *listaPasajerosEnvioServiceFake) ListPendingPassengerLists(
	_ context.Context,
) ([]CorridaPendienteListaPasajeros, error) {
	f.llamadasLista++

	return f.pendientes, f.errLista
}

func (f *listaPasajerosEnvioServiceFake) RegisterPassengerListDelivery(
	_ context.Context,
	input RegistrarEnvioListaPasajerosInput,
) (EnvioListaPasajeros, error) {
	f.llamadasEnvio++
	f.inputRecibido = input

	return f.envio, f.errEnvio
}

func TestListaPasajerosEnvioHandlerListaPendientes(
	t *testing.T,
) {
	service := &listaPasajerosEnvioServiceFake{
		pendientes: []CorridaPendienteListaPasajeros{
			{
				CorridaID:    4,
				CorridaFolio: "CORRIDA-004",
				ChoferID:     1,
			},
		},
	}

	handler :=
		NewListaPasajerosEnvioHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/corridas/listas-pasajeros/pendientes",
		nil,
	)

	response := httptest.NewRecorder()

	handler.HandlePending(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d; se esperaba %d; body=%s",
			response.Code,
			http.StatusOK,
			response.Body.String(),
		)
	}

	if service.llamadasLista != 1 {
		t.Fatalf(
			"llamadas al service = %d; se esperaba 1",
			service.llamadasLista,
		)
	}

	if !strings.Contains(
		response.Body.String(),
		`"corrida_id":4`,
	) {
		t.Fatalf(
			"respuesta inesperada: %s",
			response.Body.String(),
		)
	}
}

func TestListaPasajerosEnvioHandlerRegistraEnvio(
	t *testing.T,
) {
	service := &listaPasajerosEnvioServiceFake{
		envio: EnvioListaPasajeros{
			CorridaID:    4,
			CorridaFolio: "CORRIDA-004",
			ChoferID:     1,
		},
	}

	handler :=
		NewListaPasajerosEnvioHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/corridas/listas-pasajeros/envios",
		strings.NewReader(`{
			"corrida_id": 4,
			"chofer_id": 1
		}`),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	response := httptest.NewRecorder()

	handler.HandleDeliveries(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d; se esperaba %d; body=%s",
			response.Code,
			http.StatusOK,
			response.Body.String(),
		)
	}

	if service.llamadasEnvio != 1 {
		t.Fatalf(
			"llamadas al service = %d; se esperaba 1",
			service.llamadasEnvio,
		)
	}

	if service.inputRecibido.CorridaID != 4 ||
		service.inputRecibido.ChoferID != 1 {
		t.Fatalf(
			"input inesperado: %+v",
			service.inputRecibido,
		)
	}
}

func TestListaPasajerosEnvioHandlerRechazaJSONInvalido(
	t *testing.T,
) {
	service := &listaPasajerosEnvioServiceFake{}

	handler :=
		NewListaPasajerosEnvioHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/corridas/listas-pasajeros/envios",
		strings.NewReader(`{"corrida_id":`),
	)

	response := httptest.NewRecorder()

	handler.HandleDeliveries(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d; se esperaba %d",
			response.Code,
			http.StatusBadRequest,
		)
	}

	if service.llamadasEnvio != 0 {
		t.Fatalf(
			"llamadas al service = %d; se esperaba 0",
			service.llamadasEnvio,
		)
	}
}

func TestListaPasajerosEnvioHandlerMapeaConflicto(
	t *testing.T,
) {
	service := &listaPasajerosEnvioServiceFake{
		errEnvio: ErrChoferListaPasajerosCambio,
	}

	handler :=
		NewListaPasajerosEnvioHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/corridas/listas-pasajeros/envios",
		strings.NewReader(`{
			"corrida_id": 4,
			"chofer_id": 1
		}`),
	)

	response := httptest.NewRecorder()

	handler.HandleDeliveries(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf(
			"status = %d; se esperaba %d; body=%s",
			response.Code,
			http.StatusConflict,
			response.Body.String(),
		)
	}
}

func TestListaPasajerosEnvioHandlerRechazaMetodos(
	t *testing.T,
) {
	service := &listaPasajerosEnvioServiceFake{}

	handler :=
		NewListaPasajerosEnvioHandler(service)

	t.Run(
		"pendientes",
		func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/corridas/listas-pasajeros/pendientes",
				nil,
			)

			response := httptest.NewRecorder()

			handler.HandlePending(
				response,
				request,
			)

			if response.Code !=
				http.StatusMethodNotAllowed {
				t.Fatalf(
					"status = %d; se esperaba %d",
					response.Code,
					http.StatusMethodNotAllowed,
				)
			}

			if response.Header().Get("Allow") != "GET" {
				t.Fatalf(
					"Allow = %q; se esperaba GET",
					response.Header().Get("Allow"),
				)
			}
		},
	)

	t.Run(
		"envíos",
		func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				"/api/corridas/listas-pasajeros/envios",
				nil,
			)

			response := httptest.NewRecorder()

			handler.HandleDeliveries(
				response,
				request,
			)

			if response.Code !=
				http.StatusMethodNotAllowed {
				t.Fatalf(
					"status = %d; se esperaba %d",
					response.Code,
					http.StatusMethodNotAllowed,
				)
			}

			if response.Header().Get("Allow") != "POST" {
				t.Fatalf(
					"Allow = %q; se esperaba POST",
					response.Header().Get("Allow"),
				)
			}
		},
	)
}
