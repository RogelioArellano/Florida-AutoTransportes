package corridas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"floridaAT/internal/reservas"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cada prueba usa un esquema nuevo y lo elimina al terminar.
// No accede a las tablas públicas ni reutiliza las reservas del usuario.
func operacionRepositoryPrueba(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("OPERACION_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requiere OPERACION_TEST_DATABASE_URL para probar PostgreSQL")
	}
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("configuración inválida de la base de prueba")
	}
	config.MaxConns = 4
	config.MinConns = 0
	admin, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	nombre := fmt.Sprintf("operacion_test_%d", time.Now().UnixNano())
	esquema := pgx.Identifier{nombre}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+esquema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	admin.Close()
	config.ConnConfig.RuntimeParams["search_path"] = esquema
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, "SET search_path TO "+esquema); err != nil {
			return err
		}
		var actual string
		if err := conn.QueryRow(ctx, "SELECT current_schema()").Scan(&actual); err != nil {
			return err
		}
		if actual != nombre {
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
			t.Errorf("eliminar esquema: %v", err)
		}
		pool.Close()
	})
	archivos, err := filepath.Glob("../../migrations/*.sql")
	if err != nil || len(archivos) == 0 {
		t.Fatal("no se encontraron migraciones")
	}
	for _, archivo := range archivos {
		contenido, err := os.ReadFile(archivo)
		if err != nil {
			t.Fatal(err)
		}
		up, _, ok := strings.Cut(string(contenido), "-- +goose Down")
		if !ok {
			t.Fatalf("migración sin Down: %s", archivo)
		}
		if _, err := pool.Exec(ctx, up); err != nil {
			t.Fatalf("%s: %v", filepath.Base(archivo), err)
		}
	}
	operacionExecPrueba(t, pool, `
		INSERT INTO localidades(nombre, estado) VALUES ('Prueba', 'Michoacán');
		INSERT INTO puntos_abordaje(localidad_id,nombre) VALUES (1,'Centro'),(1,'Intermedio'),(1,'Destino');
		INSERT INTO rutas(codigo,nombre) VALUES ('OPERACION','Ruta de prueba');
		INSERT INTO ruta_paradas(ruta_id,punto_abordaje_id,orden,permite_subir,permite_bajar)
		VALUES (1,1,1,TRUE,FALSE),(1,2,2,TRUE,TRUE),(1,3,3,FALSE,TRUE);
		INSERT INTO unidades(codigo,capacidad_total,capacidad_pasajeros) VALUES ('OPERACION',17,16);
		INSERT INTO choferes(nombre_completo,telefono,licencia_numero,licencia_vigencia)
		VALUES ('Chofer prueba','0000000000','PRUEBA',CURRENT_DATE + 365);
		INSERT INTO corridas(folio,ruta_id,ruta_codigo,ruta_nombre,unidad_id,unidad_codigo,
		chofer_id,chofer_nombre,fecha_servicio,salida_programada,capacidad_pasajeros)
		VALUES ('OPERACION-1',1,'OPERACION','Ruta de prueba',1,'OPERACION',1,'Chofer prueba',CURRENT_DATE,NOW(),16);
		INSERT INTO corrida_paradas(corrida_id,ruta_parada_id,punto_abordaje_id,punto_nombre,
		localidad_nombre,estado_nombre,orden,permite_subir,permite_bajar,es_obligatoria,incluida_en_recorrido)
		VALUES (1,1,1,'Centro','Prueba','Michoacán',1,TRUE,FALSE,TRUE,TRUE),
		(1,2,2,'Intermedio','Prueba','Michoacán',2,TRUE,TRUE,TRUE,TRUE),
		(1,3,3,'Destino','Prueba','Michoacán',3,FALSE,TRUE,TRUE,TRUE);
		INSERT INTO pasajeros(nombre_completo,telefono) VALUES ('Pasajero prueba','0000000000');
		INSERT INTO reservas(corrida_id,pasajero_id,corrida_parada_origen_id,corrida_parada_destino_id,
		cantidad_pasajeros,precio_unitario,subtotal,total,estado,requiere_confirmacion,confirmada_en)
		VALUES (1,1,1,3,3,100,300,300,'CONFIRMADA',FALSE,NOW()),
		(1,1,2,3,1,100,100,100,'CONFIRMADA',FALSE,NOW());
		INSERT INTO pagos_reserva(reserva_id,monto,metodo) VALUES (1,90,'EFECTIVO');
	`)
	return pool
}

