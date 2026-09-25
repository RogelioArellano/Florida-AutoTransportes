package programaciones

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
)

// PostgresRepository también implementará HorarioStore.
//
// Reutilizamos el mismo repositorio porque tanto las
// programaciones como sus horarios pertenecen al mismo dominio.
var _ HorarioStore = (*PostgresRepository)(nil)

// Replace sustituye toda la configuración de horarios
// de una programación dentro de una transacción.
func (r *PostgresRepository) Replace(
	ctx context.Context,
	input ConfigurarHorariosInput,
) ([]HorarioParada, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"iniciar transacción de horarios: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// FOR UPDATE bloquea temporalmente la programación.
	//
	// Mientras se configuran sus horarios, otra transacción
	// no podrá modificar el mismo registro simultáneamente.
	const consultarProgramacion = `
		SELECT ruta_id
		FROM programaciones
		WHERE id = $1
			AND activa = TRUE
		FOR UPDATE;
	`

	var rutaID int64

	err = tx.QueryRow(
		ctx,
		consultarProgramacion,
		input.ProgramacionID,
	).Scan(&rutaID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProgramacionNoDisponible
	}

	if err != nil {
		return nil, fmt.Errorf(
			"consultar programación de horarios: %w",
			err,
		)
	}

	rutaParadaIDs := make(
		[]int64,
		len(input.Paradas),
	)

	for indice, parada := range input.Paradas {
		rutaParadaIDs[indice] =
			parada.RutaParadaID
	}

	ordenes, err := obtenerOrdenParadas(
		ctx,
		tx,
		rutaID,
		rutaParadaIDs,
	)
	if err != nil {
		return nil, err
	}

	if len(ordenes) != len(input.Paradas) {
		return nil, ErrParadaNoPertenece
	}

	type paradaOrdenada struct {
		configuracion ConfigurarHorarioParadaInput
		orden         int
	}

	paradasOrdenadas := make(
		[]paradaOrdenada,
		0,
		len(input.Paradas),
	)

	for _, parada := range input.Paradas {
		orden, existe := ordenes[parada.RutaParadaID]
		if !existe {
			return nil, ErrParadaNoPertenece
		}

		paradasOrdenadas = append(
			paradasOrdenadas,
			paradaOrdenada{
				configuracion: parada,
				orden:         orden,
			},
		)
	}

	// La petición puede enviar las paradas en cualquier orden.
	// Aquí las ordenamos según el recorrido real de la ruta.
	sort.Slice(
		paradasOrdenadas,
		func(i int, j int) bool {
			return paradasOrdenadas[i].orden <
				paradasOrdenadas[j].orden
		},
	)

	// Una parada posterior no puede comenzar antes de que
	// termine la ventana de la parada anterior configurada.
	for indice := 1; indice < len(paradasOrdenadas); indice++ {
		anterior :=
			paradasOrdenadas[indice-1].configuracion

		actual :=
			paradasOrdenadas[indice].configuracion

		if actual.MinutosDesdeSalidaInicio <
			anterior.MinutosDesdeSalidaFin {
			return nil, fmt.Errorf(
				"%w: la parada de orden %d comienza antes de finalizar la parada anterior",
				ErrHorariosFueraDeOrden,
				paradasOrdenadas[indice].orden,
			)
		}
	}

	const eliminarHorarios = `
		DELETE FROM programacion_parada_horarios
		WHERE programacion_id = $1;
	`

	if _, err := tx.Exec(
		ctx,
		eliminarHorarios,
		input.ProgramacionID,
	); err != nil {
		return nil, fmt.Errorf(
			"eliminar horarios anteriores: %w",
			err,
		)
	}

	const insertarHorario = `
		INSERT INTO programacion_parada_horarios (
			programacion_id,
			ruta_id,
			ruta_parada_id,
			minutos_desde_salida_inicio,
			minutos_desde_salida_fin,
			notas
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6
		);
	`

	for _, parada := range paradasOrdenadas {
		configuracion := parada.configuracion

		if _, err := tx.Exec(
			ctx,
			insertarHorario,
			input.ProgramacionID,
			rutaID,
			configuracion.RutaParadaID,
			configuracion.MinutosDesdeSalidaInicio,
			configuracion.MinutosDesdeSalidaFin,
			configuracion.Notas,
		); err != nil {
			return nil, fmt.Errorf(
				"insertar horario de parada %d: %w",
				configuracion.RutaParadaID,
				err,
			)
		}
	}

	horarios, err := listarHorarios(
		ctx,
		tx,
		input.ProgramacionID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar horarios configurados: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf(
			"confirmar transacción de horarios: %w",
			err,
		)
	}

	return horarios, nil
}

