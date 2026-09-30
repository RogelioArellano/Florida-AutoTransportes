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

type fakeReservaService struct {
	createFn func(
		ctx context.Context,
		input CreateInput,
	) (Reserva, error)

	listFn func(
		ctx context.Context,
		filter ListFilter,
	) ([]Reserva, error)

	getByIDFn func(
		ctx context.Context,
		reservaID int64,
	) (Reserva, error)

	registerPaymentFn func(
		ctx context.Context,
		input RegistrarPagoInput,
	) (Reserva, error)

	confirmFn func(
		ctx context.Context,
		input ConfirmarInput,
	) (Reserva, error)

	cancelFn func(
		ctx context.Context,
		input CancelarInput,
	) (Reserva, error)
}

var _ reservaService = (*fakeReservaService)(nil)

func (f *fakeReservaService) Create(
	ctx context.Context,
	input CreateInput,
) (Reserva, error) {
	if f.createFn == nil {
		return Reserva{}, errors.New(
			"fakeReservaService.Create no fue configurado",
		)
	}

	return f.createFn(ctx, input)
}

func (f *fakeReservaService) List(
	ctx context.Context,
	filter ListFilter,
) ([]Reserva, error) {
	if f.listFn == nil {
		return nil, errors.New(
			"fakeReservaService.List no fue configurado",
		)
	}

	return f.listFn(ctx, filter)
}

func (f *fakeReservaService) GetByID(
	ctx context.Context,
	reservaID int64,
) (Reserva, error) {
	if f.getByIDFn == nil {
		return Reserva{}, errors.New(
			"fakeReservaService.GetByID no fue configurado",
		)
	}

	return f.getByIDFn(
		ctx,
		reservaID,
	)
}

func (f *fakeReservaService) RegisterPayment(
	ctx context.Context,
	input RegistrarPagoInput,
) (Reserva, error) {
	if f.registerPaymentFn == nil {
		return Reserva{}, errors.New(
			"fakeReservaService.RegisterPayment no fue configurado",
		)
	}

	return f.registerPaymentFn(
		ctx,
		input,
	)
}

func (f *fakeReservaService) Confirm(
	ctx context.Context,
	input ConfirmarInput,
) (Reserva, error) {
	if f.confirmFn == nil {
		return Reserva{}, errors.New(
			"fakeReservaService.Confirm no fue configurado",
		)
	}

	return f.confirmFn(
		ctx,
		input,
	)
}

func (f *fakeReservaService) Cancel(
	ctx context.Context,
	input CancelarInput,
) (Reserva, error) {
	if f.cancelFn == nil {
		return Reserva{}, errors.New(
			"fakeReservaService.Cancel no fue configurado",
		)
	}

	return f.cancelFn(
		ctx,
		input,
	)
}

