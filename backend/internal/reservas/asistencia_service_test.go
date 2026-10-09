package reservas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

type asistenciaStoreFake struct {
	resultado     AsistenciaReserva
	err           error
	consultaID    int64
	abordajeInput RegistrarAbordajeInput
	cierreInput   CerrarAsistenciaInput
	consultaCalls int
	abordajeCalls int
	cierreCalls   int
	ctxRecibido   context.Context
}

var _ AsistenciaStore = (*asistenciaStoreFake)(nil)

func (f *asistenciaStoreFake) GetAttendance(
	ctx context.Context,
	reservaID int64,
) (AsistenciaReserva, error) {
	f.consultaCalls++
	f.consultaID = reservaID
	f.ctxRecibido = ctx
	return f.resultado, f.err
}

func (f *asistenciaStoreFake) RegisterBoarding(
	ctx context.Context,
	input RegistrarAbordajeInput,
) (AsistenciaReserva, error) {
	f.abordajeCalls++
	f.abordajeInput = input
	f.ctxRecibido = ctx
	return f.resultado, f.err
}

func (f *asistenciaStoreFake) CloseAttendance(
	ctx context.Context,
	input CerrarAsistenciaInput,
) (AsistenciaReserva, error) {
	f.cierreCalls++
	f.cierreInput = input
	f.ctxRecibido = ctx
	return f.resultado, f.err
}

func asistenciaIntPrueba(valor int) *int { return &valor }

func TestAsistenciaServiceConsultaResumen(t *testing.T) {
	fecha := time.Date(2026, time.October, 8, 18, 0, 0, 0, time.UTC)
	casos := []struct {
		nombre     string
		abordada   *int
		cerradaEn  *time.Time
		pendiente  *int
		noPresenta *int
	}{
		{"sin abordajes", asistenciaIntPrueba(0), nil, asistenciaIntPrueba(3), nil},
		{"parcial abierto", asistenciaIntPrueba(2), nil, asistenciaIntPrueba(1), nil},
		{"todos abordaron abierto", asistenciaIntPrueba(3), nil, asistenciaIntPrueba(0), nil},
		{"parcial cerrado", asistenciaIntPrueba(2), &fecha, asistenciaIntPrueba(0), asistenciaIntPrueba(1)},
		{"sin abordajes cerrado", asistenciaIntPrueba(0), &fecha, asistenciaIntPrueba(0), asistenciaIntPrueba(3)},
		{"todos abordaron cerrado", asistenciaIntPrueba(3), &fecha, asistenciaIntPrueba(0), asistenciaIntPrueba(0)},
		{"histórica desconocida", nil, nil, nil, nil},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			store := &asistenciaStoreFake{resultado: AsistenciaReserva{
				ReservaID:           6,
				CantidadPasajeros:   3,
				CantidadAbordada:    caso.abordada,
				AsistenciaCerradaEn: caso.cerradaEn,
			}}
			service := NewAsistenciaService(store)
			resultado, err := service.GetAttendance(context.Background(), 6)
			if err != nil {
				t.Fatal(err)
			}
			if store.consultaCalls != 1 || store.consultaID != 6 {
				t.Fatalf("consulta inesperada: llamadas=%d, id=%d", store.consultaCalls, store.consultaID)
			}
			if resultado.AsistenciaConocida != (caso.abordada != nil) ||
				resultado.AsistenciaCerrada != (caso.cerradaEn != nil) {
				t.Fatalf("banderas inesperadas: %+v", resultado)
			}
			compararCantidadAsistencia(t, "pendientes", resultado.CantidadPendiente, caso.pendiente)
			compararCantidadAsistencia(t, "no presentados", resultado.CantidadNoPresentada, caso.noPresenta)
		})
	}
}

