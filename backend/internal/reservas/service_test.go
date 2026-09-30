package reservas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeStore struct {
	createParams CreateParams
	createResult Reserva
	createErr    error
	createCalls  int

	listFilter ListFilter
	listResult []Reserva
	listErr    error
	listCalls  int

	registerPaymentInput  RegistrarPagoInput
	registerPaymentResult Reserva
	registerPaymentErr    error
	registerPaymentCalls  int

	getByIDInput  int64
	getByIDResult Reserva
	getByIDErr    error
	getByIDCalls  int

	confirmInput  ConfirmarInput
	confirmResult Reserva
	confirmErr    error
	confirmCalls  int

	cancelParams CancelParams
	cancelResult Reserva
	cancelErr    error
	cancelCalls  int
}

var _ Store = (*fakeStore)(nil)

func (f *fakeStore) Create(
	ctx context.Context,
	params CreateParams,
) (Reserva, error) {
	f.createCalls++
	f.createParams = params

	if f.createErr != nil {
		return Reserva{}, f.createErr
	}

	return f.createResult, nil
}

func (f *fakeStore) List(
	ctx context.Context,
	filter ListFilter,
) ([]Reserva, error) {
	f.listCalls++
	f.listFilter = filter

	if f.listErr != nil {
		return nil, f.listErr
	}

	return f.listResult, nil
}

func (f *fakeStore) GetByID(
	ctx context.Context,
	reservaID int64,
) (Reserva, error) {
	f.getByIDCalls++
	f.getByIDInput = reservaID

	if f.getByIDErr != nil {
		return Reserva{}, f.getByIDErr
	}

	return f.getByIDResult, nil
}

func (f *fakeStore) RegisterPayment(
	ctx context.Context,
	input RegistrarPagoInput,
) (Reserva, error) {
	f.registerPaymentCalls++
	f.registerPaymentInput = input

	if f.registerPaymentErr != nil {
		return Reserva{},
			f.registerPaymentErr
	}

	return f.registerPaymentResult, nil
}

func (f *fakeStore) Confirm(
	ctx context.Context,
	input ConfirmarInput,
) (Reserva, error) {
	f.confirmCalls++
	f.confirmInput = input

	if f.confirmErr != nil {
		return Reserva{}, f.confirmErr
	}

	return f.confirmResult, nil
}

func (f *fakeStore) Cancel(
	ctx context.Context,
	params CancelParams,
) (Reserva, error) {
	f.cancelCalls++
	f.cancelParams = params

	if f.cancelErr != nil {
		return Reserva{}, f.cancelErr
	}

	return f.cancelResult, nil
}

