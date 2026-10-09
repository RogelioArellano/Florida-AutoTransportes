package corridas

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var _ OperacionStore = (*PostgresRepository)(nil)

func (r *PostgresRepository) GetOperation(ctx context.Context, id int64) (Corrida, error) {
	corrida, err := obtenerCorridaPorID(ctx, r.pool, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Corrida{}, ErrCorridaNoEncontrada
	}
	if err != nil {
		return Corrida{}, fmt.Errorf("consultar operación de corrida: %w", err)
	}
	return corrida, nil
}

// TransitionOperation bloquea primero la corrida. La asistencia utiliza
// FOR SHARE sobre esa misma fila, y la creación de reservas FOR UPDATE.
// Así ninguna de esas operaciones puede cruzar el cambio de estado.
func (r *PostgresRepository) TransitionOperation(ctx context.Context, id int64, destino Estado) (Corrida, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Corrida{}, fmt.Errorf("iniciar operación de corrida: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	corrida, err := scanCorrida(tx.QueryRow(ctx, consultaCorridaBase+" WHERE c.id = $1 FOR UPDATE OF c", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Corrida{}, ErrCorridaNoEncontrada
	}
	if err != nil {
		return Corrida{}, fmt.Errorf("bloquear corrida: %w", err)
	}

	// Un reintento del estado actual conserva fechas y actualizado_en.
	// Una salida ya registrada también admite reintentos después de finalizar.
	reintento := corrida.Estado == destino && (destino == EstadoAbordando || destino == EstadoEnCurso || destino == EstadoCompletada)
	if destino == EstadoEnCurso && corrida.Estado == EstadoCompletada && corrida.SalidaReal != nil {
		reintento = true
	}
	if reintento {
		if (destino == EstadoEnCurso || destino == EstadoCompletada) && corrida.SalidaReal == nil {
			return Corrida{}, ErrFechasOperacionInconsistentes
		}
		if destino == EstadoCompletada && corrida.LlegadaReal == nil {
			return Corrida{}, ErrFechasOperacionInconsistentes
		}
		return confirmarOperacion(ctx, tx, id)
	}

	permitida := (destino == EstadoAbordando && corrida.Estado == EstadoProgramada) ||
		(destino == EstadoEnCurso && corrida.Estado == EstadoAbordando) ||
		(destino == EstadoCompletada && corrida.Estado == EstadoEnCurso)
	if !permitida {
		return Corrida{}, fmt.Errorf("%w: %s → %s", ErrTransicionNoPermitida, corrida.Estado, destino)
	}

	if destino == EstadoAbordando || destino == EstadoEnCurso {
		if err := validarRecursosOperacion(ctx, tx, corrida); err != nil {
			return Corrida{}, err
		}
		if _, err := primerOrigenOperacion(ctx, tx, id); err != nil {
			return Corrida{}, err
		}
	}
	if destino == EstadoEnCurso {
		origen, err := primerOrigenOperacion(ctx, tx, id)
		if err != nil {
			return Corrida{}, err
		}
		pendientes, err := contarAsistenciasPendientes(ctx, tx, id, &origen)
		if err != nil {
			return Corrida{}, err
		}
		if pendientes > 0 {
			return Corrida{}, fmt.Errorf("%w: %d reserva(s)", ErrAsistenciasOrigenPendientes, pendientes)
		}
	}
	if destino == EstadoCompletada {
		if corrida.SalidaReal == nil {
			return Corrida{}, ErrFechasOperacionInconsistentes
		}
		pendientes, err := contarAsistenciasPendientes(ctx, tx, id, nil)
		if err != nil {
			return Corrida{}, err
		}
		if pendientes > 0 {
			return Corrida{}, fmt.Errorf("%w: %d reserva(s)", ErrAsistenciasPendientes, pendientes)
		}
	}

	// statement_timestamp se evalúa DESPUÉS de obtener el bloqueo.
	// Se usa GREATEST al finalizar para conservar la relación llegada >= salida.
	// No se escriben reservas, pagos, descuentos ni reembolsos.
	_, err = tx.Exec(ctx, `UPDATE corridas SET estado = $2::TEXT,
		reservas_abiertas = CASE WHEN $2::TEXT IN ('EN_CURSO','COMPLETADA') THEN FALSE ELSE reservas_abiertas END,
		salida_real = CASE WHEN $2::TEXT = 'EN_CURSO' THEN statement_timestamp() ELSE salida_real END,
		llegada_real = CASE WHEN $2::TEXT = 'COMPLETADA' THEN GREATEST(statement_timestamp(), salida_real) ELSE llegada_real END,
		actualizado_en = statement_timestamp()
		WHERE id = $1`, id, string(destino))
	if err != nil {
		return Corrida{}, fmt.Errorf("actualizar operación de corrida: %w", err)
	}
	return confirmarOperacion(ctx, tx, id)
}

