package reservas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var _ ListaPasajerosStore = (*PostgresRepository)(nil)

// GetPassengerList obtiene el resumen y las reservas no
// canceladas de una corrida.
//
// Se utiliza una transacción de solo lectura con aislamiento
// REPEATABLE READ para que el resumen y el detalle representen
// el mismo instante, incluso si se registra o cancela una
// reserva mientras se genera la lista.
func (r *PostgresRepository) GetPassengerList(
	ctx context.Context,
	corridaID int64,
) (ListaPasajerosCorrida, error) {
	tx, err := r.pool.BeginTx(
		ctx,
		pgx.TxOptions{
			IsoLevel:   pgx.RepeatableRead,
			AccessMode: pgx.ReadOnly,
		},
	)
	if err != nil {
		return ListaPasajerosCorrida{}, fmt.Errorf(
			"iniciar consulta de lista de pasajeros: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	lista, err := consultarResumenListaPasajeros(
		ctx,
		tx,
		corridaID,
	)
	if err != nil {
		return ListaPasajerosCorrida{}, err
	}

	reservas, err := consultarReservasListaPasajeros(
		ctx,
		tx,
		corridaID,
	)
	if err != nil {
		return ListaPasajerosCorrida{}, err
	}

	lista.Reservas = reservas

	if err := tx.Commit(ctx); err != nil {
		return ListaPasajerosCorrida{}, fmt.Errorf(
			"finalizar consulta de lista de pasajeros: %w",
			err,
		)
	}

	return lista, nil
}

// consultarResumenListaPasajeros obtiene la información de la
// corrida, los acumulados financieros y la ocupación máxima
// simultánea de cualquiera de sus segmentos.
func consultarResumenListaPasajeros(
	ctx context.Context,
	tx pgx.Tx,
	corridaID int64,
) (ListaPasajerosCorrida, error) {
	const query = `
		WITH resumen AS (
			SELECT
				COUNT(*)::BIGINT AS total_reservas,

				COALESCE(
					SUM(r.cantidad_pasajeros),
					0
				)::BIGINT AS total_pasajeros,

				COALESCE(
					SUM(r.total),
					0
				)::NUMERIC(12, 2)::TEXT AS total_vendido,

				COALESCE(
					SUM(s.monto_pagado),
					0
				)::NUMERIC(12, 2)::TEXT AS total_cobrado,

				COALESCE(
					SUM(s.saldo_pendiente),
					0
				)::NUMERIC(12, 2)::TEXT AS total_por_cobrar

			FROM reservas r

			INNER JOIN vw_reservas_saldos s
				ON s.reserva_id = r.id

			WHERE r.corrida_id = $1
				AND r.estado <> 'CANCELADA'
		),

		ocupacion AS (
			SELECT
				COALESCE(
					MAX(segmentos.ocupados),
					0
				)::BIGINT AS ocupacion_maxima

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
								origen
								ON origen.id =
									r.corrida_parada_origen_id
								AND origen.corrida_id =
									r.corrida_id

							INNER JOIN corrida_paradas
								destino
								ON destino.id =
									r.corrida_parada_destino_id
								AND destino.corrida_id =
									r.corrida_id

							WHERE r.corrida_id = $1
								AND r.estado IN (
									'APARTADA',
									'CONFIRMADA',
									'ABORDADA'
								)
								AND origen.orden <=
									segmento.orden
								AND destino.orden >
									segmento.orden
						),
						0
					)::BIGINT AS ocupados

				FROM corrida_paradas segmento
				WHERE segmento.corrida_id = $1
			) AS segmentos
		)

		SELECT
			c.id,
			c.folio,
			c.ruta_id,
			c.ruta_codigo,
			c.ruta_nombre,
			TO_CHAR(
				c.fecha_servicio,
				'YYYY-MM-DD'
			),
			c.salida_programada,
			c.llegada_estimada,
			c.estado,

			c.unidad_id,
			c.unidad_codigo,
			u.placas,

			c.chofer_id,
			c.chofer_nombre,
			ch.telefono,

			c.capacidad_pasajeros,

			resumen.total_reservas,
			resumen.total_pasajeros,
			ocupacion.ocupacion_maxima,

			resumen.total_vendido,
			resumen.total_cobrado,
			resumen.total_por_cobrar

		FROM corridas c

		INNER JOIN unidades u
			ON u.id = c.unidad_id

		LEFT JOIN choferes ch
			ON ch.id = c.chofer_id

		CROSS JOIN resumen
		CROSS JOIN ocupacion

		WHERE c.id = $1;
	`

	var lista ListaPasajerosCorrida

	var placas sql.NullString
	var choferID sql.NullInt64
	var choferNombre sql.NullString
	var choferTelefono sql.NullString

	var capacidad int16
	var totalReservas int64
	var totalPasajeros int64
	var ocupacionMaxima int64

	var totalVendido string
	var totalCobrado string
	var totalPorCobrar string

	err := tx.QueryRow(
		ctx,
		query,
		corridaID,
	).Scan(
		&lista.CorridaID,
		&lista.CorridaFolio,
		&lista.RutaID,
		&lista.RutaCodigo,
		&lista.RutaNombre,
		&lista.FechaServicio,
		&lista.SalidaProgramada,
		&lista.LlegadaEstimada,
		&lista.EstadoCorrida,

		&lista.UnidadID,
		&lista.UnidadCodigo,
		&placas,

		&choferID,
		&choferNombre,
		&choferTelefono,

		&capacidad,

		&totalReservas,
		&totalPasajeros,
		&ocupacionMaxima,

		&totalVendido,
		&totalCobrado,
		&totalPorCobrar,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ListaPasajerosCorrida{},
			ErrCorridaNoEncontrada
	}

	if err != nil {
		return ListaPasajerosCorrida{}, fmt.Errorf(
			"consultar resumen de lista de pasajeros: %w",
			err,
		)
	}

	lista.UnidadPlacas =
		stringDesdeNull(placas)

	if choferID.Valid {
		valor := choferID.Int64
		lista.ChoferID = &valor
	}

	lista.ChoferNombre =
		stringDesdeNull(choferNombre)

	lista.ChoferTelefono =
		stringDesdeNull(choferTelefono)

	lista.CapacidadPasajeros =
		int(capacidad)

	lista.TotalReservas =
		int(totalReservas)

	lista.TotalPasajerosRegistrados =
		int(totalPasajeros)

	lista.OcupacionMaxima =
		int(ocupacionMaxima)

	lista.LugaresDisponiblesMinimos =
		lista.CapacidadPasajeros -
			lista.OcupacionMaxima

	lista.TotalVendido =
		Dinero(totalVendido)

	lista.TotalCobrado =
		Dinero(totalCobrado)

	lista.TotalPorCobrar =
		Dinero(totalPorCobrar)

	return lista, nil
}

// consultarReservasListaPasajeros obtiene una fila por reserva.
//
// Las canceladas se excluyen. Las reservas APARTADA,
// CONFIRMADA, ABORDADA y NO_PRESENTADA permanecen disponibles
// para consulta operativa e histórica.
func consultarReservasListaPasajeros(
	ctx context.Context,
	tx pgx.Tx,
	corridaID int64,
) ([]ReservaListaPasajeros, error) {
	const query = `
		SELECT
			r.id,
			r.folio,

			p.id,
			p.nombre_completo,
			p.telefono,

			origen.id,
			origen.punto_nombre,
			origen.orden,

			destino.id,
			destino.punto_nombre,
			destino.orden,

			r.cantidad_pasajeros,
			r.estado,
			r.requiere_confirmacion,

			r.total::NUMERIC(12, 2)::TEXT,
			s.monto_pagado::NUMERIC(12, 2)::TEXT,
			s.saldo_pendiente::NUMERIC(12, 2)::TEXT,
			s.estado_pago,

			r.observaciones

		FROM reservas r

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

		INNER JOIN vw_reservas_saldos s
			ON s.reserva_id = r.id

		WHERE r.corrida_id = $1
			AND r.estado <> 'CANCELADA'

		ORDER BY
			origen.orden,
			LOWER(p.nombre_completo),
			r.id;
	`

	rows, err := tx.Query(
		ctx,
		query,
		corridaID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar reservas de lista de pasajeros: %w",
			err,
		)
	}
	defer rows.Close()

	reservas := make(
		[]ReservaListaPasajeros,
		0,
	)

	for rows.Next() {
		var reserva ReservaListaPasajeros

		var ordenOrigen int16
		var ordenDestino int16
		var cantidadPasajeros int16

		var estadoReserva string
		var estadoPago string

		var total string
		var montoPagado string
		var saldoPendiente string

		var observaciones sql.NullString

		err := rows.Scan(
			&reserva.ReservaID,
			&reserva.ReservaFolio,

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
			&estadoReserva,
			&reserva.RequiereConfirmacion,

			&total,
			&montoPagado,
			&saldoPendiente,
			&estadoPago,

			&observaciones,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"leer reserva de lista de pasajeros: %w",
				err,
			)
		}

		reserva.OrdenOrigen =
			int(ordenOrigen)

		reserva.OrdenDestino =
			int(ordenDestino)

		reserva.CantidadPasajeros =
			int(cantidadPasajeros)

		reserva.EstadoReserva =
			Estado(estadoReserva)

		reserva.Total =
			Dinero(total)

		reserva.MontoPagado =
			Dinero(montoPagado)

		reserva.SaldoPendiente =
			Dinero(saldoPendiente)

		reserva.EstadoPago =
			EstadoPago(estadoPago)

		reserva.Observaciones =
			stringDesdeNull(observaciones)

		reservas = append(
			reservas,
			reserva,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer reservas de lista de pasajeros: %w",
			err,
		)
	}

	return reservas, nil
}
