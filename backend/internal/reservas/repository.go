package reservas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

var _ Store = (*PostgresRepository)(nil)

func NewPostgresRepository(
	pool *pgxpool.Pool,
) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

type datosCorrida struct {
	CapacidadPasajeros int
	Estado             string
	ReservasAbiertas   bool
}

type datosTramo struct {
	OrdenOrigen  int
	PermiteSubir bool
	OrdenDestino int
	PermiteBajar bool
}

const consultaReservaBase = `
	SELECT
		r.id,
		r.folio,

		r.corrida_id,
		c.folio,
		TO_CHAR(
			c.fecha_servicio,
			'YYYY-MM-DD'
		),
		c.salida_programada,

		r.pasajero_id,
		p.nombre_completo,
		p.telefono,

		r.corrida_parada_origen_id,
		origen.punto_nombre,
		origen.orden,

		r.corrida_parada_destino_id,
		destino.punto_nombre,
		destino.orden,

		r.cantidad_pasajeros,
		r.precio_unitario::TEXT,
		r.subtotal::TEXT,

		r.tipo_descuento,
		r.valor_descuento::TEXT,
		r.cantidad_pasajes_descuento,
		r.monto_descuento::TEXT,
		r.descripcion_descuento,

		r.total::TEXT,

		saldos.monto_pagado::TEXT,
		saldos.saldo_pendiente::TEXT,
		saldos.estado_pago,

		r.estado,
		r.requiere_confirmacion,
		r.confirmada_en,
		r.confirmacion_solicitada_en,
		r.confirmacion_limite_en,

		r.cancelada_en,
		r.motivo_cancelacion,
		r.observaciones,

		r.creado_en,
		r.actualizado_en

	FROM reservas r

	INNER JOIN corridas c
		ON c.id = r.corrida_id

	INNER JOIN pasajeros p
		ON p.id = r.pasajero_id

	INNER JOIN corrida_paradas origen
		ON origen.id =
			r.corrida_parada_origen_id
		AND origen.corrida_id =
			r.corrida_id

	INNER JOIN corrida_paradas destino
		ON destino.id =
			r.corrida_parada_destino_id
		AND destino.corrida_id =
			r.corrida_id

	INNER JOIN vw_reservas_saldos saldos
		ON saldos.reserva_id = r.id
`

