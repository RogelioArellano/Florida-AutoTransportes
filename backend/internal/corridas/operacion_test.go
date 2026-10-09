package corridas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type operacionStoreFake struct {
	id        int64
	destino   Estado
	llamadas  int
	resultado Corrida
	err       error
}

func (f *operacionStoreFake) GetOperation(_ context.Context, id int64) (Corrida, error) {
	f.id = id
	f.llamadas++
	return f.resultado, f.err
}
func (f *operacionStoreFake) TransitionOperation(_ context.Context, id int64, destino Estado) (Corrida, error) {
	f.id = id
	f.destino = destino
	f.llamadas++
	return f.resultado, f.err
}

func TestOperacionServiceValidaIDYPropagaErrores(t *testing.T) {
	for _, id := range []int64{0, -1} {
		store := &operacionStoreFake{}
		s := NewOperacionService(store)
		acciones := []func() (Corrida, error){
			func() (Corrida, error) { return s.GetByID(context.Background(), id) },
			func() (Corrida, error) { return s.OpenBoarding(context.Background(), OperacionInput{CorridaID: id}) },
			func() (Corrida, error) { return s.StartTrip(context.Background(), OperacionInput{CorridaID: id}) },
			func() (Corrida, error) { return s.CompleteTrip(context.Background(), OperacionInput{CorridaID: id}) },
		}
		for _, accion := range acciones {
			if _, err := accion(); !errors.Is(err, ErrDatosInvalidos) {
				t.Fatalf("id=%d: %v", id, err)
			}
		}
		if store.llamadas != 0 {
			t.Fatal("el ID inválido llegó al repositorio")
		}
	}
	store := &operacionStoreFake{err: fmt.Errorf("contexto: %w", ErrAsistenciasPendientes)}
	_, err := NewOperacionService(store).CompleteTrip(context.Background(), OperacionInput{CorridaID: 7})
	if !errors.Is(err, ErrAsistenciasPendientes) || store.id != 7 || store.destino != EstadoCompletada {
		t.Fatalf("no propagó la operación y el error: %+v, %v", store, err)
	}
}

func operacionHTTPPrueba(mux *http.ServeMux, method, ruta, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(method, ruta, strings.NewReader(body)))
	return w
}

func TestOperacionHandlerRutasYCompatibilidad(t *testing.T) {
	store := &operacionStoreFake{resultado: Corrida{ID: 7, Estado: EstadoAbordando, Paradas: []CorridaParada{}}}
	mux := http.NewServeMux()
	NewOperacionHandler(NewOperacionService(store)).RegisterRoutes(mux)
	// Las rutas más específicas del módulo de listas siguen teniendo prioridad.
	for _, ruta := range []string{"/api/corridas", "/api/corridas/{corridaID}/pasajeros",
		"/api/corridas/listas-pasajeros/pendientes", "/api/corridas/listas-pasajeros/envios"} {
		mux.HandleFunc(ruta, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(202) })
	}
	for _, caso := range []struct {
		ruta    string
		destino Estado
	}{
		{"/api/corridas/abordajes/aperturas", EstadoAbordando},
		{"/api/corridas/salidas", EstadoEnCurso},
		{"/api/corridas/finalizaciones", EstadoCompletada},
	} {
		w := operacionHTTPPrueba(mux, "POST", caso.ruta, `{"corrida_id":7}`)
		if w.Code != 200 || store.destino != caso.destino || store.id != 7 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("ruta %s: status=%d, body=%s", caso.ruta, w.Code, w.Body.String())
		}
		var respuesta struct {
			Data Corrida `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &respuesta); err != nil || respuesta.Data.ID != 7 {
			t.Fatal("respuesta inválida")
		}
		w = operacionHTTPPrueba(mux, "GET", caso.ruta, "")
		if w.Code != 405 || w.Header().Get("Allow") != "POST" {
			t.Fatalf("método de %s: %d", caso.ruta, w.Code)
		}
	}
	if w := operacionHTTPPrueba(mux, "GET", "/api/corridas/7", ""); w.Code != 200 || store.id != 7 {
		t.Fatal("falló la consulta")
	}
	if w := operacionHTTPPrueba(mux, "POST", "/api/corridas/7", ""); w.Code != 405 || w.Header().Get("Allow") != "GET" {
		t.Fatal("falló 405 en detalle")
	}
	for _, ruta := range []string{"/api/corridas", "/api/corridas/7/pasajeros", "/api/corridas/listas-pasajeros/pendientes", "/api/corridas/listas-pasajeros/envios"} {
		if w := operacionHTTPPrueba(mux, "GET", ruta, ""); w.Code != 202 {
			t.Fatalf("alteró %s", ruta)
		}
	}
}

func TestOperacionHandlerJSONEIDsInvalidos(t *testing.T) {
	store := &operacionStoreFake{}
	mux := http.NewServeMux()
	NewOperacionHandler(NewOperacionService(store)).RegisterRoutes(mux)
	for _, body := range []string{"", "null", "[]", `{}`, `{"corrida_id":0}`, `{"corrida_id":-7}`,
		`{"corrida_id":"7"}`, `{"corrida_id":1.5}`, `{"corrida_id":9223372036854775808}`,
		`{"corrida_id":7,"estado":"COMPLETADA"}`, `{"corrida_id":7} {}`,
		`{"corrida_id":7,"x":"` + strings.Repeat("x", 1<<20) + `"}`} {
		if w := operacionHTTPPrueba(mux, "POST", "/api/corridas/salidas", body); w.Code != 400 {
			t.Fatalf("body inválido: status=%d", w.Code)
		}
	}
	for _, id := range []string{"0", "-7", "abc", "9223372036854775808"} {
		if w := operacionHTTPPrueba(mux, "GET", "/api/corridas/"+id, ""); w.Code != 400 {
			t.Fatalf("id=%s status=%d", id, w.Code)
		}
	}
	if store.llamadas != 0 {
		t.Fatal("entrada inválida llegó al Store")
	}
}

func TestOperacionHandlerMapeaErrores(t *testing.T) {
	for _, caso := range []struct {
		err    error
		status int
	}{
		{ErrDatosInvalidos, 400}, {ErrCorridaNoEncontrada, 404},
		{ErrTransicionNoPermitida, 409}, {ErrCorridaSinChofer, 409}, {ErrCorridaSinTramo, 409},
		{ErrUnidadNoDisponible, 409}, {ErrChoferNoDisponible, 409}, {ErrLicenciaNoVigente, 409},
		{ErrAsistenciasOrigenPendientes, 409}, {ErrAsistenciasPendientes, 409},
		{ErrFechasOperacionInconsistentes, 409}, {context.DeadlineExceeded, 504},
		{errors.New("clave técnica secreta"), 500},
	} {
		t.Run(caso.err.Error(), func(t *testing.T) {
			store := &operacionStoreFake{err: fmt.Errorf("envuelto: %w", caso.err)}
			mux := http.NewServeMux()
			NewOperacionHandler(NewOperacionService(store)).RegisterRoutes(mux)
			w := operacionHTTPPrueba(mux, "POST", "/api/corridas/salidas", `{"corrida_id":7}`)
			if w.Code != caso.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if caso.status == 500 && strings.Contains(w.Body.String(), "secreta") {
				t.Fatal("expuso detalles internos")
			}
		})
	}
}
