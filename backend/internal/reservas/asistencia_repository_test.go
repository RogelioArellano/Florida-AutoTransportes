package reservas

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Estas pruebas requieren ASISTENCIA_TEST_DATABASE_URL. Cada prueba crea
// y elimina su propio esquema; no utiliza las tablas de la aplicación.
// Sin esa variable se omiten, conservando el uso habitual de go test ./...
func asistenciaRepositoryPrueba(t *testing.T) (*pgxpool.Pool, *AsistenciaService) {
	t.Helper()
	url := os.Getenv("ASISTENCIA_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requiere ASISTENCIA_TEST_DATABASE_URL para probar PostgreSQL")
	}
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("configuración inválida de la base de pruebas")
	}
	config.MaxConns = 4
	config.MinConns = 0
	admin, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	esquema := pgx.Identifier{fmt.Sprintf("asistencia_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+esquema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	admin.Close()
	config.ConnConfig.RuntimeParams["search_path"] = esquema
	// Fijamos y verificamos el esquema también después de conectar.
	// Así las pruebas nunca dependen de un search_path heredado.
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, "SET search_path TO "+esquema); err != nil {
			return err
		}
		var actual string
		if err := conn.QueryRow(ctx, "SELECT current_schema()").Scan(&actual); err != nil {
			return err
		}
		if (pgx.Identifier{actual}).Sanitize() != esquema {
			return errors.New("la conexión no utiliza el esquema aislado de prueba")
		}
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(ctx, "DROP SCHEMA "+esquema+" CASCADE"); err != nil {
			t.Errorf("eliminar esquema de prueba: %v", err)
		}
		pool.Close()
	})

	archivos, err := filepath.Glob("../../migrations/*.sql")
	if err != nil || len(archivos) == 0 {
		t.Fatal("no se encontraron las migraciones")
	}
	for _, archivo := range archivos {
		contenido, err := os.ReadFile(archivo)
		if err != nil {
			t.Fatal(err)
		}
		up, _, ok := strings.Cut(string(contenido), "-- +goose Down")
		if !ok {
			t.Fatalf("migración sin sección Down: %s", archivo)
		}
		if _, err := pool.Exec(ctx, up); err != nil {
			t.Fatalf("aplicar %s: %v", filepath.Base(archivo), err)
		}
	}
	const datos = `
		INSERT INTO localidades(nombre, estado) VALUES ('Prueba', 'Michoacán');
		INSERT INTO puntos_abordaje(localidad_id, nombre)
			VALUES (1, 'Centro'), (1, 'Toluca'), (1, 'Destino');
		INSERT INTO rutas(codigo, nombre) VALUES ('TEST', 'Ruta de prueba');
		INSERT INTO ruta_paradas(ruta_id, punto_abordaje_id, orden, permite_subir, permite_bajar)
			VALUES (1, 1, 1, TRUE, FALSE), (1, 2, 2, TRUE, TRUE), (1, 3, 3, FALSE, TRUE);
		INSERT INTO unidades(codigo, placas, capacidad_total, capacidad_pasajeros)
			VALUES ('TEST', 'TEST-001', 17, 16);
		INSERT INTO corridas(folio, ruta_id, ruta_codigo, ruta_nombre,
			unidad_id, unidad_codigo, fecha_servicio, salida_programada, capacidad_pasajeros)
			VALUES ('TEST-1', 1, 'TEST', 'Ruta de prueba', 1, 'TEST', CURRENT_DATE, NOW(), 16);
		INSERT INTO corrida_paradas(corrida_id, ruta_parada_id, punto_abordaje_id,
			punto_nombre, localidad_nombre, estado_nombre, orden, permite_subir,
			permite_bajar, es_obligatoria, incluida_en_recorrido)
			VALUES (1, 1, 1, 'Centro', 'Prueba', 'Michoacán', 1, TRUE, FALSE, TRUE, TRUE),
			(1, 2, 2, 'Toluca', 'Prueba', 'Michoacán', 2, TRUE, TRUE, TRUE, TRUE),
			(1, 3, 3, 'Destino', 'Prueba', 'Michoacán', 3, FALSE, TRUE, TRUE, TRUE);
		INSERT INTO pasajeros(nombre_completo, telefono) VALUES ('Pasajero prueba', '4430000000');
		INSERT INTO reservas(corrida_id, pasajero_id, corrida_parada_origen_id,
			corrida_parada_destino_id, cantidad_pasajeros, precio_unitario,
			subtotal, tipo_descuento, valor_descuento, cantidad_pasajes_descuento,
			monto_descuento, descripcion_descuento, total,
			confirmacion_solicitada_en, confirmacion_limite_en)
			VALUES (1, 1, 1, 3, 3, 100, 300, 'PORCENTAJE', 10, 1, 10, 'Prueba', 290,
			NOW(), NOW() + INTERVAL '90 minutes');
		INSERT INTO reservas(corrida_id, pasajero_id, corrida_parada_origen_id,
			corrida_parada_destino_id, cantidad_pasajeros, precio_unitario, subtotal, total)
			VALUES (1, 1, 2, 3, 1, 100, 100, 100);
		INSERT INTO pagos_reserva(reserva_id, monto, metodo, referencia)
			VALUES (1, 100, 'EFECTIVO', 'PRUEBA');
	`
	if _, err := pool.Exec(ctx, datos); err != nil {
		t.Fatal(err)
	}
	return pool, NewAsistenciaService(NewPostgresRepository(pool))
}

