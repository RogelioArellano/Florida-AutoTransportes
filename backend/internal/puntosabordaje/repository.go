package puntosabordaje

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(
	pool *pgxpool.Pool,
) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	input CreateInput,
) (PuntoAbordaje, error) {
	const query = `
		WITH nuevo AS (
			INSERT INTO puntos_abordaje (
				localidad_id,
				nombre,
				referencia,
				latitud,
				longitud
			)
			VALUES (
				$1,
				$2,
				NULLIF($3, ''),
				$4,
				$5
			)
			RETURNING
				id,
				localidad_id,
				nombre,
				referencia,
				latitud,
				longitud,
				activo,
				creado_en,
				actualizado_en
		)
		SELECT
			nuevo.id,
			nuevo.localidad_id,
			localidades.nombre,
			localidades.estado,
			nuevo.nombre,
			COALESCE(nuevo.referencia, ''),
			nuevo.latitud::DOUBLE PRECISION,
			nuevo.longitud::DOUBLE PRECISION,
			nuevo.activo,
			nuevo.creado_en,
			nuevo.actualizado_en
		FROM nuevo
		INNER JOIN localidades
			ON localidades.id = nuevo.localidad_id;
	`

	var punto PuntoAbordaje

	err := r.pool.QueryRow(
		ctx,
		query,
		input.LocalidadID,
		input.Nombre,
		input.Referencia,
		input.Latitud,
		input.Longitud,
	).Scan(
		&punto.ID,
		&punto.LocalidadID,
		&punto.LocalidadNombre,
		&punto.Estado,
		&punto.Nombre,
		&punto.Referencia,
		&punto.Latitud,
		&punto.Longitud,
		&punto.Activo,
		&punto.CreadoEn,
		&punto.ActualizadoEn,
	)
	if err != nil {
		return PuntoAbordaje{},
			mapPostgresError(err)
	}

	return punto, nil
}

func (r *PostgresRepository) List(
	ctx context.Context,
) ([]PuntoAbordaje, error) {
	const query = `
		SELECT
			puntos.id,
			puntos.localidad_id,
			localidades.nombre,
			localidades.estado,
			puntos.nombre,
			COALESCE(puntos.referencia, ''),
			puntos.latitud::DOUBLE PRECISION,
			puntos.longitud::DOUBLE PRECISION,
			puntos.activo,
			puntos.creado_en,
			puntos.actualizado_en
		FROM puntos_abordaje AS puntos
		INNER JOIN localidades
			ON localidades.id = puntos.localidad_id
		ORDER BY
			LOWER(localidades.estado),
			LOWER(localidades.nombre),
			LOWER(puntos.nombre),
			puntos.id;
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar puntos de abordaje: %w",
			err,
		)
	}
	defer rows.Close()

	puntos := make([]PuntoAbordaje, 0)

	for rows.Next() {
		var punto PuntoAbordaje

		if err := rows.Scan(
			&punto.ID,
			&punto.LocalidadID,
			&punto.LocalidadNombre,
			&punto.Estado,
			&punto.Nombre,
			&punto.Referencia,
			&punto.Latitud,
			&punto.Longitud,
			&punto.Activo,
			&punto.CreadoEn,
			&punto.ActualizadoEn,
		); err != nil {
			return nil, fmt.Errorf(
				"leer punto de abordaje: %w",
				err,
			)
		}

		puntos = append(puntos, punto)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer puntos de abordaje: %w",
			err,
		)
	}

	return puntos, nil
}

func mapPostgresError(err error) error {
	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		return fmt.Errorf(
			"guardar punto de abordaje: %w",
			err,
		)
	}

	switch {
	case postgresError.Code == "23505" &&
		postgresError.ConstraintName ==
			"uq_puntos_abordaje_localidad_nombre":
		return ErrPuntoDuplicado

	case postgresError.Code == "23503" &&
		postgresError.ConstraintName ==
			"fk_puntos_abordaje_localidad":
		return ErrLocalidadNoExiste

	default:
		return fmt.Errorf(
			"guardar punto de abordaje: %w",
			err,
		)
	}
}
