package reservas

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// VoidRefund anula un reembolso capturado por error y
// restaura el importe dentro del saldo por reembolsar.
//
// El movimiento no se elimina. Conserva importe, método,
// referencia y fecha original para mantener la auditoría.
func (r *PostgresRepository) VoidRefund(
	ctx context.Context,
	input AnularReembolsoInput,
) (ReembolsosReserva, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ReembolsosReserva{}, fmt.Errorf(
			"iniciar transacción de anulación de reembolso: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	reservaID, err := consultarReservaIDDeReembolso(
		ctx,
		tx,
		input.ReembolsoID,
	)
	if err != nil {
		return ReembolsosReserva{}, err
	}

	// El registro y la anulación de reembolsos utilizan el
	// mismo bloqueo de la reserva. Esto evita que el saldo
	// cambie simultáneamente desde otra solicitud.
	if err := bloquearReservaParaAnulacionReembolso(
		ctx,
		tx,
		reservaID,
	); err != nil {
		return ReembolsosReserva{}, err
	}

	estado, err := consultarEstadoReembolsoBloqueado(
		ctx,
		tx,
		input.ReembolsoID,
		reservaID,
	)
	if err != nil {
		return ReembolsosReserva{}, err
	}

	// La operación es idempotente. Una segunda solicitud
	// devuelve el estado actual sin reemplazar el motivo ni la
	// fecha de la primera anulación.
	if estado ==
		EstadoMovimientoReembolsoAnulado {
		resultado, err :=
			obtenerReembolsosReserva(
				ctx,
				tx,
				reservaID,
			)
		if err != nil {
			return ReembolsosReserva{}, fmt.Errorf(
				"consultar reserva con reembolso anulado: %w",
				err,
			)
		}

		if err := tx.Commit(ctx); err != nil {
			return ReembolsosReserva{}, fmt.Errorf(
				"confirmar consulta de anulación: %w",
				err,
			)
		}

		return resultado, nil
	}

	if err := actualizarReembolsoAnulado(
		ctx,
		tx,
		reservaID,
		input,
	); err != nil {
		return ReembolsosReserva{}, err
	}

	resultado, err := obtenerReembolsosReserva(
		ctx,
		tx,
		reservaID,
	)
	if err != nil {
		return ReembolsosReserva{}, fmt.Errorf(
			"consultar reserva después de anular reembolso: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return ReembolsosReserva{}, fmt.Errorf(
			"confirmar anulación de reembolso: %w",
			err,
		)
	}

	return resultado, nil
}

// consultarReservaIDDeReembolso obtiene la reserva padre antes
// de adquirir su bloqueo.
//
// Los reembolsos no se eliminan físicamente, por lo que el
// identificador permanece estable.
func consultarReservaIDDeReembolso(
	ctx context.Context,
	tx pgx.Tx,
	reembolsoID int64,
) (int64, error) {
	const query = `
		SELECT reserva_id
		FROM reembolsos_reserva
		WHERE id = $1;
	`

	var reservaID int64

	err := tx.QueryRow(
		ctx,
		query,
		reembolsoID,
	).Scan(&reservaID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrReembolsoNoEncontrado
	}

	if err != nil {
		return 0, fmt.Errorf(
			"consultar reserva del reembolso: %w",
			err,
		)
	}

	return reservaID, nil
}

// bloquearReservaParaAnulacionReembolso serializa todas las
// operaciones financieras de reembolso de la misma reserva.
func bloquearReservaParaAnulacionReembolso(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
) error {
	const query = `
		SELECT id
		FROM reservas
		WHERE id = $1
		FOR UPDATE;
	`

	var id int64

	err := tx.QueryRow(
		ctx,
		query,
		reservaID,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrReservaNoEncontrada
	}

	if err != nil {
		return fmt.Errorf(
			"bloquear reserva para anular reembolso: %w",
			err,
		)
	}

	return nil
}

func consultarEstadoReembolsoBloqueado(
	ctx context.Context,
	tx pgx.Tx,
	reembolsoID int64,
	reservaID int64,
) (EstadoMovimientoReembolso, error) {
	const query = `
		SELECT estado
		FROM reembolsos_reserva
		WHERE id = $1
			AND reserva_id = $2
		FOR UPDATE;
	`

	var estado string

	err := tx.QueryRow(
		ctx,
		query,
		reembolsoID,
		reservaID,
	).Scan(&estado)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrReembolsoNoEncontrado
	}

	if err != nil {
		return "", fmt.Errorf(
			"consultar reembolso para anulación: %w",
			err,
		)
	}

	return EstadoMovimientoReembolso(estado), nil
}

func actualizarReembolsoAnulado(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
	input AnularReembolsoInput,
) error {
	const query = `
		UPDATE reembolsos_reserva
		SET
			estado = 'ANULADO',
			anulado_en = NOW(),
			motivo_anulacion = $3::VARCHAR(500),
			actualizado_en = NOW()
		WHERE id = $1
			AND reserva_id = $2
			AND estado = 'APLICADO';
	`

	resultado, err := tx.Exec(
		ctx,
		query,
		input.ReembolsoID,
		reservaID,
		input.Motivo,
	)
	if err != nil {
		return fmt.Errorf(
			"anular reembolso: %w",
			err,
		)
	}

	if resultado.RowsAffected() != 1 {
		return fmt.Errorf(
			"anular reembolso: no se actualizó el movimiento",
		)
	}

	return nil
}
