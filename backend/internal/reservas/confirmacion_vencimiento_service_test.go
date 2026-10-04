package reservas

import (
	"context"
	"errors"
	"testing"
)

type fakeConfirmacionVencimientoStore struct {
	listResult []ConfirmacionPendiente
	listErr    error
	listCalls  int

	expireParams VencerConfirmacionParams
	expireResult Reserva
	expireErr    error
	expireCalls  int
}

var _ ConfirmacionVencimientoStore = (*fakeConfirmacionVencimientoStore)(nil)

func (f *fakeConfirmacionVencimientoStore) ListExpiredConfirmations(
	ctx context.Context,
) ([]ConfirmacionPendiente, error) {
	f.listCalls++

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func (f *fakeConfirmacionVencimientoStore) ExpireConfirmation(
	ctx context.Context,
	params VencerConfirmacionParams,
) (Reserva, error) {
	f.expireCalls++
	f.expireParams = params

	if f.expireErr != nil {
		return Reserva{}, f.expireErr
	}

	return f.expireResult, nil
}

func TestConfirmacionVencimientoServiceListExpired(
	t *testing.T,
) {
	store := &fakeConfirmacionVencimientoStore{
		listResult: []ConfirmacionPendiente{
			{
				ReservaID:    20,
				ReservaFolio: "RES-00000020",
			},
		},
	}

	service := NewConfirmacionVencimientoService(
		store,
	)

	resultado, err :=
		service.ListExpiredConfirmations(
			context.Background(),
		)
	if err != nil {
		t.Fatalf(
			"ListExpiredConfirmations() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.listCalls != 1 {
		t.Fatalf(
			"Store.ListExpiredConfirmations() fue llamado %d veces; se esperaba 1",
			store.listCalls,
		)
	}

	if len(resultado) != 1 ||
		resultado[0].ReservaID != 20 {
		t.Errorf(
			"resultado inesperado: %+v",
			resultado,
		)
	}
}

func TestConfirmacionVencimientoServiceExpireConfirmation(
	t *testing.T,
) {
	store := &fakeConfirmacionVencimientoStore{
		expireResult: Reserva{
			ID:     20,
			Folio:  "RES-00000020",
			Estado: EstadoCancelada,
		},
	}

	service := NewConfirmacionVencimientoService(
		store,
	)

	resultado, err := service.ExpireConfirmation(
		context.Background(),
		VencerConfirmacionInput{
			ReservaID: 20,
		},
	)
	if err != nil {
		t.Fatalf(
			"ExpireConfirmation() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.expireCalls != 1 {
		t.Fatalf(
			"Store.ExpireConfirmation() fue llamado %d veces; se esperaba 1",
			store.expireCalls,
		)
	}

	if store.expireParams.Input.ReservaID != 20 {
		t.Errorf(
			"ReservaID = %d; se esperaba 20",
			store.expireParams.Input.ReservaID,
		)
	}

	if store.expireParams.MotivoCancelacion !=
		motivoCancelacionConfirmacionVencida {
		t.Errorf(
			"MotivoCancelacion = %q; se esperaba %q",
			store.expireParams.MotivoCancelacion,
			motivoCancelacionConfirmacionVencida,
		)
	}

	if resultado.ID != 20 ||
		resultado.Estado != EstadoCancelada {
		t.Errorf(
			"resultado inesperado: %+v",
			resultado,
		)
	}
}

func TestConfirmacionVencimientoServiceExpireRechazaIDInvalido(
	t *testing.T,
) {
	store := &fakeConfirmacionVencimientoStore{}

	service := NewConfirmacionVencimientoService(
		store,
	)

	_, err := service.ExpireConfirmation(
		context.Background(),
		VencerConfirmacionInput{
			ReservaID: 0,
		},
	)

	if !errors.Is(err, ErrDatosInvalidos) {
		t.Fatalf(
			"se esperaba ErrDatosInvalidos; se obtuvo %v",
			err,
		)
	}

	if store.expireCalls != 0 {
		t.Error(
			"el Store no debió ser llamado",
		)
	}
}

func TestConfirmacionVencimientoServicePropagaErrores(
	t *testing.T,
) {
	errEsperado := errors.New(
		"error controlado del repository",
	)

	store := &fakeConfirmacionVencimientoStore{
		expireErr: errEsperado,
	}

	service := NewConfirmacionVencimientoService(
		store,
	)

	_, err := service.ExpireConfirmation(
		context.Background(),
		VencerConfirmacionInput{
			ReservaID: 20,
		},
	)

	if !errors.Is(err, errEsperado) {
		t.Fatalf(
			"se esperaba el error del Store; se obtuvo %v",
			err,
		)
	}
}
