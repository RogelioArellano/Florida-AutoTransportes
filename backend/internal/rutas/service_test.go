package rutas

import (
	"context"
	"errors"
	"testing"
)

// fakeStore es una implementación simulada del Store.
// Nos permite probar el Service sin utilizar PostgreSQL.
type fakeStore struct {
	createFn func(
		ctx context.Context,
		input CreateInput,
	) (Ruta, error)
}

func (f *fakeStore) Create(
	ctx context.Context,
	input CreateInput,
) (Ruta, error) {
	if f.createFn == nil {
		return Ruta{}, errors.New(
			"fakeStore.Create no fue configurado",
		)
	}

	return f.createFn(ctx, input)
}

func (f *fakeStore) List(
	ctx context.Context,
) ([]Ruta, error) {
	return []Ruta{}, nil
}

func TestServiceCreateNormalizaRuta(t *testing.T) {
	var inputRecibido CreateInput

	store := &fakeStore{
		createFn: func(
			ctx context.Context,
			input CreateInput,
		) (Ruta, error) {
			inputRecibido = input

			return Ruta{
				ID:      1,
				Codigo:  input.Codigo,
				Nombre:  input.Nombre,
				Activa:  true,
				Paradas: []RutaParada{},
			}, nil
		},
	}

	service := NewService(store)

	input := CreateInput{
		Codigo: "  mor-cdmx  ",
		Nombre: "  Morelia a Ciudad de México  ",
		Paradas: []CreateParadaInput{
			{
				PuntoAbordajeID: 1,
				PermiteSubir:    true,
				PermiteBajar:    false,
				EsObligatoria:   false,
			},
			{
				PuntoAbordajeID: 2,
				PermiteSubir:    true,
				PermiteBajar:    true,
				EsObligatoria:   false,
			},
			{
				PuntoAbordajeID: 3,
				PermiteSubir:    false,
				PermiteBajar:    true,
				EsObligatoria:   false,
			},
		},
	}

	ruta, err := service.Create(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatalf("Create devolvió un error inesperado: %v", err)
	}

	if ruta.Codigo != "MOR-CDMX" {
		t.Errorf(
			"se esperaba MOR-CDMX, se obtuvo %q",
			ruta.Codigo,
		)
	}

	if inputRecibido.Nombre != "Morelia a Ciudad de México" {
		t.Errorf(
			"el nombre no fue normalizado: %q",
			inputRecibido.Nombre,
		)
	}

	if !inputRecibido.Paradas[0].EsObligatoria {
		t.Error(
			"la primera parada debería ser obligatoria",
		)
	}

	if inputRecibido.Paradas[1].EsObligatoria {
		t.Error(
			"la parada intermedia no debería haberse modificado",
		)
	}

	if !inputRecibido.Paradas[2].EsObligatoria {
		t.Error(
			"la última parada debería ser obligatoria",
		)
	}
}

func TestServiceCreateRechazaDatosInvalidos(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		input  CreateInput
	}{
		{
			nombre: "código vacío",
			input: CreateInput{
				Nombre:  "Morelia a México",
				Paradas: paradasValidas(),
			},
		},
		{
			nombre: "nombre vacío",
			input: CreateInput{
				Codigo:  "MOR-CDMX",
				Paradas: paradasValidas(),
			},
		},
		{
			nombre: "solo una parada",
			input: CreateInput{
				Codigo: "MOR-CDMX",
				Nombre: "Morelia a México",
				Paradas: []CreateParadaInput{
					{
						PuntoAbordajeID: 1,
						PermiteSubir:    true,
					},
				},
			},
		},
		{
			nombre: "punto repetido",
			input: CreateInput{
				Codigo: "MOR-CDMX",
				Nombre: "Morelia a México",
				Paradas: []CreateParadaInput{
					{
						PuntoAbordajeID: 1,
						PermiteSubir:    true,
					},
					{
						PuntoAbordajeID: 1,
						PermiteBajar:    true,
					},
				},
			},
		},
		{
			nombre: "parada sin operación",
			input: CreateInput{
				Codigo: "MOR-CDMX",
				Nombre: "Morelia a México",
				Paradas: []CreateParadaInput{
					{
						PuntoAbordajeID: 1,
						PermiteSubir:    true,
					},
					{
						PuntoAbordajeID: 2,
						PermiteSubir:    false,
						PermiteBajar:    false,
					},
				},
			},
		},
		{
			nombre: "primera parada no permite subir",
			input: CreateInput{
				Codigo: "MOR-CDMX",
				Nombre: "Morelia a México",
				Paradas: []CreateParadaInput{
					{
						PuntoAbordajeID: 1,
						PermiteBajar:    true,
					},
					{
						PuntoAbordajeID: 2,
						PermiteBajar:    true,
					},
				},
			},
		},
		{
			nombre: "última parada no permite bajar",
			input: CreateInput{
				Codigo: "MOR-CDMX",
				Nombre: "Morelia a México",
				Paradas: []CreateParadaInput{
					{
						PuntoAbordajeID: 1,
						PermiteSubir:    true,
					},
					{
						PuntoAbordajeID: 2,
						PermiteSubir:    true,
					},
				},
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(prueba.nombre, func(t *testing.T) {
			store := &fakeStore{
				createFn: func(
					ctx context.Context,
					input CreateInput,
				) (Ruta, error) {
					t.Fatal(
						"el Store no debería ejecutarse con datos inválidos",
					)

					return Ruta{}, nil
				},
			}

			service := NewService(store)

			_, err := service.Create(
				context.Background(),
				prueba.input,
			)

			if !errors.Is(err, ErrDatosInvalidos) {
				t.Fatalf(
					"se esperaba ErrDatosInvalidos, se obtuvo %v",
					err,
				)
			}
		})
	}
}

func paradasValidas() []CreateParadaInput {
	return []CreateParadaInput{
		{
			PuntoAbordajeID: 1,
			PermiteSubir:    true,
			PermiteBajar:    false,
		},
		{
			PuntoAbordajeID: 2,
			PermiteSubir:    false,
			PermiteBajar:    true,
		},
	}
}
