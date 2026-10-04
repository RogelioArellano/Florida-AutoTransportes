package reservas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var _ ReembolsoStore = (*PostgresRepository)(nil)

type datosReservaReembolso struct {
	Estado Estado

	CancelacionReembolsable   bool
	MontoReembolsableCentavos int64
	MontoReembolsadoCentavos  int64
}

// RegisterRefund registra una salida real de dinero.
//
// La reserva se bloquea con FOR UPDATE antes de consultar los
// reembolsos aplicados. De esta manera dos solicitudes
// simultáneas no pueden utilizar el mismo saldo disponible.
func (r *PostgresRepository) RegisterRefund(
	ctx context.Context,
	input RegistrarReembolsoInput,
) (ReembolsosReserva, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ReembolsosReserva{}, fmt.Errorf(
			"iniciar transacción de reembolso: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	datos, err := consultarReservaParaReembolso(
		ctx,
		tx,
		input.ReservaID,
	)
	if err != nil {
		return ReembolsosReserva{}, err
	}

	if datos.Estado != EstadoCancelada {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: estado %s",
			ErrReservaNoAceptaReembolso,
			datos.Estado,
		)
	}

	if !datos.CancelacionReembolsable ||
		datos.MontoReembolsableCentavos <= 0 {
		return ReembolsosReserva{},
			ErrReservaNoReembolsable
	}

	saldoCentavos :=
		datos.MontoReembolsableCentavos -
			datos.MontoReembolsadoCentavos

	if saldoCentavos <= 0 {
		return ReembolsosReserva{},
			ErrReservaSinSaldoPorReembolsar
	}

	montoCentavos, err := parsearDecimal(
		string(input.Monto),
	)
	if err != nil || montoCentavos <= 0 {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: el monto del reembolso no es válido",
			ErrDatosInvalidos,
		)
	}

	if montoCentavos > saldoCentavos {
		return ReembolsosReserva{}, fmt.Errorf(
			"%w: saldo disponible %s, reembolso solicitado %s",
			ErrReembolsoExcedeSaldo,
			formatearDecimal(saldoCentavos),
			formatearDecimal(montoCentavos),
		)
	}

	if err := insertarReembolso(
		ctx,
		tx,
		input,
	); err != nil {
		return ReembolsosReserva{}, err
	}

	resultado, err := obtenerReembolsosReserva(
		ctx,
		tx,
		input.ReservaID,
	)
	if err != nil {
		return ReembolsosReserva{}, fmt.Errorf(
			"consultar reserva después del reembolso: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return ReembolsosReserva{}, fmt.Errorf(
			"confirmar transacción de reembolso: %w",
			err,
		)
	}

	return resultado, nil
}

// GetRefundsByReservation consulta el resumen y el historial
// completo de reembolsos sin abrir una transacción.
func (r *PostgresRepository) GetRefundsByReservation(
	ctx context.Context,
	reservaID int64,
) (ReembolsosReserva, error) {
	resultado, err := obtenerReembolsosReserva(
		ctx,
		r.pool,
		reservaID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReembolsosReserva{},
			ErrReservaNoEncontrada
	}

	if err != nil {
		return ReembolsosReserva{}, fmt.Errorf(
			"consultar reembolsos de reserva: %w",
			err,
		)
	}

	return resultado, nil
}

// consultarReservaParaReembolso bloquea primero la reserva y
// posteriormente suma los reembolsos aplicados.
//
// Es importante que sean dos sentencias SQL. Cuando una
// solicitud espera el bloqueo de otra, la segunda consulta ya
// puede ver los movimientos confirmados por la transacción que
// liberó el bloqueo.
func consultarReservaParaReembolso(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
) (datosReservaReembolso, error) {
	const bloquearReserva = `
		SELECT
			estado,
			cancelacion_reembolsable,
			monto_reembolsable::NUMERIC(12, 2)::TEXT
		FROM reservas
		WHERE id = $1
		FOR UPDATE;
	`

	var datos datosReservaReembolso
	var estadoTexto string
	var montoReembolsableTexto string

	err := tx.QueryRow(
		ctx,
		bloquearReserva,
		reservaID,
	).Scan(
		&estadoTexto,
		&datos.CancelacionReembolsable,
		&montoReembolsableTexto,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return datosReservaReembolso{},
			ErrReservaNoEncontrada
	}

	if err != nil {
		return datosReservaReembolso{}, fmt.Errorf(
			"consultar reserva para reembolso: %w",
			err,
		)
	}

	montoReembolsable, err := parsearDecimal(
		montoReembolsableTexto,
	)
	if err != nil {
		return datosReservaReembolso{}, fmt.Errorf(
			"interpretar monto reembolsable: %w",
			err,
		)
	}

	// Esta consulta se ejecuta después de adquirir el bloqueo.
	// Así puede ver cualquier reembolso confirmado mientras
	// esta transacción esperaba.
	const consultarReembolsado = `
		SELECT
			COALESCE(
				SUM(monto) FILTER (
					WHERE estado = 'APLICADO'
				),
				0
			)::NUMERIC(12, 2)::TEXT
		FROM reembolsos_reserva
		WHERE reserva_id = $1;
	`

	var montoReembolsadoTexto string

	err = tx.QueryRow(
		ctx,
		consultarReembolsado,
		reservaID,
	).Scan(&montoReembolsadoTexto)
	if err != nil {
		return datosReservaReembolso{}, fmt.Errorf(
			"consultar reembolsos aplicados: %w",
			err,
		)
	}

	montoReembolsado, err := parsearDecimal(
		montoReembolsadoTexto,
	)
	if err != nil {
		return datosReservaReembolso{}, fmt.Errorf(
			"interpretar reembolsos aplicados: %w",
			err,
		)
	}

	datos.Estado = Estado(estadoTexto)

	datos.MontoReembolsableCentavos =
		montoReembolsable

	datos.MontoReembolsadoCentavos =
		montoReembolsado

	return datos, nil
}