func operacionExecPrueba(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Fatal(err)
	}
}

func operacionDataPrueba(t *testing.T, mux *http.ServeMux, metodo, ruta, body string, status int) Corrida {
	t.Helper()
	w := operacionHTTPPrueba(mux, metodo, ruta, body)
	if w.Code != status {
		t.Fatalf("%s %s: status=%d body=%s", metodo, ruta, w.Code, w.Body.String())
	}
	var resultado struct {
		Data Corrida `json:"data"`
	}
	if status == 200 {
		if err := json.Unmarshal(w.Body.Bytes(), &resultado); err != nil {
			t.Fatal(err)
		}
	}
	return resultado.Data
}

func operacionFinanzasPrueba(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var resultado string
	err := pool.QueryRow(context.Background(), `SELECT json_agg(s ORDER BY reserva_id)::TEXT
		FROM vw_reservas_saldos s`).Scan(&resultado)
	if err != nil {
		t.Fatal(err)
	}
	return resultado
}

func TestOperacionRepositoryFlujoHTTPYParadaIntermedia(t *testing.T) {
	pool := operacionRepositoryPrueba(t)
	finanzas := operacionFinanzasPrueba(t, pool)
	mux := http.NewServeMux()
	NewOperacionHandler(NewOperacionService(NewPostgresRepository(pool))).RegisterRoutes(mux)
	repoReservas := reservas.NewPostgresRepository(pool)
	reservas.NewAsistenciaHandler(reservas.NewAsistenciaService(repoReservas)).RegisterRoutes(mux)
	handlerReservas := reservas.NewHandler(reservas.NewService(repoReservas))
	mux.HandleFunc("/api/reservas", handlerReservas.Handle)
	input := `{"corrida_id":1}`
	if c := operacionDataPrueba(t, mux, "GET", "/api/corridas/1", "", 200); c.Estado != EstadoProgramada || len(c.Paradas) != 3 {
		t.Fatalf("consulta: %+v", c)
	}
	operacionDataPrueba(t, mux, "POST", "/api/corridas/salidas", input, 409)
	operacionDataPrueba(t, mux, "POST", "/api/corridas/finalizaciones", input, 409)
	abierta := operacionDataPrueba(t, mux, "POST", "/api/corridas/abordajes/aperturas", input, 200)
	if abierta.Estado != EstadoAbordando || !abierta.ReservasAbiertas || abierta.SalidaReal != nil {
		t.Fatalf("apertura: %+v", abierta)
	}
	if repetida := operacionDataPrueba(t, mux, "POST", "/api/corridas/abordajes/aperturas", input, 200); !reflect.DeepEqual(abierta, repetida) {
		t.Fatal("la apertura repetida cambió los datos")
	}
	operacionDataPrueba(t, mux, "POST", "/api/corridas/salidas", input, 409)
	for _, caso := range []struct{ ruta, body string }{
		{"/api/reservas/abordajes", `{"reserva_id":1,"cantidad_abordada":2}`},
		{"/api/reservas/asistencia/cierres", `{"reserva_id":1}`},
	} {
		if w := operacionHTTPPrueba(mux, "POST", caso.ruta, caso.body); w.Code != 200 {
			t.Fatalf("asistencia: %s", w.Body.String())
		}
	}
	salida := operacionDataPrueba(t, mux, "POST", "/api/corridas/salidas", input, 200)
	if salida.Estado != EstadoEnCurso || salida.SalidaReal == nil || salida.ReservasAbiertas || salida.LlegadaReal != nil {
		t.Fatalf("salida: %+v", salida)
	}
	if repetida := operacionDataPrueba(t, mux, "POST", "/api/corridas/salidas", input, 200); !reflect.DeepEqual(salida, repetida) {
		t.Fatal("la salida repetida cambió la fecha")
	}
	operacionDataPrueba(t, mux, "POST", "/api/corridas/abordajes/aperturas", input, 409)
	operacionDataPrueba(t, mux, "POST", "/api/corridas/finalizaciones", input, 409)
	// Cierra ventas, pero permite abordar y cerrar en la siguiente parada.
	w := operacionHTTPPrueba(mux, "POST", "/api/reservas", `{"corrida_id":1,"pasajero_id":1,"corrida_parada_origen_id":2,"corrida_parada_destino_id":3,"cantidad_pasajeros":1,"precio_unitario":"100.00"}`)
	if w.Code != 409 {
		t.Fatalf("aceptó una reserva después de salir: %s", w.Body.String())
	}
	for _, caso := range []struct{ ruta, body string }{
		{"/api/reservas/abordajes", `{"reserva_id":2,"cantidad_abordada":1}`},
		{"/api/reservas/asistencia/cierres", `{"reserva_id":2}`},
	} {
		if w := operacionHTTPPrueba(mux, "POST", caso.ruta, caso.body); w.Code != 200 {
			t.Fatalf("parada intermedia: %s", w.Body.String())
		}
	}
	final := operacionDataPrueba(t, mux, "POST", "/api/corridas/finalizaciones", input, 200)
	if final.Estado != EstadoCompletada || final.LlegadaReal == nil || final.LlegadaReal.Before(*final.SalidaReal) || !final.SalidaReal.Equal(*salida.SalidaReal) {
		t.Fatalf("finalización: %+v", final)
	}
	if repetida := operacionDataPrueba(t, mux, "POST", "/api/corridas/finalizaciones", input, 200); !reflect.DeepEqual(final, repetida) {
		t.Fatal("la finalización repetida cambió la fecha")
	}
	if repetida := operacionDataPrueba(t, mux, "POST", "/api/corridas/salidas", input, 200); !reflect.DeepEqual(final, repetida) {
		t.Fatal("el reintento de salida revirtió la finalización")
	}
	if operacionFinanzasPrueba(t, pool) != finanzas {
		t.Fatal("la operación cambió pagos o adeudos")
	}
}

