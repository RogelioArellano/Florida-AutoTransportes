package reservas

import (
	"context"
	"errors"
	"testing"
)

type fakeConfirmacionAutomaticaStore struct {
	listPolitica PoliticaConfirmacionParams
	listResult   []ConfirmacionPendiente
	listErr      error
	listCalls    int

	requestParams SolicitarConfirmacionParams
	requestResult ConfirmacionPendiente
	requestErr    error
	requestCalls  int
}

var _ ConfirmacionAutomaticaStore = (*fakeConfirmacionAutomaticaStore)(nil)

func (f *fakeConfirmacionAutomaticaStore) ListPendingConfirmations(
	ctx context.Context,
	politica PoliticaConfirmacionParams,
) ([]ConfirmacionPendiente, error) {
	f.listCalls++
	f.listPolitica = politica

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func (f *fakeConfirmacionAutomaticaStore) RequestConfirmation(
	ctx context.Context,
	params SolicitarConfirmacionParams,
) (ConfirmacionPendiente, error) {
	f.requestCalls++
	f.requestParams = params

	if f.requestErr != nil {
		return ConfirmacionPendiente{}, f.requestErr
	}

	return f.requestResult, nil
}

func TestConfirmacionAutomaticaServiceListUsaPolitica(
	t *testing.T,
) {
	store := &fakeConfirmacionAutomaticaStore{
		listResult: []ConfirmacionPendiente{
			{
				ReservaID:    10,
				ReservaFolio: "RES-00000010",
			},
		},
	}

	service := NewConfirmacionAutomaticaService(store)

	resultado, err := service.ListPendingConfirmations(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"ListPendingConfirmations() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.listCalls != 1 {
		t.Fatalf(
			"Store.ListPendingConfirmations() fue llamado %d veces; se esperaba 1",
			store.listCalls,
		)
	}

	if store.listPolitica.HorasAnticipacion != 3 {
		t.Errorf(
			"HorasAnticipacion = %d; se esperaba 3",
			store.listPolitica.HorasAnticipacion,
		)
	}

	if store.listPolitica.MinutosRespuesta != 90 {
		t.Errorf(
			"MinutosRespuesta = %d; se esperaba 90",
			store.listPolitica.MinutosRespuesta,
		)
	}

	if len(resultado) != 1 || resultado[0].ReservaID != 10 {
		t.Errorf(
			"resultado inesperado: %+v",
			resultado,
		)
	}
}

func TestConfirmacionAutomaticaServiceRequestConfirmation(
	t *testing.T,
) {
	store := &fakeConfirmacionAutomaticaStore{
		requestResult: ConfirmacionPendiente{
			ReservaID:    10,
			ReservaFolio: "RES-00000010",
		},
	}

	service := NewConfirmacionAutomaticaService(store)

	resultado, err := service.RequestConfirmation(
		context.Background(),
		SolicitarConfirmacionInput{
			ReservaID: 10,
		},
	)
	if err != nil {
		t.Fatalf(
			"RequestConfirmation() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.requestCalls != 1 {
		t.Fatalf(
			"Store.RequestConfirmation() fue llamado %d veces; se esperaba 1",
			store.requestCalls,
		)
	}

	params := store.requestParams

	if params.Input.ReservaID != 10 {
		t.Errorf(
			"ReservaID = %d; se esperaba 10",
			params.Input.ReservaID,
		)
	}

	if params.HorasAnticipacion != 3 ||
		params.MinutosRespuesta != 90 {
		t.Errorf(
			"política inesperada: %+v",
			params.PoliticaConfirmacionParams,
		)
	}

	if resultado.ReservaID != 10 {
		t.Errorf(
			"resultado.ReservaID = %d; se esperaba 10",
			resultado.ReservaID,
		)
	}
}

func TestConfirmacionAutomaticaServiceRequestRechazaIDInvalido(
	t *testing.T,
) {
	store := &fakeConfirmacionAutomaticaStore{}
	service := NewConfirmacionAutomaticaService(store)

	_, err := service.RequestConfirmation(
		context.Background(),
		SolicitarConfirmacionInput{
			ReservaID: 0,
		},
	)

	if !errors.Is(err, ErrDatosInvalidos) {
		t.Fatalf(
			"se esperaba ErrDatosInvalidos; se obtuvo %v",
			err,
		)
	}

	if store.requestCalls != 0 {
		t.Error("el Store no debió ser llamado")
	}
}

func TestConfirmacionAutomaticaServicePropagaErrores(
	t *testing.T,
) {
	errEsperado := errors.New(
		"error controlado del repository",
	)

	store := &fakeConfirmacionAutomaticaStore{
		listErr: errEsperado,
	}

	service := NewConfirmacionAutomaticaService(store)

	_, err := service.ListPendingConfirmations(
		context.Background(),
	)

	if !errors.Is(err, errEsperado) {
		t.Fatalf(
			"se esperaba el error del Store; se obtuvo %v",
			err,
		)
	}
}
