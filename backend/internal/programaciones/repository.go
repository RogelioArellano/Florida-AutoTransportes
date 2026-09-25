package programaciones

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// Esta consulta contiene las columnas y relaciones comunes
// utilizadas tanto por Create como por List.
const consultaProgramacionesBase = `
	SELECT
		p.id,
		p.codigo,
		p.nombre,
		p.ruta_id,
		r.codigo,
		r.nombre,
		p.unidad_id,
		u.codigo,
		p.chofer_id,
		c.nombre_completo,
		TO_CHAR(
			p.hora_salida,
			'HH24:MI'
		),
		p.duracion_estimada_minutos,
		TO_CHAR(
			p.vigencia_desde,
			'YYYY-MM-DD'
		),
		TO_CHAR(
			p.vigencia_hasta,
			'YYYY-MM-DD'
		),
		p.activa,
		p.creado_en,
		p.actualizado_en,
		ARRAY_AGG(
			pd.dia_semana::INTEGER
			ORDER BY pd.dia_semana
		)
	FROM programaciones p
	INNER JOIN rutas r
		ON r.id = p.ruta_id
	INNER JOIN unidades u
		ON u.id = p.unidad_id
	LEFT JOIN choferes c
		ON c.id = p.chofer_id
	INNER JOIN programacion_dias pd
		ON pd.programacion_id = p.id
`

const agrupacionProgramaciones = `
	GROUP BY
		p.id,
		p.codigo,
		p.nombre,
		p.ruta_id,
		r.codigo,
		r.nombre,
		p.unidad_id,
		u.codigo,
		p.chofer_id,
		c.nombre_completo,
		p.hora_salida,
		p.duracion_estimada_minutos,
		p.vigencia_desde,
		p.vigencia_hasta,
		p.activa,
		p.creado_en,
		p.actualizado_en
`

// Create registra la programación y sus días dentro
// de una misma transacción.
func (r *PostgresRepository) Create(
	ctx context.Context,
	input CreateInput,
) (Programacion, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Programacion{}, fmt.Errorf(
			"iniciar transacción de programación: %w",
			err,
		)
	}

	// Rollback funciona como protección.
	//
	// Si alguna operación posterior devuelve un error,
	// todos los cambios pendientes serán revertidos.
	//
	// Si Commit ya fue ejecutado, Rollback no tendrá efecto.
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	rutaDisponible, err := entidadActiva(
		ctx,
		tx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM rutas
				WHERE id = $1
					AND activa = TRUE
			);
		`,
		input.RutaID,
	)
	if err != nil {
		return Programacion{}, fmt.Errorf(
			"verificar ruta de programación: %w",
			err,
		)
	}

	if !rutaDisponible {
		return Programacion{}, ErrRutaNoDisponible
	}

	unidadDisponible, err := entidadActiva(
		ctx,
		tx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM unidades
				WHERE id = $1
					AND activa = TRUE
			);
		`,
		input.UnidadID,
	)
	if err != nil {
		return Programacion{}, fmt.Errorf(
			"verificar unidad de programación: %w",
			err,
		)
	}

	if !unidadDisponible {
		return Programacion{}, ErrUnidadNoDisponible
	}

	if input.ChoferID != nil {
		choferDisponible, err := entidadActiva(
			ctx,
			tx,
			`
				SELECT EXISTS (
					SELECT 1
					FROM choferes
					WHERE id = $1
						AND activo = TRUE
				);
			`,
			*input.ChoferID,
		)
		if err != nil {
			return Programacion{}, fmt.Errorf(
				"verificar chofer de programación: %w",
				err,
			)
		}

		if !choferDisponible {
			return Programacion{}, ErrChoferNoDisponible
		}
	}

	const insertarProgramacion = `
		INSERT INTO programaciones (
			codigo,
			nombre,
			ruta_id,
			unidad_id,
			chofer_id,
			hora_salida,
			duracion_estimada_minutos,
			vigencia_desde,
			vigencia_hasta
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6::time,
			$7,
			$8::date,
			$9::date
		)
		RETURNING id;
	`

	var programacionID int64

	err = tx.QueryRow(
		ctx,
		insertarProgramacion,
		input.Codigo,
		input.Nombre,
		input.RutaID,
		input.UnidadID,
		input.ChoferID,
		input.HoraSalida,
		input.DuracionEstimadaMinutos,
		input.VigenciaDesde,
		input.VigenciaHasta,
	).Scan(&programacionID)
	if err != nil {
		return Programacion{}, traducirErrorPostgres(err)
	}

	const insertarDia = `
		INSERT INTO programacion_dias (
			programacion_id,
			dia_semana
		)
		VALUES ($1, $2);
	`

	for _, dia := range input.DiasSemana {
		if _, err := tx.Exec(
			ctx,
			insertarDia,
			programacionID,
			dia,
		); err != nil {
			return Programacion{}, fmt.Errorf(
				"insertar día %d de programación: %w",
				dia,
				err,
			)
		}
	}

	programacionCreada, err := obtenerPorID(
		ctx,
		tx,
		programacionID,
	)
	if err != nil {
		return Programacion{}, fmt.Errorf(
			"consultar programación creada: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Programacion{}, fmt.Errorf(
			"confirmar transacción de programación: %w",
			err,
		)
	}

	return programacionCreada, nil
}