func TestOperacionRepositoryConflictos(t *testing.T) {
	for _, caso := range []struct {
		nombre, preparar string
		destino          Estado
		esperado         error
	}{
		{"sin chofer", "UPDATE corridas SET chofer_id=NULL,chofer_nombre=NULL", EstadoAbordando, ErrCorridaSinChofer},
		{"unidad inactiva", "UPDATE unidades SET activa=FALSE", EstadoAbordando, ErrUnidadNoDisponible},
		{"chofer inactivo", "UPDATE choferes SET activo=FALSE", EstadoAbordando, ErrChoferNoDisponible},
		{"licencia vencida", "UPDATE choferes SET licencia_vigencia=CURRENT_DATE-2", EstadoAbordando, ErrLicenciaNoVigente},
		{"licencia faltante", "UPDATE choferes SET licencia_numero=NULL", EstadoAbordando, ErrLicenciaNoVigente},
		{"sin tramo", "UPDATE corrida_paradas SET incluida_en_recorrido=FALSE", EstadoAbordando, ErrCorridaSinTramo},
		{"cancelada", "UPDATE corridas SET estado='CANCELADA',reservas_abiertas=FALSE", EstadoAbordando, ErrTransicionNoPermitida},
		{"histórica desconocida", "UPDATE corridas SET estado='EN_CURSO',salida_real=NOW(); UPDATE reservas SET estado='ABORDADA',cantidad_abordada=NULL", EstadoCompletada, ErrAsistenciasPendientes},
		{"en curso sin salida", "UPDATE corridas SET estado='EN_CURSO'", EstadoCompletada, ErrFechasOperacionInconsistentes},
		{"reintento sin salida", "UPDATE corridas SET estado='EN_CURSO'", EstadoEnCurso, ErrFechasOperacionInconsistentes},
		{"reintento sin llegada", "UPDATE corridas SET estado='COMPLETADA',salida_real=NOW()", EstadoCompletada, ErrFechasOperacionInconsistentes},
		{"unidad inactiva antes de salir", "UPDATE corridas SET estado='ABORDANDO'; UPDATE unidades SET activa=FALSE", EstadoEnCurso, ErrUnidadNoDisponible},
		{"chofer inactivo antes de salir", "UPDATE corridas SET estado='ABORDANDO'; UPDATE choferes SET activo=FALSE", EstadoEnCurso, ErrChoferNoDisponible},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			pool := operacionRepositoryPrueba(t)
			operacionExecPrueba(t, pool, caso.preparar)
			repo := NewPostgresRepository(pool)
			antes, err := repo.GetOperation(context.Background(), 1)
			if err != nil {
				t.Fatal(err)
			}
			_, err = repo.TransitionOperation(context.Background(), 1, caso.destino)
			if !errors.Is(err, caso.esperado) {
				t.Fatalf("esperado=%v recibido=%v", caso.esperado, err)
			}
			despues, err := repo.GetOperation(context.Background(), 1)
			if err != nil || !reflect.DeepEqual(antes, despues) {
				t.Fatal("un conflicto modificó la corrida")
			}
		})
	}
}