// Create registra una reserva completa.
//
// Toda la operación utiliza una sola transacción. Si falla
// el pasajero, el pago, el cupo o cualquier otra operación,
// PostgreSQL revierte todo.
func (r *PostgresRepository) Create(
	ctx context.Context,
	params CreateParams,
) (Reserva, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Reserva{}, fmt.Errorf(
			"iniciar transacción de reserva: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Se bloquea la corrida para serializar la creación de
	// reservas. Dos solicitudes para la misma corrida no
	// podrán calcular el cupo simultáneamente.
	corrida, err := consultarCorridaParaReserva(
		ctx,
		tx,
		params.Input.CorridaID,
	)
	if err != nil {
		return Reserva{}, err
	}

	tramo, err := consultarTramo(
		ctx,
		tx,
		params.Input.CorridaID,
		params.Input.ParadaOrigenID,
		params.Input.ParadaDestinoID,
	)
	if err != nil {
		return Reserva{}, err
	}

	ocupacionMaxima, err :=
		consultarOcupacionMaxima(
			ctx,
			tx,
			params.Input.CorridaID,
			tramo.OrdenOrigen,
			tramo.OrdenDestino,
		)
	if err != nil {
		return Reserva{}, err
	}

	cupoDisponible :=
		corrida.CapacidadPasajeros -
			ocupacionMaxima

	if params.Input.CantidadPasajeros >
		cupoDisponible {
		return Reserva{}, fmt.Errorf(
			"%w: solicitados %d, disponibles %d",
			ErrCupoInsuficiente,
			params.Input.CantidadPasajeros,
			cupoDisponible,
		)
	}

	pasajeroID, err := guardarPasajero(
		ctx,
		tx,
		params.Input,
	)
	if err != nil {
		return Reserva{}, err
	}

	reservaID, err := insertarReserva(
		ctx,
		tx,
		pasajeroID,
		params,
	)
	if err != nil {
		return Reserva{}, err
	}

	// Si el origen o destino eran paradas bajo demanda,
	// ahora deben formar parte del recorrido de la corrida.
	if err := incluirParadasEnRecorrido(
		ctx,
		tx,
		params.Input.CorridaID,
		params.Input.ParadaOrigenID,
		params.Input.ParadaDestinoID,
	); err != nil {
		return Reserva{}, err
	}

	if params.Input.PagoInicial != nil {
		if err := insertarPagoInicial(
			ctx,
			tx,
			reservaID,
			*params.Input.PagoInicial,
		); err != nil {
			return Reserva{}, err
		}
	}

	reservaCreada, err := obtenerReservaPorID(
		ctx,
		tx,
		reservaID,
	)
	if err != nil {
		return Reserva{}, fmt.Errorf(
			"consultar reserva creada: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Reserva{}, fmt.Errorf(
			"confirmar transacción de reserva: %w",
			err,
		)
	}

	return reservaCreada, nil
}

// List consulta reservas aplicando filtros opcionales.
func (r *PostgresRepository) List(
	ctx context.Context,
	filter ListFilter,
) ([]Reserva, error) {
	var estado any
	var estadoPago any

	if filter.Estado != nil {
		estado = string(*filter.Estado)
	}

	if filter.EstadoPago != nil {
		estadoPago = string(*filter.EstadoPago)
	}

	query := consultaReservaBase + `
		WHERE (
			$1::BIGINT IS NULL
			OR r.corrida_id = $1::BIGINT
		)
		AND (
			$2::BIGINT IS NULL
			OR r.pasajero_id = $2::BIGINT
		)
		AND (
			$3::VARCHAR IS NULL
			OR r.estado = $3::VARCHAR
		)
		AND (
			$4::VARCHAR IS NULL
			OR saldos.estado_pago = $4::VARCHAR
		)
		AND (
			$5::DATE IS NULL
			OR c.fecha_servicio >= $5::DATE
		)
		AND (
			$6::DATE IS NULL
			OR c.fecha_servicio <= $6::DATE
		)
		AND (
			$7::TEXT IS NULL
			OR r.folio ILIKE
				'%' || $7::TEXT || '%'
			OR p.nombre_completo ILIKE
				'%' || $7::TEXT || '%'
			OR p.telefono ILIKE
				'%' || $7::TEXT || '%'
		)
		ORDER BY
			c.fecha_servicio,
			c.salida_programada,
			r.id;
	`

	rows, err := r.pool.Query(
		ctx,
		query,
		valorInt64Opcional(filter.CorridaID),
		valorInt64Opcional(filter.PasajeroID),
		estado,
		estadoPago,
		valorStringOpcional(
			filter.FechaServicioDesde,
		),
		valorStringOpcional(
			filter.FechaServicioHasta,
		),
		valorStringOpcional(filter.Busqueda),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar reservas: %w",
			err,
		)
	}
	defer rows.Close()

	reservasEncontradas := make(
		[]Reserva,
		0,
	)

	for rows.Next() {
		reserva, err := scanReserva(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"leer reserva: %w",
				err,
			)
		}

		// El listado contiene el resumen financiero.
		// Los movimientos se cargarán cuando consultemos
		// el detalle o al crear una reserva.
		reserva.Pagos = make([]Pago, 0)

		reservasEncontradas = append(
			reservasEncontradas,
			reserva,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer reservas: %w",
			err,
		)
	}

	return reservasEncontradas, nil
}

// consultarCorridaParaReserva obtiene y bloquea la corrida.
//
// El bloqueo FOR UPDATE hace que las reservas para una misma
// corrida se procesen una después de otra. Así evitamos que
// dos personas ocupen simultáneamente el último lugar.
func consultarCorridaParaReserva(
	ctx context.Context,
	tx pgx.Tx,
	corridaID int64,
) (datosCorrida, error) {
	const query = `
		SELECT
			capacidad_pasajeros,
			estado,
			reservas_abiertas
		FROM corridas
		WHERE id = $1
		FOR UPDATE;
	`

	var capacidad int16
	var datos datosCorrida

	err := tx.QueryRow(
		ctx,
		query,
		corridaID,
	).Scan(
		&capacidad,
		&datos.Estado,
		&datos.ReservasAbiertas,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return datosCorrida{},
			ErrCorridaNoDisponible
	}

	if err != nil {
		return datosCorrida{}, fmt.Errorf(
			"consultar corrida para reserva: %w",
			err,
		)
	}

	datos.CapacidadPasajeros =
		int(capacidad)

	if !datos.ReservasAbiertas {
		return datosCorrida{},
			ErrCorridaNoDisponible
	}

	switch datos.Estado {
	case "PROGRAMADA", "ABORDANDO":
		return datos, nil

	default:
		return datosCorrida{},
			ErrCorridaNoDisponible
	}
}