func TestServiceCreateCalculaDescuentoYAnticipo(
	t *testing.T,
) {
	correo := "  MARIA@EJEMPLO.COM  "
	referencia := "  TRANSFERENCIA-12345  "
	observaciones := "  Viajan dos personas  "

	input := CreateInput{
		CorridaID: 1,
		NuevoPasajero: &NuevoPasajeroInput{
			NombreCompleto: "  María González  ",
			Telefono:       "+52 443 123 4567",
			Correo:         &correo,
		},
		ParadaOrigenID:    1,
		ParadaDestinoID:   3,
		CantidadPasajeros: 2,
		PrecioUnitario:    Dinero("450"),
		Descuento: &DescuentoInput{
			Tipo:            TipoDescuento(" porcentaje "),
			Valor:           "10",
			CantidadPasajes: 1,
			Descripcion:     "  Bono por lealtad  ",
		},
		PagoInicial: &PagoInput{
			Monto:      Dinero("300"),
			Metodo:     MetodoPago(" transferencia "),
			Referencia: &referencia,
		},
		Observaciones: &observaciones,
	}

	store := &fakeStore{
		createResult: Reserva{
			ID:             1,
			Folio:          "RES-00000001",
			Subtotal:       Dinero("900.00"),
			MontoDescuento: Dinero("45.00"),
			Total:          Dinero("855.00"),
			MontoPagado:    Dinero("300.00"),
			SaldoPendiente: Dinero("555.00"),
			Estado:         EstadoConfirmada,
			EstadoPago:     EstadoPagoParcial,
		},
	}

	service := NewService(store)

	resultado, err := service.Create(
		context.Background(),
		input,
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

	params := store.createParams

	if params.Subtotal != Dinero("900.00") {
		t.Errorf(
			"Subtotal = %q; se esperaba 900.00",
			params.Subtotal,
		)
	}

	if params.MontoDescuento != Dinero("45.00") {
		t.Errorf(
			"MontoDescuento = %q; se esperaba 45.00",
			params.MontoDescuento,
		)
	}

	if params.Total != Dinero("855.00") {
		t.Errorf(
			"Total = %q; se esperaba 855.00",
			params.Total,
		)
	}

	if params.Estado != EstadoConfirmada {
		t.Errorf(
			"Estado = %q; se esperaba CONFIRMADA",
			params.Estado,
		)
	}

	if params.RequiereConfirmacion {
		t.Error(
			"una reserva con anticipo no debe requerir confirmación",
		)
	}

	if params.Input.PrecioUnitario != Dinero("450.00") {
		t.Errorf(
			"PrecioUnitario = %q; se esperaba 450.00",
			params.Input.PrecioUnitario,
		)
	}

	if params.Input.NuevoPasajero == nil {
		t.Fatal(
			"se esperaba un pasajero nuevo",
		)
	}

	pasajero := params.Input.NuevoPasajero

	if pasajero.NombreCompleto != "María González" {
		t.Errorf(
			"NombreCompleto = %q",
			pasajero.NombreCompleto,
		)
	}

	if pasajero.Telefono != "+524431234567" {
		t.Errorf(
			"Telefono = %q; se esperaba +524431234567",
			pasajero.Telefono,
		)
	}

	if pasajero.Correo == nil ||
		*pasajero.Correo != "maria@ejemplo.com" {
		t.Error(
			"el correo no fue normalizado correctamente",
		)
	}

	if params.Input.Descuento == nil {
		t.Fatal(
			"se esperaba un descuento",
		)
	}

	if params.Input.Descuento.Tipo !=
		TipoDescuentoPorcentaje {
		t.Errorf(
			"Tipo de descuento = %q",
			params.Input.Descuento.Tipo,
		)
	}

	if params.Input.Descuento.Valor != "10.00" {
		t.Errorf(
			"Valor del descuento = %q; se esperaba 10.00",
			params.Input.Descuento.Valor,
		)
	}

	if params.Input.Descuento.Descripcion !=
		"Bono por lealtad" {
		t.Errorf(
			"Descripción del descuento = %q",
			params.Input.Descuento.Descripcion,
		)
	}

	if params.Input.PagoInicial == nil {
		t.Fatal(
			"se esperaba un pago inicial",
		)
	}

	if params.Input.PagoInicial.Monto !=
		Dinero("300.00") {
		t.Errorf(
			"Monto del pago = %q",
			params.Input.PagoInicial.Monto,
		)
	}

	if params.Input.PagoInicial.Metodo !=
		MetodoPagoTransferencia {
		t.Errorf(
			"Método de pago = %q",
			params.Input.PagoInicial.Metodo,
		)
	}

	if params.Input.PagoInicial.Referencia == nil ||
		*params.Input.PagoInicial.Referencia !=
			"TRANSFERENCIA-12345" {
		t.Error(
			"la referencia no fue normalizada correctamente",
		)
	}

	if params.Input.Observaciones == nil ||
		*params.Input.Observaciones !=
			"Viajan dos personas" {
		t.Error(
			"las observaciones no fueron normalizadas",
		)
	}

	if resultado.ID != 1 {
		t.Errorf(
			"resultado.ID = %d; se esperaba 1",
			resultado.ID,
		)
	}

	// También comprobamos que copiarInput evitó modificar
	// accidentalmente las estructuras originales.
	if input.NuevoPasajero.Telefono !=
		"+52 443 123 4567" {
		t.Error(
			"Create() modificó el teléfono del input original",
		)
	}
}

func TestServiceCreateUnPasajeSinPagoQuedaApartado(
	t *testing.T,
) {
	store := &fakeStore{
		createResult: Reserva{
			ID:     1,
			Folio:  "RES-00000001",
			Estado: EstadoApartada,
		},
	}

	service := NewService(store)

	resultado, err := service.Create(
		context.Background(),
		nuevaReservaValida(),
	)
	if err != nil {
		t.Fatalf(
			"Create() devolvió un error inesperado: %v",
			err,
		)
	}

	params := store.createParams

	if params.Subtotal != Dinero("450.00") {
		t.Errorf(
			"Subtotal = %q; se esperaba 450.00",
			params.Subtotal,
		)
	}

	if params.MontoDescuento != Dinero("0.00") {
		t.Errorf(
			"MontoDescuento = %q; se esperaba 0.00",
			params.MontoDescuento,
		)
	}

	if params.Total != Dinero("450.00") {
		t.Errorf(
			"Total = %q; se esperaba 450.00",
			params.Total,
		)
	}

	if params.Estado != EstadoApartada {
		t.Errorf(
			"Estado = %q; se esperaba APARTADA",
			params.Estado,
		)
	}

	if !params.RequiereConfirmacion {
		t.Error(
			"una reserva sin pago debe requerir confirmación",
		)
	}

	if resultado.Estado != EstadoApartada {
		t.Errorf(
			"resultado.Estado = %q",
			resultado.Estado,
		)
	}
}

func TestServiceCreateCalculaDescuentoFijo(
	t *testing.T,
) {
	input := nuevaReservaValida()

	input.Descuento = &DescuentoInput{
		Tipo:            TipoDescuentoMontoFijo,
		Valor:           "100",
		CantidadPasajes: 1,
		Descripcion:     "Promoción especial",
	}

	store := &fakeStore{}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatalf(
			"Create() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.createParams.MontoDescuento !=
		Dinero("100.00") {
		t.Errorf(
			"MontoDescuento = %q; se esperaba 100.00",
			store.createParams.MontoDescuento,
		)
	}

	if store.createParams.Total !=
		Dinero("350.00") {
		t.Errorf(
			"Total = %q; se esperaba 350.00",
			store.createParams.Total,
		)
	}
}

func TestServiceCreateRedondeaDescuentoPorcentaje(
	t *testing.T,
) {
	input := nuevaReservaValida()
	input.PrecioUnitario = Dinero("100.05")

	input.Descuento = &DescuentoInput{
		Tipo:            TipoDescuentoPorcentaje,
		Valor:           "10",
		CantidadPasajes: 1,
		Descripcion:     "Descuento de prueba",
	}

	store := &fakeStore{}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatalf(
			"Create() devolvió un error inesperado: %v",
			err,
		)
	}

	// 10% de 100.05 es 10.005.
	// Debe redondearse a 10.01.
	if store.createParams.MontoDescuento !=
		Dinero("10.01") {
		t.Errorf(
			"MontoDescuento = %q; se esperaba 10.01",
			store.createParams.MontoDescuento,
		)
	}

	if store.createParams.Total !=
		Dinero("90.04") {
		t.Errorf(
			"Total = %q; se esperaba 90.04",
			store.createParams.Total,
		)
	}
}

func TestServiceCreateDescuentoTotalConfirmaSinPago(
	t *testing.T,
) {
	input := nuevaReservaValida()
	input.CantidadPasajeros = 2

	input.Descuento = &DescuentoInput{
		Tipo:            TipoDescuentoPorcentaje,
		Valor:           "100",
		CantidadPasajes: 2,
		Descripcion:     "Viaje sin costo",
	}

	store := &fakeStore{}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatalf(
			"Create() devolvió un error inesperado: %v",
			err,
		)
	}

	params := store.createParams

	if params.Subtotal != Dinero("900.00") {
		t.Errorf(
			"Subtotal = %q; se esperaba 900.00",
			params.Subtotal,
		)
	}

	if params.MontoDescuento != Dinero("900.00") {
		t.Errorf(
			"MontoDescuento = %q; se esperaba 900.00",
			params.MontoDescuento,
		)
	}

	if params.Total != Dinero("0.00") {
		t.Errorf(
			"Total = %q; se esperaba 0.00",
			params.Total,
		)
	}

	if params.Estado != EstadoConfirmada {
		t.Errorf(
			"Estado = %q; se esperaba CONFIRMADA",
			params.Estado,
		)
	}

	if params.RequiereConfirmacion {
		t.Error(
			"una reserva sin saldo no debe requerir confirmación",
		)
	}
}

