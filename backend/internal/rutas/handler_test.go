package rutas

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

// fakeRouteService sustituye al Service Real
//
// El handler no sabe si está recibiendo el servicio real
// o este objeto simulado porque ambos cumplen routeService
type fakeRouteService struct {
	createFn func(
		ctx context.Context,
		input CreateInput,
	) (Ruta, error)

	listFn func(
		ctx context.Context,
	) ([]Ruta, error)
}

func (f *fakeRouteService) Create(
	ctx context.Context,
	input CreateInput,
) (Ruta, error) {
	if f.createFn == nil {
		return Ruta{}, errors.New(
			"fakeRouteService.Create no fue configurado",
		)
	}

	return f.createFn(ctx, input)
}

func (f *fakeRouteService) List(
	ctx context.Context,
) ([]Ruta, error) {
	if f.listFn == nil {
		return nil, errors.New(
			"fakeRouteService.List no fue configurado",
		)
	}

	return f.listFn(ctx)
}

func TestHandlerList(t *testing.T) {
	service := &fakeRouteService{
		listFn: func(
			ctx context.Context,
		) ([]Ruta, error) {
			return []Ruta{
				{
					ID:     1,
					Codigo: "MOR-CDMX",
					Nombre: "Morelia a Ciudad de México",
					Activa: true,
					Paradas: []RutaParada{
						{
							ID:              1,
							PuntoAbordajeID: 10,
							PuntoNombre:     "Morelia Centro",
							LocalidadNombre: "Morelia",
							Estado:          "Michoacán",
							Orden:           1,
							PermiteSubir:    true,
							PermiteBajar:    false,
							EsObligatoria:   true,
						},
					},
				},
			}, nil
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/rutas",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"Se esperaba status %d, se obtuvo %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if contentType := recorder.Header().Get(
		"Content-Type",
	); !strings.Contains(
		contentType,
		"application/json",
	) {
		t.Errorf(
			"se esperaba Content-Type JSON, se obtuvo %q",
			contentType,
		)
	}

	var respuesta struct {
		Data []Ruta `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se puedo decodificar la respuesta: %v",
			err,
		)
	}

	if len(respuesta.Data) != 1 {
		t.Fatalf(
			"se esperaba una ruta, se obtuvieron %d",
			len(respuesta.Data),
		)
	}

	if respuesta.Data[0].Codigo != "MOR-CDMX" {
		t.Errorf(
			"se esperaba MOR-CDMX, se obtuvo %q",
			respuesta.Data[0].Codigo,
		)
	}

	if len(respuesta.Data[0].Paradas) != 1 {
		t.Errorf(
			"se esperaba una parada, se obtuvieron %d",
			len(respuesta.Data[0].Paradas),
		)
	}
}

func TestHandlerCreate(t *testing.T) {
	var inputRecibido CreateInput

	service := &fakeRouteService{
		createFn: func(
			ctx context.Context,
			input CreateInput,
		) (Ruta, error) {
			inputRecibido = input

			return Ruta{
				ID:      25,
				Codigo:  input.Codigo,
				Nombre:  input.Nombre,
				Activa:  true,
				Paradas: []RutaParada{},
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"codigo": "MOR-CDMX",
		"nombre": "Morelia a Ciudad de México",
		"paradas": [
			{
				"punto_abordaje_id": 1,
				"permite_subir": true,
				"permite_bajar": false,
				"es_obligatoria": true
			},
			{
				"punto_abordaje_id": 2,
				"permite_subir": false,
				"permite_bajar": true,
				"es_obligatoria": true
			}
		]
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/rutas",
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

	if inputRecibido.Codigo != "MOR-CDMX" {
		t.Errorf(
			"el Handler recibió el código %q",
			inputRecibido.Codigo,
		)
	}

	if len(inputRecibido.Paradas) != 2 {
		t.Fatalf(
			"se esperaban dos paradas, se obtuvieron %d",
			len(inputRecibido.Paradas),
		)
	}

	if inputRecibido.Paradas[0].PuntoAbordajeID != 1 {
		t.Errorf(
			"se esperaba el punto 1 en la primera parada",
		)
	}

	var respuesta struct {
		Data Ruta `json:"data"`
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
			"se esperaba la ruta 25, se obtuvo %d",
			respuesta.Data.ID,
		)
	}
}

func TestHandlerCreateRechazaJSONInvalido(
	t *testing.T,
) {
	casos := []struct {
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
				"codigo": "MOR-CDMX",
				"nombre": "Morelia a México",
				"paradas": [],
				"campo_inexistente": true
			}`,
		},
		{
			nombre: "dos objetos JSON",
			body: `{
				"codigo": "MOR-CDMX",
				"nombre": "Morelia a México",
				"paradas": []
			}
			{
				"codigo": "OTRA-RUTA"
			}`,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			service := &fakeRouteService{
				createFn: func(
					ctx context.Context,
					input CreateInput,
				) (Ruta, error) {
					t.Fatal(
						"el Service no debería ejecutarse con JSON inválido",
					)

					return Ruta{}, nil
				},
			}

			handler := NewHandler(service)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/rutas",
				strings.NewReader(caso.body),
			)

			recorder := httptest.NewRecorder()

			handler.Handle(recorder, request)

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
		})
	}
}

func TestHandlerCreateTraduceErroresServicio(
	t *testing.T,
) {
	casos := []struct {
		nombre         string
		errorServicio  error
		statusEsperado int
	}{
		{
			nombre: "datos inválidos",
			errorServicio: fmt.Errorf(
				"%w: código obligatorio",
				ErrDatosInvalidos,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre: "código duplicado",
			errorServicio: fmt.Errorf(
				"%w: MOR-CDMX",
				ErrCodigoDuplicado,
			),
			statusEsperado: http.StatusConflict,
		},
		{
			nombre: "punto inexistente",
			errorServicio: fmt.Errorf(
				"%w: punto 500",
				ErrPuntoNoExiste,
			),
			statusEsperado: http.StatusBadRequest,
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
		"codigo": "MOR-CDMX",
		"nombre": "Morelia a Ciudad de México",
		"paradas": [
			{
				"punto_abordaje_id": 1,
				"permite_subir": true,
				"permite_bajar": false,
				"es_obligatoria": true
			},
			{
				"punto_abordaje_id": 2,
				"permite_subir": false,
				"permite_bajar": true,
				"es_obligatoria": true
			}
		]
	}`

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			service := &fakeRouteService{
				createFn: func(
					ctx context.Context,
					input CreateInput,
				) (Ruta, error) {
					return Ruta{}, caso.errorServicio
				},
			}

			handler := NewHandler(service)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/rutas",
				strings.NewReader(body),
			)

			recorder := httptest.NewRecorder()

			handler.Handle(recorder, request)

			if recorder.Code != caso.statusEsperado {
				t.Fatalf(
					"se esperaba status %d, se obtuvo %d. Respuesta: %s",
					caso.statusEsperado,
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
		})
	}
}

func TestHandlerRechazaMetodoNoPermitido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeRouteService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/rutas",
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