// List devuelve todas las programaciones con sus datos
// descriptivos y días de operación.
func (r *PostgresRepository) List(
	ctx context.Context,
) ([]Programacion, error) {
	query := consultaProgramacionesBase +
		agrupacionProgramaciones + `
		ORDER BY
			p.activa DESC,
			p.vigencia_desde,
			p.hora_salida,
			LOWER(p.codigo),
			p.id;
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar programaciones: %w",
			err,
		)
	}
	defer rows.Close()

	programacionesEncontradas := make(
		[]Programacion,
		0,
	)

	for rows.Next() {
		programacion, err := scanProgramacion(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"leer programación: %w",
				err,
			)
		}

		programacionesEncontradas = append(
			programacionesEncontradas,
			programacion,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer programaciones: %w",
			err,
		)
	}

	return programacionesEncontradas, nil
}

// entidadActiva ejecuta una consulta SELECT EXISTS.
//
// La consulta recibida debe aceptar el identificador como $1
// y devolver un único booleano.
func entidadActiva(
	ctx context.Context,
	tx pgx.Tx,
	query string,
	id int64,
) (bool, error) {
	var activa bool

	err := tx.QueryRow(
		ctx,
		query,
		id,
	).Scan(&activa)
	if err != nil {
		return false, err
	}

	return activa, nil
}

// obtenerPorID recupera una programación dentro de la misma
// transacción en que fue creada.
func obtenerPorID(
	ctx context.Context,
	tx pgx.Tx,
	id int64,
) (Programacion, error) {
	query := consultaProgramacionesBase + `
		WHERE p.id = $1
	` + agrupacionProgramaciones

	return scanProgramacion(
		tx.QueryRow(
			ctx,
			query,
			id,
		),
	)
}

type scanner interface {
	Scan(destinos ...any) error
}

// scanProgramacion transforma una fila de PostgreSQL
// en una estructura Programacion.
func scanProgramacion(
	fila scanner,
) (Programacion, error) {
	var programacion Programacion

	var choferID sql.NullInt64
	var choferNombre sql.NullString
	var duracion sql.NullInt16
	var vigenciaHasta sql.NullString
	var diasPostgres []int32

	err := fila.Scan(
		&programacion.ID,
		&programacion.Codigo,
		&programacion.Nombre,
		&programacion.RutaID,
		&programacion.RutaCodigo,
		&programacion.RutaNombre,
		&programacion.UnidadID,
		&programacion.UnidadCodigo,
		&choferID,
		&choferNombre,
		&programacion.HoraSalida,
		&duracion,
		&programacion.VigenciaDesde,
		&vigenciaHasta,
		&programacion.Activa,
		&programacion.CreadoEn,
		&programacion.ActualizadoEn,
		&diasPostgres,
	)
	if err != nil {
		return Programacion{}, err
	}

	programacion.ChoferID = int64DesdeNull(
		choferID,
	)

	programacion.ChoferNombre = stringDesdeNull(
		choferNombre,
	)

	programacion.DuracionEstimadaMinutos =
		intDesdeNullInt16(duracion)

	programacion.VigenciaHasta = stringDesdeNull(
		vigenciaHasta,
	)

	programacion.DiasSemana = make(
		[]int,
		len(diasPostgres),
	)

	for indice, dia := range diasPostgres {
		programacion.DiasSemana[indice] = int(dia)
	}

	return programacion, nil
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

func int64DesdeNull(
	valor sql.NullInt64,
) *int64 {
	if !valor.Valid {
		return nil
	}

	resultado := valor.Int64

	return &resultado
}

func intDesdeNullInt16(
	valor sql.NullInt16,
) *int {
	if !valor.Valid {
		return nil
	}

	resultado := int(valor.Int16)

	return &resultado
}

// traducirErrorPostgres convierte restricciones conocidas
// en errores entendibles para el servicio y el futuro Handler.
func traducirErrorPostgres(
	err error,
) error {
	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		return fmt.Errorf(
			"insertar programación: %w",
			err,
		)
	}

	switch {
	case postgresError.Code == "23505" &&
		postgresError.ConstraintName ==
			"uq_programaciones_codigo":
		return ErrCodigoDuplicado

	case postgresError.Code == "23503" &&
		postgresError.ConstraintName ==
			"fk_programaciones_ruta":
		return ErrRutaNoDisponible

	case postgresError.Code == "23503" &&
		postgresError.ConstraintName ==
			"fk_programaciones_unidad":
		return ErrUnidadNoDisponible

	case postgresError.Code == "23503" &&
		postgresError.ConstraintName ==
			"fk_programaciones_chofer":
		return ErrChoferNoDisponible

	default:
		return fmt.Errorf(
			"insertar programación: %w",
			err,
		)
	}
}