// ListByProgramacion devuelve los horarios configurados
// para una programación.
func (r *PostgresRepository) ListByProgramacion(
	ctx context.Context,
	programacionID int64,
) ([]HorarioParada, error) {
	const consultarExistencia = `
		SELECT EXISTS (
			SELECT 1
			FROM programaciones
			WHERE id = $1
				AND activa = TRUE
		);
	`

	var existe bool

	err := r.pool.QueryRow(
		ctx,
		consultarExistencia,
		programacionID,
	).Scan(&existe)
	if err != nil {
		return nil, fmt.Errorf(
			"verificar programación de horarios: %w",
			err,
		)
	}

	if !existe {
		return nil, ErrProgramacionNoDisponible
	}

	horarios, err := listarHorarios(
		ctx,
		r.pool,
		programacionID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar horarios de programación: %w",
			err,
		)
	}

	return horarios, nil
}

// obtenerOrdenParadas devuelve el orden real de las paradas
// que pertenecen a una ruta.
//
// Las paradas que no pertenecen a la ruta no aparecerán
// en el mapa resultante.
func obtenerOrdenParadas(
	ctx context.Context,
	tx pgx.Tx,
	rutaID int64,
	rutaParadaIDs []int64,
) (map[int64]int, error) {
	const query = `
		SELECT
			id,
			orden
		FROM ruta_paradas
		WHERE ruta_id = $1
			AND id = ANY($2::BIGINT[])
		ORDER BY orden;
	`

	rows, err := tx.Query(
		ctx,
		query,
		rutaID,
		rutaParadaIDs,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar paradas de la ruta: %w",
			err,
		)
	}
	defer rows.Close()

	ordenes := make(map[int64]int)

	for rows.Next() {
		var rutaParadaID int64
		var orden int

		if err := rows.Scan(
			&rutaParadaID,
			&orden,
		); err != nil {
			return nil, fmt.Errorf(
				"leer parada de la ruta: %w",
				err,
			)
		}

		ordenes[rutaParadaID] = orden
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer paradas de la ruta: %w",
			err,
		)
	}

	return ordenes, nil
}

// horarioRowsQuerier permite que listarHorarios funcione
// tanto con el pool normal como dentro de una transacción.
type horarioRowsQuerier interface {
	Query(
		ctx context.Context,
		sql string,
		args ...any,
	) (pgx.Rows, error)
}

func listarHorarios(
	ctx context.Context,
	querier horarioRowsQuerier,
	programacionID int64,
) ([]HorarioParada, error) {
	const query = `
		SELECT
			h.programacion_id,
			h.ruta_parada_id,
			rp.punto_abordaje_id,
			pa.nombre,
			rp.orden,
			h.minutos_desde_salida_inicio,
			h.minutos_desde_salida_fin,
			TO_CHAR(
				p.hora_salida +
				MAKE_INTERVAL(
					mins =>
						h.minutos_desde_salida_inicio::INTEGER
				),
				'HH24:MI'
			),
			TO_CHAR(
				p.hora_salida +
				MAKE_INTERVAL(
					mins =>
						h.minutos_desde_salida_fin::INTEGER
				),
				'HH24:MI'
			),
			h.notas,
			h.creado_en,
			h.actualizado_en
		FROM programacion_parada_horarios h
		INNER JOIN programaciones p
			ON p.id = h.programacion_id
		INNER JOIN ruta_paradas rp
			ON rp.id = h.ruta_parada_id
			AND rp.ruta_id = h.ruta_id
		INNER JOIN puntos_abordaje pa
			ON pa.id = rp.punto_abordaje_id
		WHERE h.programacion_id = $1
		ORDER BY rp.orden;
	`

	rows, err := querier.Query(
		ctx,
		query,
		programacionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	horarios := make([]HorarioParada, 0)

	for rows.Next() {
		horario, err := scanHorarioParada(rows)
		if err != nil {
			return nil, err
		}

		horarios = append(
			horarios,
			horario,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return horarios, nil
}

func scanHorarioParada(
	fila scanner,
) (HorarioParada, error) {
	var horario HorarioParada

	var minutosInicio int16
	var minutosFin int16
	var notas sql.NullString

	err := fila.Scan(
		&horario.ProgramacionID,
		&horario.RutaParadaID,
		&horario.PuntoAbordajeID,
		&horario.PuntoNombre,
		&horario.Orden,
		&minutosInicio,
		&minutosFin,
		&horario.HoraEstimadaInicio,
		&horario.HoraEstimadaFin,
		&notas,
		&horario.CreadoEn,
		&horario.ActualizadoEn,
	)
	if err != nil {
		return HorarioParada{}, err
	}

	horario.MinutosDesdeSalidaInicio =
		int(minutosInicio)

	horario.MinutosDesdeSalidaFin =
		int(minutosFin)

	horario.Notas = stringDesdeNull(notas)

	return horario, nil
}
