package choferes

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeStore sustituye al repositorio PostgreSQL durante
// las pruebas del servicio.
type fakeStore struct {
	createInput  CreateInput
	createResult Chofer
	createErr    error
	createCalls  int

	listResult []Chofer
	listErr    error
	listCalls  int
}

var _ Store = (*fakeStore)(nil)

func (f *fakeStore) Create(
	ctx context.Context,
	input CreateInput,
) (Chofer, error) {
	f.createCalls++
	f.createInput = input

	if f.createErr != nil {
		return Chofer{}, f.createErr
	}

	return f.createResult, nil
}

func (f *fakeStore) List(
	ctx context.Context,
) ([]Chofer, error) {
	f.listCalls++

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func TestServiceCreateNormalizaYGuarda(
	t *testing.T,
) {
	licencia := " LIC-12345 "
	vigencia := " 2027-12-31 "

	store := &fakeStore{
		createResult: Chofer{
			ID:             1,
			NombreCompleto: "Juan Pérez",
			Telefono:       "4431234567",
			Activo:         true,
		},
	}

	service := NewService(store)

	resultado, err := service.Create(
		context.Background(),
		CreateInput{
			NombreCompleto:   "  Juan Pérez  ",
			Telefono:         "  4431234567  ",
			LicenciaNumero:   &licencia,
			LicenciaVigencia: &vigencia,
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

	if store.createInput.NombreCompleto != "Juan Pérez" {
		t.Errorf(
			"Nombre recibido = %q; se esperaba Juan Pérez",
			store.createInput.NombreCompleto,
		)
	}

	if store.createInput.Telefono != "4431234567" {
		t.Errorf(
			"Teléfono recibido = %q; se esperaba 4431234567",
			store.createInput.Telefono,
		)
	}

	if store.createInput.LicenciaNumero == nil ||
		*store.createInput.LicenciaNumero != "LIC-12345" {
		t.Errorf(
			"no se normalizó correctamente la licencia",
		)
	}

	if store.createInput.LicenciaVigencia == nil ||
		*store.createInput.LicenciaVigencia != "2027-12-31" {
		t.Errorf(
			"no se normalizó correctamente la vigencia",
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
	licencia := "   "
	vigencia := ""

	store := &fakeStore{
		createResult: Chofer{
			ID: 1,
		},
	}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			NombreCompleto:   "Juan Pérez",
			Telefono:         "4431234567",
			LicenciaNumero:   &licencia,
			LicenciaVigencia: &vigencia,
		},
	)
	if err != nil {
		t.Fatalf(
			"Create() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.createInput.LicenciaNumero != nil {
		t.Error(
			"se esperaba que licencia_numero fuera nil",
		)
	}

	if store.createInput.LicenciaVigencia != nil {
		t.Error(
			"se esperaba que licencia_vigencia fuera nil",
		)
	}
}

func TestServiceCreateValidaciones(
	t *testing.T,
) {
	licenciaValida := "LIC-12345"
	licenciaLarga := strings.Repeat("A", 51)
	fechaValida := "2027-12-31"
	fechaInvalida := "2027-02-30"

	pruebas := []struct {
		nombre string
		input  CreateInput
	}{
		{
			nombre: "nombre vacío",
			input: CreateInput{
				NombreCompleto: "",
				Telefono:       "4431234567",
			},
		},
		{
			nombre: "nombre con espacios",
			input: CreateInput{
				NombreCompleto: "   ",
				Telefono:       "4431234567",
			},
		},
		{
			nombre: "nombre demasiado largo",
			input: CreateInput{
				NombreCompleto: strings.Repeat(
					"A",
					151,
				),
				Telefono: "4431234567",
			},
		},
		{
			nombre: "teléfono vacío",
			input: CreateInput{
				NombreCompleto: "Juan Pérez",
				Telefono:       "",
			},
		},
		{
			nombre: "teléfono demasiado largo",
			input: CreateInput{
				NombreCompleto: "Juan Pérez",
				Telefono: strings.Repeat(
					"1",
					21,
				),
			},
		},
		{
			nombre: "licencia demasiado larga",
			input: CreateInput{
				NombreCompleto: "Juan Pérez",
				Telefono:       "4431234567",
				LicenciaNumero: &licenciaLarga,
			},
		},
		{
			nombre: "vigencia sin licencia",
			input: CreateInput{
				NombreCompleto:   "Juan Pérez",
				Telefono:         "4431234567",
				LicenciaVigencia: &fechaValida,
			},
		},
		{
			nombre: "fecha inexistente",
			input: CreateInput{
				NombreCompleto:   "Juan Pérez",
				Telefono:         "4431234567",
				LicenciaNumero:   &licenciaValida,
				LicenciaVigencia: &fechaInvalida,
			},
		},
		{
			nombre: "formato de fecha incorrecto",
			input: CreateInput{
				NombreCompleto: "Juan Pérez",
				Telefono:       "4431234567",
				LicenciaNumero: &licenciaValida,
				LicenciaVigencia: stringPointer(
					"31-12-2027",
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
	errorRepositorio := errors.New(
		"PostgreSQL no disponible",
	)

	store := &fakeStore{
		createErr: errorRepositorio,
	}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			NombreCompleto: "Juan Pérez",
			Telefono:       "4431234567",
		},
	)

	if !errors.Is(err, errorRepositorio) {
		t.Fatalf(
			"Error obtenido = %v; se esperaba el error del repositorio",
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

func TestServiceListRetornaChoferes(
	t *testing.T,
) {
	store := &fakeStore{
		listResult: []Chofer{
			{
				ID:             1,
				NombreCompleto: "Juan Pérez",
				Telefono:       "4431234567",
				Activo:         true,
			},
			{
				ID:             2,
				NombreCompleto: "María López",
				Telefono:       "4437654321",
				Activo:         true,
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

	if len(resultado) != 2 {
		t.Fatalf(
			"List() devolvió %d choferes; se esperaban 2",
			len(resultado),
		)
	}

	if resultado[0].NombreCompleto != "Juan Pérez" {
		t.Errorf(
			"Primer chofer = %q; se esperaba Juan Pérez",
			resultado[0].NombreCompleto,
		)
	}
}

// stringPointer es una función auxiliar utilizada únicamente
// por las pruebas para obtener un *string de manera sencilla.
func stringPointer(
	valor string,
) *string {
	return &valor
}
