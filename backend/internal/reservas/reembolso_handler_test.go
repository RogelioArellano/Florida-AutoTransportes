package reservas

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeReembolsoService struct {
	registerInput  RegistrarReembolsoInput
	registerResult ReembolsosReserva
	registerErr    error
	registerCalls  int

	getReservaID int64
	getResult    ReembolsosReserva
	getErr       error
	getCalls     int
}

var _ reembolsoService = (*fakeReembolsoService)(nil)

func (f *fakeReembolsoService) RegisterRefund(
	ctx context.Context,
	input RegistrarReembolsoInput,
) (ReembolsosReserva, error) {
	f.registerCalls++
	f.registerInput = input

	if f.registerErr != nil {
		return ReembolsosReserva{},
			f.registerErr
	}

	return f.registerResult, nil
}

func (f *fakeReembolsoService) GetRefundsByReservation(
	ctx context.Context,
	reservaID int64,
) (ReembolsosReserva, error) {
	f.getCalls++
	f.getReservaID = reservaID

	if f.getErr != nil {
		return ReembolsosReserva{}, f.getErr
	}

	return f.getResult, nil
}

func TestReembolsoHandlerRegisterCreaMovimiento(
	t *testing.T,
) {
	service := &fakeReembolsoService{
		registerResult: ReembolsosReserva{
			ReservaID:          5,
			MontoReembolsable:  Dinero("270.00"),
			MontoReembolsado:   Dinero("100.00"),
			SaldoPorReembolsar: Dinero("170.00"),
			Estado:             EstadoReembolsoParcial,
			Reembolsos:         []Reembolso{},
		},
	}

	handler := NewReembolsoHandler(service)

	body := bytes.NewBufferString(`{
		"reserva_id": 5,
		"monto": "100.00",
		"metodo": "TRANSFERENCIA",
		"referencia": "DEV-001",
		"notas": "Devolución parcial"
	}`)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/reembolsos",
		body,
	)

	recorder := httptest.NewRecorder()

	handler.HandleCollection(
		recorder,
		request,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"se esperaba status %d; se obtuvo %d: %s",
			http.StatusCreated,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if service.registerCalls != 1 {
		t.Fatalf(
			"RegisterRefund() fue llamado %d veces; se esperaba 1",
			service.registerCalls,
		)
	}

	if service.registerInput.ReservaID != 5 {
		t.Errorf(
			"ReservaID = %d; se esperaba 5",
			service.registerInput.ReservaID,
		)
	}

	if service.registerInput.Monto !=
		Dinero("100.00") {
		t.Errorf(
			"Monto = %q; se esperaba 100.00",
			service.registerInput.Monto,
		)
	}

	if !strings.Contains(
		recorder.Body.String(),
		`"estado_reembolso":"PARCIAL"`,
	) {
		t.Errorf(
			"la respuesta no contiene el estado PARCIAL: %s",
			recorder.Body.String(),
		)
	}
}

func TestReembolsoHandlerRegisterRechazaJSONInvalido(
	t *testing.T,
) {
	service := &fakeReembolsoService{}
	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/reembolsos",
		bytes.NewBufferString(`{
			"reserva_id": 5,
			"campo_desconocido": true
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleCollection(
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

	if service.registerCalls != 0 {
		t.Error(
			"el Service no debió ser llamado",
		)
	}
}

func TestReembolsoHandlerRegisterMapeaConflicto(
	t *testing.T,
) {
	service := &fakeReembolsoService{
		registerErr: fmt.Errorf(
			"%w: saldo disponible 270.00, reembolso solicitado 300.00",
			ErrReembolsoExcedeSaldo,
		),
	}

	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/reembolsos",
		bytes.NewBufferString(`{
			"reserva_id": 5,
			"monto": "300.00",
			"metodo": "EFECTIVO"
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleCollection(
		recorder,
		request,
	)

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"se esperaba status %d; se obtuvo %d: %s",
			http.StatusConflict,
			recorder.Code,
			recorder.Body.String(),
		)
	}
}

func TestReembolsoHandlerGetByReservation(
	t *testing.T,
) {
	service := &fakeReembolsoService{
		getResult: ReembolsosReserva{
			ReservaID:          5,
			MontoReembolsable:  Dinero("270.00"),
			MontoReembolsado:   Dinero("0.00"),
			SaldoPorReembolsar: Dinero("270.00"),
			Estado:             EstadoReembolsoPendiente,
			Reembolsos:         []Reembolso{},
		},
	}

	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/5/reembolsos",
		nil,
	)

	request.SetPathValue(
		"reservaID",
		"5",
	)

	recorder := httptest.NewRecorder()

	handler.HandleByReservation(
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

	if service.getCalls != 1 ||
		service.getReservaID != 5 {
		t.Error(
			"el Service no recibió reserva_id 5",
		)
	}

	if !strings.Contains(
		recorder.Body.String(),
		`"estado_reembolso":"PENDIENTE"`,
	) {
		t.Errorf(
			"la respuesta no contiene el estado PENDIENTE: %s",
			recorder.Body.String(),
		)
	}
}

func TestReembolsoHandlerGetRechazaIDInvalido(
	t *testing.T,
) {
	service := &fakeReembolsoService{}
	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/abc/reembolsos",
		nil,
	)

	request.SetPathValue(
		"reservaID",
		"abc",
	)

	recorder := httptest.NewRecorder()

	handler.HandleByReservation(
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

	if service.getCalls != 0 {
		t.Error(
			"el Service no debió ser llamado",
		)
	}
}

func TestReembolsoHandlerGetMapeaReservaNoEncontrada(
	t *testing.T,
) {
	service := &fakeReembolsoService{
		getErr: ErrReservaNoEncontrada,
	}

	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/999/reembolsos",
		nil,
	)

	request.SetPathValue(
		"reservaID",
		"999",
	)

	recorder := httptest.NewRecorder()

	handler.HandleByReservation(
		recorder,
		request,
	)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"se esperaba status %d; se obtuvo %d",
			http.StatusNotFound,
			recorder.Code,
		)
	}
}

func TestReembolsoHandlerRechazaMetodoNoPermitido(
	t *testing.T,
) {
	service := &fakeReembolsoService{}
	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/reembolsos",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandleCollection(
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

	if allow := recorder.Header().Get(
		"Allow",
	); allow != "POST" {
		t.Errorf(
			"se esperaba Allow POST; se obtuvo %q",
			allow,
		)
	}
}
