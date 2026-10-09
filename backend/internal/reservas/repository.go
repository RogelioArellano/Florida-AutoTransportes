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

// datosReservaPago contiene únicamente la información que
// necesitamos para decidir si una reserva acepta un pago.
type datosReservaPago struct {
	TotalCentavos  int64
	PagadoCentavos int64
	Estado         Estado
}

// datosReservaCancelacion contiene la información obtenida
// bajo bloqueo para decidir cómo cancelar una reserva.
type datosReservaCancelacion struct {
	Estado Estado

	CorridaID       int64
	ParadaOrigenID  int64
	ParadaDestinoID int64

	MontoPagadoCentavos int64

	PuedeCancelar     bool
	DentroLimite      bool
	LimiteReembolsoEn time.Time
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

		saldos.monto_pagado::NUMERIC(12, 2)::TEXT,
		saldos.saldo_pendiente::NUMERIC(12, 2)::TEXT,
		saldos.estado_pago,

		r.estado,
		r.requiere_confirmacion,
		r.confirmada_en,
		r.confirmacion_solicitada_en,
		r.confirmacion_limite_en,

		r.cantidad_abordada,
		r.abordada_en,
		r.asistencia_cerrada_en,
		r.observaciones_asistencia,

		r.cancelada_en,
		r.motivo_cancelacion,
		r.cancelacion_reembolsable,
		r.monto_reembolsable::NUMERIC(12, 2)::TEXT,
		r.limite_reembolso_en,

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
		if err := insertarPago(
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

// RegisterPayment agrega un abono a una reserva existente.
//
// Toda la operación se realiza dentro de una transacción.
// La reserva se bloquea con FOR UPDATE para impedir que dos
// pagos simultáneos utilicen el mismo saldo pendiente.
func (r *PostgresRepository) RegisterPayment(
	ctx context.Context,
	input RegistrarPagoInput,
) (Reserva, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Reserva{}, fmt.Errorf(
			"iniciar transacción de pago: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	datos, err := consultarReservaParaPago(
		ctx,
		tx,
		input.ReservaID,
	)
	if err != nil {
		return Reserva{}, err
	}

	montoCentavos, err := parsearDecimal(
		string(input.Monto),
	)
	if err != nil || montoCentavos <= 0 {
		return Reserva{}, fmt.Errorf(
			"%w: el monto del pago no es válido",
			ErrDatosInvalidos,
		)
	}

	saldoCentavos :=
		datos.TotalCentavos -
			datos.PagadoCentavos

	if saldoCentavos <= 0 {
		return Reserva{}, ErrReservaSinSaldo
	}

	if montoCentavos > saldoCentavos {
		return Reserva{}, fmt.Errorf(
			"%w: saldo pendiente %s, pago recibido %s",
			ErrPagoExcedeSaldo,
			formatearDecimal(saldoCentavos),
			formatearDecimal(montoCentavos),
		)
	}

	if err := insertarPago(
		ctx,
		tx,
		input.ReservaID,
		input.PagoInput,
	); err != nil {
		return Reserva{}, err
	}

	// El primer pago confirma automáticamente una reserva
	// que había sido apartada sin anticipo.
	if datos.Estado == EstadoApartada {
		if err := confirmarReservaPorPago(
			ctx,
			tx,
			input.ReservaID,
		); err != nil {
			return Reserva{}, err
		}
	}

	reservaActualizada, err :=
		obtenerReservaPorID(
			ctx,
			tx,
			input.ReservaID,
		)
	if err != nil {
		return Reserva{}, fmt.Errorf(
			"consultar reserva después del pago: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Reserva{}, fmt.Errorf(
			"confirmar transacción de pago: %w",
			err,
		)
	}

	return reservaActualizada, nil
}

// Confirm confirma manualmente una reserva.
//
// La operación es idempotente: confirmar nuevamente una
// reserva que ya está CONFIRMADA no genera error ni modifica
// incorrectamente la fecha original de confirmación.
func (r *PostgresRepository) Confirm(
	ctx context.Context,
	input ConfirmarInput,
) (Reserva, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Reserva{}, fmt.Errorf(
			"iniciar transacción de confirmación: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	estado, err := consultarEstadoReservaBloqueado(
		ctx,
		tx,
		input.ReservaID,
	)
	if err != nil {
		return Reserva{}, err
	}

	switch estado {
	case EstadoApartada:
		if err := actualizarReservaConfirmada(
			ctx,
			tx,
			input.ReservaID,
		); err != nil {
			return Reserva{}, err
		}

	case EstadoConfirmada:
		// La operación es idempotente. Si una automatización
		// repite la solicitud, devolvemos la misma reserva.

	default:
		return Reserva{}, fmt.Errorf(
			"%w: estado %s",
			ErrReservaNoAceptaConfirmacion,
			estado,
		)
	}

	reserva, err := obtenerReservaPorID(
		ctx,
		tx,
		input.ReservaID,
	)
	if err != nil {
		return Reserva{}, fmt.Errorf(
			"consultar reserva confirmada: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Reserva{}, fmt.Errorf(
			"confirmar transacción de confirmación: %w",
			err,
		)
	}

	return reserva, nil
}

// Cancel cancela una reserva y libera sus lugares.
//
// La reserva se bloquea para impedir que un pago y una
// cancelación sean procesados simultáneamente.
func (r *PostgresRepository) Cancel(
	ctx context.Context,
	params CancelParams,
) (Reserva, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Reserva{}, fmt.Errorf(
			"iniciar transacción de cancelación: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	datos, err := consultarReservaParaCancelacion(
		ctx,
		tx,
		params.Input.ReservaID,
		params.HorasLimiteReembolso,
	)
	if err != nil {
		return Reserva{}, err
	}

	// La cancelación es idempotente. Si n8n o el panel
	// repiten la solicitud, devolvemos el estado existente.
	if datos.Estado == EstadoCancelada {
		reserva, err := obtenerReservaPorID(
			ctx,
			tx,
			params.Input.ReservaID,
		)
		if err != nil {
			return Reserva{}, fmt.Errorf(
				"consultar reserva cancelada: %w",
				err,
			)
		}

		if err := tx.Commit(ctx); err != nil {
			return Reserva{}, fmt.Errorf(
				"confirmar consulta de cancelación: %w",
				err,
			)
		}

		return reserva, nil
	}

	switch datos.Estado {
	case EstadoApartada,
		EstadoConfirmada:
		// Estados permitidos.

	default:
		return Reserva{}, fmt.Errorf(
			"%w: estado %s",
			ErrReservaNoAceptaCancelacion,
			datos.Estado,
		)
	}

	if !datos.PuedeCancelar {
		return Reserva{},
			ErrSalidaYaIniciada
	}

	esReembolsable :=
		datos.MontoPagadoCentavos > 0 &&
			datos.DentroLimite

	montoReembolsableCentavos := int64(0)

	if esReembolsable {
		montoReembolsableCentavos =
			datos.MontoPagadoCentavos
	}

	if err := actualizarReservaCancelada(
		ctx,
		tx,
		params.Input,
		esReembolsable,
		montoReembolsableCentavos,
		datos.LimiteReembolsoEn,
	); err != nil {
		return Reserva{}, err
	}

	// Si el origen o destino era una parada bajo demanda,
	// comprobamos si todavía existe alguna reserva activa
	// que necesite esa parada.
	if err := recalcularParadasBajoDemanda(
		ctx,
		tx,
		datos.CorridaID,
		datos.ParadaOrigenID,
		datos.ParadaDestinoID,
	); err != nil {
		return Reserva{}, err
	}

	reserva, err := obtenerReservaPorID(
		ctx,
		tx,
		params.Input.ReservaID,
	)
	if err != nil {
		return Reserva{}, fmt.Errorf(
			"consultar reserva cancelada: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Reserva{}, fmt.Errorf(
			"confirmar transacción de cancelación: %w",
			err,
		)
	}

	return reserva, nil
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

// GetByID consulta una reserva y carga todos sus pagos.
//
// Reutiliza obtenerReservaPorID, que acepta cualquier valor
// que implemente la interfaz querier. El pool de PostgreSQL
// implementa Query y QueryRow, por lo que puede utilizarse
// directamente sin abrir una transacción.
func (r *PostgresRepository) GetByID(
	ctx context.Context,
	reservaID int64,
) (Reserva, error) {
	reserva, err := obtenerReservaPorID(
		ctx,
		r.pool,
		reservaID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Reserva{},
			ErrReservaNoEncontrada
	}

	if err != nil {
		return Reserva{}, fmt.Errorf(
			"consultar detalle de reserva: %w",
			err,
		)
	}

	return reserva, nil
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
// Antes del cierre se conserva todo el cupo reservado. Una asistencia
// parcial cerrada ocupa únicamente los pasajes que sí abordaron.
// La asistencia histórica desconocida conserva el cupo original.
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
								CASE WHEN r.estado = 'ABORDADA'
									AND r.asistencia_cerrada_en IS NOT NULL
									THEN COALESCE(r.cantidad_abordada, r.cantidad_pasajeros)
									ELSE r.cantidad_pasajeros END
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
			$1::BIGINT,
			$2::BIGINT,
			$3::BIGINT,
			$4::BIGINT,
			$5::SMALLINT,
			$6::NUMERIC(12, 2),
			$7::NUMERIC(12, 2),
			$8::VARCHAR(20),
			$9::NUMERIC(12, 2),
			$10::SMALLINT,
			$11::NUMERIC(12, 2),
			$12::VARCHAR(250),
			$13::NUMERIC(12, 2),
			$14::VARCHAR(30),
			$15::BOOLEAN,
			CASE
				WHEN $14::VARCHAR(30) = 'CONFIRMADA'
					THEN NOW()
				ELSE NULL
			END,
			$16::TEXT
		)
		RETURNING id;
	`

	var tipoDescuento any
	valorDescuento := "0.00"
	cantidadPasajesDescuento := 0
	var descripcionDescuento any

	if params.Input.Descuento != nil {
		tipoDescuento =
			string(params.Input.Descuento.Tipo)

		valorDescuento =
			params.Input.Descuento.Valor

		cantidadPasajesDescuento =
			params.Input.Descuento.CantidadPasajes

		descripcionDescuento =
			params.Input.Descuento.Descripcion
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

// consultarReservaParaPago bloquea la reserva y obtiene
// su total, sus pagos aplicados y su estado operativo.
func consultarReservaParaPago(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
) (datosReservaPago, error) {
	const bloquearReserva = `
		SELECT
			total::NUMERIC(12, 2)::TEXT,
			estado
		FROM reservas
		WHERE id = $1
		FOR UPDATE;
	`

	var totalTexto string
	var estadoTexto string

	err := tx.QueryRow(
		ctx,
		bloquearReserva,
		reservaID,
	).Scan(
		&totalTexto,
		&estadoTexto,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return datosReservaPago{},
			ErrReservaNoEncontrada
	}

	if err != nil {
		return datosReservaPago{}, fmt.Errorf(
			"consultar reserva para pago: %w",
			err,
		)
	}

	estado := Estado(estadoTexto)

	// ABORDADA también acepta pagos porque un pasajero
	// podría liquidar su adeudo después de realizar el viaje.
	switch estado {
	case EstadoApartada,
		EstadoConfirmada,
		EstadoAbordada:
		// Estado válido para recibir pagos.

	default:
		return datosReservaPago{},
			fmt.Errorf(
				"%w: estado %s",
				ErrReservaNoAceptaPagos,
				estado,
			)
	}

	totalCentavos, err :=
		parsearDecimal(totalTexto)
	if err != nil {
		return datosReservaPago{}, fmt.Errorf(
			"interpretar total de la reserva: %w",
			err,
		)
	}

	const consultarPagado = `
		SELECT
			COALESCE(
				SUM(monto),
				0
			)::NUMERIC(12, 2)::TEXT
		FROM pagos_reserva
		WHERE reserva_id = $1
			AND estado = 'APLICADO';
	`

	var pagadoTexto string

	err = tx.QueryRow(
		ctx,
		consultarPagado,
		reservaID,
	).Scan(&pagadoTexto)
	if err != nil {
		return datosReservaPago{}, fmt.Errorf(
			"consultar pagos aplicados: %w",
			err,
		)
	}

	pagadoCentavos, err :=
		parsearDecimal(pagadoTexto)
	if err != nil {
		return datosReservaPago{}, fmt.Errorf(
			"interpretar pagos aplicados: %w",
			err,
		)
	}

	return datosReservaPago{
		TotalCentavos:  totalCentavos,
		PagadoCentavos: pagadoCentavos,
		Estado:         estado,
	}, nil
}

// confirmarReservaPorPago confirma una reserva que había
// sido apartada sin anticipo.
func confirmarReservaPorPago(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
) error {
	const query = `
		UPDATE reservas
		SET
			estado = 'CONFIRMADA',
			requiere_confirmacion = FALSE,
			confirmada_en = COALESCE(
				confirmada_en,
				NOW()
			),
			confirmacion_solicitada_en = NULL,
			confirmacion_limite_en = NULL,
			actualizado_en = NOW()
		WHERE id = $1
			AND estado = 'APARTADA';
	`

	_, err := tx.Exec(
		ctx,
		query,
		reservaID,
	)
	if err != nil {
		return fmt.Errorf(
			"confirmar reserva mediante pago: %w",
			err,
		)
	}

	return nil
}

// insertarPago registra un movimiento financiero aplicado
// a una reserva.
//
// Se utiliza tanto para el anticipo inicial como para los
// abonos posteriores.
func insertarPago(
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
			$1::BIGINT,
			$2::NUMERIC(12, 2),
			$3::VARCHAR(30),
			$4::VARCHAR(150),
			$5::TEXT
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
			"insertar pago de reserva: %w",
			err,
		)
	}

	return nil
}

// consultarEstadoReservaBloqueado obtiene y bloquea una
// reserva para evitar cambios simultáneos de estado.
func consultarEstadoReservaBloqueado(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
) (Estado, error) {
	const query = `
		SELECT estado
		FROM reservas
		WHERE id = $1
		FOR UPDATE;
	`

	var estadoTexto string

	err := tx.QueryRow(
		ctx,
		query,
		reservaID,
	).Scan(&estadoTexto)
	if errors.Is(err, pgx.ErrNoRows) {
		return "",
			ErrReservaNoEncontrada
	}

	if err != nil {
		return "", fmt.Errorf(
			"consultar estado de reserva: %w",
			err,
		)
	}

	return Estado(estadoTexto), nil
}

// actualizarReservaConfirmada realiza el cambio operativo.
//
// No registra un pago. Por eso una reserva puede quedar
// CONFIRMADA y continuar con estado_pago SIN_PAGO.
func actualizarReservaConfirmada(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
) error {
	const query = `
		UPDATE reservas
		SET
			estado = 'CONFIRMADA',
			requiere_confirmacion = FALSE,
			confirmada_en = COALESCE(
				confirmada_en,
				NOW()
			),
			confirmacion_solicitada_en = NULL,
			confirmacion_limite_en = NULL,
			actualizado_en = NOW()
		WHERE id = $1;
	`

	_, err := tx.Exec(
		ctx,
		query,
		reservaID,
	)
	if err != nil {
		return fmt.Errorf(
			"actualizar reserva confirmada: %w",
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

	var cantidadAbordada sql.NullInt64
	var abordadaEn sql.NullTime
	var asistenciaCerradaEn sql.NullTime
	var observacionesAsistencia sql.NullString

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

	var cancelacionReembolsable bool
	var montoReembolsable string
	var limiteReembolsoEn sql.NullTime

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

		&cantidadAbordada,
		&abordadaEn,
		&asistenciaCerradaEn,
		&observacionesAsistencia,

		&canceladaEn,
		&motivoCancelacion,
		&cancelacionReembolsable,
		&montoReembolsable,
		&limiteReembolsoEn,

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

	reserva.CancelacionReembolsable =
		cancelacionReembolsable

	reserva.MontoReembolsable =
		Dinero(montoReembolsable)

	reserva.LimiteReembolsoEn =
		timeDesdeNull(limiteReembolsoEn)

	reserva.Observaciones =
		stringDesdeNull(observaciones)

	reserva.DetalleAsistencia = detalleAsistenciaDesdeNull(
		cantidadAbordada, abordadaEn, asistenciaCerradaEn, observacionesAsistencia,
	)

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

// consultarReservaParaCancelacion bloquea la reserva y
// calcula la política utilizando la hora de PostgreSQL.
//
// Esto evita depender del reloj o zona horaria del servidor
// donde esté ejecutándose la API.
func consultarReservaParaCancelacion(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
	horasLimite int,
) (datosReservaCancelacion, error) {
	const query = `
		SELECT
			r.estado,
			r.corrida_id,
			r.corrida_parada_origen_id,
			r.corrida_parada_destino_id,

			COALESCE(
				(
					SELECT SUM(pr.monto)
					FROM pagos_reserva pr
					WHERE pr.reserva_id = r.id
						AND pr.estado = 'APLICADO'
				),
				0
			)::NUMERIC(12, 2)::TEXT,

			NOW() < c.salida_programada,

			NOW() <= (
				c.salida_programada
				- (
					$2::INTEGER
					* INTERVAL '1 hour'
				)
			),

			c.salida_programada
				- (
					$2::INTEGER
					* INTERVAL '1 hour'
				)

		FROM reservas r

		INNER JOIN corridas c
			ON c.id = r.corrida_id

		WHERE r.id = $1

		FOR UPDATE OF r;
	`

	var datos datosReservaCancelacion
	var estadoTexto string
	var montoPagadoTexto string

	err := tx.QueryRow(
		ctx,
		query,
		reservaID,
		horasLimite,
	).Scan(
		&estadoTexto,
		&datos.CorridaID,
		&datos.ParadaOrigenID,
		&datos.ParadaDestinoID,
		&montoPagadoTexto,
		&datos.PuedeCancelar,
		&datos.DentroLimite,
		&datos.LimiteReembolsoEn,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return datosReservaCancelacion{},
			ErrReservaNoEncontrada
	}

	if err != nil {
		return datosReservaCancelacion{}, fmt.Errorf(
			"consultar reserva para cancelación: %w",
			err,
		)
	}

	montoPagadoCentavos, err :=
		parsearDecimal(montoPagadoTexto)
	if err != nil {
		return datosReservaCancelacion{}, fmt.Errorf(
			"interpretar pagos de reserva: %w",
			err,
		)
	}

	datos.Estado = Estado(estadoTexto)
	datos.MontoPagadoCentavos =
		montoPagadoCentavos

	return datos, nil
}

func actualizarReservaCancelada(
	ctx context.Context,
	tx pgx.Tx,
	input CancelarInput,
	esReembolsable bool,
	montoReembolsableCentavos int64,
	limiteReembolsoEn time.Time,
) error {
	const query = `
		UPDATE reservas
		SET
			estado = 'CANCELADA',
			requiere_confirmacion = FALSE,
			confirmacion_solicitada_en = NULL,
			confirmacion_limite_en = NULL,

			cancelada_en = NOW(),
			motivo_cancelacion = $2::VARCHAR(500),

			cancelacion_reembolsable = $3::BOOLEAN,
			monto_reembolsable =
				$4::NUMERIC(12, 2),
			limite_reembolso_en = $5,

			actualizado_en = NOW()

		WHERE id = $1::BIGINT;
	`

	_, err := tx.Exec(
		ctx,
		query,
		input.ReservaID,
		input.Motivo,
		esReembolsable,
		formatearDecimal(
			montoReembolsableCentavos,
		),
		limiteReembolsoEn,
	)
	if err != nil {
		return fmt.Errorf(
			"actualizar reserva cancelada: %w",
			err,
		)
	}

	return nil
}

// recalcularParadasBajoDemanda desactiva una parada
// opcional cuando ya no existe ninguna reserva activa que
// suba o baje pasajeros en ella.
//
// Las paradas obligatorias nunca se modifican.
func recalcularParadasBajoDemanda(
	ctx context.Context,
	tx pgx.Tx,
	corridaID int64,
	origenID int64,
	destinoID int64,
) error {
	const query = `
		UPDATE corrida_paradas cp
		SET
			incluida_en_recorrido = EXISTS (
				SELECT 1
				FROM reservas r
				WHERE r.corrida_id = cp.corrida_id
					AND r.estado IN (
						'APARTADA',
						'CONFIRMADA',
						'ABORDADA'
					)
					AND (
						r.corrida_parada_origen_id =
							cp.id
						OR
						r.corrida_parada_destino_id =
							cp.id
					)
			),
			actualizado_en = NOW()
		WHERE cp.corrida_id = $1
			AND cp.id IN ($2, $3)
			AND cp.es_obligatoria = FALSE;
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
			"recalcular paradas bajo demanda: %w",
			err,
		)
	}

	return nil
}