func TestServiceCreateExigeAnticipoParaVariosPasajes(
	t *testing.T,
) {
	input := nuevaReservaValida()
	input.CantidadPasajeros = 2

	store := &fakeStore{}
	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		input,
	)

	if !errors.Is(err, ErrAnticipoRequerido) {
		t.Fatalf(
			"se esperaba ErrAnticipoRequerido, se obtuvo %v",
			err,
		)
	}

	if store.createCalls != 0 {
		t.Error(
			"Store.Create() no debe ejecutarse",
		)
	}
}

func TestServiceCreateRechazaAnticipoMenorAl30(
	t *testing.T,
) {
	input := nuevaReservaValida()
	input.CantidadPasajeros = 2

	// Total: 900.00
	// Anticipo mínimo: 270.00
	input.PagoInicial = &PagoInput{
		Monto:  Dinero("269.99"),
		Metodo: MetodoPagoEfectivo,
	}

	store := &fakeStore{}
	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		input,
	)

	if !errors.Is(err, ErrAnticipoInsuficiente) {
		t.Fatalf(
			"se esperaba ErrAnticipoInsuficiente, se obtuvo %v",
			err,
		)
	}

	if store.createCalls != 0 {
		t.Error(
			"Store.Create() no debe ejecutarse",
		)
	}
}

