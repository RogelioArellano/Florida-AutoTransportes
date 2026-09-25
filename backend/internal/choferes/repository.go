package choferes

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implementa Store utilizando PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// El compilador comprobará que PostgresRepository
// implementa correctamente la interfaz Store.
var _ Store = (*PostgresRepository)(nil)

func NewPostgresRepository(
	pool *pgxpool.Pool,
) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

// Create registra un chofer en PostgreSQL.
func (r *PostgresRepository) Create(
	ctx context.Context,
	input CreateInput,
) (Chofer, error) {
	const query = `
		INSERT INTO choferes (
			nombre_completo,
			telefono,
			licencia_numero,
			licencia_vigencia
		)
		VALUES (
			$1,
			$2,
			$3,
			$4::date
		)
		RETURNING
			id,
			nombre_completo,
			telefono,
			licencia_numero,
			TO_CHAR(
				licencia_vigencia,
				'YYYY-MM-DD'
			),
			activo,
			creado_en,
			actualizado_en;
	`

	fila := r.pool.QueryRow(
		ctx,
		query,
		input.NombreCompleto,
		input.Telefono,
		input.LicenciaNumero,
		input.LicenciaVigencia,
	)

	chofer, err := scanChofer(fila)
	if err != nil {
		return Chofer{}, fmt.Errorf(
			"insertar chofer: %w",
			err,
		)
	}

	return chofer, nil
}

// List devuelve todos los choferes registrados.
//
// Los activos aparecen primero, seguidos por los inactivos.
// Dentro de cada grupo se ordenan por nombre.
func (r *PostgresRepository) List(
	ctx context.Context,
) ([]Chofer, error) {
	const query = `
		SELECT
			id,
			nombre_completo,
			telefono,
			licencia_numero,
			TO_CHAR(
				licencia_vigencia,
				'YYYY-MM-DD'
			),
			activo,
			creado_en,
			actualizado_en
		FROM choferes
		ORDER BY
			activo DESC,
			LOWER(nombre_completo),
			id;
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar choferes: %w",
			err,
		)
	}
	defer rows.Close()

	choferesEncontrados := make([]Chofer, 0)

	for rows.Next() {
		chofer, err := scanChofer(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"leer chofer: %w",
				err,
			)
		}

		choferesEncontrados = append(
			choferesEncontrados,
			chofer,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer choferes: %w",
			err,
		)
	}

	return choferesEncontrados, nil
}

// scanner representa cualquier valor que contiene
// un método Scan.
//
// pgx.Row y pgx.Rows cumplen esta interfaz.
type scanner interface {
	Scan(destinos ...any) error
}

// scanChofer copia las columnas obtenidas de PostgreSQL
// hacia una estructura Chofer.
//
// El orden de los destinos debe ser exactamente igual al
// orden de las columnas de las consultas anteriores.
func scanChofer(
	fila scanner,
) (Chofer, error) {
	var chofer Chofer

	var licenciaNumero sql.NullString
	var licenciaVigencia sql.NullString

	err := fila.Scan(
		&chofer.ID,
		&chofer.NombreCompleto,
		&chofer.Telefono,
		&licenciaNumero,
		&licenciaVigencia,
		&chofer.Activo,
		&chofer.CreadoEn,
		&chofer.ActualizadoEn,
	)
	if err != nil {
		return Chofer{}, err
	}

	chofer.LicenciaNumero = stringDesdeNull(
		licenciaNumero,
	)

	chofer.LicenciaVigencia = stringDesdeNull(
		licenciaVigencia,
	)

	return chofer, nil
}

// stringDesdeNull transforma un valor nullable de PostgreSQL
// en el puntero utilizado por nuestro modelo.
//
// PostgreSQL NULL -> nil
// PostgreSQL texto -> *string
func stringDesdeNull(
	valor sql.NullString,
) *string {
	if !valor.Valid {
		return nil
	}

	resultado := valor.String

	return &resultado
}
