package programaciones

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

type fakeHorarioService struct {
	configureFn func(
		ctx context.Context,
		input ConfigurarHorariosInput,
	) ([]HorarioParada, error)

	listFn func(
		ctx context.Context,
		programacionID int64,
	) ([]HorarioParada, error)
}

var _ horarioService = (*fakeHorarioService)(nil)

func (f *fakeHorarioService) Configure(
	ctx context.Context,
	input ConfigurarHorariosInput,
) ([]HorarioParada, error) {
	if f.configureFn == nil {
		return nil, errors.New(
			"fakeHorarioService.Configure no fue configurado",
		)
	}

	return f.configureFn(ctx, input)
}

func (f *fakeHorarioService) List(
	ctx context.Context,
	programacionID int64,
) ([]HorarioParada, error) {
	if f.listFn == nil {
		return nil, errors.New(
			"fakeHorarioService.List no fue configurado",
		)
	}

	return f.listFn(ctx, programacionID)
}

func TestHorarioHandlerList(
	t *testing.T,
) {
	var programacionIDRecibido int64

	service := &fakeHorarioService{
		listFn: func(
			ctx context.Context,
			programacionID int64,
		) ([]HorarioParada, error) {
			programacionIDRecibido = programacionID

			return []HorarioParada{
				{
					ProgramacionID:           programacionID,
					RutaParadaID:             3,
					PuntoAbordajeID:          2,
					PuntoNombre:              "Morelia Centro",
					Orden:                    1,
					MinutosDesdeSalidaInicio: 0,
					MinutosDesdeSalidaFin:    0,
					HoraEstimadaInicio:       "06:00",
					HoraEstimadaFin:          "06:00",
				},
				{
					ProgramacionID:           programacionID,
					RutaParadaID:             4,
					PuntoAbordajeID:          3,
					PuntoNombre:              "Pabellón Don Vasco",
					Orden:                    2,
					MinutosDesdeSalidaInicio: 10,
					MinutosDesdeSalidaFin:    15,
					HoraEstimadaInicio:       "06:10",
					HoraEstimadaFin:          "06:15",
				},
			}, nil
		},
	}

	handler := NewHorarioHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/programaciones/horarios?programacion_id=3",
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

	if programacionIDRecibido != 3 {
		t.Errorf(
			"el Handler envió programacion_id %d; se esperaba 3",
			programacionIDRecibido,
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
		Data []HorarioParada `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if len(respuesta.Data) != 2 {
		t.Fatalf(
			"se esperaban 2 horarios, se obtuvieron %d",
			len(respuesta.Data),
		)
	}

	if respuesta.Data[0].HoraEstimadaInicio != "06:00" {
		t.Errorf(
			"se esperaba 06:00, se obtuvo %q",
			respuesta.Data[0].HoraEstimadaInicio,
		)
	}

	if respuesta.Data[1].HoraEstimadaFin != "06:15" {
		t.Errorf(
			"se esperaba 06:15, se obtuvo %q",
			respuesta.Data[1].HoraEstimadaFin,
		)
	}
}

func TestHorarioHandlerListRechazaIDInvalido(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		url    string
	}{
		{
			nombre: "parámetro ausente",
			url:    "/api/programaciones/horarios",
		},
		{
			nombre: "parámetro vacío",
			url: "/api/programaciones/horarios" +
				"?programacion_id=",
		},
		{
			nombre: "parámetro no numérico",
			url: "/api/programaciones/horarios" +
				"?programacion_id=abc",
		},
		{
			nombre: "identificador cero",
			url: "/api/programaciones/horarios" +
				"?programacion_id=0",
		},
		{
			nombre: "identificador negativo",
			url: "/api/programaciones/horarios" +
				"?programacion_id=-1",
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeHorarioService{
					listFn: func(
						ctx context.Context,
						programacionID int64,
					) ([]HorarioParada, error) {
						t.Fatal(
							"el Service no debe ejecutarse con un ID inválido",
						)

						return nil, nil
					},
				}

				handler := NewHorarioHandler(service)

				request := httptest.NewRequest(
					http.MethodGet,
					prueba.url,
					nil,
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
			},
		)
	}
}

func TestHorarioHandlerConfigure(
	t *testing.T,
) {
	var inputRecibido ConfigurarHorariosInput

	service := &fakeHorarioService{
		configureFn: func(
			ctx context.Context,
			input ConfigurarHorariosInput,
		) ([]HorarioParada, error) {
			inputRecibido = input

			return []HorarioParada{
				{
					ProgramacionID:           input.ProgramacionID,
					RutaParadaID:             3,
					PuntoAbordajeID:          2,
					PuntoNombre:              "Morelia Centro",
					Orden:                    1,
					MinutosDesdeSalidaInicio: 0,
					MinutosDesdeSalidaFin:    0,
					HoraEstimadaInicio:       "06:00",
					HoraEstimadaFin:          "06:00",
				},
				{
					ProgramacionID:           input.ProgramacionID,
					RutaParadaID:             4,
					PuntoAbordajeID:          3,
					PuntoNombre:              "Pabellón Don Vasco",
					Orden:                    2,
					MinutosDesdeSalidaInicio: 10,
					MinutosDesdeSalidaFin:    15,
					HoraEstimadaInicio:       "06:10",
					HoraEstimadaFin:          "06:15",
				},
			}, nil
		},
	}

	handler := NewHorarioHandler(service)

	body := `{
		"programacion_id": 3,
		"paradas": [
			{
				"ruta_parada_id": 3,
				"minutos_desde_salida_inicio": 0,
				"minutos_desde_salida_fin": 0,
				"notas": "Salida de Morelia Centro"
			},
			{
				"ruta_parada_id": 4,
				"minutos_desde_salida_inicio": 10,
				"minutos_desde_salida_fin": 15,
				"notas": "Ventana de abordaje"
			}
		]
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/programaciones/horarios",
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
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

	if inputRecibido.ProgramacionID != 3 {
		t.Errorf(
			"se esperaba programacion_id 3, se obtuvo %d",
			inputRecibido.ProgramacionID,
		)
	}

	if len(inputRecibido.Paradas) != 2 {
		t.Fatalf(
			"se esperaban 2 paradas, se obtuvieron %d",
			len(inputRecibido.Paradas),
		)
	}

	if inputRecibido.Paradas[1].RutaParadaID != 4 {
		t.Errorf(
			"se esperaba ruta_parada_id 4, se obtuvo %d",
			inputRecibido.Paradas[1].RutaParadaID,
		)
	}

	if inputRecibido.Paradas[1].
		MinutosDesdeSalidaInicio != 10 {
		t.Errorf(
			"se esperaban 10 minutos de inicio",
		)
	}

	var respuesta struct {
		Data []HorarioParada `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if len(respuesta.Data) != 2 {
		t.Fatalf(
			"se esperaban 2 horarios, se obtuvieron %d",
			len(respuesta.Data),
		)
	}
}

func TestHorarioHandlerConfigureAceptaNotasNulas(
	t *testing.T,
) {
	service := &fakeHorarioService{
		configureFn: func(
			ctx context.Context,
			input ConfigurarHorariosInput,
		) ([]HorarioParada, error) {
			if len(input.Paradas) != 1 {
				t.Fatalf(
					"se esperaba una parada",
				)
			}

			if input.Paradas[0].Notas != nil {
				t.Error(
					"se esperaba que notas fuera nil",
				)
			}

			return []HorarioParada{}, nil
		},
	}

	handler := NewHorarioHandler(service)

	body := `{
		"programacion_id": 3,
		"paradas": [
			{
				"ruta_parada_id": 3,
				"minutos_desde_salida_inicio": 0,
				"minutos_desde_salida_fin": 0,
				"notas": null
			}
		]
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/programaciones/horarios",
		strings.NewReader(body),
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

func TestHorarioHandlerConfigureRechazaJSONInvalido(
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
				"programacion_id": 3,
				"paradas": [],
				"campo_inexistente": true
			}`,
		},
		{
			nombre: "dos objetos JSON",
			body: `{
				"programacion_id": 3,
				"paradas": []
			}
			{
				"programacion_id": 4
			}`,
		},
		{
			nombre: "minutos con tipo incorrecto",
			body: `{
				"programacion_id": 3,
				"paradas": [
					{
						"ruta_parada_id": 3,
						"minutos_desde_salida_inicio": "cero",
						"minutos_desde_salida_fin": 0,
						"notas": null
					}
				]
			}`,
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeHorarioService{
					configureFn: func(
						ctx context.Context,
						input ConfigurarHorariosInput,
					) ([]HorarioParada, error) {
						t.Fatal(
							"el Service no debe ejecutarse con JSON inválido",
						)

						return nil, nil
					},
				}

				handler := NewHorarioHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/programaciones/horarios",
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
			},
		)
	}
}

func TestHorarioHandlerConfigureTraduceErrores(
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
				"%w: minutos inválidos",
				ErrDatosInvalidos,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre: "programación no disponible",
			errorServicio: fmt.Errorf(
				"%w: programación 99",
				ErrProgramacionNoDisponible,
			),
			statusEsperado: http.StatusNotFound,
		},
		{
			nombre: "parada de otra ruta",
			errorServicio: fmt.Errorf(
				"%w: parada 6",
				ErrParadaNoPertenece,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre: "horarios fuera de orden",
			errorServicio: fmt.Errorf(
				"%w: orden incorrecto",
				ErrHorariosFueraDeOrden,
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
		"programacion_id": 3,
		"paradas": [
			{
				"ruta_parada_id": 3,
				"minutos_desde_salida_inicio": 0,
				"minutos_desde_salida_fin": 0,
				"notas": null
			}
		]
	}`

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeHorarioService{
					configureFn: func(
						ctx context.Context,
						input ConfigurarHorariosInput,
					) ([]HorarioParada, error) {
						return nil, prueba.errorServicio
					},
				}

				handler := NewHorarioHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/programaciones/horarios",
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

func TestHorarioHandlerListTraduceError(
	t *testing.T,
) {
	service := &fakeHorarioService{
		listFn: func(
			ctx context.Context,
			programacionID int64,
		) ([]HorarioParada, error) {
			return nil, ErrProgramacionNoDisponible
		},
	}

	handler := NewHorarioHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/programaciones/horarios?programacion_id=999",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusNotFound,
			recorder.Code,
		)
	}
}

func TestHorarioHandlerRechazaMetodoNoPermitido(
	t *testing.T,
) {
	handler := NewHorarioHandler(
		&fakeHorarioService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/programaciones/horarios",
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
