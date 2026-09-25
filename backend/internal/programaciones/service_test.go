package programaciones

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeStore struct {
	createInput  CreateInput
	createResult Programacion
	createErr    error
	createCalls  int

	listResult []Programacion
	listErr    error
	listCalls  int
}

var _ Store = (*fakeStore)(nil)

func (f *fakeStore) Create(
	ctx context.Context,
	input CreateInput,
) (Programacion, error) {
	f.createCalls++
	f.createInput = input

	if f.createErr != nil {
		return Programacion{}, f.createErr
	}

	return f.createResult, nil
}

func (f *fakeStore) List(
	ctx context.Context,
) ([]Programacion, error) {
	f.listCalls++

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func TestServiceCreateNormalizaYGuarda(
	t *testing.T,
) {
	choferID := int64(1)
	duracion := 240
	vigenciaHasta := " 2027-12-31 "

	store := &fakeStore{
		createResult: Programacion{
			ID:     1,
			Codigo: "MOR-CDMX-0600",
			Activa: true,
		},
	}

	service := NewService(store)

	diasOriginales := []int{7, 1, 5, 3, 2, 6, 4}

	resultado, err := service.Create(
		context.Background(),
		CreateInput{
			Codigo:                  " mor-cdmx-0600 ",
			Nombre:                  " Salida matutina ",
			RutaID:                  1,
			UnidadID:                1,
			ChoferID:                &choferID,
			HoraSalida:              " 06:00 ",
			DuracionEstimadaMinutos: &duracion,
			VigenciaDesde:           " 2026-09-25 ",
			VigenciaHasta:           &vigenciaHasta,
			DiasSemana:              diasOriginales,
		},
	)
	if err != nil {
		t.Fatalf(
			"Create() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.createCalls != 1 {
		t.Fatalf(
			"Store.Create() fue llamado %d veces; se esperaba 1",
			store.createCalls,
		)
	}

	if store.createInput.Codigo != "MOR-CDMX-0600" {
		t.Errorf(
			"Código recibido = %q",
			store.createInput.Codigo,
		)
	}

	if store.createInput.Nombre != "Salida matutina" {
		t.Errorf(
			"Nombre recibido = %q",
			store.createInput.Nombre,
		)
	}

	if store.createInput.HoraSalida != "06:00" {
		t.Errorf(
			"Hora recibida = %q",
			store.createInput.HoraSalida,
		)
	}

	if store.createInput.VigenciaDesde != "2026-09-25" {
		t.Errorf(
			"Vigencia inicial recibida = %q",
			store.createInput.VigenciaDesde,
		)
	}

	if store.createInput.VigenciaHasta == nil ||
		*store.createInput.VigenciaHasta != "2027-12-31" {
		t.Errorf(
			"no se normalizó correctamente vigencia_hasta",
		)
	}

	diasEsperados := []int{1, 2, 3, 4, 5, 6, 7}

	for indice, dia := range diasEsperados {
		if store.createInput.DiasSemana[indice] != dia {
			t.Errorf(
				"Día en posición %d = %d; se esperaba %d",
				indice,
				store.createInput.DiasSemana[indice],
				dia,
			)
		}
	}

	// El slice original no debe haber sido modificado.
	if diasOriginales[0] != 7 {
		t.Error(
			"el servicio modificó directamente el slice original",
		)
	}

	if resultado.ID != 1 {
		t.Errorf(
			"ID obtenido = %d; se esperaba 1",
			resultado.ID,
		)
	}
}

func TestServiceCreateValidaciones(
	t *testing.T,
) {
	choferInvalido := int64(0)
	duracionInvalida := 0
	duracionExcesiva := 32768
	fechaAnterior := "2026-09-24"

	inputValido := func() CreateInput {
		return CreateInput{
			Codigo:        "MOR-CDMX-0600",
			Nombre:        "Salida matutina",
			RutaID:        1,
			UnidadID:      1,
			HoraSalida:    "06:00",
			VigenciaDesde: "2026-09-25",
			DiasSemana:    []int{1, 2, 3, 4, 5, 6, 7},
		}
	}

	pruebas := []struct {
		nombre    string
		modificar func(*CreateInput)
	}{
		{
			nombre: "código vacío",
			modificar: func(input *CreateInput) {
				input.Codigo = "   "
			},
		},
		{
			nombre: "código demasiado largo",
			modificar: func(input *CreateInput) {
				input.Codigo = strings.Repeat("A", 31)
			},
		},
		{
			nombre: "nombre vacío",
			modificar: func(input *CreateInput) {
				input.Nombre = ""
			},
		},
		{
			nombre: "nombre demasiado largo",
			modificar: func(input *CreateInput) {
				input.Nombre = strings.Repeat("A", 151)
			},
		},
		{
			nombre: "ruta inválida",
			modificar: func(input *CreateInput) {
				input.RutaID = 0
			},
		},
		{
			nombre: "unidad inválida",
			modificar: func(input *CreateInput) {
				input.UnidadID = 0
			},
		},
		{
			nombre: "chofer inválido",
			modificar: func(input *CreateInput) {
				input.ChoferID = &choferInvalido
			},
		},
		{
			nombre: "hora vacía",
			modificar: func(input *CreateInput) {
				input.HoraSalida = ""
			},
		},
		{
			nombre: "hora inexistente",
			modificar: func(input *CreateInput) {
				input.HoraSalida = "25:00"
			},
		},
		{
			nombre: "formato de hora incorrecto",
			modificar: func(input *CreateInput) {
				input.HoraSalida = "6:00"
			},
		},
		{
			nombre: "duración en cero",
			modificar: func(input *CreateInput) {
				input.DuracionEstimadaMinutos =
					&duracionInvalida
			},
		},
		{
			nombre: "duración excesiva",
			modificar: func(input *CreateInput) {
				input.DuracionEstimadaMinutos =
					&duracionExcesiva
			},
		},
		{
			nombre: "vigencia inicial vacía",
			modificar: func(input *CreateInput) {
				input.VigenciaDesde = ""
			},
		},
		{
			nombre: "vigencia inicial inválida",
			modificar: func(input *CreateInput) {
				input.VigenciaDesde = "2026-02-30"
			},
		},
		{
			nombre: "vigencia final inválida",
			modificar: func(input *CreateInput) {
				fecha := "31-12-2027"
				input.VigenciaHasta = &fecha
			},
		},
		{
			nombre: "vigencia final anterior",
			modificar: func(input *CreateInput) {
				input.VigenciaHasta = &fechaAnterior
			},
		},
		{
			nombre: "sin días",
			modificar: func(input *CreateInput) {
				input.DiasSemana = nil
			},
		},
		{
			nombre: "día fuera de rango",
			modificar: func(input *CreateInput) {
				input.DiasSemana = []int{1, 8}
			},
		},
		{
			nombre: "día repetido",
			modificar: func(input *CreateInput) {
				input.DiasSemana = []int{1, 2, 2}
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				input := inputValido()
				prueba.modificar(&input)

				store := &fakeStore{}
				service := NewService(store)

				_, err := service.Create(
					context.Background(),
					input,
				)

				if err == nil {
					t.Fatal(
						"Create() no devolvió el error esperado",
					)
				}

				if !errors.Is(err, ErrDatosInvalidos) {
					t.Fatalf(
						"Error obtenido = %v; se esperaba ErrDatosInvalidos",
						err,
					)
				}

				if store.createCalls != 0 {
					t.Errorf(
						"Store.Create() fue llamado %d veces; se esperaba 0",
						store.createCalls,
					)
				}
			},
		)
	}
}

func TestServiceCreatePropagaErrorRepositorio(
	t *testing.T,
) {
	store := &fakeStore{
		createErr: ErrRutaNoDisponible,
	}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			Codigo:        "MOR-CDMX-0600",
			Nombre:        "Salida matutina",
			RutaID:        1,
			UnidadID:      1,
			HoraSalida:    "06:00",
			VigenciaDesde: "2026-09-25",
			DiasSemana:    []int{1, 2, 3, 4, 5, 6, 7},
		},
	)

	if !errors.Is(err, ErrRutaNoDisponible) {
		t.Fatalf(
			"Error obtenido = %v; se esperaba ErrRutaNoDisponible",
			err,
		)
	}

	if store.createCalls != 1 {
		t.Errorf(
			"Store.Create() fue llamado %d veces; se esperaba 1",
			store.createCalls,
		)
	}
}

func TestServiceListRetornaProgramaciones(
	t *testing.T,
) {
	store := &fakeStore{
		listResult: []Programacion{
			{
				ID:           1,
				Codigo:       "MOR-CDMX-0600",
				Nombre:       "Salida matutina",
				RutaID:       1,
				RutaCodigo:   "MOR-CDMX",
				UnidadID:     1,
				UnidadCodigo: "UNIDAD-01",
				HoraSalida:   "06:00",
				DiasSemana:   []int{1, 2, 3, 4, 5, 6, 7},
				Activa:       true,
			},
		},
	}

	service := NewService(store)

	resultado, err := service.List(
		context.Background(),
	)
	if err != nil {
		t.Fatalf(
			"List() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.listCalls != 1 {
		t.Errorf(
			"Store.List() fue llamado %d veces; se esperaba 1",
			store.listCalls,
		)
	}

	if len(resultado) != 1 {
		t.Fatalf(
			"List() devolvió %d programaciones; se esperaba 1",
			len(resultado),
		)
	}

	if resultado[0].Codigo != "MOR-CDMX-0600" {
		t.Errorf(
			"Código obtenido = %q",
			resultado[0].Codigo,
		)
	}
}
