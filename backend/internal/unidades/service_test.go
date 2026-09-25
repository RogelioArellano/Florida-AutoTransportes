package unidades

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeStore sustituye temporalmente al repositorio PostgreSQL.
// Así podemos probar el servicio sin conectar una base de datos.
type fakeStore struct {
	createInput  CreateInput
	createResult Unidad
	createErr    error
	createCalls  int

	listResult []Unidad
	listErr    error
	listCalls  int
}

// Esta línea obliga al compilador a comprobar que fakeStore
// implementa todos los métodos de Store.
var _ Store = (*fakeStore)(nil)

func (f *fakeStore) Create(
	ctx context.Context,
	input CreateInput,
) (Unidad, error) {
	f.createCalls++
	f.createInput = input

	if f.createErr != nil {
		return Unidad{}, f.createErr
	}

	return f.createResult, nil
}

func (f *fakeStore) List(
	ctx context.Context,
) ([]Unidad, error) {
	f.listCalls++

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func TestServiceCreateNormalizaYGuarda(
	t *testing.T,
) {
	placas := " abc-123 "
	marca := " Nissan "
	modelo := " Urvan "
	anio := 2024

	store := &fakeStore{
		createResult: Unidad{
			ID:                 1,
			Codigo:             "UNIDAD-01",
			CapacidadTotal:     17,
			CapacidadPasajeros: 16,
			Activa:             true,
		},
	}

	service := NewService(store)

	resultado, err := service.Create(
		context.Background(),
		CreateInput{
			Codigo:             " unidad-01 ",
			Placas:             &placas,
			Marca:              &marca,
			Modelo:             &modelo,
			Anio:               &anio,
			CapacidadTotal:     17,
			CapacidadPasajeros: 16,
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

	if store.createInput.Codigo != "UNIDAD-01" {
		t.Errorf(
			"Código recibido = %q; se esperaba UNIDAD-01",
			store.createInput.Codigo,
		)
	}

	if store.createInput.Placas == nil ||
		*store.createInput.Placas != "ABC-123" {
		t.Errorf(
			"Placas recibidas = %v; se esperaba ABC-123",
			store.createInput.Placas,
		)
	}

	if store.createInput.Marca == nil ||
		*store.createInput.Marca != "Nissan" {
		t.Errorf(
			"Marca recibida = %v; se esperaba Nissan",
			store.createInput.Marca,
		)
	}

	if store.createInput.Modelo == nil ||
		*store.createInput.Modelo != "Urvan" {
		t.Errorf(
			"Modelo recibido = %v; se esperaba Urvan",
			store.createInput.Modelo,
		)
	}

	if resultado.ID != 1 {
		t.Errorf(
			"ID obtenido = %d; se esperaba 1",
			resultado.ID,
		)
	}
}

func TestServiceCreateConvierteOpcionalesVaciosEnNil(
	t *testing.T,
) {
	placas := "   "
	modelo := ""

	store := &fakeStore{
		createResult: Unidad{
			ID: 1,
		},
	}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			Codigo:             "UNIDAD-01",
			Placas:             &placas,
			Modelo:             &modelo,
			CapacidadTotal:     17,
			CapacidadPasajeros: 16,
		},
	)
	if err != nil {
		t.Fatalf(
			"Create() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.createInput.Placas != nil {
		t.Errorf(
			"Se esperaba que placas fuera nil",
		)
	}

	if store.createInput.Modelo != nil {
		t.Errorf(
			"Se esperaba que modelo fuera nil",
		)
	}
}

func TestServiceCreateValidaciones(
	t *testing.T,
) {
	anioInvalido := 1949
	placasLargas := strings.Repeat("A", 21)

	pruebas := []struct {
		nombre string
		input  CreateInput
	}{
		{
			nombre: "código vacío",
			input: CreateInput{
				Codigo:             "   ",
				CapacidadTotal:     17,
				CapacidadPasajeros: 16,
			},
		},
		{
			nombre: "código demasiado largo",
			input: CreateInput{
				Codigo:             strings.Repeat("A", 31),
				CapacidadTotal:     17,
				CapacidadPasajeros: 16,
			},
		},
		{
			nombre: "placas demasiado largas",
			input: CreateInput{
				Codigo:             "UNIDAD-01",
				Placas:             &placasLargas,
				CapacidadTotal:     17,
				CapacidadPasajeros: 16,
			},
		},
		{
			nombre: "año fuera de rango",
			input: CreateInput{
				Codigo:             "UNIDAD-01",
				Anio:               &anioInvalido,
				CapacidadTotal:     17,
				CapacidadPasajeros: 16,
			},
		},
		{
			nombre: "capacidad total insuficiente",
			input: CreateInput{
				Codigo:             "UNIDAD-01",
				CapacidadTotal:     1,
				CapacidadPasajeros: 1,
			},
		},
		{
			nombre: "capacidad de pasajeros en cero",
			input: CreateInput{
				Codigo:             "UNIDAD-01",
				CapacidadTotal:     17,
				CapacidadPasajeros: 0,
			},
		},
		{
			nombre: "pasajeros igual a capacidad total",
			input: CreateInput{
				Codigo:             "UNIDAD-01",
				CapacidadTotal:     17,
				CapacidadPasajeros: 17,
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				store := &fakeStore{}
				service := NewService(store)

				_, err := service.Create(
					context.Background(),
					prueba.input,
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

				// Cuando los datos son inválidos, el repositorio
				// nunca debe ser ejecutado.
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
		createErr: ErrCodigoDuplicado,
	}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			Codigo:             "UNIDAD-01",
			CapacidadTotal:     17,
			CapacidadPasajeros: 16,
		},
	)

	if !errors.Is(err, ErrCodigoDuplicado) {
		t.Fatalf(
			"Error obtenido = %v; se esperaba ErrCodigoDuplicado",
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

func TestServiceListRetornaUnidades(
	t *testing.T,
) {
	store := &fakeStore{
		listResult: []Unidad{
			{
				ID:                 1,
				Codigo:             "UNIDAD-01",
				CapacidadTotal:     17,
				CapacidadPasajeros: 16,
				Activa:             true,
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
			"List() devolvió %d unidades; se esperaba 1",
			len(resultado),
		)
	}

	if resultado[0].Codigo != "UNIDAD-01" {
		t.Errorf(
			"Código obtenido = %q; se esperaba UNIDAD-01",
			resultado[0].Codigo,
		)
	}
}
