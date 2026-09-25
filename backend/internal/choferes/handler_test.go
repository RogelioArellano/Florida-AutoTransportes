package choferes

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

// fakeChoferService sustituye al servicio real.
//
// Cada prueba configura solamente la operación que necesita.
type fakeChoferService struct {
	createFn func(
		ctx context.Context,
		input CreateInput,
	) (Chofer, error)

	listFn func(
		ctx context.Context,
	) ([]Chofer, error)
}

// El compilador comprobará que fakeChoferService
// implementa la interfaz requerida por el Handler.
var _ choferService = (*fakeChoferService)(nil)

func (f *fakeChoferService) Create(
	ctx context.Context,
	input CreateInput,
) (Chofer, error) {
	if f.createFn == nil {
		return Chofer{}, errors.New(
			"fakeChoferService.Create no fue configurado",
		)
	}

	return f.createFn(ctx, input)
}

func (f *fakeChoferService) List(
	ctx context.Context,
) ([]Chofer, error) {
	if f.listFn == nil {
		return nil, errors.New(
			"fakeChoferService.List no fue configurado",
		)
	}

	return f.listFn(ctx)
}

func TestHandlerList(
	t *testing.T,
) {
	licencia := "LIC-12345"
	vigencia := "2027-12-31"

	service := &fakeChoferService{
		listFn: func(
			ctx context.Context,
		) ([]Chofer, error) {
			return []Chofer{
				{
					ID:               1,
					NombreCompleto:   "Juan Pérez",
					Telefono:         "+52 443 123 4567",
					LicenciaNumero:   &licencia,
					LicenciaVigencia: &vigencia,
					Activo:           true,
				},
			}, nil
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/choferes",
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
		Data []Chofer `json:"data"`
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
			"se esperaba un chofer, se obtuvieron %d",
			len(respuesta.Data),
		)
	}

	chofer := respuesta.Data[0]

	if chofer.NombreCompleto != "Juan Pérez" {
		t.Errorf(
			"se esperaba Juan Pérez, se obtuvo %q",
			chofer.NombreCompleto,
		)
	}

	if chofer.LicenciaNumero == nil ||
		*chofer.LicenciaNumero != "LIC-12345" {
		t.Errorf(
			"se esperaba la licencia LIC-12345",
		)
	}

	if chofer.LicenciaVigencia == nil ||
		*chofer.LicenciaVigencia != "2027-12-31" {
		t.Errorf(
			"se esperaba la vigencia 2027-12-31",
		)
	}

	if !chofer.Activo {
		t.Error(
			"se esperaba que el chofer estuviera activo",
		)
	}
}

func TestHandlerCreate(
	t *testing.T,
) {
	var inputRecibido CreateInput

	service := &fakeChoferService{
		createFn: func(
			ctx context.Context,
			input CreateInput,
		) (Chofer, error) {
			inputRecibido = input

			return Chofer{
				ID:               25,
				NombreCompleto:   input.NombreCompleto,
				Telefono:         input.Telefono,
				LicenciaNumero:   input.LicenciaNumero,
				LicenciaVigencia: input.LicenciaVigencia,
				Activo:           true,
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"nombre_completo": "Juan Pérez",
		"telefono": "+52 443 123 4567",
		"licencia_numero": "LIC-12345",
		"licencia_vigencia": "2027-12-31"
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/choferes",
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

	if inputRecibido.NombreCompleto != "Juan Pérez" {
		t.Errorf(
			"el Handler recibió el nombre %q",
			inputRecibido.NombreCompleto,
		)
	}

	if inputRecibido.Telefono != "+52 443 123 4567" {
		t.Errorf(
			"el Handler recibió el teléfono %q",
			inputRecibido.Telefono,
		)
	}

	if inputRecibido.LicenciaNumero == nil ||
		*inputRecibido.LicenciaNumero != "LIC-12345" {
		t.Errorf(
			"el Handler no recibió correctamente la licencia",
		)
	}

	if inputRecibido.LicenciaVigencia == nil ||
		*inputRecibido.LicenciaVigencia != "2027-12-31" {
		t.Errorf(
			"el Handler no recibió correctamente la vigencia",
		)
	}

	var respuesta struct {
		Data Chofer `json:"data"`
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
			"se esperaba el chofer 25, se obtuvo %d",
			respuesta.Data.ID,
		)
	}

	if !respuesta.Data.Activo {
		t.Error(
			"se esperaba que el chofer estuviera activo",
		)
	}
}

func TestHandlerCreateAceptaLicenciaNula(
	t *testing.T,
) {
	service := &fakeChoferService{
		createFn: func(
			ctx context.Context,
			input CreateInput,
		) (Chofer, error) {
			if input.LicenciaNumero != nil {
				t.Error(
					"se esperaba que licencia_numero fuera nil",
				)
			}

			if input.LicenciaVigencia != nil {
				t.Error(
					"se esperaba que licencia_vigencia fuera nil",
				)
			}

			return Chofer{
				ID:             1,
				NombreCompleto: input.NombreCompleto,
				Telefono:       input.Telefono,
				Activo:         true,
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"nombre_completo": "María López",
		"telefono": "4437654321",
		"licencia_numero": null,
		"licencia_vigencia": null
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/choferes",
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
			body:   `{"nombre_completo":`,
		},
		{
			nombre: "campo desconocido",
			body: `{
				"nombre_completo": "Juan Pérez",
				"telefono": "4431234567",
				"campo_inexistente": true
			}`,
		},
		{
			nombre: "dos objetos JSON",
			body: `{
				"nombre_completo": "Juan Pérez",
				"telefono": "4431234567"
			}
			{
				"nombre_completo": "María López"
			}`,
		},
		{
			nombre: "tipo de teléfono incorrecto",
			body: `{
				"nombre_completo": "Juan Pérez",
				"telefono": 4431234567
			}`,
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeChoferService{
					createFn: func(
						ctx context.Context,
						input CreateInput,
					) (Chofer, error) {
						t.Fatal(
							"el Service no debe ejecutarse con JSON inválido",
						)

						return Chofer{}, nil
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/choferes",
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
				"%w: el teléfono es obligatorio",
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
				"PostgreSQL no disponible",
			),
			statusEsperado: http.StatusInternalServerError,
		},
	}

	body := `{
		"nombre_completo": "Juan Pérez",
		"telefono": "4431234567"
	}`

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeChoferService{
					createFn: func(
						ctx context.Context,
						input CreateInput,
					) (Chofer, error) {
						return Chofer{}, prueba.errorServicio
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/choferes",
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
	service := &fakeChoferService{
		listFn: func(
			ctx context.Context,
		) ([]Chofer, error) {
			return nil, errors.New(
				"error consultando PostgreSQL",
			)
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/choferes",
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
		&fakeChoferService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/choferes",
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
