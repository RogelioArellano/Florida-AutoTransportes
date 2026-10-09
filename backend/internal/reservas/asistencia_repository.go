package reservas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var _ AsistenciaStore = (*PostgresRepository)(nil)

const consultaAsistenciaBase = `
	SELECT r.id, r.folio, r.corrida_id,
		r.corrida_parada_origen_id, origen.punto_nombre,
		r.estado, r.cantidad_pasajeros, r.cantidad_abordada,
		r.abordada_en, r.asistencia_cerrada_en,
		r.observaciones_asistencia
	FROM reservas r
	JOIN corrida_paradas origen
		ON origen.id = r.corrida_parada_origen_id
		AND origen.corrida_id = r.corrida_id
`

// GetAttendance consulta incluso reservas canceladas o históricas.
// El Service calcula pendientes y ausentes después de esta lectura.
func (r *PostgresRepository) GetAttendance(
	ctx context.Context, reservaID int64,
) (AsistenciaReserva, error) {
	return consultarAsistencia(ctx, r.pool, reservaID, false)
}

// RegisterBoarding guarda un TOTAL acumulado. El bloqueo serializa
// esta operación con otros abordajes, cierres, pagos y cancelaciones.
func (r *PostgresRepository) RegisterBoarding(
	ctx context.Context, input RegistrarAbordajeInput,
) (AsistenciaReserva, error) {
	if err := validarReservaIDAsistencia(input.ReservaID); err != nil {
		return AsistenciaReserva{}, err
	}
	if input.CantidadAbordada <= 0 || input.CantidadAbordada > maxCantidadPasajeros {
		return AsistenciaReserva{}, fmt.Errorf("%w: cantidad_abordada fuera de rango", ErrDatosInvalidos)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AsistenciaReserva{}, fmt.Errorf("iniciar transacción de abordaje: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	asistencia, estadoCorrida, err := bloquearAsistencia(ctx, tx, input.ReservaID)
	if err != nil {
		return AsistenciaReserva{}, err
	}
	if asistencia.CantidadAbordada == nil {
		return AsistenciaReserva{}, ErrAsistenciaHistoricaDesconocida
	}
	if !estadoReservaPermiteAsistencia(asistencia.EstadoReserva) {
		return AsistenciaReserva{}, fmt.Errorf("%w: estado %s", ErrReservaNoAceptaAbordaje, asistencia.EstadoReserva)
	}
	if input.CantidadAbordada > asistencia.CantidadPasajeros {
		return AsistenciaReserva{}, ErrCantidadAbordadaExcedeReserva
	}

	// Un reintento idéntico no cambia cantidades, fechas ni notas,
	// incluso si después se cerró la asistencia o terminó la corrida.
	if input.CantidadAbordada == *asistencia.CantidadAbordada {
		return confirmarAsistencia(ctx, tx, asistencia)
	}
	if asistencia.AsistenciaCerradaEn != nil {
		return AsistenciaReserva{}, ErrAsistenciaCerrada
	}
	if input.CantidadAbordada < *asistencia.CantidadAbordada {
		return AsistenciaReserva{}, ErrCantidadAbordadaRetrocede
	}
	if err := validarEstadoCorridaAsistencia(estadoCorrida); err != nil {
		return AsistenciaReserva{}, err
	}

	// statement_timestamp() se evalúa después de adquirir los bloqueos.
	// NOW() usaría el inicio de la transacción, que pudo pasar esperando.
	const query = `
		UPDATE reservas
		SET cantidad_abordada = $2,
			estado = 'ABORDADA',
			abordada_en = COALESCE(abordada_en, statement_timestamp()),
			requiere_confirmacion = FALSE,
			confirmacion_solicitada_en = NULL,
			confirmacion_limite_en = NULL,
			observaciones_asistencia = COALESCE($3::TEXT, observaciones_asistencia),
			actualizado_en = statement_timestamp()
		WHERE id = $1;
	`
	if _, err := tx.Exec(ctx, query, input.ReservaID, input.CantidadAbordada, input.Observaciones); err != nil {
		return AsistenciaReserva{}, fmt.Errorf("actualizar abordaje: %w", err)
	}
	asistencia, err = consultarAsistencia(ctx, tx, input.ReservaID, false)
	if err != nil {
		return AsistenciaReserva{}, err
	}
	return confirmarAsistencia(ctx, tx, asistencia)
}

// CloseAttendance cierra solamente la reserva indicada. Debe ejecutarse
// cuando se termine su abordaje en el punto de origen correspondiente.
// Si alguien abordó conserva ABORDADA; con cero cambia a NO_PRESENTADA.
func (r *PostgresRepository) CloseAttendance(
	ctx context.Context, input CerrarAsistenciaInput,
) (AsistenciaReserva, error) {
	if err := validarReservaIDAsistencia(input.ReservaID); err != nil {
		return AsistenciaReserva{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return AsistenciaReserva{}, fmt.Errorf("iniciar transacción de cierre de asistencia: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	asistencia, estadoCorrida, err := bloquearAsistencia(ctx, tx, input.ReservaID)
	if err != nil {
		return AsistenciaReserva{}, err
	}
	if asistencia.CantidadAbordada == nil {
		return AsistenciaReserva{}, ErrAsistenciaHistoricaDesconocida
	}
	if asistencia.AsistenciaCerradaEn != nil {
		return confirmarAsistencia(ctx, tx, asistencia)
	}
	if !estadoReservaPermiteAsistencia(asistencia.EstadoReserva) {
		return AsistenciaReserva{}, fmt.Errorf("%w: estado %s", ErrReservaNoAceptaCierreAsistencia, asistencia.EstadoReserva)
	}
	if err := validarEstadoCorridaAsistencia(estadoCorrida); err != nil {
		return AsistenciaReserva{}, err
	}

	const query = `
		UPDATE reservas
		SET estado = CASE WHEN cantidad_abordada > 0
				THEN 'ABORDADA' ELSE 'NO_PRESENTADA' END,
			asistencia_cerrada_en = statement_timestamp(),
			requiere_confirmacion = FALSE,
			confirmacion_solicitada_en = NULL,
			confirmacion_limite_en = NULL,
			observaciones_asistencia = COALESCE($2::TEXT, observaciones_asistencia),
			actualizado_en = statement_timestamp()
		WHERE id = $1;
	`
	if _, err := tx.Exec(ctx, query, input.ReservaID, input.Observaciones); err != nil {
		return AsistenciaReserva{}, fmt.Errorf("cerrar asistencia: %w", err)
	}
	asistencia, err = consultarAsistencia(ctx, tx, input.ReservaID, false)
	if err != nil {
		return AsistenciaReserva{}, err
	}
	return confirmarAsistencia(ctx, tx, asistencia)
}

// Primero protegemos la corrida contra cambios de estado con FOR SHARE;
// luego bloqueamos la reserva con FOR UPDATE. El bloqueo compartido permite
// trabajar con distintas reservas sin bloquear toda la corrida en exclusiva.
func bloquearAsistencia(
	ctx context.Context, tx pgx.Tx, reservaID int64,
) (AsistenciaReserva, string, error) {
	const query = `
		SELECT c.estado
		FROM corridas c
		JOIN reservas r ON r.corrida_id = c.id
		WHERE r.id = $1
		FOR SHARE OF c;
	`
	var estadoCorrida string
	if err := tx.QueryRow(ctx, query, reservaID).Scan(&estadoCorrida); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AsistenciaReserva{}, "", ErrReservaNoEncontrada
		}
		return AsistenciaReserva{}, "", fmt.Errorf("bloquear corrida para asistencia: %w", err)
	}
	asistencia, err := consultarAsistencia(ctx, tx, reservaID, true)
	return asistencia, estadoCorrida, err
}

func consultarAsistencia(
	ctx context.Context, q querier, reservaID int64, bloquear bool,
) (AsistenciaReserva, error) {
	query := consultaAsistenciaBase + " WHERE r.id = $1"
	if bloquear {
		query += " FOR UPDATE OF r"
	}
	return scanAsistencia(q.QueryRow(ctx, query, reservaID))
}

func scanAsistencia(fila scanner) (AsistenciaReserva, error) {
	var asistencia AsistenciaReserva
	var estado string
	var cantidad int16
	var abordada sql.NullInt64
	var primera, cierre sql.NullTime
	var notas sql.NullString
	err := fila.Scan(
		&asistencia.ReservaID, &asistencia.ReservaFolio, &asistencia.CorridaID,
		&asistencia.ParadaOrigenID, &asistencia.ParadaOrigenNombre,
		&estado, &cantidad, &abordada, &primera, &cierre, &notas,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AsistenciaReserva{}, ErrReservaNoEncontrada
	}
	if err != nil {
		return AsistenciaReserva{}, fmt.Errorf("leer asistencia de reserva: %w", err)
	}
	asistencia.EstadoReserva = Estado(estado)
	asistencia.CantidadPasajeros = int(cantidad)
	if abordada.Valid {
		valor := int(abordada.Int64)
		asistencia.CantidadAbordada = &valor
	}
	if primera.Valid {
		asistencia.AbordadaEn = &primera.Time
	}
	if cierre.Valid {
		asistencia.AsistenciaCerradaEn = &cierre.Time
	}
	if notas.Valid {
		asistencia.ObservacionesAsistencia = &notas.String
	}
	return asistencia, nil
}

func estadoReservaPermiteAsistencia(estado Estado) bool {
	return estado == EstadoApartada || estado == EstadoConfirmada || estado == EstadoAbordada
}

func validarEstadoCorridaAsistencia(estado string) error {
	switch estado {
	case "PROGRAMADA", "ABORDANDO", "EN_CURSO":
		return nil
	default:
		return fmt.Errorf("%w: estado %s", ErrCorridaNoAceptaAsistencia, estado)
	}
}

func confirmarAsistencia(
	ctx context.Context, tx pgx.Tx, asistencia AsistenciaReserva,
) (AsistenciaReserva, error) {
	if err := tx.Commit(ctx); err != nil {
		return AsistenciaReserva{}, fmt.Errorf("confirmar transacción de asistencia: %w", err)
	}
	return asistencia, nil
}