func TestAsistenciaRepositoryParcialYReintentos(t *testing.T) {
	pool, service := asistenciaRepositoryPrueba(t)
	ctx := context.Background()
	finanzas := asistenciaFinanzasPrueba(t, pool)
	nota := "Subieron dos personas"
	primera, err := service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 1, CantidadAbordada: 2, Observaciones: &nota})
	if err != nil {
		t.Fatal(err)
	}
	if primera.EstadoReserva != EstadoAbordada || primera.AbordadaEn == nil {
		t.Fatalf("abordaje inesperado: %+v", primera)
	}
	compararCantidadAsistencia(t, "pendientes", primera.CantidadPendiente, asistenciaIntPrueba(1))
	nuevaNota := "No debe reemplazar la original"
	repetida, err := service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 1, CantidadAbordada: 2, Observaciones: &nuevaNota})
	if err != nil || repetida.AbordadaEn == nil || !repetida.AbordadaEn.Equal(*primera.AbordadaEn) ||
		repetida.ObservacionesAsistencia == nil || *repetida.ObservacionesAsistencia != nota {
		t.Fatalf("reintento modificó el registro: %+v, error=%v", repetida, err)
	}
	for _, caso := range []struct {
		cantidad int
		err      error
	}{
		{1, ErrCantidadAbordadaRetrocede}, {4, ErrCantidadAbordadaExcedeReserva},
	} {
		_, err := service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 1, CantidadAbordada: caso.cantidad})
		if !errors.Is(err, caso.err) {
			t.Fatalf("cantidad %d: %v", caso.cantidad, err)
		}
	}
	cierre, err := service.CloseAttendance(ctx, CerrarAsistenciaInput{ReservaID: 1})
	if err != nil || cierre.EstadoReserva != EstadoAbordada || cierre.AsistenciaCerradaEn == nil {
		t.Fatalf("cierre parcial: %+v, error=%v", cierre, err)
	}
	compararCantidadAsistencia(t, "ausentes", cierre.CantidadNoPresentada, asistenciaIntPrueba(1))
	compararCantidadAsistencia(t, "pendientes", cierre.CantidadPendiente, asistenciaIntPrueba(0))
	recierre, err := service.CloseAttendance(ctx, CerrarAsistenciaInput{ReservaID: 1, Observaciones: &nuevaNota})
	if err != nil || recierre.AsistenciaCerradaEn == nil || !recierre.AsistenciaCerradaEn.Equal(*cierre.AsistenciaCerradaEn) ||
		recierre.ObservacionesAsistencia == nil || *recierre.ObservacionesAsistencia != nota {
		t.Fatalf("reintento modificó el cierre: %+v, error=%v", recierre, err)
	}
	_, err = service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 1, CantidadAbordada: 3})
	if !errors.Is(err, ErrAsistenciaCerrada) {
		t.Fatalf("permitió aumentar después del cierre: %v", err)
	}
	otra, err := service.GetAttendance(ctx, 2)
	if err != nil || otra.AsistenciaCerrada || otra.EstadoReserva != EstadoApartada {
		t.Fatalf("se alteró la reserva de otra parada: %+v, error=%v", otra, err)
	}
	if despues := asistenciaFinanzasPrueba(t, pool); despues != finanzas {
		t.Fatalf("la asistencia modificó datos financieros: antes=%s después=%s", finanzas, despues)
	}
	var requiere bool
	var sinSolicitud bool
	if err := pool.QueryRow(ctx, `SELECT requiere_confirmacion,
		confirmacion_solicitada_en IS NULL AND confirmacion_limite_en IS NULL
		FROM reservas WHERE id = 1`).Scan(&requiere, &sinSolicitud); err != nil || requiere || !sinSolicitud {
		t.Fatalf("no se limpió la confirmación pendiente: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE corridas SET estado = 'COMPLETADA'"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CloseAttendance(ctx, CerrarAsistenciaInput{ReservaID: 1}); err != nil {
		t.Fatalf("rechazó un cierre ya registrado: %v", err)
	}
	if _, err := service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 1, CantidadAbordada: 2}); err != nil {
		t.Fatalf("rechazó un abordaje ya registrado: %v", err)
	}
}

