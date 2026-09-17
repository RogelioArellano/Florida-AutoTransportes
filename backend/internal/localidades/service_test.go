package localidades

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeStore sustituye temporalmente al repositorio PostgreSQL.
// Permite probar el servicio sin iniciar una base de datos.
type fakeStore struct {
	createInput  CreateInput
	createResult Localidad
	createErr    error
	createCalls  int

	listResult []Localidad
	listErr    error
	listCalls  int
}

// Esta asignación no ejecuta nada.
// Solo obliga al compilador a comprobar que fakeStore cumple Store.
var _ Store = (*fakeStore)(nil)

func (f *fakeStore) Create(
	ctx context.Context,
	input CreateInput,
) (Localidad, error) {
	f.createCalls++
	f.createInput = input

	if f.createErr != nil {
		return Localidad{}, f.createErr
	}

	return f.createResult, nil
}

func (f *fakeStore) List(
	ctx context.Context,
) ([]Localidad, error) {
	f.listCalls++

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func TestServiceCreateNormalizesAndSaves(
	t *testing.T,
) {
	store := &fakeStore{
		createResult: Localidad{
			ID:     1,
			Nombre: "Morelia",
			Estado: "Michoacán",
			Activa: true,
		},
	}

	service := NewService(store)

	result, err := service.Create(
		context.Background(),
		CreateInput{
			Nombre: "  Morelia  ",
			Estado: "  Michoacán  ",
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

	if store.createInput.Nombre != "Morelia" {
		t.Errorf(
			"Nombre recibido por Store = %q; se esperaba %q",
			store.createInput.Nombre,
			"Morelia",
		)
	}

	if store.createInput.Estado != "Michoacán" {
		t.Errorf(
			"Estado recibido por Store = %q; se esperaba %q",
			store.createInput.Estado,
			"Michoacán",
		)
	}

	if result.ID != 1 {
		t.Errorf(
			"ID obtenido = %d; se esperaba 1",
			result.ID,
		)
	}
}

func TestServiceCreateValidations(
	t *testing.T,
) {
	tests := []struct {
		name      string
		input     CreateInput
		wantField string
	}{
		{
			name: "nombre vacío",
			input: CreateInput{
				Nombre: "",
				Estado: "Michoacán",
			},
			wantField: "nombre",
		},
		{
			name: "nombre con espacios",
			input: CreateInput{
				Nombre: "   ",
				Estado: "Michoacán",
			},
			wantField: "nombre",
		},
		{
			name: "nombre demasiado largo",
			input: CreateInput{
				Nombre: strings.Repeat("a", 101),
				Estado: "Michoacán",
			},
			wantField: "nombre",
		},
		{
			name: "estado vacío",
			input: CreateInput{
				Nombre: "Morelia",
				Estado: "",
			},
			wantField: "estado",
		},
		{
			name: "estado demasiado largo",
			input: CreateInput{
				Nombre: "Morelia",
				Estado: strings.Repeat("a", 101),
			},
			wantField: "estado",
		},
	}

	for _, test := range tests {
		t.Run(
			test.name,
			func(t *testing.T) {
				store := &fakeStore{}
				service := NewService(store)

				_, err := service.Create(
					context.Background(),
					test.input,
				)

				if err == nil {
					t.Fatal(
						"Create() no devolvió el error esperado",
					)
				}

				var validationError *ValidationError

				if !errors.As(err, &validationError) {
					t.Fatalf(
						"Error obtenido = %T; se esperaba ValidationError",
						err,
					)
				}

				if validationError.Field != test.wantField {
					t.Errorf(
						"Campo inválido = %q; se esperaba %q",
						validationError.Field,
						test.wantField,
					)
				}

				// Si la validación falla, el repositorio no debe ejecutarse.
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

func TestServiceCreateReturnsDuplicateError(
	t *testing.T,
) {
	store := &fakeStore{
		createErr: ErrLocalidadDuplicada,
	}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			Nombre: "Morelia",
			Estado: "Michoacán",
		},
	)

	if !errors.Is(err, ErrLocalidadDuplicada) {
		t.Fatalf(
			"Error obtenido = %v; se esperaba ErrLocalidadDuplicada",
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

func TestServiceListReturnsLocalidades(
	t *testing.T,
) {
	store := &fakeStore{
		listResult: []Localidad{
			{
				ID:     1,
				Nombre: "Morelia",
				Estado: "Michoacán",
				Activa: true,
			},
			{
				ID:     2,
				Nombre: "Toluca",
				Estado: "Estado de México",
				Activa: true,
			},
		},
	}

	service := NewService(store)

	result, err := service.List(context.Background())
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

	if len(result) != 2 {
		t.Fatalf(
			"List() devolvió %d localidades; se esperaban 2",
			len(result),
		)
	}

	if result[0].Nombre != "Morelia" {
		t.Errorf(
			"Primera localidad = %q; se esperaba Morelia",
			result[0].Nombre,
		)
	}
}
