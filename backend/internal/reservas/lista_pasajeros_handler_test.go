package reservas

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type listaPasajerosServiceFake struct {
	resultado ListaPasajerosCorrida
	err       error

	corridaIDRecibido int64
	llamadas          int
}

func (f *listaPasajerosServiceFake) GetPassengerList(
	_ context.Context,
	corridaID int64,
) (ListaPasajerosCorrida, error) {
	f.llamadas++
	f.corridaIDRecibido = corridaID

	return f.resultado, f.err
}

func TestListaPasajerosHandlerObtieneLista(
	t *testing.T,
) {
	service := &listaPasajerosServiceFake{
		resultado: ListaPasajerosCorrida{
			CorridaID:                 4,
			CorridaFolio:              "CORRIDA-004",
			CapacidadPasajeros:        16,
			OcupacionMaxima:           1,
			LugaresDisponiblesMinimos: 15,
			Reservas: []ReservaListaPasajeros{
				{
					ReservaID:         6,
					ReservaFolio:      "RES-00000006",
					PasajeroNombre:    "Pasajero prueba",
					CantidadPasajeros: 1,
				},
			},
		},
	}

	handler := NewListaPasajerosHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/corridas/4/pasajeros",
		nil,
	)

	request.SetPathValue(
		"corridaID",
		"4",
	)

	response := httptest.NewRecorder()

	handler.Handle(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d; se esperaba %d; body=%s",
			response.Code,
			http.StatusOK,
			response.Body.String(),
		)
	}

	if service.llamadas != 1 {
		t.Fatalf(
			"llamadas al service = %d; se esperaba 1",
			service.llamadas,
		)
	}

	if service.corridaIDRecibido != 4 {
		t.Fatalf(
			"corridaID recibido = %d; se esperaba 4",
			service.corridaIDRecibido,
		)
	}

	var body struct {
		Data ListaPasajerosCorrida `json:"data"`
	}

	if err := json.NewDecoder(
		response.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decodificar respuesta: %v",
			err,
		)
	}

	if body.Data.CorridaID != 4 {
		t.Fatalf(
			"corrida_id = %d; se esperaba 4",
			body.Data.CorridaID,
		)
	}

	if len(body.Data.Reservas) != 1 {
		t.Fatalf(
			"reservas = %d; se esperaba 1",
			len(body.Data.Reservas),
		)
	}
}

func TestListaPasajerosHandlerRechazaIDInvalido(
	t *testing.T,
) {
	service := &listaPasajerosServiceFake{}
	handler := NewListaPasajerosHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/corridas/invalida/pasajeros",
		nil,
	)

	request.SetPathValue(
		"corridaID",
		"invalida",
	)

	response := httptest.NewRecorder()

	handler.Handle(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d; se esperaba %d; body=%s",
			response.Code,
			http.StatusBadRequest,
			response.Body.String(),
		)
	}

	if service.llamadas != 0 {
		t.Fatalf(
			"el service recibió %d llamadas; se esperaba 0",
			service.llamadas,
		)
	}
}

func TestListaPasajerosHandlerMapeaNoEncontrada(
	t *testing.T,
) {
	service := &listaPasajerosServiceFake{
		err: ErrCorridaNoEncontrada,
	}

	handler := NewListaPasajerosHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/corridas/999999/pasajeros",
		nil,
	)

	request.SetPathValue(
		"corridaID",
		"999999",
	)

	response := httptest.NewRecorder()

	handler.Handle(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf(
			"status = %d; se esperaba %d; body=%s",
			response.Code,
			http.StatusNotFound,
			response.Body.String(),
		)
	}

	if !strings.Contains(
		response.Body.String(),
		"la corrida no existe",
	) {
		t.Fatalf(
			"respuesta inesperada: %s",
			response.Body.String(),
		)
	}
}

func TestListaPasajerosHandlerRechazaMetodo(
	t *testing.T,
) {
	service := &listaPasajerosServiceFake{}
	handler := NewListaPasajerosHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/corridas/4/pasajeros",
		nil,
	)

	request.SetPathValue(
		"corridaID",
		"4",
	)

	response := httptest.NewRecorder()

	handler.Handle(response, request)

	if response.Code != http.StatusMethodNotAllowed {
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

	if service.llamadas != 0 {
		t.Fatalf(
			"el service recibió %d llamadas; se esperaba 0",
			service.llamadas,
		)
	}
}