func TestAsistenciaRepositoryIncrementoConservaPrimerAbordaje(t *testing.T) {
	_, service := asistenciaRepositoryPrueba(t)
	ctx := context.Background()
	primera, err := service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 1, CantidadAbordada: 1})
	if err != nil {
		t.Fatal(err)
	}
	ultima, err := service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 1, CantidadAbordada: 3})
	if err != nil || ultima.AbordadaEn == nil || primera.AbordadaEn == nil || !ultima.AbordadaEn.Equal(*primera.AbordadaEn) {
		t.Fatalf("incremento modificó la fecha inicial: %+v, %v", ultima, err)
	}
	compararCantidadAsistencia(t, "abordados", ultima.CantidadAbordada, asistenciaIntPrueba(3))
}

func TestAsistenciaRepositoryAusenciaYParadaIntermedia(t *testing.T) {
	pool, service := asistenciaRepositoryPrueba(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE corridas SET estado = 'EN_CURSO', reservas_abiertas = FALSE;
		UPDATE reservas SET estado = 'CONFIRMADA', confirmada_en = NOW(), requiere_confirmacion = FALSE WHERE id = 2`); err != nil {
		t.Fatal(err)
	}
	cierre, err := service.CloseAttendance(ctx, CerrarAsistenciaInput{ReservaID: 1})
	if err != nil || cierre.EstadoReserva != EstadoNoPresentada || cierre.AbordadaEn != nil {
		t.Fatalf("ausencia inesperada: %+v, %v", cierre, err)
	}
	compararCantidadAsistencia(t, "ausentes", cierre.CantidadNoPresentada, asistenciaIntPrueba(3))
	abordaje, err := service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 2, CantidadAbordada: 1})
	if err != nil || abordaje.ParadaOrigenNombre != "Toluca" || abordaje.EstadoReserva != EstadoAbordada {
		t.Fatalf("rechazó abordaje intermedio: %+v, %v", abordaje, err)
	}
}

func TestAsistenciaRepositoryRechazaEstados(t *testing.T) {
	for _, caso := range []struct {
		nombre, sql      string
		abordaje, cierre error
	}{
		{"cancelada", `UPDATE reservas SET estado = 'CANCELADA', cancelada_en = NOW(), motivo_cancelacion = 'Prueba' WHERE id = 1`, ErrReservaNoAceptaAbordaje, ErrReservaNoAceptaCierreAsistencia},
		{"histórica", `UPDATE reservas SET estado = 'ABORDADA', cantidad_abordada = NULL WHERE id = 1`, ErrAsistenciaHistoricaDesconocida, ErrAsistenciaHistoricaDesconocida},
		{"corrida completada", `UPDATE corridas SET estado = 'COMPLETADA'`, ErrCorridaNoAceptaAsistencia, ErrCorridaNoAceptaAsistencia},
		{"corrida cancelada", `UPDATE corridas SET estado = 'CANCELADA', reservas_abiertas = FALSE`, ErrCorridaNoAceptaAsistencia, ErrCorridaNoAceptaAsistencia},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			pool, service := asistenciaRepositoryPrueba(t)
			if _, err := pool.Exec(context.Background(), caso.sql); err != nil {
				t.Fatal(err)
			}
			_, err := service.RegisterBoarding(context.Background(), RegistrarAbordajeInput{ReservaID: 1, CantidadAbordada: 1})
			if !errors.Is(err, caso.abordaje) {
				t.Fatalf("abordaje: %v", err)
			}
			_, err = service.CloseAttendance(context.Background(), CerrarAsistenciaInput{ReservaID: 1})
			if !errors.Is(err, caso.cierre) {
				t.Fatalf("cierre: %v", err)
			}
		})
	}
}

func TestAsistenciaRepositoryNoEncontrada(t *testing.T) {
	_, service := asistenciaRepositoryPrueba(t)
	ctx := context.Background()
	_, consulta := service.GetAttendance(ctx, 999999)
	_, abordaje := service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 999999, CantidadAbordada: 1})
	_, cierre := service.CloseAttendance(ctx, CerrarAsistenciaInput{ReservaID: 999999})
	for _, err := range []error{consulta, abordaje, cierre} {
		if !errors.Is(err, ErrReservaNoEncontrada) {
			t.Fatalf("error inesperado: %v", err)
		}
	}
}

// Esta prueba necesita PostgreSQL con conexiones independientes para
// comprobar que dos solicitudes no hacen retroceder el total acumulado.
func TestAsistenciaRepositoryConcurrente(t *testing.T) {
	_, service := asistenciaRepositoryPrueba(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	inicio := make(chan struct{})
	errores := make(chan error, 2)
	var grupo sync.WaitGroup
	for _, cantidad := range []int{2, 3} {
		grupo.Add(1)
		go func(cantidad int) {
			defer grupo.Done()
			<-inicio
			_, err := service.RegisterBoarding(ctx, RegistrarAbordajeInput{ReservaID: 1, CantidadAbordada: cantidad})
			errores <- err
		}(cantidad)
	}
	close(inicio)
	grupo.Wait()
	close(errores)
	for err := range errores {
		if err != nil && !errors.Is(err, ErrCantidadAbordadaRetrocede) {
			t.Fatal(err)
		}
	}
	final, err := service.GetAttendance(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	compararCantidadAsistencia(t, "total final", final.CantidadAbordada, asistenciaIntPrueba(3))
}

func asistenciaFinanzasPrueba(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	const query = `SELECT jsonb_build_object(
		'precio', r.precio_unitario, 'subtotal', r.subtotal, 'total', r.total,
		'tipo_descuento', r.tipo_descuento, 'valor_descuento', r.valor_descuento,
		'pasajes_descuento', r.cantidad_pasajes_descuento, 'monto_descuento', r.monto_descuento,
		'descripcion_descuento', r.descripcion_descuento, 'reembolsable', r.cancelacion_reembolsable,
		'monto_reembolsable', r.monto_reembolsable, 'limite_reembolso', r.limite_reembolso_en,
		'pagos', (SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM pagos_reserva p WHERE p.reserva_id = r.id),
		'reembolsos', (SELECT jsonb_agg(to_jsonb(x) ORDER BY x.id) FROM reembolsos_reserva x WHERE x.reserva_id = r.id)
	)::TEXT FROM reservas r WHERE r.id = 1`
	var resultado string
	if err := pool.QueryRow(context.Background(), query).Scan(&resultado); err != nil {
		t.Fatal(err)
	}
	return resultado
}
