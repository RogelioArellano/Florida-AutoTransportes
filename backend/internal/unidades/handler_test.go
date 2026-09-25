package unidades

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeUnidadService sustituye al Service real.
//
// En lugar de ejecutar reglas de negocio o acceder a PostgreSQL,
// ejecutará las funciones que configuremos en cada prueba.
type fakeUnidadService struct {
	createFn func(
		ctx context.Context,
		input CreateInput,
	) (Unidad, error)

	listFn func(
		ctx context.Context,
	) ([]Unidad, error)
}

// Esta comprobación garantiza que el servicio simulado
// implementa la interfaz requerida por el Handler.
var _ unidadService = (*fakeUnidadService)(nil)

func (f *fakeUnidadService) Create(
	ctx context.Context,
	input CreateInput,
) (Unidad, error) {
	if f.createFn == nil {
		return Unidad{}, errors.New(
			"fakeUnidadService.Create no fue configurado",
		)
	}

	return f.createFn(ctx, input)
}

func (f *fakeUnidadService) List(
	ctx context.Context,
) ([]Unidad, error) {
	if f.listFn == nil {
		return nil, errors.New(
			"fakeUnidadService.List no fue configurado",
		)
	}

	return f.listFn(ctx)
}

func TestHandlerList(
	t *testing.T,
) {
	placas := "ABC-123"
	marca := "Nissan"
	modelo := "Urvan"
	anio := 2024

	service := &fakeUnidadService{
		listFn: func(
			ctx context.Context,
		) ([]Unidad, error) {
			return []Unidad{
				{
					ID:                 1,
					Codigo:             "UNIDAD-01",
					Placas:             &placas,
					Marca:              &marca,
					Modelo:             &modelo,
					Anio:               &anio,
					CapacidadTotal:     17,
					CapacidadPasajeros: 16,
					Activa:             true,
				},
			}, nil
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/unidades",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	contentType := recorder.Header().Get(
		"Content-Type",
	)

	if !strings.Contains(
		contentType,
		"application/json",
	) {
		t.Errorf(
			"se esperaba Content-Type JSON, se obtuvo %q",
			contentType,
		)
	}

	var respuesta struct {
		Data []Unidad `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if len(respuesta.Data) != 1 {
		t.Fatalf(
			"se esperaba una unidad, se obtuvieron %d",
			len(respuesta.Data),
		)
	}

	unidad := respuesta.Data[0]

	if unidad.Codigo != "UNIDAD-01" {
		t.Errorf(
			"se esperaba UNIDAD-01, se obtuvo %q",
			unidad.Codigo,
		)
	}

	if unidad.Placas == nil ||
		*unidad.Placas != "ABC-123" {
		t.Errorf(
			"se esperaban las placas ABC-123",
		)
	}

	if unidad.CapacidadPasajeros != 16 {
		t.Errorf(
			"se esperaban 16 pasajeros, se obtuvieron %d",
			unidad.CapacidadPasajeros,
		)
	}
}

func TestHandlerCreate(
	t *testing.T,
) {
	var inputRecibido CreateInput

	service := &fakeUnidadService{
		createFn: func(
			ctx context.Context,
			input CreateInput,
		) (Unidad, error) {
			inputRecibido = input

			return Unidad{
				ID:                 25,
				Codigo:             input.Codigo,
				Placas:             input.Placas,
				Marca:              input.Marca,
				Modelo:             input.Modelo,
				Anio:               input.Anio,
				CapacidadTotal:     input.CapacidadTotal,
				CapacidadPasajeros: input.CapacidadPasajeros,
				Activa:             true,
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"codigo": "UNIDAD-01",
		"placas": "ABC-123",
		"marca": "Nissan",
		"modelo": "Urvan",
		"anio": 2024,
		"capacidad_total": 17,
		"capacidad_pasajeros": 16
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/unidades",
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusCreated,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if inputRecibido.Codigo != "UNIDAD-01" {
		t.Errorf(
			"el Handler recibió el código %q",
			inputRecibido.Codigo,
		)
	}

	if inputRecibido.Placas == nil ||
		*inputRecibido.Placas != "ABC-123" {
		t.Errorf(
			"el Handler no recibió correctamente las placas",
		)
	}

	if inputRecibido.Anio == nil ||
		*inputRecibido.Anio != 2024 {
		t.Errorf(
			"el Handler no recibió correctamente el año",
		)
	}

	if inputRecibido.CapacidadTotal != 17 {
		t.Errorf(
			"se esperaba capacidad total 17, se obtuvo %d",
			inputRecibido.CapacidadTotal,
		)
	}

	if inputRecibido.CapacidadPasajeros != 16 {
		t.Errorf(
			"se esperaban 16 pasajeros, se obtuvieron %d",
			inputRecibido.CapacidadPasajeros,
		)
	}

	var respuesta struct {
		Data Unidad `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if respuesta.Data.ID != 25 {
		t.Errorf(
			"se esperaba la unidad 25, se obtuvo %d",
			respuesta.Data.ID,
		)
	}

	if !respuesta.Data.Activa {
		t.Error(
			"se esperaba que la unidad estuviera activa",
		)
	}
}

func TestHandlerCreateAceptaCamposOpcionalesNulos(
	t *testing.T,
) {
	service := &fakeUnidadService{
		createFn: func(
			ctx context.Context,
			input CreateInput,
		) (Unidad, error) {
			if input.Placas != nil {
				t.Error(
					"se esperaba que placas fuera nil",
				)
			}

			if input.Marca != nil {
				t.Error(
					"se esperaba que marca fuera nil",
				)
			}

			if input.Modelo != nil {
				t.Error(
					"se esperaba que modelo fuera nil",
				)
			}

			if input.Anio != nil {
				t.Error(
					"se esperaba que año fuera nil",
				)
			}

			return Unidad{
				ID:                 1,
				Codigo:             input.Codigo,
				CapacidadTotal:     input.CapacidadTotal,
				CapacidadPasajeros: input.CapacidadPasajeros,
				Activa:             true,
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"codigo": "UNIDAD-02",
		"placas": null,
		"marca": null,
		"modelo": null,
		"anio": null,
		"capacidad_total": 20,
		"capacidad_pasajeros": 18
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/unidades",
		strings.NewReader(body),
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusCreated,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestHandlerCreateRechazaJSONInvalido(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		body   string
	}{
		{
			nombre: "JSON incompleto",
			body:   `{"codigo":`,
		},
		{
			nombre: "campo desconocido",
			body: `{
				"codigo": "UNIDAD-01",
				"capacidad_total": 17,
				"capacidad_pasajeros": 16,
				"campo_inexistente": true
			}`,
		},
		{
			nombre: "dos objetos JSON",
			body: `{
				"codigo": "UNIDAD-01",
				"capacidad_total": 17,
				"capacidad_pasajeros": 16
			}
			{
				"codigo": "UNIDAD-02"
			}`,
		},
		{
			nombre: "tipo de dato incorrecto",
			body: `{
				"codigo": "UNIDAD-01",
				"capacidad_total": "diecisiete",
				"capacidad_pasajeros": 16
			}`,
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeUnidadService{
					createFn: func(
						ctx context.Context,
						input CreateInput,
					) (Unidad, error) {
						t.Fatal(
							"el Service no debe ejecutarse con JSON inválido",
						)

						return Unidad{}, nil
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/unidades",
					strings.NewReader(prueba.body),
				)

				recorder := httptest.NewRecorder()

				handler.Handle(
					recorder,
					request,
				)

				if recorder.Code != http.StatusBadRequest {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d",
						http.StatusBadRequest,
						recorder.Code,
					)
				}

				var respuesta struct {
					Error string `json:"error"`
				}

				if err := json.NewDecoder(
					recorder.Body,
				).Decode(&respuesta); err != nil {
					t.Fatalf(
						"no se pudo decodificar el error: %v",
						err,
					)
				}

				if respuesta.Error == "" {
					t.Error(
						"se esperaba un mensaje de error",
					)
				}
			},
		)
	}
}

func TestHandlerCreateTraduceErroresServicio(
	t *testing.T,
) {
	pruebas := []struct {
		nombre         string
		errorServicio  error
		statusEsperado int
	}{
		{
			nombre: "datos inválidos",
			errorServicio: fmt.Errorf(
				"%w: capacidad incorrecta",
				ErrDatosInvalidos,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre: "código duplicado",
			errorServicio: fmt.Errorf(
				"%w: UNIDAD-01",
				ErrCodigoDuplicado,
			),
			statusEsperado: http.StatusConflict,
		},
		{
			nombre: "placas duplicadas",
			errorServicio: fmt.Errorf(
				"%w: ABC-123",
				ErrPlacasDuplicadas,
			),
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "tiempo agotado",
			errorServicio:  context.DeadlineExceeded,
			statusEsperado: http.StatusGatewayTimeout,
		},
		{
			nombre: "error interno",
			errorServicio: errors.New(
				"PostgreSQL no disponible",
			),
			statusEsperado: http.StatusInternalServerError,
		},
	}

	body := `{
		"codigo": "UNIDAD-01",
		"capacidad_total": 17,
		"capacidad_pasajeros": 16
	}`

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeUnidadService{
					createFn: func(
						ctx context.Context,
						input CreateInput,
					) (Unidad, error) {
						return Unidad{}, prueba.errorServicio
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/unidades",
					strings.NewReader(body),
				)

				recorder := httptest.NewRecorder()

				handler.Handle(
					recorder,
					request,
				)

				if recorder.Code != prueba.statusEsperado {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						prueba.statusEsperado,
						recorder.Code,
						recorder.Body.String(),
					)
				}

				var respuesta struct {
					Error string `json:"error"`
				}

				if err := json.NewDecoder(
					recorder.Body,
				).Decode(&respuesta); err != nil {
					t.Fatalf(
						"no se pudo decodificar el error: %v",
						err,
					)
				}

				if respuesta.Error == "" {
					t.Error(
						"se esperaba un mensaje de error",
					)
				}
			},
		)
	}
}

func TestHandlerListTraduceErrorServicio(
	t *testing.T,
) {
	service := &fakeUnidadService{
		listFn: func(
			ctx context.Context,
		) ([]Unidad, error) {
			return nil, errors.New(
				"error consultando PostgreSQL",
			)
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/unidades",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusInternalServerError,
			recorder.Code,
		)
	}
}

func TestHandlerRechazaMetodoNoPermitido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeUnidadService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/unidades",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}

	if allow := recorder.Header().Get("Allow"); allow != "GET, POST" {
		t.Errorf(
			"se esperaba Allow GET, POST, se obtuvo %q",
			allow,
		)
	}
}
