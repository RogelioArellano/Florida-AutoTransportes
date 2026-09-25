package corridas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeStore struct {
	generateInput  GenerarInput
	generateResult Corrida
	generateErr    error
	generateCalls  int

	listFilter ListFilter
	listResult []Corrida
	listErr    error
	listCalls  int
}

var _ Store = (*fakeStore)(nil)

func (f *fakeStore) Generate(
	ctx context.Context,
	input GenerarInput,
) (Corrida, error) {
	f.generateCalls++
	f.generateInput = input

	if f.generateErr != nil {
		return Corrida{}, f.generateErr
	}

	return f.generateResult, nil
}

func (f *fakeStore) List(
	ctx context.Context,
	filter ListFilter,
) ([]Corrida, error) {
	f.listCalls++
	f.listFilter = filter

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func TestServiceGenerateNormalizaYGuarda(
	t *testing.T,
) {
	observaciones := "  Corrida de prueba  "

	store := &fakeStore{
		generateResult: Corrida{
			ID:               1,
			Folio:            "MOR-CDMX-0600-20260926",
			FechaServicio:    "2026-09-26",
			Estado:           EstadoProgramada,
			ReservasAbiertas: true,
		},
	}

	service := NewService(store)

	resultado, err := service.Generate(
		context.Background(),
		GenerarInput{
			ProgramacionID: 3,
			FechaServicio:  " 2026-09-26 ",
			Observaciones:  &observaciones,
		},
	)
	if err != nil {
		t.Fatalf(
			"Generate() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.generateCalls != 1 {
		t.Fatalf(
			"Store.Generate() fue llamado %d veces; se esperaba 1",
			store.generateCalls,
		)
	}

	if store.generateInput.ProgramacionID != 3 {
		t.Errorf(
			"ProgramacionID recibido = %d; se esperaba 3",
			store.generateInput.ProgramacionID,
		)
	}

	if store.generateInput.FechaServicio != "2026-09-26" {
		t.Errorf(
			"Fecha recibida = %q",
			store.generateInput.FechaServicio,
		)
	}

	if store.generateInput.Observaciones == nil ||
		*store.generateInput.Observaciones !=
			"Corrida de prueba" {
		t.Errorf(
			"las observaciones no fueron normalizadas",
		)
	}

	if resultado.ID != 1 {
		t.Errorf(
			"ID obtenido = %d; se esperaba 1",
			resultado.ID,
		)
	}
}

func TestServiceGenerateValidaciones(
	t *testing.T,
) {
	inputValido := func() GenerarInput {
		return GenerarInput{
			ProgramacionID: 3,
			FechaServicio:  "2026-09-26",
		}
	}

	pruebas := []struct {
		nombre    string
		modificar func(*GenerarInput)
	}{
		{
			nombre: "programación inválida",
			modificar: func(input *GenerarInput) {
				input.ProgramacionID = 0
			},
		},
		{
			nombre: "fecha vacía",
			modificar: func(input *GenerarInput) {
				input.FechaServicio = "   "
			},
		},
		{
			nombre: "fecha inexistente",
			modificar: func(input *GenerarInput) {
				input.FechaServicio = "2026-02-30"
			},
		},
		{
			nombre: "formato de fecha incorrecto",
			modificar: func(input *GenerarInput) {
				input.FechaServicio = "26-09-2026"
			},
		},
		{
			nombre: "observaciones demasiado largas",
			modificar: func(input *GenerarInput) {
				observaciones := strings.Repeat(
					"A",
					1001,
				)
				input.Observaciones = &observaciones
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

				_, err := service.Generate(
					context.Background(),
					input,
				)

				if err == nil {
					t.Fatal(
						"Generate() no devolvió el error esperado",
					)
				}

				if !errors.Is(err, ErrDatosInvalidos) {
					t.Fatalf(
						"Error obtenido = %v; se esperaba ErrDatosInvalidos",
						err,
					)
				}

				if store.generateCalls != 0 {
					t.Errorf(
						"Store.Generate() fue llamado %d veces; se esperaba 0",
						store.generateCalls,
					)
				}
			},
		)
	}
}

func TestServiceGeneratePropagaErrorRepositorio(
	t *testing.T,
) {
	store := &fakeStore{
		generateErr: ErrProgramacionNoOperaFecha,
	}

	service := NewService(store)

	_, err := service.Generate(
		context.Background(),
		GenerarInput{
			ProgramacionID: 3,
			FechaServicio:  "2026-09-27",
		},
	)

	if !errors.Is(
		err,
		ErrProgramacionNoOperaFecha,
	) {
		t.Fatalf(
			"Error obtenido = %v; se esperaba ErrProgramacionNoOperaFecha",
			err,
		)
	}

	if store.generateCalls != 1 {
		t.Errorf(
			"Store.Generate() fue llamado %d veces; se esperaba 1",
			store.generateCalls,
		)
	}
}

func TestServiceListNormalizaFiltros(
	t *testing.T,
) {
	fechaDesde := " 2026-09-25 "
	fechaHasta := " 2026-09-30 "
	estado := Estado(" programada ")

	store := &fakeStore{
		listResult: []Corrida{
			{
				ID:            1,
				Folio:         "MOR-CDMX-0600-20260926",
				FechaServicio: "2026-09-26",
				Estado:        EstadoProgramada,
			},
		},
	}

	service := NewService(store)

	resultado, err := service.List(
		context.Background(),
		ListFilter{
			FechaDesde: &fechaDesde,
			FechaHasta: &fechaHasta,
			Estado:     &estado,
		},
	)
	if err != nil {
		t.Fatalf(
			"List() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.listCalls != 1 {
		t.Fatalf(
			"Store.List() fue llamado %d veces; se esperaba 1",
			store.listCalls,
		)
	}

	if store.listFilter.FechaDesde == nil ||
		*store.listFilter.FechaDesde != "2026-09-25" {
		t.Errorf(
			"fecha_desde no fue normalizada",
		)
	}

	if store.listFilter.FechaHasta == nil ||
		*store.listFilter.FechaHasta != "2026-09-30" {
		t.Errorf(
			"fecha_hasta no fue normalizada",
		)
	}

	if store.listFilter.Estado == nil ||
		*store.listFilter.Estado != EstadoProgramada {
		t.Errorf(
			"estado no fue normalizado",
		)
	}

	if len(resultado) != 1 {
		t.Fatalf(
			"se esperaba una corrida, se obtuvieron %d",
			len(resultado),
		)
	}
}

func TestServiceListValidaciones(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		filter ListFilter
	}{
		{
			nombre: "fecha inicial inválida",
			filter: ListFilter{
				FechaDesde: stringPointer(
					"2026-02-30",
				),
			},
		},
		{
			nombre: "fecha final inválida",
			filter: ListFilter{
				FechaHasta: stringPointer(
					"30-09-2026",
				),
			},
		},
		{
			nombre: "rango invertido",
			filter: ListFilter{
				FechaDesde: stringPointer(
					"2026-09-30",
				),
				FechaHasta: stringPointer(
					"2026-09-25",
				),
			},
		},
		{
			nombre: "estado inválido",
			filter: ListFilter{
				Estado: estadoPointer(
					"DESCONOCIDA",
				),
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				store := &fakeStore{}
				service := NewService(store)

				_, err := service.List(
					context.Background(),
					prueba.filter,
				)

				if err == nil {
					t.Fatal(
						"List() no devolvió el error esperado",
					)
				}

				if !errors.Is(err, ErrDatosInvalidos) {
					t.Fatalf(
						"Error obtenido = %v; se esperaba ErrDatosInvalidos",
						err,
					)
				}

				if store.listCalls != 0 {
					t.Errorf(
						"Store.List() fue llamado %d veces; se esperaba 0",
						store.listCalls,
					)
				}
			},
		)
	}
}

func stringPointer(
	valor string,
) *string {
	return &valor
}

func estadoPointer(
	valor Estado,
) *Estado {
	return &valor
}
