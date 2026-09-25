package programaciones

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeHorarioStore struct {
	replaceInput  ConfigurarHorariosInput
	replaceResult []HorarioParada
	replaceErr    error
	replaceCalls  int

	listResult []HorarioParada
	listErr    error
	listCalls  int
}

var _ HorarioStore = (*fakeHorarioStore)(nil)

func (f *fakeHorarioStore) Replace(
	ctx context.Context,
	input ConfigurarHorariosInput,
) ([]HorarioParada, error) {
	f.replaceCalls++
	f.replaceInput = input

	if f.replaceErr != nil {
		return nil, f.replaceErr
	}

	return f.replaceResult, nil
}

func (f *fakeHorarioStore) ListByProgramacion(
	ctx context.Context,
	programacionID int64,
) ([]HorarioParada, error) {
	f.listCalls++

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func TestHorarioServiceConfigureNormalizaYGuarda(
	t *testing.T,
) {
	notas := "  Ventana estimada de abordaje  "

	store := &fakeHorarioStore{
		replaceResult: []HorarioParada{
			{
				ProgramacionID:           1,
				RutaParadaID:             4,
				PuntoNombre:              "Pabellón Don Vasco",
				Orden:                    2,
				MinutosDesdeSalidaInicio: 10,
				MinutosDesdeSalidaFin:    15,
			},
		},
	}

	service := NewHorarioService(store)

	resultado, err := service.Configure(
		context.Background(),
		ConfigurarHorariosInput{
			ProgramacionID: 1,
			Paradas: []ConfigurarHorarioParadaInput{
				{
					RutaParadaID:             3,
					MinutosDesdeSalidaInicio: 0,
					MinutosDesdeSalidaFin:    0,
				},
				{
					RutaParadaID:             4,
					MinutosDesdeSalidaInicio: 10,
					MinutosDesdeSalidaFin:    15,
					Notas:                    &notas,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"Configure() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.replaceCalls != 1 {
		t.Fatalf(
			"Store.Replace() fue llamado %d veces; se esperaba 1",
			store.replaceCalls,
		)
	}

	if store.replaceInput.ProgramacionID != 1 {
		t.Errorf(
			"ProgramacionID recibido = %d; se esperaba 1",
			store.replaceInput.ProgramacionID,
		)
	}

	if len(store.replaceInput.Paradas) != 2 {
		t.Fatalf(
			"se esperaban 2 paradas, se obtuvieron %d",
			len(store.replaceInput.Paradas),
		)
	}

	notasRecibidas :=
		store.replaceInput.Paradas[1].Notas

	if notasRecibidas == nil ||
		*notasRecibidas !=
			"Ventana estimada de abordaje" {
		t.Errorf(
			"las notas no fueron normalizadas correctamente",
		)
	}

	// El valor original no debe modificarse porque el servicio
	// construye un nuevo slice de paradas.
	if notas != "  Ventana estimada de abordaje  " {
		t.Error(
			"el servicio modificó directamente el valor original",
		)
	}

	if len(resultado) != 1 {
		t.Fatalf(
			"se esperaba un horario, se obtuvieron %d",
			len(resultado),
		)
	}
}

func TestHorarioServiceConfigureValidaciones(
	t *testing.T,
) {
	inputValido := func() ConfigurarHorariosInput {
		return ConfigurarHorariosInput{
			ProgramacionID: 1,
			Paradas: []ConfigurarHorarioParadaInput{
				{
					RutaParadaID:             3,
					MinutosDesdeSalidaInicio: 0,
					MinutosDesdeSalidaFin:    0,
				},
				{
					RutaParadaID:             4,
					MinutosDesdeSalidaInicio: 10,
					MinutosDesdeSalidaFin:    15,
				},
			},
		}
	}

	pruebas := []struct {
		nombre    string
		modificar func(*ConfigurarHorariosInput)
	}{
		{
			nombre: "programación inválida",
			modificar: func(input *ConfigurarHorariosInput) {
				input.ProgramacionID = 0
			},
		},
		{
			nombre: "sin paradas",
			modificar: func(input *ConfigurarHorariosInput) {
				input.Paradas = nil
			},
		},
		{
			nombre: "identificador de parada inválido",
			modificar: func(input *ConfigurarHorariosInput) {
				input.Paradas[0].RutaParadaID = 0
			},
		},
		{
			nombre: "parada repetida",
			modificar: func(input *ConfigurarHorariosInput) {
				input.Paradas[1].RutaParadaID =
					input.Paradas[0].RutaParadaID
			},
		},
		{
			nombre: "minutos iniciales negativos",
			modificar: func(input *ConfigurarHorariosInput) {
				input.Paradas[0].
					MinutosDesdeSalidaInicio = -1
			},
		},
		{
			nombre: "minutos finales menores",
			modificar: func(input *ConfigurarHorariosInput) {
				input.Paradas[1].
					MinutosDesdeSalidaInicio = 15

				input.Paradas[1].
					MinutosDesdeSalidaFin = 10
			},
		},
		{
			nombre: "minutos fuera del rango SMALLINT",
			modificar: func(input *ConfigurarHorariosInput) {
				input.Paradas[1].
					MinutosDesdeSalidaFin = 32768
			},
		},
		{
			nombre: "notas demasiado largas",
			modificar: func(input *ConfigurarHorariosInput) {
				notas := strings.Repeat("A", 251)
				input.Paradas[0].Notas = &notas
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				input := inputValido()
				prueba.modificar(&input)

				store := &fakeHorarioStore{}
				service := NewHorarioService(store)

				_, err := service.Configure(
					context.Background(),
					input,
				)

				if err == nil {
					t.Fatal(
						"Configure() no devolvió el error esperado",
					)
				}

				if !errors.Is(err, ErrDatosInvalidos) {
					t.Fatalf(
						"Error obtenido = %v; se esperaba ErrDatosInvalidos",
						err,
					)
				}

				if store.replaceCalls != 0 {
					t.Errorf(
						"Store.Replace() fue llamado %d veces; se esperaba 0",
						store.replaceCalls,
					)
				}
			},
		)
	}
}

func TestHorarioServiceConfigurePropagaError(
	t *testing.T,
) {
	store := &fakeHorarioStore{
		replaceErr: ErrParadaNoPertenece,
	}

	service := NewHorarioService(store)

	_, err := service.Configure(
		context.Background(),
		ConfigurarHorariosInput{
			ProgramacionID: 1,
			Paradas: []ConfigurarHorarioParadaInput{
				{
					RutaParadaID:             999,
					MinutosDesdeSalidaInicio: 0,
					MinutosDesdeSalidaFin:    0,
				},
			},
		},
	)

	if !errors.Is(err, ErrParadaNoPertenece) {
		t.Fatalf(
			"Error obtenido = %v; se esperaba ErrParadaNoPertenece",
			err,
		)
	}

	if store.replaceCalls != 1 {
		t.Errorf(
			"Store.Replace() fue llamado %d veces; se esperaba 1",
			store.replaceCalls,
		)
	}
}

func TestHorarioServiceList(
	t *testing.T,
) {
	store := &fakeHorarioStore{
		listResult: []HorarioParada{
			{
				ProgramacionID:           1,
				RutaParadaID:             3,
				PuntoNombre:              "Morelia Centro",
				Orden:                    1,
				MinutosDesdeSalidaInicio: 0,
				MinutosDesdeSalidaFin:    0,
				HoraEstimadaInicio:       "06:00",
				HoraEstimadaFin:          "06:00",
			},
		},
	}

	service := NewHorarioService(store)

	resultado, err := service.List(
		context.Background(),
		1,
	)
	if err != nil {
		t.Fatalf(
			"List() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.listCalls != 1 {
		t.Errorf(
			"Store.ListByProgramacion() fue llamado %d veces; se esperaba 1",
			store.listCalls,
		)
	}

	if len(resultado) != 1 {
		t.Fatalf(
			"se esperaba un horario, se obtuvieron %d",
			len(resultado),
		)
	}

	if resultado[0].HoraEstimadaInicio != "06:00" {
		t.Errorf(
			"Hora obtenida = %q; se esperaba 06:00",
			resultado[0].HoraEstimadaInicio,
		)
	}
}

func TestHorarioServiceListRechazaIDInvalido(
	t *testing.T,
) {
	store := &fakeHorarioStore{}
	service := NewHorarioService(store)

	_, err := service.List(
		context.Background(),
		0,
	)

	if !errors.Is(err, ErrDatosInvalidos) {
		t.Fatalf(
			"Error obtenido = %v; se esperaba ErrDatosInvalidos",
			err,
		)
	}

	if store.listCalls != 0 {
		t.Errorf(
			"Store.ListByProgramacion() fue llamado %d veces; se esperaba 0",
			store.listCalls,
		)
	}
}
