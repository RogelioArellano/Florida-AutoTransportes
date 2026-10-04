package reservas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var _ ConfirmacionVencimientoStore = (*PostgresRepository)(nil)

type datosVencimientoConfirmacion struct {
	Estado                   Estado
	RequiereConfirmacion     bool
	ConfirmacionSolicitadaEn sql.NullTime
	ConfirmacionLimiteEn     sql.NullTime

	CorridaID       int64
	ParadaOrigenID  int64
	ParadaDestinoID int64
	EstadoCorrida   string

	MotivoCancelacion sql.NullString
	TienePagos        bool
	PlazoVencido      bool
}

// ListExpiredConfirmations obtiene las solicitudes cuyo
// límite de respuesta ya fue alcanzado y que todavía pueden
// cancelarse por falta de confirmación.
func (r *PostgresRepository) ListExpiredConfirmations(
	ctx context.Context,
) ([]ConfirmacionPendiente, error) {
	query := consultaConfirmacionPendienteBase + `
		WHERE r.estado = 'APARTADA'
			AND r.requiere_confirmacion = TRUE
			AND r.confirmacion_solicitada_en IS NOT NULL
			AND r.confirmacion_limite_en IS NOT NULL
			AND r.confirmacion_limite_en <= NOW()

			AND c.estado IN (
				'PROGRAMADA',
				'ABORDANDO'
			)

			AND NOT EXISTS (
				SELECT 1
				FROM pagos_reserva pr
				WHERE pr.reserva_id = r.id
					AND pr.estado = 'APLICADO'
			)

		ORDER BY
			r.confirmacion_limite_en,
			r.id;
	`

	rows, err := r.pool.Query(
		ctx,
		query,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar confirmaciones vencidas: %w",
			err,
		)
	}
	defer rows.Close()

	vencidas := make(
		[]ConfirmacionPendiente,
		0,
	)

	for rows.Next() {
		vencida, err :=
			scanConfirmacionPendiente(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"leer confirmación vencida: %w",
				err,
			)
		}

		vencidas = append(
			vencidas,
			vencida,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer confirmaciones vencidas: %w",
			err,
		)
	}

	return vencidas, nil
}

// ExpireConfirmation cancela una reserva que no fue
// confirmada dentro del límite permitido.
//
// La reserva se bloquea antes de comprobar el estado y el
// plazo. Así se serializa esta operación con pagos,
// confirmaciones y cancelaciones manuales.
func (r *PostgresRepository) ExpireConfirmation(
	ctx context.Context,
	params VencerConfirmacionParams,
) (Reserva, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Reserva{}, fmt.Errorf(
			"iniciar transacción de vencimiento de confirmación: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	datos, err :=
		consultarReservaParaVencimientoConfirmacion(
			ctx,
			tx,
			params.Input.ReservaID,
		)
	if err != nil {
		return Reserva{}, err
	}

	// Una repetición de la misma cancelación automática es
	// idempotente. Se devuelve la reserva sin modificar la
	// fecha ni el motivo original.
	if datos.Estado == EstadoCancelada &&
		datos.MotivoCancelacion.Valid &&
		datos.MotivoCancelacion.String ==
			params.MotivoCancelacion {
		reserva, err := obtenerReservaPorID(
			ctx,
			tx,
			params.Input.ReservaID,
		)
		if err != nil {
			return Reserva{}, fmt.Errorf(
				"consultar reserva con confirmación vencida: %w",
				err,
			)
		}

		if err := tx.Commit(ctx); err != nil {
			return Reserva{}, fmt.Errorf(
				"confirmar consulta de vencimiento existente: %w",
				err,
			)
		}

		return reserva, nil
	}

	if datos.Estado != EstadoApartada ||
		!datos.RequiereConfirmacion ||
		!datos.ConfirmacionSolicitadaEn.Valid ||
		!datos.ConfirmacionLimiteEn.Valid ||
		(datos.EstadoCorrida != "PROGRAMADA" && datos.EstadoCorrida != "ABORDANDO") ||
		datos.TienePagos {
		return Reserva{}, fmt.Errorf(
			"%w: estado de reserva %s",
			ErrReservaNoAceptaVencimientoConfirmacion,
			datos.Estado,
		)
	}

	if !datos.PlazoVencido {
		return Reserva{},
			ErrSolicitudConfirmacionNoVencida
	}

	if err :=
		actualizarReservaCanceladaPorConfirmacionVencida(
			ctx,
			tx,
			params.Input.ReservaID,
			params.MotivoCancelacion,
		); err != nil {
		return Reserva{}, err
	}

	// Al cancelar se libera el cupo y se comprueba si las
	// paradas opcionales todavía son necesarias para otras
	// reservas activas de la corrida.
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
			"consultar reserva después del vencimiento: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Reserva{}, fmt.Errorf(
			"confirmar vencimiento de confirmación: %w",
			err,
		)
	}

	return reserva, nil
}

func consultarReservaParaVencimientoConfirmacion(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
) (datosVencimientoConfirmacion, error) {
	const query = `
		SELECT
			r.estado,
			r.requiere_confirmacion,
			r.confirmacion_solicitada_en,
			r.confirmacion_limite_en,

			r.corrida_id,
			r.corrida_parada_origen_id,
			r.corrida_parada_destino_id,
			c.estado,

			r.motivo_cancelacion,

			EXISTS (
				SELECT 1
				FROM pagos_reserva pr
				WHERE pr.reserva_id = r.id
					AND pr.estado = 'APLICADO'
			),

			COALESCE(
				r.confirmacion_limite_en <= NOW(),
				FALSE
			)

		FROM reservas r

		INNER JOIN corridas c
			ON c.id = r.corrida_id

		WHERE r.id = $1

		FOR UPDATE OF r;
	`

	var datos datosVencimientoConfirmacion
	var estadoReserva string

	err := tx.QueryRow(
		ctx,
		query,
		reservaID,
	).Scan(
		&estadoReserva,
		&datos.RequiereConfirmacion,
		&datos.ConfirmacionSolicitadaEn,
		&datos.ConfirmacionLimiteEn,
		&datos.CorridaID,
		&datos.ParadaOrigenID,
		&datos.ParadaDestinoID,
		&datos.EstadoCorrida,
		&datos.MotivoCancelacion,
		&datos.TienePagos,
		&datos.PlazoVencido,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return datosVencimientoConfirmacion{},
			ErrReservaNoEncontrada
	}

	if err != nil {
		return datosVencimientoConfirmacion{}, fmt.Errorf(
			"consultar reserva para vencer confirmación: %w",
			err,
		)
	}

	datos.Estado = Estado(estadoReserva)

	return datos, nil
}

func actualizarReservaCanceladaPorConfirmacionVencida(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
	motivo string,
) error {
	const query = `
		UPDATE reservas
		SET
			estado = 'CANCELADA',
			requiere_confirmacion = FALSE,
			confirmacion_solicitada_en = NULL,
			confirmacion_limite_en = NULL,

			cancelada_en = NOW(),
			motivo_cancelacion =
				$2::VARCHAR(500),

			cancelacion_reembolsable = FALSE,
			monto_reembolsable = 0,
			limite_reembolso_en = NULL,

			actualizado_en = NOW()

		WHERE id = $1;
	`

	_, err := tx.Exec(
		ctx,
		query,
		reservaID,
		motivo,
	)
	if err != nil {
		return fmt.Errorf(
			"cancelar reserva por confirmación vencida: %w",
			err,
		)
	}

	return nil
}
