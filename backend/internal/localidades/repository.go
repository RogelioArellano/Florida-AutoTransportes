package localidades

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
) (Localidad, error) {
	const query = `
		INSERT INTO localidades (
			nombre,
			estado
		)
		VALUES ($1, $2)
		RETURNING
			id,
			nombre,
			estado,
			activa,
			creado_en,
			actualizado_en;
	`

	var localidad Localidad

	err := r.pool.QueryRow(
		ctx,
		query,
		input.Nombre,
		input.Estado,
	).Scan(
		&localidad.ID,
		&localidad.Nombre,
		&localidad.Estado,
		&localidad.Activa,
		&localidad.CreadoEn,
		&localidad.ActualizadoEn,
	)
	if err != nil {
		var postgresError *pgconn.PgError

		if errors.As(err, &postgresError) &&
			postgresError.Code == "23505" &&
			postgresError.ConstraintName ==
				"uq_localidades_nombre_estado" {
			return Localidad{}, ErrLocalidadDuplicada
		}

		return Localidad{}, fmt.Errorf(
			"insertar localidad: %w",
			err,
		)
	}

	return localidad, nil
}

func (r *PostgresRepository) List(
	ctx context.Context,
) ([]Localidad, error) {
	const query = `
		SELECT
			id,
			nombre,
			estado,
			activa,
			creado_en,
			actualizado_en
		FROM localidades
		ORDER BY
			LOWER(estado),
			LOWER(nombre),
			id;
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar localidades: %w",
			err,
		)
	}
	defer rows.Close()

	localidades := make([]Localidad, 0)

	for rows.Next() {
		var localidad Localidad

		if err := rows.Scan(
			&localidad.ID,
			&localidad.Nombre,
			&localidad.Estado,
			&localidad.Activa,
			&localidad.CreadoEn,
			&localidad.ActualizadoEn,
		); err != nil {
			return nil, fmt.Errorf(
				"leer localidad: %w",
				err,
			)
		}

		localidades = append(localidades, localidad)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer localidades: %w",
			err,
		)
	}

	return localidades, nil
}