func confirmarOperacion(ctx context.Context, tx pgx.Tx, id int64) (Corrida, error) {
	corrida, err := obtenerCorridaPorID(ctx, tx, id)
	if err != nil {
		return Corrida{}, fmt.Errorf("consultar resultado operativo: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Corrida{}, fmt.Errorf("confirmar operación de corrida: %w", err)
	}
	return corrida, nil
}

func validarRecursosOperacion(ctx context.Context, tx pgx.Tx, corrida Corrida) error {
	var activa bool
	// FOR SHARE mantiene la disponibilidad hasta confirmar la transición.
	if err := tx.QueryRow(ctx, "SELECT activa FROM unidades WHERE id = $1 FOR SHARE", corrida.UnidadID).Scan(&activa); err != nil {
		return fmt.Errorf("consultar unidad para operación: %w", err)
	}
	if !activa {
		return ErrUnidadNoDisponible
	}
	if corrida.ChoferID == nil {
		return ErrCorridaSinChofer
	}
	var activo, licenciaVigente bool
	err := tx.QueryRow(ctx, `SELECT activo,
		licencia_numero IS NOT NULL AND licencia_vigencia IS NOT NULL
		AND licencia_vigencia >= (statement_timestamp() AT TIME ZONE 'America/Mexico_City')::DATE
		FROM choferes WHERE id = $1 FOR SHARE`, *corrida.ChoferID).Scan(&activo, &licenciaVigente)
	if err != nil {
		return fmt.Errorf("consultar chofer para operación: %w", err)
	}
	if !activo {
		return ErrChoferNoDisponible
	}
	if !licenciaVigente {
		return ErrLicenciaNoVigente
	}
	return nil
}

func primerOrigenOperacion(ctx context.Context, tx pgx.Tx, id int64) (int64, error) {
	var origen int64
	err := tx.QueryRow(ctx, `SELECT o.id FROM corrida_paradas o
		WHERE o.corrida_id=$1 AND o.incluida_en_recorrido AND o.permite_subir
		AND EXISTS (SELECT 1 FROM corrida_paradas d WHERE d.corrida_id=o.corrida_id
			AND d.incluida_en_recorrido AND d.permite_bajar AND d.orden > o.orden)
		ORDER BY o.orden LIMIT 1`, id).Scan(&origen)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrCorridaSinTramo
	}
	if err != nil {
		return 0, fmt.Errorf("consultar primer punto de abordaje: %w", err)
	}
	return origen, nil
}

func contarAsistenciasPendientes(ctx context.Context, tx pgx.Tx, id int64, origen *int64) (int, error) {
	var pendientes int
	// CANCELADA no requiere asistencia. NULL histórico bloquea el cierre:
	// no se inventan cantidades ni se declaran ausencias automáticamente.
	err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM reservas WHERE corrida_id=$1
		AND estado <> 'CANCELADA'
		AND (asistencia_cerrada_en IS NULL OR cantidad_abordada IS NULL)
		AND ($2::BIGINT IS NULL OR corrida_parada_origen_id=$2)`, id, origen).Scan(&pendientes)
	if err != nil {
		return 0, fmt.Errorf("consultar asistencias pendientes: %w", err)
	}
	return pendientes, nil
}
