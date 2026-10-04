package reservas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeReembolsoStore struct {
	registerInput  RegistrarReembolsoInput
	registerResult ReembolsosReserva
	registerErr    error
	registerCalls  int

	getReservaID int64
	getResult    ReembolsosReserva
	getErr       error
	getCalls     int
}

var _ ReembolsoStore = (*fakeReembolsoStore)(nil)

func (f *fakeReembolsoStore) RegisterRefund(
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

func (f *fakeReembolsoStore) GetRefundsByReservation(
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

func TestReembolsoServiceRegisterRefundNormalizaInput(
	t *testing.T,
) {
	referencia := "  TRANSFERENCIA-DEV-001  "
	notas := "  Reembolso solicitado por el pasajero  "

	store := &fakeReembolsoStore{
		registerResult: ReembolsosReserva{
			ReservaID:          5,
			MontoReembolsable:  Dinero("270.00"),
			MontoReembolsado:   Dinero("100.00"),
			SaldoPorReembolsar: Dinero("170.00"),
			Estado:             EstadoReembolsoParcial,
			Reembolsos:         []Reembolso{},
		},
	}

	service := NewReembolsoService(store)

	resultado, err := service.RegisterRefund(
		context.Background(),
		RegistrarReembolsoInput{
			ReservaID:  5,
			Monto:      Dinero("100"),
			Metodo:     MetodoReembolso(" transferencia "),
			Referencia: &referencia,
			Notas:      &notas,
		},
	)
	if err != nil {
		t.Fatalf(
			"RegisterRefund() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.registerCalls != 1 {
		t.Fatalf(
			"Store.RegisterRefund() fue llamado %d veces; se esperaba 1",
			store.registerCalls,
		)
	}

	input := store.registerInput

	if input.Monto != Dinero("100.00") {
		t.Errorf(
			"Monto = %q; se esperaba 100.00",
			input.Monto,
		)
	}

	if input.Metodo !=
		MetodoReembolsoTransferencia {
		t.Errorf(
			"Método = %q; se esperaba TRANSFERENCIA",
			input.Metodo,
		)
	}

	if input.Referencia == nil ||
		*input.Referencia !=
			"TRANSFERENCIA-DEV-001" {
		t.Error(
			"la referencia no fue normalizada correctamente",
		)
	}

	if input.Notas == nil ||
		*input.Notas !=
			"Reembolso solicitado por el pasajero" {
		t.Error(
			"las notas no fueron normalizadas correctamente",
		)
	}

	if resultado.Estado !=
		EstadoReembolsoParcial {
		t.Errorf(
			"Estado = %q; se esperaba PARCIAL",
			resultado.Estado,
		)
	}
}

func TestReembolsoServiceRegisterRefundRechazaDatosInvalidos(
	t *testing.T,
) {
	referenciaLarga := strings.Repeat("R", 151)
	notasLargas := strings.Repeat("N", 1001)

	pruebas := []struct {
		nombre string
		input  RegistrarReembolsoInput
	}{
		{
			nombre: "reserva inválida",
			input: RegistrarReembolsoInput{
				ReservaID: 0,
				Monto:     Dinero("100.00"),
				Metodo:    MetodoReembolsoEfectivo,
			},
		},
		{
			nombre: "monto vacío",
			input: RegistrarReembolsoInput{
				ReservaID: 5,
				Monto:     Dinero(""),
				Metodo:    MetodoReembolsoEfectivo,
			},
		},
		{
			nombre: "monto cero",
			input: RegistrarReembolsoInput{
				ReservaID: 5,
				Monto:     Dinero("0.00"),
				Metodo:    MetodoReembolsoEfectivo,
			},
		},
		{
			nombre: "monto con tres decimales",
			input: RegistrarReembolsoInput{
				ReservaID: 5,
				Monto:     Dinero("100.001"),
				Metodo:    MetodoReembolsoEfectivo,
			},
		},
		{
			nombre: "método inválido",
			input: RegistrarReembolsoInput{
				ReservaID: 5,
				Monto:     Dinero("100.00"),
				Metodo:    MetodoReembolso("CHEQUE"),
			},
		},
		{
			nombre: "referencia demasiado larga",
			input: RegistrarReembolsoInput{
				ReservaID:  5,
				Monto:      Dinero("100.00"),
				Metodo:     MetodoReembolsoEfectivo,
				Referencia: &referenciaLarga,
			},
		},
		{
			nombre: "notas demasiado largas",
			input: RegistrarReembolsoInput{
				ReservaID: 5,
				Monto:     Dinero("100.00"),
				Metodo:    MetodoReembolsoEfectivo,
				Notas:     &notasLargas,
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				store := &fakeReembolsoStore{}
				service := NewReembolsoService(store)

				_, err := service.RegisterRefund(
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

				if store.registerCalls != 0 {
					t.Error(
						"el Store no debió ser llamado",
					)
				}
			},
		)
	}
}

func TestReembolsoServiceRegisterRefundPropagaError(
	t *testing.T,
) {
	errEsperado := errors.New(
		"error controlado del repository",
	)

	store := &fakeReembolsoStore{
		registerErr: errEsperado,
	}

	service := NewReembolsoService(store)

	_, err := service.RegisterRefund(
		context.Background(),
		RegistrarReembolsoInput{
			ReservaID: 5,
			Monto:     Dinero("100.00"),
			Metodo:    MetodoReembolsoEfectivo,
		},
	)

	if !errors.Is(err, errEsperado) {
		t.Fatalf(
			"se esperaba el error del Store; se obtuvo %v",
			err,
		)
	}
}

func TestReembolsoServiceGetRefundsByReservation(
	t *testing.T,
) {
	store := &fakeReembolsoStore{
		getResult: ReembolsosReserva{
			ReservaID:          5,
			Estado:             EstadoReembolsoPendiente,
			SaldoPorReembolsar: Dinero("270.00"),
			Reembolsos:         []Reembolso{},
		},
	}

	service := NewReembolsoService(store)

	resultado, err :=
		service.GetRefundsByReservation(
			context.Background(),
			5,
		)
	if err != nil {
		t.Fatalf(
			"GetRefundsByReservation() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.getCalls != 1 ||
		store.getReservaID != 5 {
		t.Error(
			"el Store no recibió reserva_id 5",
		)
	}

	if resultado.SaldoPorReembolsar !=
		Dinero("270.00") {
		t.Errorf(
			"SaldoPorReembolsar = %q; se esperaba 270.00",
			resultado.SaldoPorReembolsar,
		)
	}
}

func TestReembolsoServiceGetRefundsRechazaIDInvalido(
	t *testing.T,
) {
	store := &fakeReembolsoStore{}
	service := NewReembolsoService(store)

	_, err := service.GetRefundsByReservation(
		context.Background(),
		0,
	)

	if !errors.Is(err, ErrDatosInvalidos) {
		t.Fatalf(
			"se esperaba ErrDatosInvalidos; se obtuvo %v",
			err,
		)
	}

	if store.getCalls != 0 {
		t.Error(
			"el Store no debió ser llamado",
		)
	}
}