func TestOperacionRepositoryNoEncontradaYCanceladasExcluidas(t *testing.T) {
	pool := operacionRepositoryPrueba(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()
	if _, err := repo.GetOperation(ctx, 9999); !errors.Is(err, ErrCorridaNoEncontrada) {
		t.Fatalf("consulta inexistente: %v", err)
	}
	if _, err := repo.TransitionOperation(ctx, 9999, EstadoAbordando); !errors.Is(err, ErrCorridaNoEncontrada) {
		t.Fatalf("operación inexistente: %v", err)
	}
	operacionExecPrueba(t, pool, `UPDATE reservas SET estado='CANCELADA',cancelada_en=NOW(),motivo_cancelacion='Prueba'`)
	for _, destino := range []Estado{EstadoAbordando, EstadoEnCurso, EstadoCompletada} {
		if _, err := repo.TransitionOperation(ctx, 1, destino); err != nil {
			t.Fatalf("destino=%s: %v", destino, err)
		}
	}
}

// Ejecutar con PostgreSQL nativo para comprobar dos conexiones simultáneas.
func TestOperacionRepositorySalidasConcurrentes(t *testing.T) {
	pool := operacionRepositoryPrueba(t)
	operacionExecPrueba(t, pool, `UPDATE corridas SET estado='ABORDANDO';
		UPDATE reservas SET estado='NO_PRESENTADA',asistencia_cerrada_en=NOW() WHERE id=1`)
	repo := NewPostgresRepository(pool)
	var resultados [2]Corrida
	var errores [2]error
	var wg sync.WaitGroup
	inicio := make(chan struct{})
	for i := range resultados {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-inicio
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resultados[i], errores[i] = repo.TransitionOperation(ctx, 1, EstadoEnCurso)
		}(i)
	}
	close(inicio)
	wg.Wait()
	for _, err := range errores {
		if err != nil {
			t.Fatal(err)
		}
	}
	if resultados[0].SalidaReal == nil || !reflect.DeepEqual(resultados[0], resultados[1]) {
		t.Fatal("dos salidas generaron resultados diferentes")
	}
}

// La finalización puede ganar el bloqueo y rechazar una asistencia pendiente,
// o esperar al cierre y completar. En ambos casos nunca completa con pendientes.
func TestOperacionRepositoryFinalizacionYAsistenciaConcurrentes(t *testing.T) {
	pool := operacionRepositoryPrueba(t)
	operacionExecPrueba(t, pool, `UPDATE corridas SET estado='EN_CURSO',salida_real=NOW();
		UPDATE reservas SET estado='NO_PRESENTADA',asistencia_cerrada_en=NOW() WHERE id=1`)
	repo := NewPostgresRepository(pool)
	asistencia := reservas.NewAsistenciaService(reservas.NewPostgresRepository(pool))
	var errCierre, errFinal error
	var wg sync.WaitGroup
	inicio := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-inicio
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, errCierre = asistencia.CloseAttendance(ctx, reservas.CerrarAsistenciaInput{ReservaID: 2})
	}()
	go func() {
		defer wg.Done()
		<-inicio
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, errFinal = repo.TransitionOperation(ctx, 1, EstadoCompletada)
	}()
	close(inicio)
	wg.Wait()
	if errCierre != nil {
		t.Fatal(errCierre)
	}
	if errFinal != nil && !errors.Is(errFinal, ErrAsistenciasPendientes) {
		t.Fatal(errFinal)
	}
	final, err := repo.TransitionOperation(context.Background(), 1, EstadoCompletada)
	if err != nil || final.Estado != EstadoCompletada {
		t.Fatalf("final=%+v error=%v", final, err)
	}
	var abiertas int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM reservas
		WHERE corrida_id=1 AND estado<>'CANCELADA' AND asistencia_cerrada_en IS NULL`).Scan(&abiertas); err != nil {
		t.Fatal(err)
	}
	if abiertas != 0 {
		t.Fatal("completó con asistencia abierta")
	}
}
