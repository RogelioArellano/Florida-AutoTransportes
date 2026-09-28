package corridas

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

// fakeCorridaService sustituye al Service real.
//
// De esta forma probamos solamente las responsabilidades
// del Handler:
//   - leer la petición HTTP;
//   - decodificar el JSON;
//   - construir filtros;
//   - llamar al Service;
//   - producir la respuesta HTTP.
type fakeCorridaService struct {
	generateFn func(
		ctx context.Context,
		input GenerarInput,
	) (Corrida, error)

	listFn func(
		ctx context.Context,
		filter ListFilter,
	) ([]Corrida, error)
}

// Esta línea obliga al compilador a comprobar que nuestro
// fake implementa correctamente corridaService.
var _ corridaService = (*fakeCorridaService)(nil)

func (f *fakeCorridaService) Generate(
	ctx context.Context,
	input GenerarInput,
) (Corrida, error) {
	if f.generateFn == nil {
		return Corrida{}, errors.New(
			"fakeCorridaService.Generate no fue configurado",
		)
	}

	return f.generateFn(ctx, input)
}

func (f *fakeCorridaService) List(
	ctx context.Context,
	filter ListFilter,
) ([]Corrida, error) {
	if f.listFn == nil {
		return nil, errors.New(
			"fakeCorridaService.List no fue configurado",
		)
	}

	return f.listFn(ctx, filter)
}

