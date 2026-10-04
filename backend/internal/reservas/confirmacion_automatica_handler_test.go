package reservas

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeConfirmacionAutomaticaService struct {
	listResult []ConfirmacionPendiente
	listErr    error
	listCalls  int

	requestInput  SolicitarConfirmacionInput
	requestResult ConfirmacionPendiente
	requestErr    error
	requestCalls  int
}

var _ confirmacionAutomaticaService = (*fakeConfirmacionAutomaticaService)(nil)

func (f *fakeConfirmacionAutomaticaService) ListPendingConfirmations(
	ctx context.Context,
) ([]ConfirmacionPendiente, error) {
	f.listCalls++

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func (f *fakeConfirmacionAutomaticaService) RequestConfirmation(
	ctx context.Context,
	input SolicitarConfirmacionInput,
) (ConfirmacionPendiente, error) {
	f.requestCalls++
	f.requestInput = input

	if f.requestErr != nil {
		return ConfirmacionPendiente{},
			f.requestErr
	}

	return f.requestResult, nil
}

func TestConfirmacionAutomaticaHandlerListPending(
	t *testing.T,
) {
	salida := time.Date(
		2026,
		time.October,
		4,
		15,
		0,
		0,
		0,
		time.FixedZone("CST", -6*60*60),
	)

	service := &fakeConfirmacionAutomaticaService{
		listResult: []ConfirmacionPendiente{
			{
				ReservaID:           10,
				ReservaFolio:        "RES-00000010",
				PasajeroNombre:      "Pasajero de prueba",
				PasajeroTelefono:    "4430000000",
				CorridaID:           4,
				CorridaFolio:        "MOR-CDMX-20261004",
				SalidaProgramada:    salida,
				CantidadPasajeros:   1,
				ParadaOrigenNombre:  "Morelia Centro",
				ParadaDestinoNombre: "Observatorio",
			},
		},
	}

	handler := NewConfirmacionAutomaticaHandler(
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/confirmaciones/pendientes",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandlePending(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"se esperaba status %d; se obtuvo %d: %s",
			http.StatusOK,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if service.listCalls != 1 {
		t.Fatalf(
			"ListPendingConfirmations() fue llamado %d veces; se esperaba 1",
			service.listCalls,
		)
	}

	if !strings.Contains(
		recorder.Body.String(),
		`"reserva_folio":"RES-00000010"`,
	) {
		t.Errorf(
			"la respuesta no contiene la reserva esperada: %s",
			recorder.Body.String(),
		)
	}
}

func TestConfirmacionAutomaticaHandlerRequestConfirmation(
	t *testing.T,
) {
	solicitadaEn := time.Now()
	limiteEn := solicitadaEn.Add(
		90 * time.Minute,
	)

	service := &fakeConfirmacionAutomaticaService{
		requestResult: ConfirmacionPendiente{
			ReservaID:                10,
			ReservaFolio:             "RES-00000010",
			ConfirmacionSolicitadaEn: &solicitadaEn,
			ConfirmacionLimiteEn:     &limiteEn,
		},
	}

	handler := NewConfirmacionAutomaticaHandler(
		service,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/confirmaciones/solicitudes",
		bytes.NewBufferString(`{
			"reserva_id": 10
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleRequests(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"se esperaba status %d; se obtuvo %d: %s",
			http.StatusOK,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if service.requestCalls != 1 {
		t.Fatalf(
			"RequestConfirmation() fue llamado %d veces; se esperaba 1",
			service.requestCalls,
		)
	}

	if service.requestInput.ReservaID != 10 {
		t.Errorf(
			"ReservaID = %d; se esperaba 10",
			service.requestInput.ReservaID,
		)
	}

	if !strings.Contains(
		recorder.Body.String(),
		`"confirmacion_solicitada_en":`,
	) {
		t.Errorf(
			"la respuesta no contiene la fecha de solicitud: %s",
			recorder.Body.String(),
		)
	}
}

func TestConfirmacionAutomaticaHandlerRequestRechazaJSONInvalido(
	t *testing.T,
) {
	service := &fakeConfirmacionAutomaticaService{}

	handler := NewConfirmacionAutomaticaHandler(
		service,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/confirmaciones/solicitudes",
		bytes.NewBufferString(`{
			"reserva_id": 10,
			"campo_desconocido": true
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleRequests(
		recorder,
		request,
	)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"se esperaba status %d; se obtuvo %d",
			http.StatusBadRequest,
			recorder.Code,
		)
	}

	if service.requestCalls != 0 {
		t.Error(
			"el Service no debió ser llamado",
		)
	}
}

func TestConfirmacionAutomaticaHandlerMapeaConflictos(
	t *testing.T,
) {
	casos := []struct {
		nombre string
		err    error
	}{
		{
			nombre: "reserva no acepta solicitud",
			err: fmt.Errorf(
				"%w: estado de reserva CONFIRMADA",
				ErrReservaNoAceptaSolicitudConfirmacion,
			),
		},
		{
			nombre: "fuera de ventana",
			err:    ErrSolicitudConfirmacionFueraDeVentana,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			service :=
				&fakeConfirmacionAutomaticaService{
					requestErr: caso.err,
				}

			handler :=
				NewConfirmacionAutomaticaHandler(
					service,
				)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/reservas/confirmaciones/solicitudes",
				bytes.NewBufferString(`{
					"reserva_id": 10
				}`),
			)

			recorder := httptest.NewRecorder()

			handler.HandleRequests(
				recorder,
				request,
			)

			if recorder.Code !=
				http.StatusConflict {
				t.Fatalf(
					"se esperaba status %d; se obtuvo %d: %s",
					http.StatusConflict,
					recorder.Code,
					recorder.Body.String(),
				)
			}
		})
	}
}

func TestConfirmacionAutomaticaHandlerMapeaNoEncontrada(
	t *testing.T,
) {
	service := &fakeConfirmacionAutomaticaService{
		requestErr: ErrReservaNoEncontrada,
	}

	handler := NewConfirmacionAutomaticaHandler(
		service,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/confirmaciones/solicitudes",
		bytes.NewBufferString(`{
			"reserva_id": 999
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleRequests(
		recorder,
		request,
	)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"se esperaba status %d; se obtuvo %d: %s",
			http.StatusNotFound,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestConfirmacionAutomaticaHandlerRechazaMetodos(
	t *testing.T,
) {
	service := &fakeConfirmacionAutomaticaService{}

	handler := NewConfirmacionAutomaticaHandler(
		service,
	)

	casos := []struct {
		nombre    string
		metodo    string
		manejador func(
			http.ResponseWriter,
			*http.Request,
		)
		allow string
	}{
		{
			nombre:    "pendientes",
			metodo:    http.MethodPost,
			manejador: handler.HandlePending,
			allow:     "GET",
		},
		{
			nombre:    "solicitudes",
			metodo:    http.MethodGet,
			manejador: handler.HandleRequests,
			allow:     "POST",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			request := httptest.NewRequest(
				caso.metodo,
				"/api/reservas/confirmaciones/"+
					caso.nombre,
				nil,
			)

			recorder := httptest.NewRecorder()

			caso.manejador(
				recorder,
				request,
			)

			if recorder.Code !=
				http.StatusMethodNotAllowed {
				t.Fatalf(
					"se esperaba status %d; se obtuvo %d",
					http.StatusMethodNotAllowed,
					recorder.Code,
				)
			}

			if valor := recorder.Header().Get(
				"Allow",
			); valor != caso.allow {
				t.Errorf(
					"se esperaba Allow %s; se obtuvo %q",
					caso.allow,
					valor,
				)
			}
		})
	}
}
