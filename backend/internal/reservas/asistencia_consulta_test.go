package reservas

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAsistenciaConsultasDetalleReserva(t *testing.T) {
	store := &fakeStore{getByIDResult: Reserva{
		ID: 6, Estado: EstadoAbordada, CantidadPasajeros: 3,
		Total: "450.00", MontoPagado: "100.00", SaldoPendiente: "350.00", EstadoPago: EstadoPagoParcial,
		DetalleAsistencia: DetalleAsistencia{CantidadAbordada: asistenciaIntPrueba(2)},
	}}
	resultado, err := NewService(store).GetByID(context.Background(), 6)
	if err != nil {
		t.Fatal(err)
	}
	compararCantidadAsistencia(t, "pendientes", resultado.CantidadPendiente, asistenciaIntPrueba(1))
	if !resultado.AsistenciaConocida || resultado.AsistenciaCerrada || resultado.CantidadNoPresentada != nil {
		t.Fatalf("resumen inesperado: %+v", resultado.DetalleAsistencia)
	}
	if resultado.Total != "450.00" || resultado.MontoPagado != "100.00" || resultado.SaldoPendiente != "350.00" || resultado.EstadoPago != EstadoPagoParcial {
		t.Fatal("la asistencia alteró la información financiera")
	}
	if store.getByIDResult.CantidadPendiente != nil {
		t.Fatal("se modificó el resultado original del Store")
	}
}

func TestAsistenciaConsultasResumenLista(t *testing.T) {
	fecha := time.Now()
	fila := func(cantidad int, abordada *int, cerrada bool) ReservaListaPasajeros {
		var cierre *time.Time
		if cerrada {
			cierre = &fecha
		}
		estado := EstadoApartada
		if abordada == nil || *abordada > 0 {
			estado = EstadoAbordada
		} else if cerrada {
			estado = EstadoNoPresentada
		}
		return ReservaListaPasajeros{
			EstadoReserva: estado, CantidadPasajeros: cantidad,
			DetalleAsistencia: DetalleAsistencia{CantidadAbordada: abordada, AsistenciaCerradaEn: cierre},
		}
	}
	for _, caso := range []struct {
		nombre                          string
		filas                           []ReservaListaPasajeros
		abordados, pendientes, ausentes *int
		abiertas, desconocidas          int
	}{
		{"vacía", nil, asistenciaIntPrueba(0), asistenciaIntPrueba(0), asistenciaIntPrueba(0), 0, 0},
		{"abierta", []ReservaListaPasajeros{fila(3, asistenciaIntPrueba(2), false), fila(1, asistenciaIntPrueba(0), false)}, asistenciaIntPrueba(2), asistenciaIntPrueba(2), nil, 2, 0},
		{"mixta", []ReservaListaPasajeros{fila(3, asistenciaIntPrueba(2), true), fila(1, asistenciaIntPrueba(0), false)}, asistenciaIntPrueba(2), asistenciaIntPrueba(1), nil, 1, 0},
		{"cerrada", []ReservaListaPasajeros{fila(3, asistenciaIntPrueba(2), true), fila(1, asistenciaIntPrueba(0), true)}, asistenciaIntPrueba(2), asistenciaIntPrueba(0), asistenciaIntPrueba(2), 0, 0},
		{"histórica", []ReservaListaPasajeros{fila(3, nil, false), fila(1, asistenciaIntPrueba(0), false)}, nil, nil, nil, 1, 1},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			store := &listaPasajerosStoreFake{resultado: ListaPasajerosCorrida{
				CorridaID: 4, Reservas: caso.filas,
				TotalVendido: "450.00", TotalCobrado: "100.00", TotalPorCobrar: "350.00",
			}}
			lista, err := NewListaPasajerosService(store).GetPassengerList(context.Background(), 4)
			if err != nil {
				t.Fatal(err)
			}
			compararCantidadAsistencia(t, "abordados", lista.TotalPasajerosAbordados, caso.abordados)
			compararCantidadAsistencia(t, "pendientes", lista.TotalPasajerosPendientes, caso.pendientes)
			compararCantidadAsistencia(t, "ausentes", lista.TotalPasajerosNoPresentados, caso.ausentes)
			if lista.TotalReservasAsistenciaAbierta != caso.abiertas || lista.TotalReservasAsistenciaDesconocida != caso.desconocidas || lista.Reservas == nil {
				t.Fatalf("resumen inesperado: %+v", lista)
			}
			if lista.TotalVendido != "450.00" || lista.TotalCobrado != "100.00" || lista.TotalPorCobrar != "350.00" {
				t.Fatal("se alteraron los importes de la corrida")
			}
			for _, original := range store.resultado.Reservas {
				if original.CantidadPendiente != nil {
					t.Fatal("se modificó una fila compartida por el Store")
				}
			}
		})
	}
}