func TestHandlerCreate(t *testing.T) {
	var inputRecibido CreateInput

	service := &fakeReservaService{
		createFn: func(
			ctx context.Context,
			input CreateInput,
		) (Reserva, error) {
			inputRecibido = input

			return Reserva{
				ID:                   1,
				Folio:                "RES-00000001",
				CorridaID:            input.CorridaID,
				PasajeroID:           10,
				PasajeroNombre:       "María González",
				ParadaOrigenID:       input.ParadaOrigenID,
				ParadaDestinoID:      input.ParadaDestinoID,
				CantidadPasajeros:    input.CantidadPasajeros,
				PrecioUnitario:       input.PrecioUnitario,
				Subtotal:             Dinero("900.00"),
				MontoDescuento:       Dinero("45.00"),
				Total:                Dinero("855.00"),
				MontoPagado:          Dinero("300.00"),
				SaldoPendiente:       Dinero("555.00"),
				Estado:               EstadoConfirmada,
				EstadoPago:           EstadoPagoParcial,
				RequiereConfirmacion: false,
				Pagos:                []Pago{},
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"corrida_id": 1,
		"pasajero_id": null,
		"nuevo_pasajero": {
			"nombre_completo": "María González",
			"telefono": "4431234567",
			"correo": null,
			"notas": null
		},
		"corrida_parada_origen_id": 1,
		"corrida_parada_destino_id": 3,
		"cantidad_pasajeros": 2,
		"precio_unitario": "450.00",
		"descuento": {
			"tipo": "PORCENTAJE",
			"valor": "10.00",
			"cantidad_pasajes": 1,
			"descripcion": "Bono por lealtad"
		},
		"pago_inicial": {
			"monto": "300.00",
			"metodo": "TRANSFERENCIA",
			"referencia": "TRANSFERENCIA-12345",
			"notas": null
		},
		"observaciones": "Viajan dos personas"
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas",
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusCreated,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	contentType := recorder.Header().Get(
		"Content-Type",
	)

	if !strings.Contains(
		contentType,
		"application/json",
	) {
		t.Errorf(
			"se esperaba Content-Type JSON, se obtuvo %q",
			contentType,
		)
	}

	if inputRecibido.CorridaID != 1 {
		t.Errorf(
			"CorridaID = %d; se esperaba 1",
			inputRecibido.CorridaID,
		)
	}

	if inputRecibido.NuevoPasajero == nil {
		t.Fatal(
			"no se recibió el pasajero nuevo",
		)
	}

	if inputRecibido.NuevoPasajero.NombreCompleto !=
		"María González" {
		t.Errorf(
			"NombreCompleto = %q",
			inputRecibido.NuevoPasajero.NombreCompleto,
		)
	}

	if inputRecibido.ParadaOrigenID != 1 {
		t.Errorf(
			"ParadaOrigenID = %d; se esperaba 1",
			inputRecibido.ParadaOrigenID,
		)
	}

	if inputRecibido.ParadaDestinoID != 3 {
		t.Errorf(
			"ParadaDestinoID = %d; se esperaba 3",
			inputRecibido.ParadaDestinoID,
		)
	}

	if inputRecibido.CantidadPasajeros != 2 {
		t.Errorf(
			"CantidadPasajeros = %d; se esperaba 2",
			inputRecibido.CantidadPasajeros,
		)
	}

	if inputRecibido.PrecioUnitario !=
		Dinero("450.00") {
		t.Errorf(
			"PrecioUnitario = %q",
			inputRecibido.PrecioUnitario,
		)
	}

	if inputRecibido.Descuento == nil {
		t.Fatal(
			"no se recibió el descuento",
		)
	}

	if inputRecibido.Descuento.Tipo !=
		TipoDescuentoPorcentaje {
		t.Errorf(
			"Tipo descuento = %q",
			inputRecibido.Descuento.Tipo,
		)
	}

	if inputRecibido.PagoInicial == nil {
		t.Fatal(
			"no se recibió el pago inicial",
		)
	}

	if inputRecibido.PagoInicial.Monto !=
		Dinero("300.00") {
		t.Errorf(
			"Pago inicial = %q",
			inputRecibido.PagoInicial.Monto,
		)
	}

	var respuesta struct {
		Data Reserva `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if respuesta.Data.ID != 1 {
		t.Errorf(
			"se esperaba reserva 1, se obtuvo %d",
			respuesta.Data.ID,
		)
	}

	if respuesta.Data.Folio != "RES-00000001" {
		t.Errorf(
			"se obtuvo el folio %q",
			respuesta.Data.Folio,
		)
	}

	if respuesta.Data.Total != Dinero("855.00") {
		t.Errorf(
			"se obtuvo total %q",
			respuesta.Data.Total,
		)
	}

	if respuesta.Data.Estado !=
		EstadoConfirmada {
		t.Errorf(
			"se obtuvo estado %q",
			respuesta.Data.Estado,
		)
	}

	if respuesta.Data.EstadoPago !=
		EstadoPagoParcial {
		t.Errorf(
			"se obtuvo estado de pago %q",
			respuesta.Data.EstadoPago,
		)
	}
}

func TestHandlerListConFiltros(t *testing.T) {
	var filtroRecibido ListFilter

	service := &fakeReservaService{
		listFn: func(
			ctx context.Context,
			filter ListFilter,
		) ([]Reserva, error) {
			filtroRecibido = filter

			return []Reserva{
				{
					ID:             1,
					Folio:          "RES-00000001",
					CorridaID:      1,
					PasajeroID:     10,
					PasajeroNombre: "María González",
					Total:          Dinero("855.00"),
					MontoPagado:    Dinero("300.00"),
					SaldoPendiente: Dinero("555.00"),
					Estado:         EstadoConfirmada,
					EstadoPago:     EstadoPagoParcial,
					Pagos:          []Pago{},
				},
			}, nil
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas"+
			"?corrida_id=1"+
			"&pasajero_id=10"+
			"&estado=%20CONFIRMADA%20"+
			"&estado_pago=PAGO_PARCIAL"+
			"&fecha_servicio_desde=2026-09-26"+
			"&fecha_servicio_hasta=2026-09-30"+
			"&busqueda=%20María%20",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusOK,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if filtroRecibido.CorridaID == nil ||
		*filtroRecibido.CorridaID != 1 {
		t.Error(
			"corrida_id no fue transferido correctamente",
		)
	}

	if filtroRecibido.PasajeroID == nil ||
		*filtroRecibido.PasajeroID != 10 {
		t.Error(
			"pasajero_id no fue transferido correctamente",
		)
	}

	if filtroRecibido.Estado == nil ||
		*filtroRecibido.Estado !=
			EstadoConfirmada {
		t.Error(
			"estado no fue transferido correctamente",
		)
	}

	if filtroRecibido.EstadoPago == nil ||
		*filtroRecibido.EstadoPago !=
			EstadoPagoParcial {
		t.Error(
			"estado_pago no fue transferido correctamente",
		)
	}

	if filtroRecibido.FechaServicioDesde == nil ||
		*filtroRecibido.FechaServicioDesde !=
			"2026-09-26" {
		t.Error(
			"fecha_servicio_desde no fue transferida",
		)
	}

	if filtroRecibido.FechaServicioHasta == nil ||
		*filtroRecibido.FechaServicioHasta !=
			"2026-09-30" {
		t.Error(
			"fecha_servicio_hasta no fue transferida",
		)
	}

	if filtroRecibido.Busqueda == nil ||
		*filtroRecibido.Busqueda != "María" {
		t.Error(
			"busqueda no fue transferida correctamente",
		)
	}

	var respuesta struct {
		Data []Reserva `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if len(respuesta.Data) != 1 {
		t.Fatalf(
			"se esperaba una reserva, se obtuvieron %d",
			len(respuesta.Data),
		)
	}

	if respuesta.Data[0].Folio !=
		"RES-00000001" {
		t.Errorf(
			"se obtuvo el folio %q",
			respuesta.Data[0].Folio,
		)
	}
}

func TestHandlerRegisterPayment(
	t *testing.T,
) {
	var inputRecibido RegistrarPagoInput

	service := &fakeReservaService{
		registerPaymentFn: func(
			ctx context.Context,
			input RegistrarPagoInput,
		) (Reserva, error) {
			inputRecibido = input

			referencia :=
				"LIQUIDACION-RES-00000002"

			return Reserva{
				ID:             2,
				Folio:          "RES-00000002",
				Total:          Dinero("855.00"),
				MontoPagado:    Dinero("855.00"),
				SaldoPendiente: Dinero("0.00"),
				Estado:         EstadoConfirmada,
				EstadoPago:     EstadoPagoPagada,
				Pagos: []Pago{
					{
						ID:        1,
						ReservaID: 2,
						Monto:     Dinero("300.00"),
						Metodo:    MetodoPagoTransferencia,
						Estado:    EstadoMovimientoAplicado,
					},
					{
						ID:         2,
						ReservaID:  2,
						Monto:      Dinero("555.00"),
						Metodo:     MetodoPagoTransferencia,
						Referencia: &referencia,
						Estado:     EstadoMovimientoAplicado,
					},
				},
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"reserva_id": 2,
		"monto": "555.00",
		"metodo": "TRANSFERENCIA",
		"referencia": "LIQUIDACION-RES-00000002",
		"notas": "Pago final de la reserva"
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/pagos",
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	handler.HandlePayments(
		recorder,
		request,
	)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusCreated,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if inputRecibido.ReservaID != 2 {
		t.Errorf(
			"ReservaID = %d; se esperaba 2",
			inputRecibido.ReservaID,
		)
	}

	if inputRecibido.Monto !=
		Dinero("555.00") {
		t.Errorf(
			"Monto = %q; se esperaba 555.00",
			inputRecibido.Monto,
		)
	}

	if inputRecibido.Metodo !=
		MetodoPagoTransferencia {
		t.Errorf(
			"Metodo = %q; se esperaba TRANSFERENCIA",
			inputRecibido.Metodo,
		)
	}

	if inputRecibido.Referencia == nil ||
		*inputRecibido.Referencia !=
			"LIQUIDACION-RES-00000002" {
		t.Error(
			"no se recibió correctamente la referencia",
		)
	}

	var respuesta struct {
		Data Reserva `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if respuesta.Data.ID != 2 {
		t.Errorf(
			"reserva ID = %d; se esperaba 2",
			respuesta.Data.ID,
		)
	}

	if respuesta.Data.MontoPagado !=
		Dinero("855.00") {
		t.Errorf(
			"MontoPagado = %q; se esperaba 855.00",
			respuesta.Data.MontoPagado,
		)
	}

	if respuesta.Data.SaldoPendiente !=
		Dinero("0.00") {
		t.Errorf(
			"SaldoPendiente = %q; se esperaba 0.00",
			respuesta.Data.SaldoPendiente,
		)
	}

	if respuesta.Data.EstadoPago !=
		EstadoPagoPagada {
		t.Errorf(
			"EstadoPago = %q; se esperaba PAGADA",
			respuesta.Data.EstadoPago,
		)
	}

	if len(respuesta.Data.Pagos) != 2 {
		t.Errorf(
			"se esperaban 2 pagos, se obtuvieron %d",
			len(respuesta.Data.Pagos),
		)
	}
}

func TestHandlerRegisterPaymentTraduceErrores(
	t *testing.T,
) {
	pruebas := []struct {
		nombre         string
		errorServicio  error
		statusEsperado int
	}{
		{
			nombre: "datos inválidos",
			errorServicio: fmt.Errorf(
				"%w: monto inválido",
				ErrDatosInvalidos,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre:         "reserva inexistente",
			errorServicio:  ErrReservaNoEncontrada,
			statusEsperado: http.StatusNotFound,
		},
		{
			nombre:         "reserva no acepta pagos",
			errorServicio:  ErrReservaNoAceptaPagos,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "reserva sin saldo",
			errorServicio:  ErrReservaSinSaldo,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre: "pago excede saldo",
			errorServicio: fmt.Errorf(
				"%w: saldo pendiente 100.00",
				ErrPagoExcedeSaldo,
			),
			statusEsperado: http.StatusConflict,
		},
		{
			nombre: "error interno",
			errorServicio: errors.New(
				"PostgreSQL no disponible",
			),
			statusEsperado: http.StatusInternalServerError,
		},
	}

	body := `{
		"reserva_id": 2,
		"monto": "100.00",
		"metodo": "EFECTIVO",
		"referencia": null,
		"notas": null
	}`

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeReservaService{
					registerPaymentFn: func(
						ctx context.Context,
						input RegistrarPagoInput,
					) (Reserva, error) {
						return Reserva{},
							prueba.errorServicio
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/reservas/pagos",
					strings.NewReader(body),
				)

				recorder :=
					httptest.NewRecorder()

				handler.HandlePayments(
					recorder,
					request,
				)

				if recorder.Code !=
					prueba.statusEsperado {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						prueba.statusEsperado,
						recorder.Code,
						recorder.Body.String(),
					)
				}
			},
		)
	}
}

func TestHandlerPaymentsRechazaMetodoNoPermitido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeReservaService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/pagos",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandlePayments(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusMethodNotAllowed {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}

	if allow := recorder.Header().Get(
		"Allow",
	); allow != "POST" {
		t.Errorf(
			"se esperaba Allow POST, se obtuvo %q",
			allow,
		)
	}
}

func TestHandlerGetByID(
	t *testing.T,
) {
	var reservaIDRecibido int64

	service := &fakeReservaService{
		getByIDFn: func(
			ctx context.Context,
			reservaID int64,
		) (Reserva, error) {
			reservaIDRecibido = reservaID

			return Reserva{
				ID:             2,
				Folio:          "RES-00000002",
				PasajeroNombre: "Carlos Ramírez",
				Total:          Dinero("855.00"),
				MontoPagado:    Dinero("855.00"),
				SaldoPendiente: Dinero("0.00"),
				Estado:         EstadoConfirmada,
				EstadoPago:     EstadoPagoPagada,
				Pagos: []Pago{
					{
						ID:        1,
						ReservaID: 2,
						Monto:     Dinero("300.00"),
						Metodo:    MetodoPagoTransferencia,
						Estado:    EstadoMovimientoAplicado,
					},
					{
						ID:        2,
						ReservaID: 2,
						Monto:     Dinero("555.00"),
						Metodo:    MetodoPagoTransferencia,
						Estado:    EstadoMovimientoAplicado,
					},
				},
			}, nil
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/2",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandleDetail(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusOK,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if reservaIDRecibido != 2 {
		t.Errorf(
			"reservaID recibido = %d; se esperaba 2",
			reservaIDRecibido,
		)
	}

	var respuesta struct {
		Data Reserva `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if respuesta.Data.ID != 2 {
		t.Errorf(
			"reserva ID = %d; se esperaba 2",
			respuesta.Data.ID,
		)
	}

	if respuesta.Data.Folio !=
		"RES-00000002" {
		t.Errorf(
			"folio = %q; se esperaba RES-00000002",
			respuesta.Data.Folio,
		)
	}

	if respuesta.Data.EstadoPago !=
		EstadoPagoPagada {
		t.Errorf(
			"EstadoPago = %q; se esperaba PAGADA",
			respuesta.Data.EstadoPago,
		)
	}

	if len(respuesta.Data.Pagos) != 2 {
		t.Errorf(
			"se esperaban 2 pagos, se obtuvieron %d",
			len(respuesta.Data.Pagos),
		)
	}
}

func TestHandlerGetByIDRechazaIDInvalido(
	t *testing.T,
) {
	rutas := []string{
		"/api/reservas/",
		"/api/reservas/abc",
		"/api/reservas/0",
		"/api/reservas/-1",
		"/api/reservas/2/otro",
	}

	for _, ruta := range rutas {
		t.Run(
			ruta,
			func(t *testing.T) {
				service := &fakeReservaService{
					getByIDFn: func(
						ctx context.Context,
						reservaID int64,
					) (Reserva, error) {
						t.Fatal(
							"GetByID no debe ejecutarse con un ID inválido",
						)

						return Reserva{}, nil
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodGet,
					ruta,
					nil,
				)

				recorder :=
					httptest.NewRecorder()

				handler.HandleDetail(
					recorder,
					request,
				)

				if recorder.Code !=
					http.StatusBadRequest {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						http.StatusBadRequest,
						recorder.Code,
						recorder.Body.String(),
					)
				}
			},
		)
	}
}

func TestHandlerGetByIDReservaNoEncontrada(
	t *testing.T,
) {
	service := &fakeReservaService{
		getByIDFn: func(
			ctx context.Context,
			reservaID int64,
		) (Reserva, error) {
			return Reserva{},
				ErrReservaNoEncontrada
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/999",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandleDetail(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusNotFound {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusNotFound,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	var respuesta struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if respuesta.Error == "" {
		t.Error(
			"se esperaba un mensaje de error",
		)
	}
}

func TestHandlerGetByIDRechazaMetodoNoPermitido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeReservaService{},
	)

	request := httptest.NewRequest(
		http.MethodDelete,
		"/api/reservas/2",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandleDetail(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusMethodNotAllowed {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}

	if allow := recorder.Header().Get(
		"Allow",
	); allow != "GET" {
		t.Errorf(
			"se esperaba Allow GET, se obtuvo %q",
			allow,
		)
	}
}

func TestHandlerConfirm(
	t *testing.T,
) {
	var inputRecibido ConfirmarInput

	service := &fakeReservaService{
		confirmFn: func(
			ctx context.Context,
			input ConfirmarInput,
		) (Reserva, error) {
			inputRecibido = input

			return Reserva{
				ID:                   3,
				Folio:                "RES-00000003",
				Total:                Dinero("450.00"),
				MontoPagado:          Dinero("0.00"),
				SaldoPendiente:       Dinero("450.00"),
				Estado:               EstadoConfirmada,
				EstadoPago:           EstadoPagoSinPago,
				RequiereConfirmacion: false,
				Pagos:                []Pago{},
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"reserva_id": 3
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/confirmaciones",
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	handler.HandleConfirmations(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusOK,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if inputRecibido.ReservaID != 3 {
		t.Errorf(
			"ReservaID = %d; se esperaba 3",
			inputRecibido.ReservaID,
		)
	}

	var respuesta struct {
		Data Reserva `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if respuesta.Data.Estado !=
		EstadoConfirmada {
		t.Errorf(
			"Estado = %q; se esperaba CONFIRMADA",
			respuesta.Data.Estado,
		)
	}

	if respuesta.Data.EstadoPago !=
		EstadoPagoSinPago {
		t.Errorf(
			"EstadoPago = %q; se esperaba SIN_PAGO",
			respuesta.Data.EstadoPago,
		)
	}

	if respuesta.Data.RequiereConfirmacion {
		t.Error(
			"la reserva confirmada no debe requerir confirmación",
		)
	}
}

func TestHandlerConfirmTraduceErrores(
	t *testing.T,
) {
	pruebas := []struct {
		nombre         string
		errorServicio  error
		statusEsperado int
	}{
		{
			nombre: "datos inválidos",
			errorServicio: fmt.Errorf(
				"%w: reserva_id inválido",
				ErrDatosInvalidos,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre:         "reserva inexistente",
			errorServicio:  ErrReservaNoEncontrada,
			statusEsperado: http.StatusNotFound,
		},
		{
			nombre:         "estado no confirmable",
			errorServicio:  ErrReservaNoAceptaConfirmacion,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre: "error interno",
			errorServicio: errors.New(
				"PostgreSQL no disponible",
			),
			statusEsperado: http.StatusInternalServerError,
		},
	}

	body := `{
		"reserva_id": 3
	}`

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeReservaService{
					confirmFn: func(
						ctx context.Context,
						input ConfirmarInput,
					) (Reserva, error) {
						return Reserva{},
							prueba.errorServicio
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/reservas/confirmaciones",
					strings.NewReader(body),
				)

				recorder :=
					httptest.NewRecorder()

				handler.HandleConfirmations(
					recorder,
					request,
				)

				if recorder.Code !=
					prueba.statusEsperado {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						prueba.statusEsperado,
						recorder.Code,
						recorder.Body.String(),
					)
				}
			},
		)
	}
}

func TestHandlerConfirmRechazaJSONInvalido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeReservaService{
			confirmFn: func(
				ctx context.Context,
				input ConfirmarInput,
			) (Reserva, error) {
				t.Fatal(
					"Confirm no debe ejecutarse con JSON inválido",
				)

				return Reserva{}, nil
			},
		},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/confirmaciones",
		strings.NewReader(
			`{"reserva_id": "tres"}`,
		),
	)

	recorder := httptest.NewRecorder()

	handler.HandleConfirmations(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusBadRequest {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusBadRequest,
			recorder.Code,
		)
	}
}

func TestHandlerConfirmRechazaMetodoNoPermitido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeReservaService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/confirmaciones",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandleConfirmations(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusMethodNotAllowed {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}

	if allow := recorder.Header().Get(
		"Allow",
	); allow != "POST" {
		t.Errorf(
			"se esperaba Allow POST, se obtuvo %q",
			allow,
		)
	}
}

func TestHandlerListSinFiltros(t *testing.T) {
	service := &fakeReservaService{
		listFn: func(
			ctx context.Context,
			filter ListFilter,
		) ([]Reserva, error) {
			if filter.CorridaID != nil ||
				filter.PasajeroID != nil ||
				filter.Estado != nil ||
				filter.EstadoPago != nil ||
				filter.FechaServicioDesde != nil ||
				filter.FechaServicioHasta != nil ||
				filter.Busqueda != nil {
				t.Error(
					"se esperaba un filtro vacío",
				)
			}

			return []Reserva{}, nil
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusOK,
			recorder.Code,
		)
	}
}

func TestHandlerCreateRechazaJSONInvalido(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		body   string
	}{
		{
			nombre: "JSON incompleto",
			body:   `{"corrida_id":`,
		},
		{
			nombre: "campo desconocido",
			body: `{
				"corrida_id": 1,
				"campo_inexistente": true
			}`,
		},
		{
			nombre: "dos objetos JSON",
			body: `{
				"corrida_id": 1
			}
			{
				"corrida_id": 2
			}`,
		},
		{
			nombre: "corrida con tipo incorrecto",
			body: `{
				"corrida_id": "uno"
			}`,
		},
		{
			nombre: "precio numérico en lugar de texto",
			body: `{
				"corrida_id": 1,
				"precio_unitario": 450.00
			}`,
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeReservaService{
					createFn: func(
						ctx context.Context,
						input CreateInput,
					) (Reserva, error) {
						t.Fatal(
							"el Service no debe ejecutarse con JSON inválido",
						)

						return Reserva{}, nil
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/reservas",
					strings.NewReader(prueba.body),
				)

				recorder := httptest.NewRecorder()

				handler.Handle(
					recorder,
					request,
				)

				if recorder.Code !=
					http.StatusBadRequest {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						http.StatusBadRequest,
						recorder.Code,
						recorder.Body.String(),
					)
				}

				var respuesta struct {
					Error string `json:"error"`
				}

				if err := json.NewDecoder(
					recorder.Body,
				).Decode(&respuesta); err != nil {
					t.Fatalf(
						"no se pudo decodificar el error: %v",
						err,
					)
				}

				if respuesta.Error == "" {
					t.Error(
						"se esperaba un mensaje de error",
					)
				}
			},
		)
	}
}

func TestHandlerCancel(
	t *testing.T,
) {
	var inputRecibido CancelarInput

	limite := time.Date(
		2026,
		time.September,
		30,
		3,
		0,
		0,
		0,
		time.FixedZone("CST", -6*60*60),
	)

	service := &fakeReservaService{
		cancelFn: func(
			ctx context.Context,
			input CancelarInput,
		) (Reserva, error) {
			inputRecibido = input

			motivo :=
				"El pasajero canceló su viaje"

			return Reserva{
				ID:                      4,
				Folio:                   "RES-00000004",
				Estado:                  EstadoCancelada,
				EstadoPago:              EstadoPagoParcial,
				CanceladaEn:             &limite,
				MotivoCancelacion:       &motivo,
				CancelacionReembolsable: true,
				MontoReembolsable:       Dinero("300.00"),
				LimiteReembolsoEn:       &limite,
				Pagos:                   []Pago{},
			}, nil
		},
	}

	handler := NewHandler(service)

	body := `{
		"reserva_id": 4,
		"motivo": "El pasajero canceló su viaje"
	}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/cancelaciones",
		strings.NewReader(body),
	)

	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	recorder := httptest.NewRecorder()

	handler.HandleCancellations(
		recorder,
		request,
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d. Respuesta: %s",
			http.StatusOK,
			recorder.Code,
			recorder.Body.String(),
		)
	}

	if inputRecibido.ReservaID != 4 {
		t.Errorf(
			"ReservaID = %d; se esperaba 4",
			inputRecibido.ReservaID,
		)
	}

	if inputRecibido.Motivo !=
		"El pasajero canceló su viaje" {
		t.Errorf(
			"Motivo = %q",
			inputRecibido.Motivo,
		)
	}

	var respuesta struct {
		Data Reserva `json:"data"`
	}

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&respuesta); err != nil {
		t.Fatalf(
			"no se pudo decodificar la respuesta: %v",
			err,
		)
	}

	if respuesta.Data.Estado !=
		EstadoCancelada {
		t.Errorf(
			"Estado = %q; se esperaba CANCELADA",
			respuesta.Data.Estado,
		)
	}

	if !respuesta.Data.CancelacionReembolsable {
		t.Error(
			"se esperaba una cancelación reembolsable",
		)
	}

	if respuesta.Data.MontoReembolsable !=
		Dinero("300.00") {
		t.Errorf(
			"MontoReembolsable = %q; se esperaba 300.00",
			respuesta.Data.MontoReembolsable,
		)
	}
}

func TestHandlerCancelTraduceErrores(
	t *testing.T,
) {
	pruebas := []struct {
		nombre         string
		errorServicio  error
		statusEsperado int
	}{
		{
			nombre: "datos inválidos",
			errorServicio: fmt.Errorf(
				"%w: motivo obligatorio",
				ErrDatosInvalidos,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre:         "reserva inexistente",
			errorServicio:  ErrReservaNoEncontrada,
			statusEsperado: http.StatusNotFound,
		},
		{
			nombre:         "estado no cancelable",
			errorServicio:  ErrReservaNoAceptaCancelacion,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "salida ya iniciada",
			errorServicio:  ErrSalidaYaIniciada,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre: "error interno",
			errorServicio: errors.New(
				"PostgreSQL no disponible",
			),
			statusEsperado: http.StatusInternalServerError,
		},
	}

	body := `{
		"reserva_id": 4,
		"motivo": "Cancelación solicitada"
	}`

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeReservaService{
					cancelFn: func(
						ctx context.Context,
						input CancelarInput,
					) (Reserva, error) {
						return Reserva{},
							prueba.errorServicio
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/reservas/cancelaciones",
					strings.NewReader(body),
				)

				recorder :=
					httptest.NewRecorder()

				handler.HandleCancellations(
					recorder,
					request,
				)

				if recorder.Code !=
					prueba.statusEsperado {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						prueba.statusEsperado,
						recorder.Code,
						recorder.Body.String(),
					)
				}
			},
		)
	}
}

func TestHandlerCancelRechazaJSONInvalido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeReservaService{
			cancelFn: func(
				ctx context.Context,
				input CancelarInput,
			) (Reserva, error) {
				t.Fatal(
					"Cancel no debe ejecutarse con JSON inválido",
				)

				return Reserva{}, nil
			},
		},
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/reservas/cancelaciones",
		strings.NewReader(
			`{"reserva_id": "cuatro"}`,
		),
	)

	recorder := httptest.NewRecorder()

	handler.HandleCancellations(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusBadRequest {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusBadRequest,
			recorder.Code,
		)
	}
}

func TestHandlerCancelRechazaMetodoNoPermitido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeReservaService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas/cancelaciones",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HandleCancellations(
		recorder,
		request,
	)

	if recorder.Code !=
		http.StatusMethodNotAllowed {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}

	if allow := recorder.Header().Get(
		"Allow",
	); allow != "POST" {
		t.Errorf(
			"se esperaba Allow POST, se obtuvo %q",
			allow,
		)
	}
}

func TestHandlerListRechazaIDInvalido(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		url    string
	}{
		{
			nombre: "corrida no numérica",
			url: "/api/reservas" +
				"?corrida_id=abc",
		},
		{
			nombre: "pasajero decimal",
			url: "/api/reservas" +
				"?pasajero_id=1.5",
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeReservaService{
					listFn: func(
						ctx context.Context,
						filter ListFilter,
					) ([]Reserva, error) {
						t.Fatal(
							"el Service no debe ejecutarse con un ID inválido",
						)

						return nil, nil
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodGet,
					prueba.url,
					nil,
				)

				recorder := httptest.NewRecorder()

				handler.Handle(
					recorder,
					request,
				)

				if recorder.Code !=
					http.StatusBadRequest {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						http.StatusBadRequest,
						recorder.Code,
						recorder.Body.String(),
					)
				}
			},
		)
	}
}

func TestHandlerCreateTraduceErroresServicio(
	t *testing.T,
) {
	pruebas := []struct {
		nombre         string
		errorServicio  error
		statusEsperado int
	}{
		{
			nombre: "datos inválidos",
			errorServicio: fmt.Errorf(
				"%w: precio inválido",
				ErrDatosInvalidos,
			),
			statusEsperado: http.StatusBadRequest,
		},
		{
			nombre:         "corrida no disponible",
			errorServicio:  ErrCorridaNoDisponible,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "pasajero no disponible",
			errorServicio:  ErrPasajeroNoDisponible,
			statusEsperado: http.StatusNotFound,
		},
		{
			nombre:         "paradas inválidas",
			errorServicio:  ErrParadasInvalidas,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "cupo insuficiente",
			errorServicio:  ErrCupoInsuficiente,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "anticipo requerido",
			errorServicio:  ErrAnticipoRequerido,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "anticipo insuficiente",
			errorServicio:  ErrAnticipoInsuficiente,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "pago mayor al total",
			errorServicio:  ErrPagoExcedeTotal,
			statusEsperado: http.StatusConflict,
		},
		{
			nombre:         "tiempo agotado",
			errorServicio:  context.DeadlineExceeded,
			statusEsperado: http.StatusGatewayTimeout,
		},
		{
			nombre: "error interno",
			errorServicio: errors.New(
				"PostgreSQL no disponible",
			),
			statusEsperado: http.StatusInternalServerError,
		},
	}

	body := `{
		"corrida_id": 1,
		"pasajero_id": 10,
		"nuevo_pasajero": null,
		"corrida_parada_origen_id": 1,
		"corrida_parada_destino_id": 3,
		"cantidad_pasajeros": 1,
		"precio_unitario": "450.00",
		"descuento": null,
		"pago_inicial": null,
		"observaciones": null
	}`

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				service := &fakeReservaService{
					createFn: func(
						ctx context.Context,
						input CreateInput,
					) (Reserva, error) {
						return Reserva{},
							prueba.errorServicio
					},
				}

				handler := NewHandler(service)

				request := httptest.NewRequest(
					http.MethodPost,
					"/api/reservas",
					strings.NewReader(body),
				)

				recorder := httptest.NewRecorder()

				handler.Handle(
					recorder,
					request,
				)

				if recorder.Code !=
					prueba.statusEsperado {
					t.Fatalf(
						"se esperaba status %d, se obtuvo %d. Respuesta: %s",
						prueba.statusEsperado,
						recorder.Code,
						recorder.Body.String(),
					)
				}

				var respuesta struct {
					Error string `json:"error"`
				}

				if err := json.NewDecoder(
					recorder.Body,
				).Decode(&respuesta); err != nil {
					t.Fatalf(
						"no se pudo decodificar el error: %v",
						err,
					)
				}

				if respuesta.Error == "" {
					t.Error(
						"se esperaba un mensaje de error",
					)
				}
			},
		)
	}
}

func TestHandlerListTraduceErrorServicio(
	t *testing.T,
) {
	service := &fakeReservaService{
		listFn: func(
			ctx context.Context,
			filter ListFilter,
		) ([]Reserva, error) {
			return nil, errors.New(
				"error consultando PostgreSQL",
			)
		},
	}

	handler := NewHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/reservas",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code !=
		http.StatusInternalServerError {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusInternalServerError,
			recorder.Code,
		)
	}
}

func TestHandlerRechazaMetodoNoPermitido(
	t *testing.T,
) {
	handler := NewHandler(
		&fakeReservaService{},
	)

	request := httptest.NewRequest(
		http.MethodPut,
		"/api/reservas",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request)

	if recorder.Code !=
		http.StatusMethodNotAllowed {
		t.Fatalf(
			"se esperaba status %d, se obtuvo %d",
			http.StatusMethodNotAllowed,
			recorder.Code,
		)
	}

	allow := recorder.Header().Get("Allow")

	if allow != "GET, POST" {
		t.Errorf(
			"se esperaba Allow GET, POST, se obtuvo %q",
			allow,
		)
	}
}