// consultarTramo valida que las dos paradas pertenezcan a
// la corrida y que permitan la operación solicitada.
func consultarTramo(
	ctx context.Context,
	tx pgx.Tx,
	corridaID int64,
	origenID int64,
	destinoID int64,
) (datosTramo, error) {
	const query = `
		SELECT
			origen.orden,
			origen.permite_subir,
			destino.orden,
			destino.permite_bajar
		FROM corrida_paradas origen
		INNER JOIN corrida_paradas destino
			ON destino.corrida_id =
				origen.corrida_id
		WHERE origen.corrida_id = $1
			AND origen.id = $2
			AND destino.id = $3;
	`

	var datos datosTramo
	var ordenOrigen int16
	var ordenDestino int16

	err := tx.QueryRow(
		ctx,
		query,
		corridaID,
		origenID,
		destinoID,
	).Scan(
		&ordenOrigen,
		&datos.PermiteSubir,
		&ordenDestino,
		&datos.PermiteBajar,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return datosTramo{},
			ErrParadasInvalidas
	}

	if err != nil {
		return datosTramo{}, fmt.Errorf(
			"consultar tramo de reserva: %w",
			err,
		)
	}

	datos.OrdenOrigen = int(ordenOrigen)
	datos.OrdenDestino = int(ordenDestino)

	if datos.OrdenOrigen >= datos.OrdenDestino {
		return datosTramo{},
			ErrParadasInvalidas
	}

	if !datos.PermiteSubir ||
		!datos.PermiteBajar {
		return datosTramo{},
			ErrParadasInvalidas
	}

	return datos, nil
}

// consultarOcupacionMaxima calcula la ocupación de cada
// segmento atravesado por la nueva reserva.
//
// Una reserva de orden 1 a orden 3 ocupa:
//
//	segmento 1 -> entre las paradas 1 y 2
//	segmento 2 -> entre las paradas 2 y 3
//
// La reserva no ocupa un segmento después de su destino.
func consultarOcupacionMaxima(
	ctx context.Context,
	tx pgx.Tx,
	corridaID int64,
	ordenOrigen int,
	ordenDestino int,
) (int, error) {
	const query = `
		SELECT
			COALESCE(
				MAX(ocupacion.ocupados),
				0
			)
		FROM (
			SELECT
				COALESCE(
					(
						SELECT
							SUM(
								r.cantidad_pasajeros
							)
						FROM reservas r

						INNER JOIN corrida_paradas
							origen_reserva
							ON origen_reserva.id =
								r.corrida_parada_origen_id
							AND origen_reserva.corrida_id =
								r.corrida_id

						INNER JOIN corrida_paradas
							destino_reserva
							ON destino_reserva.id =
								r.corrida_parada_destino_id
							AND destino_reserva.corrida_id =
								r.corrida_id

						WHERE r.corrida_id = $1
							AND r.estado IN (
								'APARTADA',
								'CONFIRMADA',
								'ABORDADA'
							)
							AND origen_reserva.orden <=
								segmento.orden
							AND destino_reserva.orden >
								segmento.orden
					),
					0
				) AS ocupados

			FROM GENERATE_SERIES(
				$2::INTEGER,
				$3::INTEGER - 1
			) AS segmento(orden)
		) AS ocupacion;
	`

	var ocupacion int64

	err := tx.QueryRow(
		ctx,
		query,
		corridaID,
		ordenOrigen,
		ordenDestino,
	).Scan(&ocupacion)
	if err != nil {
		return 0, fmt.Errorf(
			"consultar ocupación del tramo: %w",
			err,
		)
	}

	return int(ocupacion), nil
}

