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

// fakeProgramacionService sustituye al Service real.
//
// Cada prueba configura solamente los métodos que necesita.
type fakeProgramacionService struct {
	createFn func(
		ctx context.Context,
		input CreateInput,
	) (Programacion, error)

	listFn func(
		ctx context.Context,
	) ([]Programacion, error)
}

var _ programacionService = (*fakeProgramacionService)(nil)

func (f *fakeProgramacionService) Create(
	ctx context.Context,
	input CreateInput,
) (Programacion, error) {
	if f.createFn == nil {
		return Programacion{}, errors.New(
			"fakeProgramacionService.Create no fue configurado",
		)
	}

	return f.createFn(ctx, input)
}

func (f *fakeProgramacionService) List(
	ctx context.Context,
) ([]Programacion, error) {
	if f.listFn == nil {
		return nil, errors.New(
			"fakeProgramacionService.List no fue configurado",
		)
	}

	return f.listFn(ctx)
}

func TestHandlerList(
	t *testing.T,
) {
	choferID := int64(3)
	choferNombre := "Juan Pérez"
	duracion := 240

	service := &fakeProgramacionService{
		listFn: func(
			ctx context.Context,
		) ([]Programacion, error) {
			return []Programacion{
				{
					ID:                      1,
					Codigo:                  "MOR-CDMX-0600",
					Nombre:                  "Salida matutina",
					RutaID:                  4,
					RutaCodigo:              "MOR-CDMX",
					RutaNombre:              "Morelia a CDMX",
					UnidadID:                2,
					UnidadCodigo:            "UNIDAD-01",
					ChoferID:                &choferID,
					ChoferNombre:            &choferNombre,
					HoraSalida:              "06:00",
					DuracionEstimadaMinutos: &duracion,
					VigenciaDesde:           "2026-09-25",
					DiasSemana:              []int{1, 2, 3, 4, 5, 6, 7},
					Activa:                  true,
				},
			}, nil
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/programaciones",
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
		Data []Programacion `json:"data"`
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
			"se esperaba una programación, se obtuvieron %d",
			len(respuesta.Data),
		)
	}

	programacion := respuesta.Data[0]

	if programacion.Codigo != "MOR-CDMX-0600" {
		t.Errorf(
			"se esperaba MOR-CDMX-0600, se obtuvo %q",
			programacion.Codigo,
		)
	}

	if programacion.RutaCodigo != "MOR-CDMX" {
		t.Errorf(
			"se esperaba la ruta MOR-CDMX, se obtuvo %q",
			programacion.RutaCodigo,
		)
	}

	if programacion.ChoferID == nil ||
		*programacion.ChoferID != 3 {
		t.Errorf(
			"se esperaba el chofer 3",
		)
	}

	if len(programacion.DiasSemana) != 7 {
		t.Errorf(
			"se esperaban 7 días, se obtuvieron %d",
			len(programacion.DiasSemana),
		)
	}
}

