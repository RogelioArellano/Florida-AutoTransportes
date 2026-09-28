package reservas

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxImporteCentavos   int64 = 999_999_999_999
	maxCantidadPasajeros       = 100
)

var (
	ErrDatosInvalidos = errors.New(
		"datos de reserva inválidos",
	)

	ErrCorridaNoDisponible = errors.New(
		"la corrida no existe o no acepta reservas",
	)

	ErrPasajeroNoDisponible = errors.New(
		"el pasajero no existe o está inactivo",
	)

	ErrParadasInvalidas = errors.New(
		"el origen o destino no son válidos para la corrida",
	)

	ErrCupoInsuficiente = errors.New(
		"no existe cupo suficiente para el tramo solicitado",
	)

	ErrAnticipoRequerido = errors.New(
		"se requiere un anticipo para reservar más de un pasaje",
	)

	ErrAnticipoInsuficiente = errors.New(
		"el anticipo debe cubrir al menos el 30% del total",
	)

	ErrPagoExcedeTotal = errors.New(
		"el pago no puede exceder el total de la reserva",
	)
)

// CreateParams contiene la información ya validada,
// normalizada y calculada por el Service.
//
// El Repository no volverá a calcular estos importes.
// Su responsabilidad será comprobar los datos dependientes
// de PostgreSQL y guardar todo en una transacción.
type CreateParams struct {
	Input CreateInput

	Subtotal       Dinero
	MontoDescuento Dinero
	Total          Dinero

	Estado               Estado
	RequiereConfirmacion bool
}

// Store define las operaciones de persistencia que necesita
// el Service de reservas.
type Store interface {
	Create(
		ctx context.Context,
		params CreateParams,
	) (Reserva, error)

	List(
		ctx context.Context,
		filter ListFilter,
	) ([]Reserva, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{
		store: store,
	}
}

// Create valida y calcula una reserva antes de enviarla
// al Repository.
func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Reserva, error) {
	// Copiamos las estructuras anidadas para evitar modificar
	// accidentalmente los valores originales recibidos por
	// quien llamó al Service.
	input = copiarInput(input)

	if input.CorridaID <= 0 {
		return Reserva{}, fmt.Errorf(
			"%w: se debe seleccionar una corrida válida",
			ErrDatosInvalidos,
		)
	}

	if err := validarPasajero(&input); err != nil {
		return Reserva{}, err
	}

	if input.ParadaOrigenID <= 0 {
		return Reserva{}, fmt.Errorf(
			"%w: se debe seleccionar una parada de origen",
			ErrDatosInvalidos,
		)
	}

	if input.ParadaDestinoID <= 0 {
		return Reserva{}, fmt.Errorf(
			"%w: se debe seleccionar una parada de destino",
			ErrDatosInvalidos,
		)
	}

	if input.ParadaOrigenID == input.ParadaDestinoID {
		return Reserva{}, fmt.Errorf(
			"%w: el origen y el destino deben ser diferentes",
			ErrDatosInvalidos,
		)
	}

	if input.CantidadPasajeros <= 0 {
		return Reserva{}, fmt.Errorf(
			"%w: la cantidad de pasajeros debe ser mayor que cero",
			ErrDatosInvalidos,
		)
	}

	if input.CantidadPasajeros >
		maxCantidadPasajeros {
		return Reserva{}, fmt.Errorf(
			"%w: la cantidad de pasajeros no puede exceder %d",
			ErrDatosInvalidos,
			maxCantidadPasajeros,
		)
	}

	precioUnitarioCentavos, err := parsearDecimal(
		string(input.PrecioUnitario),
	)
	if err != nil || precioUnitarioCentavos <= 0 {
		return Reserva{}, fmt.Errorf(
			"%w: precio_unitario debe ser un importe positivo con máximo dos decimales",
			ErrDatosInvalidos,
		)
	}

	if precioUnitarioCentavos >
		maxImporteCentavos {
		return Reserva{}, fmt.Errorf(
			"%w: precio_unitario excede el importe permitido",
			ErrDatosInvalidos,
		)
	}

	input.PrecioUnitario = Dinero(
		formatearDecimal(precioUnitarioCentavos),
	)

	if precioUnitarioCentavos >
		maxImporteCentavos/
			int64(input.CantidadPasajeros) {
		return Reserva{}, fmt.Errorf(
			"%w: el subtotal excede el importe permitido",
			ErrDatosInvalidos,
		)
	}

	subtotalCentavos :=
		precioUnitarioCentavos *
			int64(input.CantidadPasajeros)

	montoDescuentoCentavos, err :=
		validarYCalcularDescuento(
			&input,
			precioUnitarioCentavos,
			subtotalCentavos,
		)
	if err != nil {
		return Reserva{}, err
	}

	totalCentavos :=
		subtotalCentavos -
			montoDescuentoCentavos

	if totalCentavos < 0 {
		return Reserva{}, fmt.Errorf(
			"%w: el descuento no puede superar el subtotal",
			ErrDatosInvalidos,
		)
	}

	estado := EstadoApartada
	requiereConfirmacion := true

	if err := validarPagoInicial(
		&input,
		totalCentavos,
	); err != nil {
		return Reserva{}, err
	}

	// Un pago, incluso parcial, funciona como confirmación.
	if input.PagoInicial != nil {
		estado = EstadoConfirmada
		requiereConfirmacion = false
	}

	// Una reserva con descuento del 100% no necesita pago
	// porque su saldo es cero.
	if totalCentavos == 0 {
		estado = EstadoConfirmada
		requiereConfirmacion = false
	}

	input.Observaciones, err =
		normalizarTextoOpcional(
			input.Observaciones,
			1000,
			"observaciones",
		)
	if err != nil {
		return Reserva{}, err
	}

	params := CreateParams{
		Input:                input,
		Subtotal:             Dinero(formatearDecimal(subtotalCentavos)),
		MontoDescuento:       Dinero(formatearDecimal(montoDescuentoCentavos)),
		Total:                Dinero(formatearDecimal(totalCentavos)),
		Estado:               estado,
		RequiereConfirmacion: requiereConfirmacion,
	}

	return s.store.Create(ctx, params)
}