func TestAsistenciaServiceCanceladaNoEsPendienteNiAusente(t *testing.T) {
	store := &asistenciaStoreFake{resultado: AsistenciaReserva{
		ReservaID:         6,
		EstadoReserva:     EstadoCancelada,
		CantidadPasajeros: 3,
		CantidadAbordada:  asistenciaIntPrueba(0),
	}}
	resultado, err := NewAsistenciaService(store).GetAttendance(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	compararCantidadAsistencia(t, "pendientes", resultado.CantidadPendiente, asistenciaIntPrueba(0))
	if resultado.CantidadNoPresentada != nil || resultado.AsistenciaCerrada {
		t.Fatal("una cancelación no debe registrarse como cierre de asistencia con ausencias")
	}
}

func TestAsistenciaServiceRegistraTotalYNormalizaNotas(t *testing.T) {
	nota := "  Subieron dos personas  "
	store := &asistenciaStoreFake{resultado: AsistenciaReserva{
		ReservaID:         6,
		EstadoReserva:     EstadoAbordada,
		CantidadPasajeros: 3,
		CantidadAbordada:  asistenciaIntPrueba(2),
	}}
	service := NewAsistenciaService(store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	resultado, err := service.RegisterBoarding(ctx, RegistrarAbordajeInput{
		ReservaID:        6,
		CantidadAbordada: 2,
		Observaciones:    &nota,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.abordajeCalls != 1 || store.abordajeInput.ReservaID != 6 ||
		store.abordajeInput.CantidadAbordada != 2 {
		t.Fatalf("input inesperado: %+v", store.abordajeInput)
	}
	if store.ctxRecibido != ctx {
		t.Fatal("no se conservó el contexto de la solicitud")
	}
	if store.abordajeInput.Observaciones == nil || *store.abordajeInput.Observaciones != "Subieron dos personas" {
		t.Fatalf("notas inesperadas: %v", store.abordajeInput.Observaciones)
	}
	if nota != "  Subieron dos personas  " {
		t.Fatal("se modificaron las notas originales del llamador")
	}
	compararCantidadAsistencia(t, "pendientes", resultado.CantidadPendiente, asistenciaIntPrueba(1))
	if resultado.CantidadNoPresentada != nil {
		t.Fatal("una asistencia abierta no debe declarar ausencias definitivas")
	}
}

func TestAsistenciaServiceCierraAsistenciaParcial(t *testing.T) {
	fecha := time.Date(2026, time.October, 8, 18, 0, 0, 0, time.UTC)
	nota := "  El tercer pasajero no llegó  "
	store := &asistenciaStoreFake{resultado: AsistenciaReserva{
		ReservaID:           6,
		EstadoReserva:       EstadoAbordada,
		CantidadPasajeros:   3,
		CantidadAbordada:    asistenciaIntPrueba(2),
		AsistenciaCerradaEn: &fecha,
	}}
	service := NewAsistenciaService(store)
	resultado, err := service.CloseAttendance(context.Background(), CerrarAsistenciaInput{
		ReservaID:     6,
		Observaciones: &nota,
	})
	if err != nil {
		t.Fatal(err)
	}
	if store.cierreCalls != 1 || store.cierreInput.ReservaID != 6 ||
		store.cierreInput.Observaciones == nil || *store.cierreInput.Observaciones != "El tercer pasajero no llegó" {
		t.Fatalf("input inesperado: %+v", store.cierreInput)
	}
	if resultado.EstadoReserva != EstadoAbordada || !resultado.AsistenciaCerrada {
		t.Fatalf("estado inesperado: %+v", resultado)
	}
	compararCantidadAsistencia(t, "pendientes", resultado.CantidadPendiente, asistenciaIntPrueba(0))
	compararCantidadAsistencia(t, "no presentados", resultado.CantidadNoPresentada, asistenciaIntPrueba(1))
}

func TestAsistenciaServiceRechazaIDsInvalidos(t *testing.T) {
	for _, id := range []int64{0, -1} {
		for _, operacion := range []string{"consulta", "abordaje", "cierre"} {
			t.Run(fmt.Sprintf("%s/%d", operacion, id), func(t *testing.T) {
				store := &asistenciaStoreFake{}
				service := NewAsistenciaService(store)
				var err error
				switch operacion {
				case "consulta":
					_, err = service.GetAttendance(context.Background(), id)
				case "abordaje":
					_, err = service.RegisterBoarding(context.Background(), RegistrarAbordajeInput{ReservaID: id, CantidadAbordada: 1})
				case "cierre":
					_, err = service.CloseAttendance(context.Background(), CerrarAsistenciaInput{ReservaID: id})
				}
				if !errors.Is(err, ErrDatosInvalidos) || store.consultaCalls+store.abordajeCalls+store.cierreCalls != 0 {
					t.Fatalf("datos inválidos consultaron persistencia: error=%v", err)
				}
			})
		}
	}
}

func TestAsistenciaServiceRechazaCantidadesInvalidas(t *testing.T) {
	for _, cantidad := range []int{0, -1, maxCantidadPasajeros + 1} {
		t.Run(fmt.Sprint(cantidad), func(t *testing.T) {
			store := &asistenciaStoreFake{}
			service := NewAsistenciaService(store)
			_, err := service.RegisterBoarding(context.Background(), RegistrarAbordajeInput{
				ReservaID: 6, CantidadAbordada: cantidad,
			})
			if !errors.Is(err, ErrDatosInvalidos) || store.abordajeCalls != 0 {
				t.Fatalf("cantidad inválida llegó a persistencia: error=%v", err)
			}
		})
	}
}

func TestAsistenciaServiceValidaObservaciones(t *testing.T) {
	for _, operacion := range []string{"abordaje", "cierre"} {
		for _, caso := range []struct {
			nombre string
			nota   string
			valida bool
		}{
			{"vacía", "   ", true},
			{"límite unicode", strings.Repeat("á", maxObservacionesAsistencia), true},
			{"excede límite", strings.Repeat("á", maxObservacionesAsistencia+1), false},
		} {
			t.Run(operacion+"/"+caso.nombre, func(t *testing.T) {
				store := &asistenciaStoreFake{}
				service := NewAsistenciaService(store)
				var err error
				var recibida *string
				if operacion == "abordaje" {
					_, err = service.RegisterBoarding(context.Background(), RegistrarAbordajeInput{
						ReservaID: 6, CantidadAbordada: 1, Observaciones: &caso.nota,
					})
					recibida = store.abordajeInput.Observaciones
				} else {
					_, err = service.CloseAttendance(context.Background(), CerrarAsistenciaInput{
						ReservaID: 6, Observaciones: &caso.nota,
					})
					recibida = store.cierreInput.Observaciones
				}
				llamadas := store.abordajeCalls + store.cierreCalls
				if caso.valida {
					if err != nil || llamadas != 1 {
						t.Fatalf("notas válidas rechazadas: error=%v, llamadas=%d", err, llamadas)
					}
					if caso.nombre == "vacía" && recibida != nil {
						t.Fatal("notas vacías deben normalizarse a nil")
					}
				} else if !errors.Is(err, ErrDatosInvalidos) || llamadas != 0 {
					t.Fatalf("notas inválidas llegaron a persistencia: error=%v", err)
				}
			})
		}
	}
}

func TestAsistenciaServicePropagaErrores(t *testing.T) {
	for _, operacion := range []string{"consulta", "abordaje", "cierre"} {
		for _, esperado := range []error{
			ErrReservaNoEncontrada,
			ErrAsistenciaCerrada,
			ErrCantidadAbordadaExcedeReserva,
			ErrCantidadAbordadaRetrocede,
			ErrAsistenciaHistoricaDesconocida,
			context.DeadlineExceeded,
		} {
			t.Run(operacion+"/"+esperado.Error(), func(t *testing.T) {
				store := &asistenciaStoreFake{err: fmt.Errorf("persistencia: %w", esperado)}
				service := NewAsistenciaService(store)
				var err error
				switch operacion {
				case "consulta":
					_, err = service.GetAttendance(context.Background(), 6)
				case "abordaje":
					_, err = service.RegisterBoarding(context.Background(), RegistrarAbordajeInput{ReservaID: 6, CantidadAbordada: 1})
				case "cierre":
					_, err = service.CloseAttendance(context.Background(), CerrarAsistenciaInput{ReservaID: 6})
				}
				if !errors.Is(err, esperado) {
					t.Fatalf("error=%v; esperado=%v", err, esperado)
				}
			})
		}
	}
}

func compararCantidadAsistencia(t *testing.T, nombre string, actual, esperada *int) {
	t.Helper()
	if actual == nil || esperada == nil {
		if actual != esperada {
			t.Fatalf("%s: presencia de cantidad inesperada, actual=%v esperada=%v", nombre, actual, esperada)
		}
		return
	}
	if *actual != *esperada {
		t.Fatalf("%s=%d; se esperaba %d", nombre, *actual, *esperada)
	}
}