func insertarReembolso(
	ctx context.Context,
	tx pgx.Tx,
	input RegistrarReembolsoInput,
) error {
	const query = `
		INSERT INTO reembolsos_reserva (
			reserva_id,
			monto,
			metodo,
			referencia,
			notas
		)
		VALUES (
			$1::BIGINT,
			$2::NUMERIC(12, 2),
			$3::VARCHAR(20),
			$4::VARCHAR(150),
			$5::TEXT
		);
	`

	_, err := tx.Exec(
		ctx,
		query,
		input.ReservaID,
		string(input.Monto),
		string(input.Metodo),
		input.Referencia,
		input.Notas,
	)
	if err != nil {
		return fmt.Errorf(
			"insertar reembolso de reserva: %w",
			err,
		)
	}

	return nil
}

func obtenerReembolsosReserva(
	ctx context.Context,
	querier querier,
	reservaID int64,
) (ReembolsosReserva, error) {
	const query = `
		SELECT
			reserva_id,
			cancelacion_reembolsable,
			monto_reembolsable::NUMERIC(12, 2)::TEXT,
			monto_reembolsado::NUMERIC(12, 2)::TEXT,
			saldo_por_reembolsar::NUMERIC(12, 2)::TEXT,
			estado_reembolso
		FROM vw_reservas_reembolsos
		WHERE reserva_id = $1;
	`

	var resultado ReembolsosReserva
	var montoReembolsable string
	var montoReembolsado string
	var saldoPorReembolsar string
	var estado string

	err := querier.QueryRow(
		ctx,
		query,
		reservaID,
	).Scan(
		&resultado.ReservaID,
		&resultado.CancelacionReembolsable,
		&montoReembolsable,
		&montoReembolsado,
		&saldoPorReembolsar,
		&estado,
	)
	if err != nil {
		return ReembolsosReserva{}, err
	}

	resultado.MontoReembolsable =
		Dinero(montoReembolsable)

	resultado.MontoReembolsado =
		Dinero(montoReembolsado)

	resultado.SaldoPorReembolsar =
		Dinero(saldoPorReembolsar)

	resultado.Estado =
		EstadoReembolso(estado)

	reembolsos, err := listarReembolsosReserva(
		ctx,
		querier,
		reservaID,
	)
	if err != nil {
		return ReembolsosReserva{}, err
	}

	resultado.Reembolsos = reembolsos

	return resultado, nil
}

func listarReembolsosReserva(
	ctx context.Context,
	querier querier,
	reservaID int64,
) ([]Reembolso, error) {
	const query = `
		SELECT
			id,
			folio,
			reserva_id,
			monto::NUMERIC(12, 2)::TEXT,
			metodo,
			referencia,
			estado,
			reembolsado_en,
			anulado_en,
			motivo_anulacion,
			notas,
			creado_en,
			actualizado_en
		FROM reembolsos_reserva
		WHERE reserva_id = $1
		ORDER BY
			reembolsado_en,
			id;
	`

	rows, err := querier.Query(
		ctx,
		query,
		reservaID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar movimientos de reembolso: %w",
			err,
		)
	}
	defer rows.Close()

	reembolsos := make([]Reembolso, 0)

	for rows.Next() {
		reembolso, err := scanReembolso(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"leer movimiento de reembolso: %w",
				err,
			)
		}

		reembolsos = append(
			reembolsos,
			reembolso,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer movimientos de reembolso: %w",
			err,
		)
	}

	return reembolsos, nil
}

func scanReembolso(
	fila scanner,
) (Reembolso, error) {
	var reembolso Reembolso

	var monto string
	var metodo string
	var estado string

	var referencia sql.NullString
	var anuladoEn sql.NullTime
	var motivoAnulacion sql.NullString
	var notas sql.NullString

	err := fila.Scan(
		&reembolso.ID,
		&reembolso.Folio,
		&reembolso.ReservaID,
		&monto,
		&metodo,
		&referencia,
		&estado,
		&reembolso.ReembolsadoEn,
		&anuladoEn,
		&motivoAnulacion,
		&notas,
		&reembolso.CreadoEn,
		&reembolso.ActualizadoEn,
	)
	if err != nil {
		return Reembolso{}, err
	}

	reembolso.Monto =
		Dinero(monto)

	reembolso.Metodo =
		MetodoReembolso(metodo)

	reembolso.Estado =
		EstadoMovimientoReembolso(estado)

	reembolso.Referencia =
		stringDesdeNull(referencia)

	reembolso.AnuladoEn =
		timeDesdeNull(anuladoEn)

	reembolso.MotivoAnulacion =
		stringDesdeNull(motivoAnulacion)

	reembolso.Notas =
		stringDesdeNull(notas)

	return reembolso, nil
}
