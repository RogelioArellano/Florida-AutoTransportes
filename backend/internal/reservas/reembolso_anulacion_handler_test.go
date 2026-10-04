package reservas

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// VoidRefund completa el contrato nuevo para el fake utilizado
// por las pruebas existentes de ReembolsoHandler.
func (f *fakeReembolsoService) VoidRefund(
	ctx context.Context,
	input AnularReembolsoInput,
) (ReembolsosReserva, error) {
	return ReembolsosReserva{}, nil
}

type fakeAnulacionReembolsoService struct {
	input  AnularReembolsoInput
	result ReembolsosReserva
	err    error
	calls  int
}

var _ reembolsoService = (*fakeAnulacionReembolsoService)(nil)

func (f *fakeAnulacionReembolsoService) RegisterRefund(
	ctx context.Context,
	input RegistrarReembolsoInput,
) (ReembolsosReserva, error) {
	return ReembolsosReserva{}, nil
}

func (f *fakeAnulacionReembolsoService) GetRefundsByReservation(
	ctx context.Context,
	reservaID int64,
) (ReembolsosReserva, error) {
	return ReembolsosReserva{}, nil
}

func (f *fakeAnulacionReembolsoService) VoidRefund(
	ctx context.Context,
	input AnularReembolsoInput,
) (ReembolsosReserva, error) {
	f.calls++
	f.input = input

	if f.err != nil {
		return ReembolsosReserva{}, f.err
	}

	return f.result, nil
}

func TestReembolsoHandlerVoidRefundAnulaMovimiento(
	t *testing.T,
) {
	service := &fakeAnulacionReembolsoService{
		result: ReembolsosReserva{
			ReservaID:          5,
			MontoReembolsable:  Dinero("270.00"),
			MontoReembolsado:   Dinero("100.00"),
			SaldoPorReembolsar: Dinero("170.00"),
			Estado:             EstadoReembolsoParcial,
			Reembolsos:         []Reembolso{},
		},
	}

	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/reembolsos/anulaciones",
		bytes.NewBufferString(`{
			"reembolso_id": 2,
			"motivo": "Transferencia no realizada"
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleVoids(
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

	if service.calls != 1 {
		t.Fatalf(
			"VoidRefund() fue llamado %d veces; se esperaba 1",
			service.calls,
		)
	}

	if service.input.ReembolsoID != 2 {
		t.Errorf(
			"ReembolsoID = %d; se esperaba 2",
			service.input.ReembolsoID,
		)
	}

	if service.input.Motivo !=
		"Transferencia no realizada" {
		t.Errorf(
			"Motivo = %q; no coincide",
			service.input.Motivo,
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

func TestReembolsoHandlerVoidRefundRechazaJSONInvalido(
	t *testing.T,
) {
	service := &fakeAnulacionReembolsoService{}
	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/reembolsos/anulaciones",
		bytes.NewBufferString(`{
			"reembolso_id": 2,
			"campo_desconocido": true
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleVoids(
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

	if service.calls != 0 {
		t.Error(
			"el Service no debió ser llamado",
		)
	}
}

func TestReembolsoHandlerVoidRefundMapeaNoEncontrado(
	t *testing.T,
) {
	service := &fakeAnulacionReembolsoService{
		err: ErrReembolsoNoEncontrado,
	}

	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/reembolsos/anulaciones",
		bytes.NewBufferString(`{
			"reembolso_id": 999,
			"motivo": "Prueba de movimiento inexistente"
		}`),
	)

	recorder := httptest.NewRecorder()

	handler.HandleVoids(
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

func TestReembolsoHandlerVoidRefundRechazaMetodo(
	t *testing.T,
) {
	service := &fakeAnulacionReembolsoService{}
	handler := NewReembolsoHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/reembolsos/anulaciones",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandleVoids(
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