func TestHandlerGenerate(t *testing.T) {
	var inputRecibido GenerarInput

	service := &fakeCorridaService{
		generateFn: func(
			ctx context.Context,
			input GenerarInput,
		) (Corrida, error) {
			inputRecibido = input

			return Corrida{
				ID:                 1,
				Folio:              "MOR-CDMX-0600-20260926",
				ProgramacionID:     &input.ProgramacionID,
				FechaServicio:      input.FechaServicio,
				CapacidadPasajeros: 16,
				Estado:             EstadoProgramada,
				ReservasAbiertas:   true,
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"programacion_id": 4,
		"fecha_servicio": "2026-09-26",
		"observaciones": "Primera corrida generada desde la API"
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/corridas",
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

	if inputRecibido.ProgramacionID != 4 {
		t.Errorf(
			"se recibió programacion_id = %d; se esperaba 4",
			inputRecibido.ProgramacionID,
		)
	}

	if inputRecibido.FechaServicio != "2026-09-26" {
		t.Errorf(
			"se recibió fecha_servicio = %q",
			inputRecibido.FechaServicio,
		)
	}

	if inputRecibido.Observaciones == nil {
		t.Fatal(
			"se esperaba recibir observaciones",
		)
	}

	if *inputRecibido.Observaciones !=
		"Primera corrida generada desde la API" {
		t.Errorf(
			"se recibieron observaciones incorrectas: %q",
			*inputRecibido.Observaciones,
		)
	}

	var respuesta struct {
		Data Corrida `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if respuesta.Data.ID != 1 {
		t.Errorf(
			"se esperaba la corrida 1, se obtuvo %d",
			respuesta.Data.ID,
		)
	}

	if respuesta.Data.Folio !=
		"MOR-CDMX-0600-20260926" {
		t.Errorf(
			"se obtuvo el folio %q",
			respuesta.Data.Folio,
		)
	}

	if respuesta.Data.Estado != EstadoProgramada {
		t.Errorf(
			"se esperaba estado PROGRAMADA, se obtuvo %q",
			respuesta.Data.Estado,
		)
	}

	if !respuesta.Data.ReservasAbiertas {
		t.Error(
			"se esperaba que las reservas estuvieran abiertas",
		)
	}
}

func TestHandlerGenerateAceptaObservacionesNulas(
	t *testing.T,
) {
	service := &fakeCorridaService{
		generateFn: func(
			ctx context.Context,
			input GenerarInput,
		) (Corrida, error) {
			if input.Observaciones != nil {
				t.Error(
					"se esperaba que observaciones fuera nil",
				)
			}

			return Corrida{
				ID:               1,
				Folio:            "MOR-CDMX-0600-20260927",
				FechaServicio:    input.FechaServicio,
				Estado:           EstadoProgramada,
				ReservasAbiertas: true,
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"programacion_id": 4,
		"fecha_servicio": "2026-09-27",
		"observaciones": null
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/corridas",
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

func TestHandlerListConFiltros(t *testing.T) {
	var filtroRecibido ListFilter

	service := &fakeCorridaService{
		listFn: func(
			ctx context.Context,
			filter ListFilter,
		) ([]Corrida, error) {
			filtroRecibido = filter

			return []Corrida{
				{
					ID:               1,
					Folio:            "MOR-CDMX-0600-20260926",
					FechaServicio:    "2026-09-26",
					Estado:           EstadoProgramada,
					ReservasAbiertas: true,
					Paradas: []CorridaParada{
						{
							ID:                  1,
							CorridaID:           1,
							PuntoNombre:         "Morelia Centro",
							Orden:               1,
							EsObligatoria:       true,
							IncluidaEnRecorrido: true,
						},
					},
				},
			}, nil
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/corridas?fecha_desde=%202026-09-26%20"+
			"&fecha_hasta=2026-09-30"+
			"&estado=%20PROGRAMADA%20",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusOK,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if filtroRecibido.FechaDesde == nil ||
		*filtroRecibido.FechaDesde != "2026-09-26" {
		t.Error(
			"fecha_desde no fue transferida correctamente",
		)
	}

	if filtroRecibido.FechaHasta == nil ||
		*filtroRecibido.FechaHasta != "2026-09-30" {
		t.Error(
			"fecha_hasta no fue transferida correctamente",
		)
	}

	if filtroRecibido.Estado == nil ||
		*filtroRecibido.Estado != EstadoProgramada {
		t.Error(
			"estado no fue transferido correctamente",
		)
	}

	var respuesta struct {
		Data []Corrida `json:"data"`
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
			"se esperaba una corrida, se obtuvieron %d",
			len(respuesta.Data),
		)
	}

	if respuesta.Data[0].Folio !=
		"MOR-CDMX-0600-20260926" {
		t.Errorf(
			"se obtuvo el folio %q",
			respuesta.Data[0].Folio,
		)
	}

	if len(respuesta.Data[0].Paradas) != 1 {
		t.Fatalf(
			"se esperaba una parada, se obtuvieron %d",
			len(respuesta.Data[0].Paradas),
		)
	}

	if respuesta.Data[0].Paradas[0].PuntoNombre !=
		"Morelia Centro" {
		t.Errorf(
			"se obtuvo la parada %q",
			respuesta.Data[0].Paradas[0].PuntoNombre,
		)
	}
}

func TestHandlerListSinFiltros(t *testing.T) {
	service := &fakeCorridaService{
		listFn: func(
			ctx context.Context,
			filter ListFilter,
		) ([]Corrida, error) {
			if filter.FechaDesde != nil {
				t.Error(
					"se esperaba FechaDesde nil",
				)
			}

			if filter.FechaHasta != nil {
				t.Error(
					"se esperaba FechaHasta nil",
				)
			}

			if filter.Estado != nil {
				t.Error(
					"se esperaba Estado nil",
				)
			}

			return []Corrida{}, nil
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/corridas",
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
}

func TestHandlerGenerateRechazaJSONInvalido(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		body   string
	}{
		{
			nombre: "JSON incompleto",
			body:   `{"programacion_id":`,
		},
		{
			nombre: "campo desconocido",
			body: `{
				"programacion_id": 4,
				"fecha_servicio": "2026-09-26",
				"campo_inexistente": true
			}`,
		},
		{
			nombre: "dos objetos JSON",
			body: `{
				"programacion_id": 4,
				"fecha_servicio": "2026-09-26"
			}
			{
				"programacion_id": 4
			}`,
		},
		{
			nombre: "programación con tipo incorrecto",
			body: `{
				"programacion_id": "cuatro",
				"fecha_servicio": "2026-09-26"
			}`,
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeCorridaService{
					generateFn: func(
						ctx context.Context,
						input GenerarInput,
					) (Corrida, error) {
						t.Fatal(
							"el Service no debe ejecutarse con JSON inválido",
						)

						return Corrida{}, nil
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/corridas",
					strings.NewReader(prueba.body),
				)

				recorder := httptest.NewRecorder()

				handler.Handle(
					recorder,
					request,
				)

				if recorder.Code !=
					http.StatusBadRequest {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						http.StatusBadRequest,
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

func TestHandlerGenerateTraduceErroresServicio(
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
				"%w: fecha_servicio es obligatoria",
				ErrDatosInvalidos,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre:         "programación no disponible",
			errorServicio:  ErrProgramacionNoDisponible,
			statusEsperado: http.StatusNotFound,
		},
		{
			nombre:         "programación no opera en la fecha",
			errorServicio:  ErrProgramacionNoOperaFecha,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "corrida duplicada",
			errorServicio:  ErrCorridaDuplicada,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "programación sin paradas",
			errorServicio:  ErrProgramacionSinParadas,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "unidad no disponible",
			errorServicio:  ErrUnidadNoDisponible,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "chofer no disponible",
			errorServicio:  ErrChoferNoDisponible,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "licencia no vigente",
			errorServicio:  ErrLicenciaNoVigente,
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
		"programacion_id": 4,
		"fecha_servicio": "2026-09-26"
	}`

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeCorridaService{
					generateFn: func(
						ctx context.Context,
						input GenerarInput,
					) (Corrida, error) {
						return Corrida{},
							prueba.errorServicio
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/corridas",
					strings.NewReader(body),
				)

				recorder := httptest.NewRecorder()

				handler.Handle(
					recorder,
					request,
				)

				if recorder.Code !=
					prueba.statusEsperado {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						prueba.statusEsperado,
						recorder.Code,
						recorder.Body.String(),
					)
				}
			},
		)
	}
}

func TestHandlerListTraduceErroresServicio(
	t *testing.T,
) {
	pruebas := []struct {
		nombre         string
		errorServicio  error
		statusEsperado int
	}{
		{
			nombre: "filtro inválido",
			errorServicio: fmt.Errorf(
				"%w: fecha_desde inválida",
				ErrDatosInvalidos,
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
				"error consultando PostgreSQL",
			),
			statusEsperado: http.StatusInternalServerError,
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeCorridaService{
					listFn: func(
						ctx context.Context,
						filter ListFilter,
					) ([]Corrida, error) {
						return nil,
							prueba.errorServicio
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodGet,
					"/api/corridas",
					nil,
				)

				recorder := httptest.NewRecorder()

				handler.Handle(
					recorder,
					request,
				)

				if recorder.Code !=
					prueba.statusEsperado {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						prueba.statusEsperado,
						recorder.Code,
						recorder.Body.String(),
					)
				}
			},
		)
	}
}

func TestHandlerRechazaMetodoNoPermitido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeCorridaService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/corridas",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code !=
		http.StatusMethodNotAllowed {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}

	allow := recorder.Header().Get("Allow")

	if allow != "GET, POST" {
		t.Errorf(
			"se esperaba Allow GET, POST, se obtuvo %q",
			allow,
		)
	}
}