func asistenciaConsultasMuxPrueba(pool *pgxpool.Pool) *http.ServeMux {
	repository := NewPostgresRepository(pool)
	reservas := NewHandler(NewService(repository))
	lista := NewListaPasajerosHandler(NewListaPasajerosService(repository))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/reservas", reservas.Handle)
	mux.HandleFunc("/api/reservas/", reservas.HandleDetail)
	mux.HandleFunc("/api/corridas/{corridaID}/pasajeros", lista.Handle)
	NewAsistenciaHandler(NewAsistenciaService(repository)).RegisterRoutes(mux)
	return mux
}

func asistenciaConsultasDataPrueba[T any](t *testing.T, respuesta *httptest.ResponseRecorder, status int) T {
	t.Helper()
	if respuesta.Code != status {
		t.Fatalf("status=%d; esperado=%d; body=%s", respuesta.Code, status, respuesta.Body.String())
	}
	var body struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(respuesta.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Data
}

// Ejecuta handlers, servicios y SQL reales en el esquema aislado del
// helper de integración. Requiere ASISTENCIA_TEST_DATABASE_URL.
func TestAsistenciaConsultasFlujoHTTP(t *testing.T) {
	pool, _ := asistenciaRepositoryPrueba(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "UPDATE corridas SET capacidad_pasajeros = 4"); err != nil {
		t.Fatal(err)
	}
	mux := asistenciaConsultasMuxPrueba(pool)
	finanzas := asistenciaFinanzasPrueba(t, pool)
	consultarLista := func() ListaPasajerosCorrida {
		return asistenciaConsultasDataPrueba[ListaPasajerosCorrida](t, asistenciaHTTPPrueba(mux, "GET", "/api/corridas/1/pasajeros", ""), 200)
	}
	consultarReserva := func() Reserva {
		return asistenciaConsultasDataPrueba[Reserva](t, asistenciaHTTPPrueba(mux, "GET", "/api/reservas/1", ""), 200)
	}
	antes := consultarReserva()
	compararCantidadAsistencia(t, "pendientes iniciales", antes.CantidadPendiente, asistenciaIntPrueba(3))
	lista := consultarLista()
	if lista.OcupacionMaxima != 4 || lista.LugaresDisponiblesMinimos != 0 {
		t.Fatalf("ocupación inicial: %+v", lista)
	}
	compararCantidadAsistencia(t, "abordados iniciales", lista.TotalPasajerosAbordados, asistenciaIntPrueba(0))
	const nuevaReserva = `{"corrida_id":1,"pasajero_id":1,"corrida_parada_origen_id":2,"corrida_parada_destino_id":3,"cantidad_pasajeros":1,"precio_unitario":"100.00"}`
	sinCupo := asistenciaHTTPPrueba(mux, "POST", "/api/reservas", nuevaReserva)
	if sinCupo.Code != http.StatusConflict {
		t.Fatalf("permitió sobreventa: %s", sinCupo.Body.String())
	}

	asistenciaRespuestaPrueba(t, asistenciaHTTPPrueba(mux, "POST", "/api/reservas/abordajes", `{"reserva_id":1,"cantidad_abordada":2}`))
	abierta := consultarReserva()
	compararCantidadAsistencia(t, "detalle parcial", abierta.CantidadAbordada, asistenciaIntPrueba(2))
	if consultarLista().OcupacionMaxima != 4 {
		t.Fatal("liberó cupo antes de cerrar asistencia")
	}
	sinCupo = asistenciaHTTPPrueba(mux, "POST", "/api/reservas", nuevaReserva)
	if sinCupo.Code != http.StatusConflict {
		t.Fatal("liberó el lugar del pasajero todavía pendiente")
	}

	asistenciaRespuestaPrueba(t, asistenciaHTTPPrueba(mux, "POST", "/api/reservas/asistencia/cierres", `{"reserva_id":1}`))
	cerrada := consultarReserva()
	compararCantidadAsistencia(t, "ausentes del detalle", cerrada.CantidadNoPresentada, asistenciaIntPrueba(1))
	lista = consultarLista()
	if lista.OcupacionMaxima != 3 || lista.LugaresDisponiblesMinimos != 1 || lista.TotalPasajerosRegistrados != 4 {
		t.Fatalf("cupo después del cierre: %+v", lista)
	}
	compararCantidadAsistencia(t, "abordados de la corrida", lista.TotalPasajerosAbordados, asistenciaIntPrueba(2))
	compararCantidadAsistencia(t, "pendientes de la otra parada", lista.TotalPasajerosPendientes, asistenciaIntPrueba(1))
	if lista.TotalPasajerosNoPresentados != nil || lista.TotalReservasAsistenciaAbierta != 1 {
		t.Fatal("declaró ausencias definitivas antes de cerrar todas las reservas")
	}
	if lista.TotalVendido != "390.00" || lista.TotalCobrado != "100.00" || lista.TotalPorCobrar != "290.00" {
		t.Fatalf("importes inesperados: %+v", lista)
	}
	if asistenciaFinanzasPrueba(t, pool) != finanzas {
		t.Fatal("el cierre alteró pagos o descuentos")
	}

	creada := asistenciaConsultasDataPrueba[Reserva](t, asistenciaHTTPPrueba(mux, "POST", "/api/reservas", nuevaReserva), 201)
	if !creada.AsistenciaConocida {
		t.Fatal("la nueva reserva no devolvió asistencia conocida")
	}
	compararCantidadAsistencia(t, "pendientes de la nueva reserva", creada.CantidadPendiente, asistenciaIntPrueba(1))
	asistenciaRespuestaPrueba(t, asistenciaHTTPPrueba(mux, "POST", "/api/reservas/asistencia/cierres", `{"reserva_id":2}`))
	cuerpo := fmt.Sprintf(`{"reserva_id":%d}`, creada.ID)
	asistenciaRespuestaPrueba(t, asistenciaHTTPPrueba(mux, "POST", "/api/reservas/asistencia/cierres", cuerpo))
	final := consultarLista()
	compararCantidadAsistencia(t, "ausentes finales", final.TotalPasajerosNoPresentados, asistenciaIntPrueba(3))
	compararCantidadAsistencia(t, "pendientes finales", final.TotalPasajerosPendientes, asistenciaIntPrueba(0))
	if final.OcupacionMaxima != 2 || final.TotalPasajerosRegistrados != 5 {
		t.Fatalf("resumen final: %+v", final)
	}
	// El listado general también recibe los campos de asistencia.
	reservas := asistenciaConsultasDataPrueba[[]Reserva](t, asistenciaHTTPPrueba(mux, "GET", "/api/reservas?corrida_id=1", ""), 200)
	if len(reservas) != 3 {
		t.Fatalf("reservas=%d; se esperaban 3", len(reservas))
	}
	for _, reserva := range reservas {
		if !reserva.AsistenciaCerrada {
			t.Fatalf("reserva sin resumen: %+v", reserva)
		}
	}
}

func TestAsistenciaConsultasHistoricaYCupoConservador(t *testing.T) {
	pool, _ := asistenciaRepositoryPrueba(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE reservas SET estado = 'ABORDADA', cantidad_abordada = NULL WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	repository := NewPostgresRepository(pool)
	reserva, err := NewService(repository).GetByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if reserva.AsistenciaConocida || reserva.CantidadAbordada != nil || reserva.CantidadPendiente != nil || reserva.CantidadNoPresentada != nil {
		t.Fatal("se inventó asistencia histórica")
	}
	lista, err := NewListaPasajerosService(repository).GetPassengerList(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if lista.OcupacionMaxima != 4 || lista.TotalReservasAsistenciaDesconocida != 1 {
		t.Fatalf("resumen histórico: %+v", lista)
	}
	if lista.TotalPasajerosAbordados != nil || lista.TotalPasajerosPendientes != nil || lista.TotalPasajerosNoPresentados != nil {
		t.Fatal("se presentaron como exactos los totales desconocidos")
	}
}
