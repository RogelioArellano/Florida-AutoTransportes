package reservas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// VoidRefund completa el contrato nuevo para el fake utilizado
// por las pruebas existentes de ReembolsoService.
func (f *fakeReembolsoStore) VoidRefund(
	ctx context.Context,
	input AnularReembolsoInput,
) (ReembolsosReserva, error) {
	return ReembolsosReserva{}, nil
}

type fakeAnulacionReembolsoStore struct {
	input  AnularReembolsoInput
	result ReembolsosReserva
	err    error
	calls  int
}

var _ ReembolsoStore = (*fakeAnulacionReembolsoStore)(nil)

func (f *fakeAnulacionReembolsoStore) RegisterRefund(
	ctx context.Context,
	input RegistrarReembolsoInput,
) (ReembolsosReserva, error) {
	return ReembolsosReserva{}, nil
}

func (f *fakeAnulacionReembolsoStore) GetRefundsByReservation(
	ctx context.Context,
	reservaID int64,
) (ReembolsosReserva, error) {
	return ReembolsosReserva{}, nil
}

func (f *fakeAnulacionReembolsoStore) VoidRefund(
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

func TestReembolsoServiceVoidRefundNormalizaMotivo(
	t *testing.T,
) {
	store := &fakeAnulacionReembolsoStore{
		result: ReembolsosReserva{
			ReservaID:          5,
			MontoReembolsado:   Dinero("100.00"),
			SaldoPorReembolsar: Dinero("170.00"),
			Estado:             EstadoReembolsoParcial,
		},
	}

	service := NewReembolsoService(store)

	resultado, err := service.VoidRefund(
		context.Background(),
		AnularReembolsoInput{
			ReembolsoID: 2,
			Motivo:      "  Transferencia no realizada  ",
		},
	)
	if err != nil {
		t.Fatalf(
			"VoidRefund() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.calls != 1 {
		t.Fatalf(
			"Store.VoidRefund() fue llamado %d veces; se esperaba 1",
			store.calls,
		)
	}

	if store.input.ReembolsoID != 2 {
		t.Errorf(
			"ReembolsoID = %d; se esperaba 2",
			store.input.ReembolsoID,
		)
	}

	if store.input.Motivo !=
		"Transferencia no realizada" {
		t.Errorf(
			"Motivo = %q; no fue normalizado",
			store.input.Motivo,
		)
	}

	if resultado.SaldoPorReembolsar !=
		Dinero("170.00") {
		t.Errorf(
			"SaldoPorReembolsar = %q; se esperaba 170.00",
			resultado.SaldoPorReembolsar,
		)
	}
}

func TestReembolsoServiceVoidRefundRechazaDatosInvalidos(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		input  AnularReembolsoInput
	}{
		{
			nombre: "identificador inválido",
			input: AnularReembolsoInput{
				ReembolsoID: 0,
				Motivo:      "Captura incorrecta",
			},
		},
		{
			nombre: "motivo vacío",
			input: AnularReembolsoInput{
				ReembolsoID: 2,
				Motivo:      "   ",
			},
		},
		{
			nombre: "motivo demasiado largo",
			input: AnularReembolsoInput{
				ReembolsoID: 2,
				Motivo: strings.Repeat(
					"M",
					501,
				),
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				store :=
					&fakeAnulacionReembolsoStore{}

				service :=
					NewReembolsoService(store)

				_, err := service.VoidRefund(
					context.Background(),
					prueba.input,
				)

				if !errors.Is(
					err,
					ErrDatosInvalidos,
				) {
					t.Fatalf(
						"se esperaba ErrDatosInvalidos; se obtuvo %v",
						err,
					)
				}

				if store.calls != 0 {
					t.Error(
						"el Store no debió ser llamado",
					)
				}
			},
		)
	}
}

func TestReembolsoServiceVoidRefundPropagaError(
	t *testing.T,
) {
	errEsperado := errors.New(
		"error controlado del repository",
	)

	store := &fakeAnulacionReembolsoStore{
		err: errEsperado,
	}

	service := NewReembolsoService(store)

	_, err := service.VoidRefund(
		context.Background(),
		AnularReembolsoInput{
			ReembolsoID: 2,
			Motivo:      "Captura incorrecta",
		},
	)

	if !errors.Is(err, errEsperado) {
		t.Fatalf(
			"se esperaba el error del Store; se obtuvo %v",
			err,
		)
	}
}