// List valida y normaliza los filtros antes de consultar
// el Repository.
func (s *Service) List(
	ctx context.Context,
	filter ListFilter,
) ([]Reserva, error) {
	if filter.CorridaID != nil &&
		*filter.CorridaID <= 0 {
		return nil, fmt.Errorf(
			"%w: corrida_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	if filter.PasajeroID != nil &&
		*filter.PasajeroID <= 0 {
		return nil, fmt.Errorf(
			"%w: pasajero_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	if filter.Estado != nil {
		estado := Estado(
			strings.ToUpper(
				strings.TrimSpace(
					string(*filter.Estado),
				),
			),
		)

		if !esEstadoReservaValido(estado) {
			return nil, fmt.Errorf(
				"%w: el estado de reserva no es válido",
				ErrDatosInvalidos,
			)
		}

		filter.Estado = &estado
	}

	if filter.EstadoPago != nil {
		estadoPago := EstadoPago(
			strings.ToUpper(
				strings.TrimSpace(
					string(*filter.EstadoPago),
				),
			),
		)

		if !esEstadoPagoValido(estadoPago) {
			return nil, fmt.Errorf(
				"%w: el estado de pago no es válido",
				ErrDatosInvalidos,
			)
		}

		filter.EstadoPago = &estadoPago
	}

	var err error

	filter.FechaServicioDesde, err =
		normalizarFechaOpcional(
			filter.FechaServicioDesde,
			"fecha_servicio_desde",
		)
	if err != nil {
		return nil, err
	}

	filter.FechaServicioHasta, err =
		normalizarFechaOpcional(
			filter.FechaServicioHasta,
			"fecha_servicio_hasta",
		)
	if err != nil {
		return nil, err
	}

	if filter.FechaServicioDesde != nil &&
		filter.FechaServicioHasta != nil {
		fechaDesde, _ := time.Parse(
			"2006-01-02",
			*filter.FechaServicioDesde,
		)

		fechaHasta, _ := time.Parse(
			"2006-01-02",
			*filter.FechaServicioHasta,
		)

		if fechaHasta.Before(fechaDesde) {
			return nil, fmt.Errorf(
				"%w: fecha_servicio_hasta no puede ser anterior a fecha_servicio_desde",
				ErrDatosInvalidos,
			)
		}
	}

	filter.Busqueda, err =
		normalizarTextoOpcional(
			filter.Busqueda,
			150,
			"busqueda",
		)
	if err != nil {
		return nil, err
	}

	return s.store.List(ctx, filter)
}

// copiarInput evita efectos secundarios sobre estructuras
// anidadas recibidas mediante punteros.
func copiarInput(input CreateInput) CreateInput {
	if input.NuevoPasajero != nil {
		nuevoPasajero := *input.NuevoPasajero
		input.NuevoPasajero = &nuevoPasajero
	}

	if input.Descuento != nil {
		descuento := *input.Descuento
		input.Descuento = &descuento
	}

	if input.PagoInicial != nil {
		pagoInicial := *input.PagoInicial
		input.PagoInicial = &pagoInicial
	}

	return input
}

// validarPasajero comprueba que se proporcione exactamente
// un pasajero existente o los datos de uno nuevo.
func validarPasajero(input *CreateInput) error {
	tienePasajeroExistente :=
		input.PasajeroID != nil

	tienePasajeroNuevo :=
		input.NuevoPasajero != nil

	if tienePasajeroExistente ==
		tienePasajeroNuevo {
		return fmt.Errorf(
			"%w: se debe proporcionar exactamente pasajero_id o nuevo_pasajero",
			ErrDatosInvalidos,
		)
	}

	if input.PasajeroID != nil {
		if *input.PasajeroID <= 0 {
			return fmt.Errorf(
				"%w: pasajero_id debe ser válido",
				ErrDatosInvalidos,
			)
		}

		return nil
	}

	pasajero := input.NuevoPasajero

	pasajero.NombreCompleto =
		strings.TrimSpace(
			pasajero.NombreCompleto,
		)

	if pasajero.NombreCompleto == "" {
		return fmt.Errorf(
			"%w: el nombre del pasajero es obligatorio",
			ErrDatosInvalidos,
		)
	}

	if utf8.RuneCountInString(
		pasajero.NombreCompleto,
	) > 150 {
		return fmt.Errorf(
			"%w: el nombre del pasajero no puede exceder 150 caracteres",
			ErrDatosInvalidos,
		)
	}

	telefono, err := normalizarTelefono(
		pasajero.Telefono,
	)
	if err != nil {
		return err
	}

	pasajero.Telefono = telefono

	pasajero.Correo, err =
		normalizarCorreo(
			pasajero.Correo,
		)
	if err != nil {
		return err
	}

	pasajero.Notas, err =
		normalizarTextoOpcional(
			pasajero.Notas,
			1000,
			"notas del pasajero",
		)
	if err != nil {
		return err
	}

	return nil
}

func validarYCalcularDescuento(
	input *CreateInput,
	precioUnitarioCentavos int64,
	subtotalCentavos int64,
) (int64, error) {
	if input.Descuento == nil {
		return 0, nil
	}

	descuento := input.Descuento

	descuento.Tipo = TipoDescuento(
		strings.ToUpper(
			strings.TrimSpace(
				string(descuento.Tipo),
			),
		),
	)

	if descuento.Tipo !=
		TipoDescuentoPorcentaje &&
		descuento.Tipo !=
			TipoDescuentoMontoFijo {
		return 0, fmt.Errorf(
			"%w: el tipo de descuento no es válido",
			ErrDatosInvalidos,
		)
	}

	if descuento.CantidadPasajes <= 0 {
		return 0, fmt.Errorf(
			"%w: cantidad_pasajes del descuento debe ser mayor que cero",
			ErrDatosInvalidos,
		)
	}

	if descuento.CantidadPasajes >
		input.CantidadPasajeros {
		return 0, fmt.Errorf(
			"%w: el descuento no puede aplicarse a más pasajes de los reservados",
			ErrDatosInvalidos,
		)
	}

	descuento.Descripcion =
		strings.TrimSpace(
			descuento.Descripcion,
		)

	if descuento.Descripcion == "" {
		return 0, fmt.Errorf(
			"%w: la descripción del descuento es obligatoria",
			ErrDatosInvalidos,
		)
	}

	if utf8.RuneCountInString(
		descuento.Descripcion,
	) > 250 {
		return 0, fmt.Errorf(
			"%w: la descripción del descuento no puede exceder 250 caracteres",
			ErrDatosInvalidos,
		)
	}

	valor, err := parsearDecimal(
		descuento.Valor,
	)
	if err != nil || valor <= 0 {
		return 0, fmt.Errorf(
			"%w: el valor del descuento debe ser positivo y tener máximo dos decimales",
			ErrDatosInvalidos,
		)
	}

	descuento.Valor =
		formatearDecimal(valor)

	baseDescuento :=
		precioUnitarioCentavos *
			int64(descuento.CantidadPasajes)

	var montoDescuento int64

	switch descuento.Tipo {
	case TipoDescuentoPorcentaje:
		// 100.00% se representa como 10000.
		if valor > 10_000 {
			return 0, fmt.Errorf(
				"%w: el porcentaje de descuento no puede exceder 100",
				ErrDatosInvalidos,
			)
		}

		// Se suma 5000 antes de dividir para redondear
		// al centavo más cercano.
		montoDescuento = (baseDescuento*valor + 5_000) / 10_000

	case TipoDescuentoMontoFijo:
		if valor > baseDescuento {
			return 0, fmt.Errorf(
				"%w: el descuento fijo no puede superar el importe de los pasajes a los que se aplica",
				ErrDatosInvalidos,
			)
		}

		montoDescuento = valor
	}

	if montoDescuento > subtotalCentavos {
		return 0, fmt.Errorf(
			"%w: el descuento no puede superar el subtotal",
			ErrDatosInvalidos,
		)
	}

	return montoDescuento, nil
}

func validarPagoInicial(
	input *CreateInput,
	totalCentavos int64,
) error {
	if input.PagoInicial == nil {
		if input.CantidadPasajeros > 1 &&
			totalCentavos > 0 {
			return fmt.Errorf(
				"%w: para %d pasajes se debe registrar al menos el 30%% de anticipo",
				ErrAnticipoRequerido,
				input.CantidadPasajeros,
			)
		}

		return nil
	}

	pago := input.PagoInicial

	montoCentavos, err := parsearDecimal(
		string(pago.Monto),
	)
	if err != nil || montoCentavos <= 0 {
		return fmt.Errorf(
			"%w: el monto del pago debe ser positivo y tener máximo dos decimales",
			ErrDatosInvalidos,
		)
	}

	if montoCentavos > totalCentavos {
		return fmt.Errorf(
			"%w: total %s, pago recibido %s",
			ErrPagoExcedeTotal,
			formatearDecimal(totalCentavos),
			formatearDecimal(montoCentavos),
		)
	}

	pago.Monto = Dinero(
		formatearDecimal(montoCentavos),
	)

	pago.Metodo = MetodoPago(
		strings.ToUpper(
			strings.TrimSpace(
				string(pago.Metodo),
			),
		),
	)

	if !esMetodoPagoValido(pago.Metodo) {
		return fmt.Errorf(
			"%w: el método de pago no es válido",
			ErrDatosInvalidos,
		)
	}

	pago.Referencia, err =
		normalizarTextoOpcional(
			pago.Referencia,
			150,
			"referencia del pago",
		)
	if err != nil {
		return err
	}

	pago.Notas, err =
		normalizarTextoOpcional(
			pago.Notas,
			1000,
			"notas del pago",
		)
	if err != nil {
		return err
	}

	if input.CantidadPasajeros > 1 {
		// División entera redondeada hacia arriba.
		anticipoMinimo :=
			(totalCentavos*30 + 99) / 100

		if montoCentavos < anticipoMinimo {
			return fmt.Errorf(
				"%w: se requieren al menos %s y se recibieron %s",
				ErrAnticipoInsuficiente,
				formatearDecimal(anticipoMinimo),
				formatearDecimal(montoCentavos),
			)
		}
	}

	return nil
}

// parsearDecimal convierte un texto decimal con máximo dos
// posiciones a un entero expresado en centavos.
//
// Ejemplos:
//
//	"450"    -> 45000
//	"450.5"  -> 45050
//	"450.50" -> 45050
func parsearDecimal(valor string) (int64, error) {
	texto := strings.TrimSpace(valor)

	if texto == "" {
		return 0, errors.New(
			"el decimal está vacío",
		)
	}

	if strings.HasPrefix(texto, "-") ||
		strings.HasPrefix(texto, "+") {
		return 0, errors.New(
			"el decimal no debe incluir signo",
		)
	}

	partes := strings.Split(texto, ".")

	if len(partes) > 2 ||
		partes[0] == "" {
		return 0, errors.New(
			"formato decimal inválido",
		)
	}

	if !contieneSoloDigitos(partes[0]) {
		return 0, errors.New(
			"la parte entera contiene caracteres inválidos",
		)
	}

	parteDecimal := ""

	if len(partes) == 2 {
		parteDecimal = partes[1]

		if parteDecimal == "" ||
			len(parteDecimal) > 2 ||
			!contieneSoloDigitos(parteDecimal) {
			return 0, errors.New(
				"la parte decimal es inválida",
			)
		}
	}

	switch len(parteDecimal) {
	case 0:
		parteDecimal = "00"

	case 1:
		parteDecimal += "0"
	}

	unidades, err := strconv.ParseInt(
		partes[0],
		10,
		64,
	)
	if err != nil {
		return 0, err
	}

	centavos, err := strconv.ParseInt(
		parteDecimal,
		10,
		64,
	)
	if err != nil {
		return 0, err
	}

	if unidades >
		(maxImporteCentavos-centavos)/100 {
		return 0, errors.New(
			"el decimal excede el importe permitido",
		)
	}

	return unidades*100 + centavos, nil
}

func formatearDecimal(centavos int64) string {
	return fmt.Sprintf(
		"%d.%02d",
		centavos/100,
		centavos%100,
	)
}

func contieneSoloDigitos(texto string) bool {
	if texto == "" {
		return false
	}

	for _, caracter := range texto {
		if caracter < '0' ||
			caracter > '9' {
			return false
		}
	}

	return true
}

func normalizarTelefono(
	telefono string,
) (string, error) {
	texto := strings.TrimSpace(telefono)

	var resultado strings.Builder
	cantidadDigitos := 0

	for indice, caracter := range texto {
		switch {
		case caracter >= '0' &&
			caracter <= '9':
			resultado.WriteRune(caracter)
			cantidadDigitos++

		case caracter == '+' &&
			indice == 0:
			resultado.WriteRune(caracter)

		case caracter == ' ',
			caracter == '-',
			caracter == '(',
			caracter == ')':
			// Son separadores permitidos y se eliminan.

		default:
			return "", fmt.Errorf(
				"%w: el teléfono contiene caracteres no permitidos",
				ErrDatosInvalidos,
			)
		}
	}

	if cantidadDigitos < 10 ||
		cantidadDigitos > 15 {
		return "", fmt.Errorf(
			"%w: el teléfono debe contener entre 10 y 15 dígitos",
			ErrDatosInvalidos,
		)
	}

	return resultado.String(), nil
}

func normalizarCorreo(
	correo *string,
) (*string, error) {
	if correo == nil {
		return nil, nil
	}

	texto := strings.ToLower(
		strings.TrimSpace(*correo),
	)

	if texto == "" {
		return nil, nil
	}

	if utf8.RuneCountInString(texto) > 254 {
		return nil, fmt.Errorf(
			"%w: el correo no puede exceder 254 caracteres",
			ErrDatosInvalidos,
		)
	}

	direccion, err := mail.ParseAddress(texto)
	if err != nil ||
		direccion.Address != texto {
		return nil, fmt.Errorf(
			"%w: el correo no tiene un formato válido",
			ErrDatosInvalidos,
		)
	}

	return &texto, nil
}

func normalizarTextoOpcional(
	valor *string,
	maximo int,
	campo string,
) (*string, error) {
	if valor == nil {
		return nil, nil
	}

	texto := strings.TrimSpace(*valor)

	if texto == "" {
		return nil, nil
	}

	if utf8.RuneCountInString(texto) >
		maximo {
		return nil, fmt.Errorf(
			"%w: %s no puede exceder %d caracteres",
			ErrDatosInvalidos,
			campo,
			maximo,
		)
	}

	return &texto, nil
}

func normalizarFechaOpcional(
	valor *string,
	campo string,
) (*string, error) {
	if valor == nil {
		return nil, nil
	}

	texto := strings.TrimSpace(*valor)

	if texto == "" {
		return nil, nil
	}

	const formato = "2006-01-02"

	fecha, err := time.Parse(
		formato,
		texto,
	)
	if err != nil ||
		fecha.Format(formato) != texto {
		return nil, fmt.Errorf(
			"%w: %s debe utilizar el formato AAAA-MM-DD",
			ErrDatosInvalidos,
			campo,
		)
	}

	return &texto, nil
}

func esEstadoReservaValido(
	estado Estado,
) bool {
	switch estado {
	case EstadoApartada,
		EstadoConfirmada,
		EstadoCancelada,
		EstadoAbordada,
		EstadoNoPresentada:
		return true

	default:
		return false
	}
}

func esEstadoPagoValido(
	estado EstadoPago,
) bool {
	switch estado {
	case EstadoPagoSinPago,
		EstadoPagoParcial,
		EstadoPagoPagada,
		EstadoPagoSaldoAFavor:
		return true

	default:
		return false
	}
}

func esMetodoPagoValido(
	metodo MetodoPago,
) bool {
	switch metodo {
	case MetodoPagoEfectivo,
		MetodoPagoTransferencia,
		MetodoPagoTarjeta,
		MetodoPagoOtro:
		return true

	default:
		return false
	}
}
