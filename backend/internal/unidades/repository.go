package unidades

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implementa Store utilizando PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

/*
Esta comprobación ocurre durante la compilación.
Si PostgresRepository deja de implementar Store,
Go mostrará un error inmediatamente.
*/
var _ Store = (*PostgresRepository)(nil)

func NewPostgresRepository(
	pool *pgxpool.Pool,
) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

// Create registra una unidad y devuelve el registro completo
// generado por PostgreSQL.
func (r *PostgresRepository) Create(
	ctx context.Context,
	input CreateInput,
) (Unidad, error) {
	const query = `
		INSERT INTO unidades (
			codigo,
			placas,
			marca,
			modelo,
			anio,
			capacidad_total,
			capacidad_pasajeros
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7
		)
		RETURNING
			id,
			codigo,
			placas,
			marca,
			modelo,
			anio,
			capacidad_total,
			capacidad_pasajeros,
			activa,
			creado_en,
			actualizado_en;
	`

	fila := r.pool.QueryRow(
		ctx,
		query,
		input.Codigo,
		input.Placas,
		input.Marca,
		input.Modelo,
		input.Anio,
		input.CapacidadTotal,
		input.CapacidadPasajeros,
	)

	unidad, err := scanUnidad(fila)
	if err != nil {
		var postgresError *pgconn.PgError

		if errors.As(err, &postgresError) &&
			postgresError.Code == "23505" { //Error postgre para unicidad TODO hacer constantes de estos códigos
			switch postgresError.ConstraintName {
			case "uq_unidades_codigo":
				return Unidad{}, ErrCodigoDuplicado

			case "uq_unidades_placas":
				return Unidad{}, ErrPlacasDuplicadas
			}
		}

		return Unidad{}, fmt.Errorf(
			"insertar unidad: %w",
			err,
		)
	}

	return unidad, nil
}

/*
List devuelve todas las unidades registradas.

Se incluyen también las unidades inactivas porque este endpoint
pertenece al catálogo administrativo. Posteriormente podremos
crear consultas que devuelvan únicamente las unidades activas.
*/
func (r *PostgresRepository) List(
	ctx context.Context,
) ([]Unidad, error) {
	const query = `
		SELECT
			id,
			codigo,
			placas,
			marca,
			modelo,
			anio,
			capacidad_total,
			capacidad_pasajeros,
			activa,
			creado_en,
			actualizado_en
		FROM unidades
		ORDER BY
			activa DESC,
			LOWER(codigo),
			id;
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar unidades: %w",
			err,
		)
	}
	defer rows.Close()

	unidades := make([]Unidad, 0)

	for rows.Next() {
		unidad, err := scanUnidad(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"leer unidad: %w",
				err,
			)
		}

		unidades = append(unidades, unidad)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer unidades: %w",
			err,
		)
	}

	return unidades, nil
}

/*
Scanner representa cualquier elemento capaz de copiar
columnas de PostgreSQL hacia variables de Go.

Tanto pgx.Row como pgx.Rows contienen un método Scan,
por eso ambos cumplen esta interfaz.
*/
type scanner interface {
	Scan(destinos ...any) error
}

/*
scanUnidad concentra la lectura de una fila.

Esto evita repetir el mismo bloque Scan en Create y List.
El orden de los destinos debe coincidir exactamente con
el orden de las columnas de las consultas SQL.
*/
func scanUnidad(
	fila scanner,
) (Unidad, error) {
	var unidad Unidad

	var placas sql.NullString
	var marca sql.NullString
	var modelo sql.NullString
	var anio sql.NullInt16

	err := fila.Scan(
		&unidad.ID,
		&unidad.Codigo,
		&placas,
		&marca,
		&modelo,
		&anio,
		&unidad.CapacidadTotal,
		&unidad.CapacidadPasajeros,
		&unidad.Activa,
		&unidad.CreadoEn,
		&unidad.ActualizadoEn,
	)
	if err != nil {
		return Unidad{}, err
	}

	unidad.Placas = stringOpcional(placas)
	unidad.Marca = stringOpcional(marca)
	unidad.Modelo = stringOpcional(modelo)
	unidad.Anio = intOpcional(anio)

	return unidad, nil
}

// stringOpcional transforma un sql.NullString en *string.
//
// PostgreSQL NULL  -> nil
// PostgreSQL texto -> puntero al texto
func stringOpcional(
	valor sql.NullString,
) *string {
	if !valor.Valid {
		return nil
	}

	resultado := valor.String

	return &resultado
}

// intOpcional transforma un SMALLINT nullable de PostgreSQL
// en un *int utilizado por nuestro modelo.
func intOpcional(
	valor sql.NullInt16,
) *int {
	if !valor.Valid {
		return nil
	}

	resultado := int(valor.Int16)

	return &resultado
}