func guardarPasajero(
	ctx context.Context,
	tx pgx.Tx,
	input CreateInput,
) (int64, error) {
	if input.PasajeroID != nil {
		const consultar = `
			SELECT id
			FROM pasajeros
			WHERE id = $1
				AND activo = TRUE;
		`

		var pasajeroID int64

		err := tx.QueryRow(
			ctx,
			consultar,
			*input.PasajeroID,
		).Scan(&pasajeroID)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0,
				ErrPasajeroNoDisponible
		}

		if err != nil {
			return 0, fmt.Errorf(
				"consultar pasajero: %w",
				err,
			)
		}

		return pasajeroID, nil
	}

	if input.NuevoPasajero == nil {
		return 0, fmt.Errorf(
			"%w: faltan los datos del pasajero",
			ErrDatosInvalidos,
		)
	}

	const insertar = `
		INSERT INTO pasajeros (
			nombre_completo,
			telefono,
			correo,
			notas
		)
		VALUES (
			$1,
			$2,
			$3,
			$4
		)
		RETURNING id;
	`

	pasajero := input.NuevoPasajero

	var pasajeroID int64

	err := tx.QueryRow(
		ctx,
		insertar,
		pasajero.NombreCompleto,
		pasajero.Telefono,
		pasajero.Correo,
		pasajero.Notas,
	).Scan(&pasajeroID)
	if err != nil {
		return 0, fmt.Errorf(
			"insertar pasajero: %w",
			err,
		)
	}

	return pasajeroID, nil
}

func insertarReserva(
	ctx context.Context,
	tx pgx.Tx,
	pasajeroID int64,
	params CreateParams,
) (int64, error) {
	const query = `
		INSERT INTO reservas (
			corrida_id,
			pasajero_id,
			corrida_parada_origen_id,
			corrida_parada_destino_id,
			cantidad_pasajeros,
			precio_unitario,
			subtotal,
			tipo_descuento,
			valor_descuento,
			cantidad_pasajes_descuento,
			monto_descuento,
			descripcion_descuento,
			total,
			estado,
			requiere_confirmacion,
			confirmada_en,
			observaciones
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6::NUMERIC,
			$7::NUMERIC,
			$8,
			$9::NUMERIC,
			$10,
			$11::NUMERIC,
			$12,
			$13::NUMERIC,
			$14,
			$15,
			CASE
				WHEN $14 = 'CONFIRMADA'
					THEN NOW()
				ELSE NULL
			END,
			$16
		)
		RETURNING id;
	`

	var tipoDescuento any
	var valorDescuento = "0.00"
	var cantidadPasajesDescuento int
	var descripcionDescuento any

	if params.Input.Descuento != nil {
		tipoDescuento =
			string(params.Input.Descuento.Tipo)

		valorDescuento =
			params.Input.Descuento.Valor

		cantidadPasajesDescuento =
			params.Input.Descuento.
				CantidadPasajes

		descripcionDescuento =
			params.Input.Descuento.
				Descripcion
	}

	var reservaID int64

	err := tx.QueryRow(
		ctx,
		query,
		params.Input.CorridaID,
		pasajeroID,
		params.Input.ParadaOrigenID,
		params.Input.ParadaDestinoID,
		params.Input.CantidadPasajeros,
		string(params.Input.PrecioUnitario),
		string(params.Subtotal),
		tipoDescuento,
		valorDescuento,
		cantidadPasajesDescuento,
		string(params.MontoDescuento),
		descripcionDescuento,
		string(params.Total),
		string(params.Estado),
		params.RequiereConfirmacion,
		params.Input.Observaciones,
	).Scan(&reservaID)
	if err != nil {
		return 0, fmt.Errorf(
			"insertar reserva: %w",
			err,
		)
	}

	return reservaID, nil
}

func incluirParadasEnRecorrido(
	ctx context.Context,
	tx pgx.Tx,
	corridaID int64,
	origenID int64,
	destinoID int64,
) error {
	const query = `
		UPDATE corrida_paradas
		SET
			incluida_en_recorrido = TRUE,
			actualizado_en = NOW()
		WHERE corrida_id = $1
			AND id IN ($2, $3)
			AND incluida_en_recorrido = FALSE;
	`

	_, err := tx.Exec(
		ctx,
		query,
		corridaID,
		origenID,
		destinoID,
	)
	if err != nil {
		return fmt.Errorf(
			"incluir paradas en recorrido: %w",
			err,
		)
	}

	return nil
}

