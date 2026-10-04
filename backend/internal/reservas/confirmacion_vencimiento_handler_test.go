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

type fakeConfirmacionVencimientoService struct {
	listResult []ConfirmacionPendiente
	listErr    error
	listCalls  int

	expireInput  VencerConfirmacionInput
	expireResult Reserva
	expireErr    error
	expireCalls  int
}

var _ confirmacionVencimientoService = (*fakeConfirmacionVencimientoService)(nil)

func (f *fakeConfirmacionVencimientoService) ListExpiredConfirmations(
	ctx context.Context,
) ([]ConfirmacionPendiente, error) {
	f.listCalls++

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func (f *fakeConfirmacionVencimientoService) ExpireConfirmation(
	ctx context.Context,
	input VencerConfirmacionInput,
) (Reserva, error) {
	f.expireCalls++
	f.expireInput = input

	if f.expireErr != nil {
		return Reserva{}, f.expireErr
	}

	return f.expireResult, nil
}

func TestConfirmacionVencimientoHandlerListExpired(
	t *testing.T,
) {
	solicitadaEn := time.Now().Add(
		-2 * time.Hour,
	)

	limiteEn := time.Now().Add(
		-30 * time.Minute,
	)

	service := &fakeConfirmacionVencimientoService{
		listResult: []ConfirmacionPendiente{
			{
				ReservaID:                20,
				ReservaFolio:             "RES-00000020",
				PasajeroNombre:           "Pasajero de prueba",
				ConfirmacionSolicitadaEn: &solicitadaEn,
				ConfirmacionLimiteEn:     &limiteEn,
			},
		},
	}

	handler := NewConfirmacionVencimientoHandler(
		service,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/confirmaciones/vencidas",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandleExpired(
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
			"ListExpiredConfirmations() fue llamado %d veces; se esperaba 1",
			service.listCalls,
		)
	}

	if !strings.Contains(
		recorder.Body.String(),
		`"reserva_folio":"RES-00000020"`,
	) {
		t.Errorf(
			"la respuesta no contiene la reserva vencida: %s",
			recorder.Body.String(),
		)
	}
}

func TestConfirmacionVencimientoHandlerExpireConfirmation(
	t *testing.T,
) {
	service := &fakeConfirmacionVencimientoService{
		expireResult: Reserva{
			ID:                   20,
			Folio:                "RES-00000020",
			Estado:               EstadoCancelada,
			RequiereConfirmacion: false,
		},
	}

	handler := NewConfirmacionVencimientoHandler(
		service,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/confirmaciones/vencimientos",
		bytes.NewBufferString(`{
			"reserva_id": 20
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleExpirations(
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

	if service.expireCalls != 1 {
		t.Fatalf(
			"ExpireConfirmation() fue llamado %d veces; se esperaba 1",
			service.expireCalls,
		)
	}

	if service.expireInput.ReservaID != 20 {
		t.Errorf(
			"ReservaID = %d; se esperaba 20",
			service.expireInput.ReservaID,
		)
	}

	if !strings.Contains(
		recorder.Body.String(),
		`"estado":"CANCELADA"`,
	) {
		t.Errorf(
			"la respuesta no contiene el estado CANCELADA: %s",
			recorder.Body.String(),
		)
	}
}

func TestConfirmacionVencimientoHandlerRechazaJSONInvalido(
	t *testing.T,
) {
	service := &fakeConfirmacionVencimientoService{}

	handler := NewConfirmacionVencimientoHandler(
		service,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/confirmaciones/vencimientos",
		bytes.NewBufferString(`{
			"reserva_id": 20,
			"campo_desconocido": true
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleExpirations(
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

	if service.expireCalls != 0 {
		t.Error(
			"el Service no debió ser llamado",
		)
	}
}

func TestConfirmacionVencimientoHandlerMapeaConflictos(
	t *testing.T,
) {
	casos := []struct {
		nombre string
		err    error
	}{
		{
			nombre: "plazo no vencido",
			err:    ErrSolicitudConfirmacionNoVencida,
		},
		{
			nombre: "reserva no acepta vencimiento",
			err: fmt.Errorf(
				"%w: estado de reserva CONFIRMADA",
				ErrReservaNoAceptaVencimientoConfirmacion,
			),
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			service :=
				&fakeConfirmacionVencimientoService{
					expireErr: caso.err,
				}

			handler :=
				NewConfirmacionVencimientoHandler(
					service,
				)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/reservas/confirmaciones/vencimientos",
				bytes.NewBufferString(`{
					"reserva_id": 20
				}`),
			)

			recorder := httptest.NewRecorder()

			handler.HandleExpirations(
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

func TestConfirmacionVencimientoHandlerMapeaNoEncontrada(
	t *testing.T,
) {
	service := &fakeConfirmacionVencimientoService{
		expireErr: ErrReservaNoEncontrada,
	}

	handler := NewConfirmacionVencimientoHandler(
		service,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/confirmaciones/vencimientos",
		bytes.NewBufferString(`{
			"reserva_id": 999999
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleExpirations(
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

func TestConfirmacionVencimientoHandlerRechazaMetodos(
	t *testing.T,
) {
	service := &fakeConfirmacionVencimientoService{}

	handler := NewConfirmacionVencimientoHandler(
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
			nombre:    "vencidas",
			metodo:    http.MethodPost,
			manejador: handler.HandleExpired,
			allow:     "GET",
		},
		{
			nombre:    "vencimientos",
			metodo:    http.MethodGet,
			manejador: handler.HandleExpirations,
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