func TestServiceCreateAceptaAnticipoExactoDel30(
	t *testing.T,
) {
	input := nuevaReservaValida()
	input.CantidadPasajeros = 2

	input.PagoInicial = &PagoInput{
		Monto:  Dinero("270.00"),
		Metodo: MetodoPagoEfectivo,
	}

	store := &fakeStore{}
	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		input,
	)
	if err != nil {
		t.Fatalf(
			"Create() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.createParams.Estado !=
		EstadoConfirmada {
		t.Errorf(
			"Estado = %q; se esperaba CONFIRMADA",
			store.createParams.Estado,
		)
	}

	if store.createParams.RequiereConfirmacion {
		t.Error(
			"la reserva con anticipo no debe requerir confirmación",
		)
	}
}

func TestServiceCreateValidaciones(
	t *testing.T,
) {
	idPasajero := int64(1)

	pruebas := []struct {
		nombre        string
		modificar     func(*CreateInput)
		errorEsperado error
	}{
		{
			nombre: "corrida inválida",
			modificar: func(input *CreateInput) {
				input.CorridaID = 0
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "sin pasajero",
			modificar: func(input *CreateInput) {
				input.NuevoPasajero = nil
				input.PasajeroID = nil
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "dos tipos de pasajero",
			modificar: func(input *CreateInput) {
				input.PasajeroID = &idPasajero
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "nombre vacío",
			modificar: func(input *CreateInput) {
				input.NuevoPasajero.NombreCompleto = " "
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "teléfono inválido",
			modificar: func(input *CreateInput) {
				input.NuevoPasajero.Telefono = "443-ABC"
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "correo inválido",
			modificar: func(input *CreateInput) {
				correo := "correo-invalido"
				input.NuevoPasajero.Correo = &correo
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "origen inválido",
			modificar: func(input *CreateInput) {
				input.ParadaOrigenID = 0
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "destino inválido",
			modificar: func(input *CreateInput) {
				input.ParadaDestinoID = 0
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "origen igual a destino",
			modificar: func(input *CreateInput) {
				input.ParadaDestinoID =
					input.ParadaOrigenID
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "cantidad en cero",
			modificar: func(input *CreateInput) {
				input.CantidadPasajeros = 0
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "precio vacío",
			modificar: func(input *CreateInput) {
				input.PrecioUnitario = ""
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "precio negativo",
			modificar: func(input *CreateInput) {
				input.PrecioUnitario = "-450.00"
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "precio con tres decimales",
			modificar: func(input *CreateInput) {
				input.PrecioUnitario = "450.001"
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "descuento desconocido",
			modificar: func(input *CreateInput) {
				input.Descuento = &DescuentoInput{
					Tipo:            "DESCONOCIDO",
					Valor:           "10.00",
					CantidadPasajes: 1,
					Descripcion:     "Prueba",
				}
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "descuento para demasiados pasajes",
			modificar: func(input *CreateInput) {
				input.Descuento = &DescuentoInput{
					Tipo:            TipoDescuentoPorcentaje,
					Valor:           "10.00",
					CantidadPasajes: 2,
					Descripcion:     "Prueba",
				}
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "porcentaje mayor a cien",
			modificar: func(input *CreateInput) {
				input.Descuento = &DescuentoInput{
					Tipo:            TipoDescuentoPorcentaje,
					Valor:           "100.01",
					CantidadPasajes: 1,
					Descripcion:     "Prueba",
				}
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "descuento fijo mayor al pasaje",
			modificar: func(input *CreateInput) {
				input.Descuento = &DescuentoInput{
					Tipo:            TipoDescuentoMontoFijo,
					Valor:           "451.00",
					CantidadPasajes: 1,
					Descripcion:     "Prueba",
				}
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "método de pago desconocido",
			modificar: func(input *CreateInput) {
				input.PagoInicial = &PagoInput{
					Monto:  "100.00",
					Metodo: "CHEQUE",
				}
			},
			errorEsperado: ErrDatosInvalidos,
		},
		{
			nombre: "pago superior al total",
			modificar: func(input *CreateInput) {
				input.PagoInicial = &PagoInput{
					Monto:  "500.00",
					Metodo: MetodoPagoEfectivo,
				}
			},
			errorEsperado: ErrPagoExcedeTotal,
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				input := nuevaReservaValida()
				prueba.modificar(&input)

				store := &fakeStore{}
				service := NewService(store)

				_, err := service.Create(
					context.Background(),
					input,
				)

				if !errors.Is(
					err,
					prueba.errorEsperado,
				) {
					t.Fatalf(
						"se esperaba %v, se obtuvo %v",
						prueba.errorEsperado,
						err,
					)
				}

				if store.createCalls != 0 {
					t.Error(
						"Store.Create() no debe ejecutarse con datos inválidos",
					)
				}
			},
		)
	}
}

func TestServiceCreatePropagaErrorStore(
	t *testing.T,
) {
	errorStore := errors.New(
		"error de PostgreSQL",
	)

	store := &fakeStore{
		createErr: errorStore,
	}

	service := NewService(store)

	_, err := service.Create(
		context.Background(),
		nuevaReservaValida(),
	)

	if !errors.Is(err, errorStore) {
		t.Fatalf(
			"se esperaba el error del Store, se obtuvo %v",
			err,
		)
	}
}

func TestServiceListNormalizaFiltros(
	t *testing.T,
) {
	corridaID := int64(1)
	pasajeroID := int64(2)
	estado := Estado(" confirmada ")
	estadoPago := EstadoPago(" pago_parcial ")
	fechaDesde := " 2026-09-26 "
	fechaHasta := " 2026-09-30 "
	busqueda := "  María  "

	store := &fakeStore{
		listResult: []Reserva{
			{
				ID:     1,
				Folio:  "RES-00000001",
				Estado: EstadoConfirmada,
			},
		},
	}

	service := NewService(store)

	resultado, err := service.List(
		context.Background(),
		ListFilter{
			CorridaID:          &corridaID,
			PasajeroID:         &pasajeroID,
			Estado:             &estado,
			EstadoPago:         &estadoPago,
			FechaServicioDesde: &fechaDesde,
			FechaServicioHasta: &fechaHasta,
			Busqueda:           &busqueda,
		},
	)
	if err != nil {
		t.Fatalf(
			"List() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.listCalls != 1 {
		t.Fatalf(
			"Store.List() fue llamado %d veces; se esperaba 1",
			store.listCalls,
		)
	}

	filter := store.listFilter

	if filter.Estado == nil ||
		*filter.Estado != EstadoConfirmada {
		t.Error(
			"el estado no fue normalizado",
		)
	}

	if filter.EstadoPago == nil ||
		*filter.EstadoPago != EstadoPagoParcial {
		t.Error(
			"el estado de pago no fue normalizado",
		)
	}

	if filter.FechaServicioDesde == nil ||
		*filter.FechaServicioDesde != "2026-09-26" {
		t.Error(
			"fecha_servicio_desde no fue normalizada",
		)
	}

	if filter.FechaServicioHasta == nil ||
		*filter.FechaServicioHasta != "2026-09-30" {
		t.Error(
			"fecha_servicio_hasta no fue normalizada",
		)
	}

	if filter.Busqueda == nil ||
		*filter.Busqueda != "María" {
		t.Error(
			"la búsqueda no fue normalizada",
		)
	}

	if len(resultado) != 1 {
		t.Fatalf(
			"se esperaba una reserva, se obtuvieron %d",
			len(resultado),
		)
	}
}

func TestServiceListValidaciones(
	t *testing.T,
) {
	pruebas := []struct {
		nombre    string
		construir func() ListFilter
	}{
		{
			nombre: "corrida inválida",
			construir: func() ListFilter {
				id := int64(0)
				return ListFilter{
					CorridaID: &id,
				}
			},
		},
		{
			nombre: "pasajero inválido",
			construir: func() ListFilter {
				id := int64(-1)
				return ListFilter{
					PasajeroID: &id,
				}
			},
		},
		{
			nombre: "estado inválido",
			construir: func() ListFilter {
				estado := Estado("DESCONOCIDO")
				return ListFilter{
					Estado: &estado,
				}
			},
		},
		{
			nombre: "estado de pago inválido",
			construir: func() ListFilter {
				estado := EstadoPago("DESCONOCIDO")
				return ListFilter{
					EstadoPago: &estado,
				}
			},
		},
		{
			nombre: "fecha inicial inválida",
			construir: func() ListFilter {
				fecha := "2026-02-30"
				return ListFilter{
					FechaServicioDesde: &fecha,
				}
			},
		},
		{
			nombre: "fecha final anterior",
			construir: func() ListFilter {
				desde := "2026-09-30"
				hasta := "2026-09-26"

				return ListFilter{
					FechaServicioDesde: &desde,
					FechaServicioHasta: &hasta,
				}
			},
		},
		{
			nombre: "búsqueda demasiado larga",
			construir: func() ListFilter {
				busqueda := strings.Repeat(
					"a",
					151,
				)

				return ListFilter{
					Busqueda: &busqueda,
				}
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				store := &fakeStore{}
				service := NewService(store)

				_, err := service.List(
					context.Background(),
					prueba.construir(),
				)

				if !errors.Is(
					err,
					ErrDatosInvalidos,
				) {
					t.Fatalf(
						"se esperaba ErrDatosInvalidos, se obtuvo %v",
						err,
					)
				}

				if store.listCalls != 0 {
					t.Error(
						"Store.List() no debe ejecutarse con filtros inválidos",
					)
				}
			},
		)
	}
}

func TestValidarYNormalizarPago(t *testing.T) {
	referencia := "  TRANSFERENCIA-ABONO-001  "
	notas := "  Liquidación de la reserva  "

	pago := PagoInput{
		Monto:      Dinero("555"),
		Metodo:     MetodoPago(" transferencia "),
		Referencia: &referencia,
		Notas:      &notas,
	}

	montoCentavos, err :=
		validarYNormalizarPago(&pago)
	if err != nil {
		t.Fatalf(
			"validarYNormalizarPago() devolvió un error inesperado: %v",
			err,
		)
	}

	if montoCentavos != 55_500 {
		t.Errorf(
			"montoCentavos = %d; se esperaba 55500",
			montoCentavos,
		)
	}

	if pago.Monto != Dinero("555.00") {
		t.Errorf(
			"pago.Monto = %q; se esperaba 555.00",
			pago.Monto,
		)
	}

	if pago.Metodo != MetodoPagoTransferencia {
		t.Errorf(
			"pago.Metodo = %q; se esperaba TRANSFERENCIA",
			pago.Metodo,
		)
	}

	if pago.Referencia == nil ||
		*pago.Referencia !=
			"TRANSFERENCIA-ABONO-001" {
		t.Error(
			"la referencia no fue normalizada correctamente",
		)
	}

	if pago.Notas == nil ||
		*pago.Notas !=
			"Liquidación de la reserva" {
		t.Error(
			"las notas no fueron normalizadas correctamente",
		)
	}
}

func TestValidarYNormalizarPagoRechazaDatosInvalidos(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		pago   *PagoInput
	}{
		{
			nombre: "pago ausente",
			pago:   nil,
		},
		{
			nombre: "monto vacío",
			pago: &PagoInput{
				Monto:  "",
				Metodo: MetodoPagoEfectivo,
			},
		},
		{
			nombre: "monto en cero",
			pago: &PagoInput{
				Monto:  "0.00",
				Metodo: MetodoPagoEfectivo,
			},
		},
		{
			nombre: "monto negativo",
			pago: &PagoInput{
				Monto:  "-100.00",
				Metodo: MetodoPagoEfectivo,
			},
		},
		{
			nombre: "más de dos decimales",
			pago: &PagoInput{
				Monto:  "100.001",
				Metodo: MetodoPagoEfectivo,
			},
		},
		{
			nombre: "método desconocido",
			pago: &PagoInput{
				Monto:  "100.00",
				Metodo: "CHEQUE",
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				_, err :=
					validarYNormalizarPago(
						prueba.pago,
					)

				if !errors.Is(
					err,
					ErrDatosInvalidos,
				) {
					t.Fatalf(
						"se esperaba ErrDatosInvalidos, se obtuvo %v",
						err,
					)
				}
			},
		)
	}
}

func TestServiceRegisterPaymentNormalizaYGuarda(
	t *testing.T,
) {
	referencia := "  LIQUIDACION-001  "
	notas := "  Pago final de la reserva  "

	store := &fakeStore{
		registerPaymentResult: Reserva{
			ID:             2,
			Folio:          "RES-00000002",
			Total:          Dinero("855.00"),
			MontoPagado:    Dinero("855.00"),
			SaldoPendiente: Dinero("0.00"),
			Estado:         EstadoConfirmada,
			EstadoPago:     EstadoPagoPagada,
		},
	}

	service := NewService(store)

	resultado, err :=
		service.RegisterPayment(
			context.Background(),
			RegistrarPagoInput{
				ReservaID: 2,
				PagoInput: PagoInput{
					Monto:      Dinero("555"),
					Metodo:     MetodoPago(" transferencia "),
					Referencia: &referencia,
					Notas:      &notas,
				},
			},
		)
	if err != nil {
		t.Fatalf(
			"RegisterPayment() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.registerPaymentCalls != 1 {
		t.Fatalf(
			"Store.RegisterPayment() fue llamado %d veces; se esperaba 1",
			store.registerPaymentCalls,
		)
	}

	input := store.registerPaymentInput

	if input.ReservaID != 2 {
		t.Errorf(
			"ReservaID = %d; se esperaba 2",
			input.ReservaID,
		)
	}

	if input.Monto != Dinero("555.00") {
		t.Errorf(
			"Monto = %q; se esperaba 555.00",
			input.Monto,
		)
	}

	if input.Metodo !=
		MetodoPagoTransferencia {
		t.Errorf(
			"Metodo = %q; se esperaba TRANSFERENCIA",
			input.Metodo,
		)
	}

	if input.Referencia == nil ||
		*input.Referencia != "LIQUIDACION-001" {
		t.Error(
			"la referencia no fue normalizada",
		)
	}

	if input.Notas == nil ||
		*input.Notas !=
			"Pago final de la reserva" {
		t.Error(
			"las notas no fueron normalizadas",
		)
	}

	if resultado.EstadoPago !=
		EstadoPagoPagada {
		t.Errorf(
			"EstadoPago = %q; se esperaba PAGADA",
			resultado.EstadoPago,
		)
	}
}

func TestServiceRegisterPaymentValidaciones(
	t *testing.T,
) {
	pruebas := []struct {
		nombre    string
		construir func() RegistrarPagoInput
	}{
		{
			nombre: "reserva inválida",
			construir: func() RegistrarPagoInput {
				return RegistrarPagoInput{
					ReservaID: 0,
					PagoInput: PagoInput{
						Monto:  "100.00",
						Metodo: MetodoPagoEfectivo,
					},
				}
			},
		},
		{
			nombre: "monto inválido",
			construir: func() RegistrarPagoInput {
				return RegistrarPagoInput{
					ReservaID: 1,
					PagoInput: PagoInput{
						Monto:  "0.00",
						Metodo: MetodoPagoEfectivo,
					},
				}
			},
		},
		{
			nombre: "método inválido",
			construir: func() RegistrarPagoInput {
				return RegistrarPagoInput{
					ReservaID: 1,
					PagoInput: PagoInput{
						Monto:  "100.00",
						Metodo: "CHEQUE",
					},
				}
			},
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				store := &fakeStore{}
				service := NewService(store)

				_, err :=
					service.RegisterPayment(
						context.Background(),
						prueba.construir(),
					)

				if !errors.Is(
					err,
					ErrDatosInvalidos,
				) {
					t.Fatalf(
						"se esperaba ErrDatosInvalidos, se obtuvo %v",
						err,
					)
				}

				if store.registerPaymentCalls != 0 {
					t.Error(
						"Store.RegisterPayment() no debe ejecutarse con datos inválidos",
					)
				}
			},
		)
	}
}

func TestServiceRegisterPaymentPropagaErrorStore(
	t *testing.T,
) {
	errorStore := errors.New(
		"error de PostgreSQL",
	)

	store := &fakeStore{
		registerPaymentErr: errorStore,
	}

	service := NewService(store)

	_, err := service.RegisterPayment(
		context.Background(),
		RegistrarPagoInput{
			ReservaID: 1,
			PagoInput: PagoInput{
				Monto:  "100.00",
				Metodo: MetodoPagoEfectivo,
			},
		},
	)

	if !errors.Is(err, errorStore) {
		t.Fatalf(
			"se esperaba el error del Store, se obtuvo %v",
			err,
		)
	}
}

func TestServiceGetByIDRetornaDetalle(
	t *testing.T,
) {
	store := &fakeStore{
		getByIDResult: Reserva{
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
					ID:        2,
					ReservaID: 2,
					Monto:     Dinero("555.00"),
					Metodo:    MetodoPagoTransferencia,
					Estado:    EstadoMovimientoAplicado,
				},
			},
		},
	}

	service := NewService(store)

	resultado, err := service.GetByID(
		context.Background(),
		2,
	)
	if err != nil {
		t.Fatalf(
			"GetByID() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.getByIDCalls != 1 {
		t.Fatalf(
			"Store.GetByID() fue llamado %d veces; se esperaba 1",
			store.getByIDCalls,
		)
	}

	if store.getByIDInput != 2 {
		t.Errorf(
			"reservaID recibido = %d; se esperaba 2",
			store.getByIDInput,
		)
	}

	if resultado.ID != 2 {
		t.Errorf(
			"resultado.ID = %d; se esperaba 2",
			resultado.ID,
		)
	}

	if resultado.EstadoPago !=
		EstadoPagoPagada {
		t.Errorf(
			"EstadoPago = %q; se esperaba PAGADA",
			resultado.EstadoPago,
		)
	}

	if len(resultado.Pagos) != 2 {
		t.Errorf(
			"se esperaban 2 pagos, se obtuvieron %d",
			len(resultado.Pagos),
		)
	}
}

func TestServiceGetByIDRechazaIDInvalido(
	t *testing.T,
) {
	store := &fakeStore{}
	service := NewService(store)

	_, err := service.GetByID(
		context.Background(),
		0,
	)

	if !errors.Is(err, ErrDatosInvalidos) {
		t.Fatalf(
			"se esperaba ErrDatosInvalidos, se obtuvo %v",
			err,
		)
	}

	if store.getByIDCalls != 0 {
		t.Error(
			"Store.GetByID() no debe ejecutarse con un ID inválido",
		)
	}
}

func TestServiceGetByIDPropagaErrorStore(
	t *testing.T,
) {
	store := &fakeStore{
		getByIDErr: ErrReservaNoEncontrada,
	}

	service := NewService(store)

	_, err := service.GetByID(
		context.Background(),
		999,
	)

	if !errors.Is(
		err,
		ErrReservaNoEncontrada,
	) {
		t.Fatalf(
			"se esperaba ErrReservaNoEncontrada, se obtuvo %v",
			err,
		)
	}

	if store.getByIDCalls != 1 {
		t.Errorf(
			"Store.GetByID() fue llamado %d veces; se esperaba 1",
			store.getByIDCalls,
		)
	}
}

func TestServiceConfirmValidaYGuarda(
	t *testing.T,
) {
	store := &fakeStore{
		confirmResult: Reserva{
			ID:                   3,
			Folio:                "RES-00000003",
			Estado:               EstadoConfirmada,
			EstadoPago:           EstadoPagoSinPago,
			RequiereConfirmacion: false,
			Pagos:                []Pago{},
		},
	}

	service := NewService(store)

	resultado, err := service.Confirm(
		context.Background(),
		ConfirmarInput{
			ReservaID: 3,
		},
	)
	if err != nil {
		t.Fatalf(
			"Confirm() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.confirmCalls != 1 {
		t.Fatalf(
			"Store.Confirm() fue llamado %d veces; se esperaba 1",
			store.confirmCalls,
		)
	}

	if store.confirmInput.ReservaID != 3 {
		t.Errorf(
			"ReservaID = %d; se esperaba 3",
			store.confirmInput.ReservaID,
		)
	}

	if resultado.Estado != EstadoConfirmada {
		t.Errorf(
			"Estado = %q; se esperaba CONFIRMADA",
			resultado.Estado,
		)
	}

	if resultado.EstadoPago != EstadoPagoSinPago {
		t.Errorf(
			"EstadoPago = %q; se esperaba SIN_PAGO",
			resultado.EstadoPago,
		)
	}

	if resultado.RequiereConfirmacion {
		t.Error(
			"la reserva confirmada no debe requerir confirmación",
		)
	}
}

func TestServiceConfirmRechazaIDInvalido(
	t *testing.T,
) {
	store := &fakeStore{}
	service := NewService(store)

	_, err := service.Confirm(
		context.Background(),
		ConfirmarInput{
			ReservaID: 0,
		},
	)

	if !errors.Is(err, ErrDatosInvalidos) {
		t.Fatalf(
			"se esperaba ErrDatosInvalidos, se obtuvo %v",
			err,
		)
	}

	if store.confirmCalls != 0 {
		t.Error(
			"Store.Confirm() no debe ejecutarse con un ID inválido",
		)
	}
}

func TestServiceConfirmPropagaErrorStore(
	t *testing.T,
) {
	store := &fakeStore{
		confirmErr: ErrReservaNoAceptaConfirmacion,
	}

	service := NewService(store)

	_, err := service.Confirm(
		context.Background(),
		ConfirmarInput{
			ReservaID: 3,
		},
	)

	if !errors.Is(
		err,
		ErrReservaNoAceptaConfirmacion,
	) {
		t.Fatalf(
			"se esperaba ErrReservaNoAceptaConfirmacion, se obtuvo %v",
			err,
		)
	}

	if store.confirmCalls != 1 {
		t.Errorf(
			"Store.Confirm() fue llamado %d veces; se esperaba 1",
			store.confirmCalls,
		)
	}
}

func TestServiceCancelNormalizaYEnviaPolitica(
	t *testing.T,
) {
	store := &fakeStore{
		cancelResult: Reserva{
			ID:                      4,
			Estado:                  EstadoCancelada,
			CancelacionReembolsable: true,
			MontoReembolsable:       "300.00",
		},
	}

	service := NewService(store)

	resultado, err := service.Cancel(
		context.Background(),
		CancelarInput{
			ReservaID: 4,
			Motivo:    "  El pasajero canceló su viaje  ",
		},
	)
	if err != nil {
		t.Fatalf(
			"Cancel() devolvió un error inesperado: %v",
			err,
		)
	}

	if store.cancelCalls != 1 {
		t.Fatalf(
			"Store.Cancel() fue llamado %d veces; se esperaba 1",
			store.cancelCalls,
		)
	}

	if store.cancelParams.Input.ReservaID != 4 {
		t.Errorf(
			"ReservaID = %d; se esperaba 4",
			store.cancelParams.Input.ReservaID,
		)
	}

	if store.cancelParams.Input.Motivo !=
		"El pasajero canceló su viaje" {
		t.Errorf(
			"Motivo = %q",
			store.cancelParams.Input.Motivo,
		)
	}

	if store.cancelParams.HorasLimiteReembolso != 3 {
		t.Errorf(
			"HorasLimiteReembolso = %d; se esperaba 3",
			store.cancelParams.HorasLimiteReembolso,
		)
	}

	if resultado.Estado != EstadoCancelada {
		t.Errorf(
			"Estado = %q; se esperaba CANCELADA",
			resultado.Estado,
		)
	}
}

func TestServiceCancelValidaciones(
	t *testing.T,
) {
	pruebas := []struct {
		nombre string
		input  CancelarInput
	}{
		{
			nombre: "reserva inválida",
			input: CancelarInput{
				ReservaID: 0,
				Motivo:    "Cancelación",
			},
		},
		{
			nombre: "motivo vacío",
			input: CancelarInput{
				ReservaID: 1,
				Motivo:    "   ",
			},
		},
		{
			nombre: "motivo demasiado largo",
			input: CancelarInput{
				ReservaID: 1,
				Motivo: strings.Repeat(
					"a",
					501,
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

				_, err := service.Cancel(
					context.Background(),
					prueba.input,
				)

				if !errors.Is(
					err,
					ErrDatosInvalidos,
				) {
					t.Fatalf(
						"se esperaba ErrDatosInvalidos, se obtuvo %v",
						err,
					)
				}

				if store.cancelCalls != 0 {
					t.Error(
						"Store.Cancel() no debe ejecutarse con datos inválidos",
					)
				}
			},
		)
	}
}

func TestServiceCancelPropagaErrorStore(
	t *testing.T,
) {
	store := &fakeStore{
		cancelErr: ErrSalidaYaIniciada,
	}

	service := NewService(store)

	_, err := service.Cancel(
		context.Background(),
		CancelarInput{
			ReservaID: 1,
			Motivo:    "Cancelación de prueba",
		},
	)

	if !errors.Is(err, ErrSalidaYaIniciada) {
		t.Fatalf(
			"se esperaba ErrSalidaYaIniciada, se obtuvo %v",
			err,
		)
	}
}

func TestParsearDecimal(t *testing.T) {
	pruebas := []struct {
		nombre   string
		entrada  string
		esperado int64
		valido   bool
	}{
		{
			nombre:   "entero",
			entrada:  "450",
			esperado: 45000,
			valido:   true,
		},
		{
			nombre:   "un decimal",
			entrada:  "450.5",
			esperado: 45050,
			valido:   true,
		},
		{
			nombre:   "dos decimales",
			entrada:  "450.50",
			esperado: 45050,
			valido:   true,
		},
		{
			nombre:  "vacío",
			entrada: "",
			valido:  false,
		},
		{
			nombre:  "negativo",
			entrada: "-1.00",
			valido:  false,
		},
		{
			nombre:  "tres decimales",
			entrada: "1.001",
			valido:  false,
		},
		{
			nombre:  "separador coma",
			entrada: "450,00",
			valido:  false,
		},
		{
			nombre:  "texto",
			entrada: "cuatrocientos",
			valido:  false,
		},
	}

	for _, prueba := range pruebas {
		t.Run(
			prueba.nombre,
			func(t *testing.T) {
				resultado, err :=
					parsearDecimal(
						prueba.entrada,
					)

				if prueba.valido && err != nil {
					t.Fatalf(
						"se obtuvo un error inesperado: %v",
						err,
					)
				}

				if !prueba.valido && err == nil {
					t.Fatal(
						"se esperaba un error",
					)
				}

				if prueba.valido &&
					resultado != prueba.esperado {
					t.Errorf(
						"resultado = %d; se esperaba %d",
						resultado,
						prueba.esperado,
					)
				}
			},
		)
	}
}

func nuevaReservaValida() CreateInput {
	return CreateInput{
		CorridaID: 1,
		NuevoPasajero: &NuevoPasajeroInput{
			NombreCompleto: "María González",
			Telefono:       "4431234567",
		},
		ParadaOrigenID:    1,
		ParadaDestinoID:   3,
		CantidadPasajeros: 1,
		PrecioUnitario:    Dinero("450.00"),
	}
}