func insertarPagoInicial(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
	pago PagoInput,
) error {
	const query = `
		INSERT INTO pagos_reserva (
			reserva_id,
			monto,
			metodo,
			referencia,
			notas
		)
		VALUES (
			$1,
			$2::NUMERIC,
			$3,
			$4,
			$5
		);
	`

	_, err := tx.Exec(
		ctx,
		query,
		reservaID,
		string(pago.Monto),
		string(pago.Metodo),
		pago.Referencia,
		pago.Notas,
	)
	if err != nil {
		return fmt.Errorf(
			"insertar pago inicial: %w",
			err,
		)
	}

	return nil
}

type querier interface {
	Query(
		ctx context.Context,
		sql string,
		args ...any,
	) (pgx.Rows, error)

	QueryRow(
		ctx context.Context,
		sql string,
		args ...any,
	) pgx.Row
}

func obtenerReservaPorID(
	ctx context.Context,
	querier querier,
	reservaID int64,
) (Reserva, error) {
	query := consultaReservaBase + `
		WHERE r.id = $1;
	`

	reserva, err := scanReserva(
		querier.QueryRow(
			ctx,
			query,
			reservaID,
		),
	)
	if err != nil {
		return Reserva{}, err
	}

	pagos, err := listarPagosReserva(
		ctx,
		querier,
		reservaID,
	)
	if err != nil {
		return Reserva{}, err
	}

	reserva.Pagos = pagos

	return reserva, nil
}

func listarPagosReserva(
	ctx context.Context,
	querier querier,
	reservaID int64,
) ([]Pago, error) {
	const query = `
		SELECT
			id,
			reserva_id,
			monto::TEXT,
			metodo,
			referencia,
			estado,
			pagado_en,
			anulado_en,
			motivo_anulacion,
			notas,
			creado_en,
			actualizado_en
		FROM pagos_reserva
		WHERE reserva_id = $1
		ORDER BY
			pagado_en,
			id;
	`

	rows, err := querier.Query(
		ctx,
		query,
		reservaID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar pagos de reserva: %w",
			err,
		)
	}
	defer rows.Close()

	pagos := make([]Pago, 0)

	for rows.Next() {
		pago, err := scanPago(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"leer pago de reserva: %w",
				err,
			)
		}

		pagos = append(pagos, pago)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer pagos de reserva: %w",
			err,
		)
	}

	return pagos, nil
}

type scanner interface {
	Scan(destinos ...any) error
}