func TestHandlerCreate(
	t *testing.T,
) {
	var inputRecibido CreateInput

	service := &fakeProgramacionService{
		createFn: func(
			ctx context.Context,
			input CreateInput,
		) (Programacion, error) {
			inputRecibido = input

			return Programacion{
				ID:                      25,
				Codigo:                  input.Codigo,
				Nombre:                  input.Nombre,
				RutaID:                  input.RutaID,
				RutaCodigo:              "MOR-CDMX",
				RutaNombre:              "Morelia a CDMX",
				UnidadID:                input.UnidadID,
				UnidadCodigo:            "UNIDAD-01",
				ChoferID:                input.ChoferID,
				ChoferNombre:            stringPointerHandler("Juan Pérez"),
				HoraSalida:              input.HoraSalida,
				DuracionEstimadaMinutos: input.DuracionEstimadaMinutos,
				VigenciaDesde:           input.VigenciaDesde,
				VigenciaHasta:           input.VigenciaHasta,
				DiasSemana:              input.DiasSemana,
				Activa:                  true,
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"codigo": "MOR-CDMX-0600",
		"nombre": "Salida matutina",
		"ruta_id": 4,
		"unidad_id": 2,
		"chofer_id": 3,
		"hora_salida": "06:00",
		"duracion_estimada_minutos": 240,
		"vigencia_desde": "2026-09-25",
		"vigencia_hasta": null,
		"dias_semana": [1, 2, 3, 4, 5, 6, 7]
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/programaciones",
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

	if inputRecibido.Codigo != "MOR-CDMX-0600" {
		t.Errorf(
			"el Handler recibió el código %q",
			inputRecibido.Codigo,
		)
	}

	if inputRecibido.RutaID != 4 {
		t.Errorf(
			"se esperaba ruta_id 4, se obtuvo %d",
			inputRecibido.RutaID,
		)
	}

	if inputRecibido.UnidadID != 2 {
		t.Errorf(
			"se esperaba unidad_id 2, se obtuvo %d",
			inputRecibido.UnidadID,
		)
	}

	if inputRecibido.ChoferID == nil ||
		*inputRecibido.ChoferID != 3 {
		t.Errorf(
			"se esperaba chofer_id 3",
		)
	}

	if inputRecibido.DuracionEstimadaMinutos == nil ||
		*inputRecibido.DuracionEstimadaMinutos != 240 {
		t.Errorf(
			"se esperaba una duración de 240 minutos",
		)
	}

	if len(inputRecibido.DiasSemana) != 7 {
		t.Fatalf(
			"se esperaban 7 días, se obtuvieron %d",
			len(inputRecibido.DiasSemana),
		)
	}

	var respuesta struct {
		Data Programacion `json:"data"`
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
			"se esperaba la programación 25, se obtuvo %d",
			respuesta.Data.ID,
		)
	}

	if !respuesta.Data.Activa {
		t.Error(
			"se esperaba que la programación estuviera activa",
		)
	}
}

func TestHandlerCreateAceptaOpcionalesNulos(
	t *testing.T,
) {
	service := &fakeProgramacionService{
		createFn: func(
			ctx context.Context,
			input CreateInput,
		) (Programacion, error) {
			if input.ChoferID != nil {
				t.Error(
					"se esperaba que chofer_id fuera nil",
				)
			}

			if input.DuracionEstimadaMinutos != nil {
				t.Error(
					"se esperaba que la duración fuera nil",
				)
			}

			if input.VigenciaHasta != nil {
				t.Error(
					"se esperaba que vigencia_hasta fuera nil",
				)
			}

			return Programacion{
				ID:            1,
				Codigo:        input.Codigo,
				Nombre:        input.Nombre,
				RutaID:        input.RutaID,
				UnidadID:      input.UnidadID,
				HoraSalida:    input.HoraSalida,
				VigenciaDesde: input.VigenciaDesde,
				DiasSemana:    input.DiasSemana,
				Activa:        true,
			}, nil
		},
	}

	handler := NewHandler(service)

	// Los campos opcionales pueden omitirse completamente.
	body := `{
		"codigo": "MOR-CDMX-0600",
		"nombre": "Salida matutina",
		"ruta_id": 4,
		"unidad_id": 2,
		"hora_salida": "06:00",
		"vigencia_desde": "2026-09-25",
		"dias_semana": [1, 2, 3, 4, 5, 6, 7]
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/programaciones",
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
				"codigo": "MOR-CDMX-0600",
				"nombre": "Salida matutina",
				"ruta_id": 4,
				"unidad_id": 2,
				"hora_salida": "06:00",
				"vigencia_desde": "2026-09-25",
				"dias_semana": [1],
				"campo_inexistente": true
			}`,
		},
		{
			nombre: "dos objetos JSON",
			body: `{
				"codigo": "MOR-CDMX-0600",
				"nombre": "Salida matutina",
				"ruta_id": 4,
				"unidad_id": 2,
				"hora_salida": "06:00",
				"vigencia_desde": "2026-09-25",
				"dias_semana": [1]
			}
			{
				"codigo": "OTRA-PROGRAMACION"
			}`,
		},
		{
			nombre: "días con tipo incorrecto",
			body: `{
				"codigo": "MOR-CDMX-0600",
				"nombre": "Salida matutina",
				"ruta_id": 4,
				"unidad_id": 2,
				"hora_salida": "06:00",
				"vigencia_desde": "2026-09-25",
				"dias_semana": "todos"
			}`,
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeProgramacionService{
					createFn: func(
						ctx context.Context,
						input CreateInput,
					) (Programacion, error) {
						t.Fatal(
							"el Service no debe ejecutarse con JSON inválido",
						)

						return Programacion{}, nil
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/programaciones",
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
				"%w: hora inválida",
				ErrDatosInvalidos,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre: "código duplicado",
			errorServicio: fmt.Errorf(
				"%w: MOR-CDMX-0600",
				ErrCodigoDuplicado,
			),
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "ruta no disponible",
			errorServicio:  ErrRutaNoDisponible,
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre:         "unidad no disponible",
			errorServicio:  ErrUnidadNoDisponible,
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre:         "chofer no disponible",
			errorServicio:  ErrChoferNoDisponible,
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
		"codigo": "MOR-CDMX-0600",
		"nombre": "Salida matutina",
		"ruta_id": 4,
		"unidad_id": 2,
		"hora_salida": "06:00",
		"vigencia_desde": "2026-09-25",
		"dias_semana": [1, 2, 3, 4, 5, 6, 7]
	}`

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeProgramacionService{
					createFn: func(
						ctx context.Context,
						input CreateInput,
					) (Programacion, error) {
						return Programacion{},
							prueba.errorServicio
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/programaciones",
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
	service := &fakeProgramacionService{
		listFn: func(
			ctx context.Context,
		) ([]Programacion, error) {
			return nil, errors.New(
				"error consultando PostgreSQL",
			)
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/programaciones",
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
		&fakeProgramacionService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/programaciones",
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

// stringPointerHandler devuelve un *string para construir
// resultados simulados dentro de las pruebas.
func stringPointerHandler(
	valor string,
) *string {
	return &valor
}
