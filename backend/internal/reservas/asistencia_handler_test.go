package reservas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Usamos el Service real y el Store fake de las pruebas del service.
// Así se comprueban rutas, JSON y validación sin conectarse a PostgreSQL.
func asistenciaMuxPrueba(store *asistenciaStoreFake) *http.ServeMux {
	mux := http.NewServeMux()
	// Simula las rutas existentes y comprueba que las nuevas son
	// más específicas que la consulta genérica de reservas.
	mux.HandleFunc("/api/reservas/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	mux.HandleFunc("/api/reservas/{reservaID}/reembolsos", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	NewAsistenciaHandler(NewAsistenciaService(store)).RegisterRoutes(mux)
	return mux
}

func asistenciaHTTPPrueba(mux http.Handler, metodo, ruta, cuerpo string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(metodo, ruta, strings.NewReader(cuerpo))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func asistenciaRespuestaPrueba(t *testing.T, response *httptest.ResponseRecorder) AsistenciaReserva {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d, body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
		response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("encabezados inesperados: %v", response.Header())
	}
	var resultado struct {
		Data *AsistenciaReserva `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &resultado); err != nil {
		t.Fatal(err)
	}
	if resultado.Data == nil {
		t.Fatal("falta el objeto data")
	}
	return *resultado.Data
}

func TestAsistenciaHandlerConsulta(t *testing.T) {
	store := &asistenciaStoreFake{resultado: AsistenciaReserva{
		ReservaID: 6, EstadoReserva: EstadoAbordada,
		CantidadPasajeros: 3, CantidadAbordada: asistenciaIntPrueba(2),
	}}
	response := asistenciaHTTPPrueba(asistenciaMuxPrueba(store), "GET", "/api/reservas/6/asistencia", "")
	resultado := asistenciaRespuestaPrueba(t, response)
	if store.consultaCalls != 1 || store.consultaID != 6 || resultado.ReservaID != 6 {
		t.Fatalf("consulta inesperada: %+v", resultado)
	}
	compararCantidadAsistencia(t, "pendientes", resultado.CantidadPendiente, asistenciaIntPrueba(1))
	if resultado.CantidadNoPresentada != nil || !resultado.AsistenciaConocida {
		t.Fatalf("resumen inesperado: %+v", resultado)
	}
	limite, ok := store.ctxRecibido.Deadline()
	if !ok || time.Until(limite) > 5*time.Second {
		t.Fatal("la consulta debe tener un timeout máximo de 5 segundos")
	}
}

func TestAsistenciaHandlerRegistraAbordaje(t *testing.T) {
	store := &asistenciaStoreFake{resultado: AsistenciaReserva{
		ReservaID: 6, EstadoReserva: EstadoAbordada,
		CantidadPasajeros: 3, CantidadAbordada: asistenciaIntPrueba(2),
	}}
	response := asistenciaHTTPPrueba(asistenciaMuxPrueba(store), "POST", "/api/reservas/abordajes",
		`{"reserva_id":6,"cantidad_abordada":2,"observaciones":"  Subieron dos personas  "}`)
	resultado := asistenciaRespuestaPrueba(t, response)
	input := store.abordajeInput
	if store.abordajeCalls != 1 || input.ReservaID != 6 || input.CantidadAbordada != 2 ||
		input.Observaciones == nil || *input.Observaciones != "Subieron dos personas" {
		t.Fatalf("input inesperado: %+v", input)
	}
	compararCantidadAsistencia(t, "abordados", resultado.CantidadAbordada, asistenciaIntPrueba(2))
	limite, ok := store.ctxRecibido.Deadline()
	if !ok || time.Until(limite) > 10*time.Second {
		t.Fatal("el abordaje debe tener un timeout máximo de 10 segundos")
	}
}

func TestAsistenciaHandlerCierraParcial(t *testing.T) {
	fecha := time.Now().UTC()
	store := &asistenciaStoreFake{resultado: AsistenciaReserva{
		ReservaID: 6, EstadoReserva: EstadoAbordada, CantidadPasajeros: 3,
		CantidadAbordada: asistenciaIntPrueba(2), AsistenciaCerradaEn: &fecha,
	}}
	response := asistenciaHTTPPrueba(asistenciaMuxPrueba(store), "POST", "/api/reservas/asistencia/cierres",
		`{"reserva_id":6,"observaciones":"  El tercer pasajero no llegó  "}`)
	resultado := asistenciaRespuestaPrueba(t, response)
	input := store.cierreInput
	if store.cierreCalls != 1 || input.ReservaID != 6 || input.Observaciones == nil ||
		*input.Observaciones != "El tercer pasajero no llegó" {
		t.Fatalf("input inesperado: %+v", input)
	}
	if !resultado.AsistenciaCerrada || resultado.EstadoReserva != EstadoAbordada {
		t.Fatalf("estado inesperado: %+v", resultado)
	}
	compararCantidadAsistencia(t, "pendientes", resultado.CantidadPendiente, asistenciaIntPrueba(0))
	compararCantidadAsistencia(t, "ausentes", resultado.CantidadNoPresentada, asistenciaIntPrueba(1))
}

func TestAsistenciaHandlerConservaHistoricaDesconocida(t *testing.T) {
	store := &asistenciaStoreFake{resultado: AsistenciaReserva{
		ReservaID: 6, EstadoReserva: EstadoAbordada, CantidadPasajeros: 3,
	}}
	response := asistenciaHTTPPrueba(asistenciaMuxPrueba(store), "GET", "/api/reservas/6/asistencia", "")
	resultado := asistenciaRespuestaPrueba(t, response)
	if resultado.AsistenciaConocida || resultado.CantidadAbordada != nil ||
		resultado.CantidadPendiente != nil || resultado.CantidadNoPresentada != nil {
		t.Fatalf("se inventaron cantidades históricas: %+v", resultado)
	}
	// NULL debe aparecer explícitamente, sin convertirse a cero ni omitirse.
	for _, campo := range []string{"cantidad_abordada", "cantidad_pendiente", "cantidad_no_presentada"} {
		if !strings.Contains(response.Body.String(), `"`+campo+`":null`) {
			t.Fatalf("falta %s:null en %s", campo, response.Body.String())
		}
	}
}

func TestAsistenciaHandlerRechazaIDsRuta(t *testing.T) {
	for _, id := range []string{"0", "-1", "abc", "9223372036854775808"} {
		t.Run(id, func(t *testing.T) {
			store := &asistenciaStoreFake{}
			response := asistenciaHTTPPrueba(asistenciaMuxPrueba(store), "GET", "/api/reservas/"+id+"/asistencia", "")
			if response.Code != http.StatusBadRequest || store.consultaCalls != 0 {
				t.Fatalf("status=%d, llamadas=%d", response.Code, store.consultaCalls)
			}
		})
	}
}

func TestAsistenciaHandlerRechazaCuerposInvalidos(t *testing.T) {
	for _, ruta := range []string{"/api/reservas/abordajes", "/api/reservas/asistencia/cierres"} {
		for _, cuerpo := range []string{
			"", "null", "[]", `"texto"`, `{`, `{}`, `{"reserva_id":0}`,
			`{"reserva_id":"6"}`, `{"reserva_id":6,"campo_desconocido":true}`,
			`{"reserva_id":6} {"reserva_id":7}`,
			`{"reserva_id":6,"observaciones":"` + strings.Repeat("a", 1<<20) + `"}`,
		} {
			t.Run(ruta+"/"+fmt.Sprint(len(cuerpo))+"/"+cuerpo[:min(len(cuerpo), 40)], func(t *testing.T) {
				store := &asistenciaStoreFake{}
				response := asistenciaHTTPPrueba(asistenciaMuxPrueba(store), "POST", ruta, cuerpo)
				if response.Code != http.StatusBadRequest || store.abordajeCalls+store.cierreCalls != 0 {
					t.Fatalf("status=%d, body=%s", response.Code, response.Body.String())
				}
			})
		}
	}
	for _, cuerpo := range []string{
		`{"reserva_id":6}`, `{"reserva_id":6,"cantidad_abordada":0}`,
		`{"reserva_id":6,"cantidad_abordada":-1}`, `{"reserva_id":6,"cantidad_abordada":101}`,
		`{"reserva_id":6,"cantidad_abordada":"2"}`, `{"reserva_id":6,"cantidad_abordada":1.5}`,
	} {
		store := &asistenciaStoreFake{}
		response := asistenciaHTTPPrueba(asistenciaMuxPrueba(store), "POST", "/api/reservas/abordajes", cuerpo)
		if response.Code != http.StatusBadRequest || store.abordajeCalls != 0 {
			t.Fatalf("cantidad inválida aceptada: cuerpo=%s, status=%d", cuerpo, response.Code)
		}
	}
}

func TestAsistenciaHandlerMapeaErrores(t *testing.T) {
	casos := []struct {
		err    error
		status int
	}{
		{ErrDatosInvalidos, http.StatusBadRequest},
		{ErrReservaNoEncontrada, http.StatusNotFound},
		{ErrReservaNoAceptaAbordaje, http.StatusConflict},
		{ErrReservaNoAceptaCierreAsistencia, http.StatusConflict},
		{ErrCorridaNoAceptaAsistencia, http.StatusConflict},
		{ErrAsistenciaCerrada, http.StatusConflict},
		{ErrCantidadAbordadaExcedeReserva, http.StatusConflict},
		{ErrCantidadAbordadaRetrocede, http.StatusConflict},
		{ErrAsistenciaHistoricaDesconocida, http.StatusConflict},
		{context.DeadlineExceeded, http.StatusGatewayTimeout},
		{errors.New("detalle técnico privado"), http.StatusInternalServerError},
	}
	for _, operacion := range []struct{ metodo, ruta, cuerpo string }{
		{"GET", "/api/reservas/6/asistencia", ""},
		{"POST", "/api/reservas/abordajes", `{"reserva_id":6,"cantidad_abordada":1}`},
		{"POST", "/api/reservas/asistencia/cierres", `{"reserva_id":6}`},
	} {
		for _, caso := range casos {
			t.Run(operacion.ruta+"/"+caso.err.Error(), func(t *testing.T) {
				store := &asistenciaStoreFake{err: fmt.Errorf("operación: %w", caso.err)}
				response := asistenciaHTTPPrueba(asistenciaMuxPrueba(store), operacion.metodo, operacion.ruta, operacion.cuerpo)
				if response.Code != caso.status {
					t.Fatalf("status=%d; esperado=%d; body=%s", response.Code, caso.status, response.Body.String())
				}
				var body struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Error == "" {
					t.Fatalf("error JSON inesperado: %s", response.Body.String())
				}
				if caso.status == http.StatusInternalServerError && strings.Contains(body.Error, "detalle técnico privado") {
					t.Fatal("se expuso un detalle técnico al cliente")
				}
			})
		}
	}
}

func TestAsistenciaHandlerRechazaMetodos(t *testing.T) {
	for _, caso := range []struct{ ruta, permitido, metodo string }{
		{"/api/reservas/6/asistencia", "GET", "POST"},
		{"/api/reservas/abordajes", "POST", "GET"},
		{"/api/reservas/asistencia/cierres", "POST", "DELETE"},
	} {
		t.Run(caso.ruta, func(t *testing.T) {
			store := &asistenciaStoreFake{}
			response := asistenciaHTTPPrueba(asistenciaMuxPrueba(store), caso.metodo, caso.ruta, "")
			if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != caso.permitido ||
				store.consultaCalls+store.abordajeCalls+store.cierreCalls != 0 {
				t.Fatalf("respuesta inesperada: status=%d, Allow=%s", response.Code, response.Header().Get("Allow"))
			}
		})
	}
}

func TestAsistenciaHandlerConservaRutasExistentes(t *testing.T) {
	mux := asistenciaMuxPrueba(&asistenciaStoreFake{})
	for _, caso := range []struct {
		ruta   string
		status int
	}{
		{"/api/reservas/6", http.StatusTeapot},
		{"/api/reservas/6/reembolsos", http.StatusNoContent},
	} {
		response := asistenciaHTTPPrueba(mux, "GET", caso.ruta, "")
		if response.Code != caso.status {
			t.Fatalf("ruta %s: status=%d", caso.ruta, response.Code)
		}
	}
}