func scanReserva(
	fila scanner,
) (Reserva, error) {
	var reserva Reserva

	var ordenOrigen int16
	var ordenDestino int16
	var cantidadPasajeros int16
	var cantidadPasajesDescuento int16

	var tipoDescuento sql.NullString
	var descripcionDescuento sql.NullString

	var confirmadaEn sql.NullTime
	var confirmacionSolicitadaEn sql.NullTime
	var confirmacionLimiteEn sql.NullTime

	var canceladaEn sql.NullTime
	var motivoCancelacion sql.NullString
	var observaciones sql.NullString

	var precioUnitario string
	var subtotal string
	var valorDescuento string
	var montoDescuento string
	var total string
	var montoPagado string
	var saldoPendiente string

	var estadoReserva string
	var estadoPago string

	err := fila.Scan(
		&reserva.ID,
		&reserva.Folio,

		&reserva.CorridaID,
		&reserva.CorridaFolio,
		&reserva.FechaServicio,
		&reserva.SalidaProgramada,

		&reserva.PasajeroID,
		&reserva.PasajeroNombre,
		&reserva.PasajeroTelefono,

		&reserva.ParadaOrigenID,
		&reserva.ParadaOrigenNombre,
		&ordenOrigen,

		&reserva.ParadaDestinoID,
		&reserva.ParadaDestinoNombre,
		&ordenDestino,

		&cantidadPasajeros,
		&precioUnitario,
		&subtotal,

		&tipoDescuento,
		&valorDescuento,
		&cantidadPasajesDescuento,
		&montoDescuento,
		&descripcionDescuento,

		&total,

		&montoPagado,
		&saldoPendiente,
		&estadoPago,

		&estadoReserva,
		&reserva.RequiereConfirmacion,
		&confirmadaEn,
		&confirmacionSolicitadaEn,
		&confirmacionLimiteEn,

		&canceladaEn,
		&motivoCancelacion,
		&observaciones,

		&reserva.CreadoEn,
		&reserva.ActualizadoEn,
	)
	if err != nil {
		return Reserva{}, err
	}

	reserva.OrdenOrigen = int(ordenOrigen)
	reserva.OrdenDestino = int(ordenDestino)

	reserva.CantidadPasajeros =
		int(cantidadPasajeros)

	reserva.PrecioUnitario =
		Dinero(precioUnitario)

	reserva.Subtotal =
		Dinero(subtotal)

	reserva.ValorDescuento =
		Dinero(valorDescuento)

	reserva.CantidadPasajesDescuento =
		int(cantidadPasajesDescuento)

	reserva.MontoDescuento =
		Dinero(montoDescuento)

	reserva.Total =
		Dinero(total)

	reserva.MontoPagado =
		Dinero(montoPagado)

	reserva.SaldoPendiente =
		Dinero(saldoPendiente)

	reserva.Estado =
		Estado(estadoReserva)

	reserva.EstadoPago =
		EstadoPago(estadoPago)

	reserva.TipoDescuento =
		tipoDescuentoDesdeNull(
			tipoDescuento,
		)

	reserva.DescripcionDescuento =
		stringDesdeNull(
			descripcionDescuento,
		)

	reserva.ConfirmadaEn =
		timeDesdeNull(confirmadaEn)

	reserva.ConfirmacionSolicitadaEn =
		timeDesdeNull(
			confirmacionSolicitadaEn,
		)

	reserva.ConfirmacionLimiteEn =
		timeDesdeNull(
			confirmacionLimiteEn,
		)

	reserva.CanceladaEn =
		timeDesdeNull(canceladaEn)

	reserva.MotivoCancelacion =
		stringDesdeNull(motivoCancelacion)

	reserva.Observaciones =
		stringDesdeNull(observaciones)

	return reserva, nil
}

func scanPago(
	fila scanner,
) (Pago, error) {
	var pago Pago

	var monto string
	var metodo string
	var estado string

	var referencia sql.NullString
	var anuladoEn sql.NullTime
	var motivoAnulacion sql.NullString
	var notas sql.NullString

	err := fila.Scan(
		&pago.ID,
		&pago.ReservaID,
		&monto,
		&metodo,
		&referencia,
		&estado,
		&pago.PagadoEn,
		&anuladoEn,
		&motivoAnulacion,
		&notas,
		&pago.CreadoEn,
		&pago.ActualizadoEn,
	)
	if err != nil {
		return Pago{}, err
	}

	pago.Monto = Dinero(monto)
	pago.Metodo = MetodoPago(metodo)

	pago.Estado =
		EstadoMovimientoPago(estado)

	pago.Referencia =
		stringDesdeNull(referencia)

	pago.AnuladoEn =
		timeDesdeNull(anuladoEn)

	pago.MotivoAnulacion =
		stringDesdeNull(motivoAnulacion)

	pago.Notas =
		stringDesdeNull(notas)

	return pago, nil
}

func tipoDescuentoDesdeNull(
	valor sql.NullString,
) *TipoDescuento {
	if !valor.Valid {
		return nil
	}

	resultado :=
		TipoDescuento(valor.String)

	return &resultado
}

func stringDesdeNull(
	valor sql.NullString,
) *string {
	if !valor.Valid {
		return nil
	}

	resultado := valor.String

	return &resultado
}

func timeDesdeNull(
	valor sql.NullTime,
) *time.Time {
	if !valor.Valid {
		return nil
	}

	resultado := valor.Time

	return &resultado
}

func valorInt64Opcional(
	valor *int64,
) any {
	if valor == nil {
		return nil
	}

	return *valor
}

func valorStringOpcional(
	valor *string,
) any {
	if valor == nil {
		return nil
	}

	return *valor
}
