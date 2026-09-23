package rutas

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

// Create guarda la ruta y todas sus paradas dentro
// de una misma transacción.
func (r *Repository) Create(
	ctx context.Context,
	input CreateInput,
) (Ruta, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Ruta{}, fmt.Errorf(
			"iniciar transacción para crear ruta: %w",
			err,
		)
	}

	// Si la función termina antes del Commit, Rollback deshace
	// cualquier cambio realizado durante la transacción.
	//
	// Después de un Commit exitoso, Rollback no modifica nada.
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := validarPuntosDisponibles(
		ctx,
		tx,
		input.Paradas,
	); err != nil {
		return Ruta{}, err
	}

	var ruta Ruta

	err = tx.QueryRow(
		ctx,
		`
			INSERT INTO rutas (
				codigo,
				nombre
			)
			VALUES ($1, $2)
			RETURNING
				id,
				codigo,
				nombre,
				activa,
				creado_en,
				actualizado_en
		`,
		input.Codigo,
		input.Nombre,
	).Scan(
		&ruta.ID,
		&ruta.Codigo,
		&ruta.Nombre,
		&ruta.Activa,
		&ruta.CreadoEn,
		&ruta.ActualizadoEn,
	)
	if err != nil {
		return Ruta{}, traducirErrorPostgres(err)
	}

	for indice, parada := range input.Paradas {
		orden := indice + 1

		_, err = tx.Exec(
			ctx,
			`
				INSERT INTO ruta_paradas (
					ruta_id,
					punto_abordaje_id,
					orden,
					permite_subir,
					permite_bajar,
					es_obligatoria
				)
				VALUES ($1, $2, $3, $4, $5, $6)
			`,
			ruta.ID,
			parada.PuntoAbordajeID,
			orden,
			parada.PermiteSubir,
			parada.PermiteBajar,
			parada.EsObligatoria,
		)
		if err != nil {
			return Ruta{}, traducirErrorPostgres(err)
		}
	}

	ruta.Paradas, err = consultarParadasRuta(
		ctx,
		tx,
		ruta.ID,
	)
	if err != nil {
		return Ruta{}, fmt.Errorf(
			"consultar paradas de la ruta creada: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Ruta{}, fmt.Errorf(
			"confirmar creación de ruta: %w",
			err,
		)
	}

	return ruta, nil
}

// List devuelve las rutas con sus paradas ordenadas.
func (r *Repository) List(
	ctx context.Context,
) ([]Ruta, error) {
	rows, err := r.pool.Query(
		ctx,
		`
			SELECT
				id,
				codigo,
				nombre,
				activa,
				creado_en,
				actualizado_en
			FROM rutas
			ORDER BY nombre, id
		`,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar rutas: %w",
			err,
		)
	}
	defer rows.Close()

	rutasEncontradas := make([]Ruta, 0)

	// Guardaremos la posición de cada ruta dentro del slice.
	// Esto permitirá agregarle sus paradas posteriormente.
	indicePorRutaID := make(map[int64]int)

	for rows.Next() {
		var ruta Ruta

		if err := rows.Scan(
			&ruta.ID,
			&ruta.Codigo,
			&ruta.Nombre,
			&ruta.Activa,
			&ruta.CreadoEn,
			&ruta.ActualizadoEn,
		); err != nil {
			return nil, fmt.Errorf(
				"leer ruta: %w",
				err,
			)
		}

		// Inicializar el slice evita enviar "paradas": null.
		ruta.Paradas = make([]RutaParada, 0)

		indicePorRutaID[ruta.ID] = len(rutasEncontradas)

		rutasEncontradas = append(
			rutasEncontradas,
			ruta,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer rutas: %w",
			err,
		)
	}

	if len(rutasEncontradas) == 0 {
		return rutasEncontradas, nil
	}

	rowsParadas, err := r.pool.Query(
		ctx,
		`
			SELECT
				rp.ruta_id,
				rp.id,
				rp.punto_abordaje_id,
				pa.nombre,
				l.nombre,
				l.estado,
				rp.orden,
				rp.permite_subir,
				rp.permite_bajar,
				rp.es_obligatoria,
				rp.creado_en
			FROM ruta_paradas AS rp
			INNER JOIN puntos_abordaje AS pa
				ON pa.id = rp.punto_abordaje_id
			INNER JOIN localidades AS l
				ON l.id = pa.localidad_id
			ORDER BY
				rp.ruta_id,
				rp.orden
		`,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar paradas de rutas: %w",
			err,
		)
	}
	defer rowsParadas.Close()

	for rowsParadas.Next() {
		var rutaID int64
		var parada RutaParada

		if err := rowsParadas.Scan(
			&rutaID,
			&parada.ID,
			&parada.PuntoAbordajeID,
			&parada.PuntoNombre,
			&parada.LocalidadNombre,
			&parada.Estado,
			&parada.Orden,
			&parada.PermiteSubir,
			&parada.PermiteBajar,
			&parada.EsObligatoria,
			&parada.CreadoEn,
		); err != nil {
			return nil, fmt.Errorf(
				"leer parada de ruta: %w",
				err,
			)
		}

		indiceRuta, existe := indicePorRutaID[rutaID]
		if !existe {
			continue
		}

		rutasEncontradas[indiceRuta].Paradas = append(
			rutasEncontradas[indiceRuta].Paradas,
			parada,
		)
	}

	if err := rowsParadas.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer paradas de rutas: %w",
			err,
		)
	}

	return rutasEncontradas, nil
}

// validarPuntosDisponibles comprueba que todos los puntos
// existan y que tanto el punto como su localidad estén activos.
func validarPuntosDisponibles(
	ctx context.Context,
	tx pgx.Tx,
	paradas []CreateParadaInput,
) error {
	puntoIDs := make([]int64, 0, len(paradas))

	for _, parada := range paradas {
		puntoIDs = append(
			puntoIDs,
			parada.PuntoAbordajeID,
		)
	}

	var cantidadDisponible int

	err := tx.QueryRow(
		ctx,
		`
			SELECT COUNT(*)
			FROM puntos_abordaje AS pa
			INNER JOIN localidades AS l
				ON l.id = pa.localidad_id
			WHERE
				pa.id = ANY($1::BIGINT[])
				AND pa.activo = TRUE
				AND l.activa = TRUE
		`,
		puntoIDs,
	).Scan(&cantidadDisponible)
	if err != nil {
		return fmt.Errorf(
			"validar puntos de abordaje: %w",
			err,
		)
	}

	if cantidadDisponible != len(puntoIDs) {
		return fmt.Errorf(
			"%w: algún punto no existe o está inactivo",
			ErrPuntoNoExiste,
		)
	}

	return nil
}

// consultarParadasRuta obtiene las paradas de una ruta.
// Recibe pgx.Tx porque durante Create todavía estamos
// trabajando dentro de la transacción.
func consultarParadasRuta(
	ctx context.Context,
	tx pgx.Tx,
	rutaID int64,
) ([]RutaParada, error) {
	rows, err := tx.Query(
		ctx,
		`
			SELECT
				rp.id,
				rp.punto_abordaje_id,
				pa.nombre,
				l.nombre,
				l.estado,
				rp.orden,
				rp.permite_subir,
				rp.permite_bajar,
				rp.es_obligatoria,
				rp.creado_en
			FROM ruta_paradas AS rp
			INNER JOIN puntos_abordaje AS pa
				ON pa.id = rp.punto_abordaje_id
			INNER JOIN localidades AS l
				ON l.id = pa.localidad_id
			WHERE rp.ruta_id = $1
			ORDER BY rp.orden
		`,
		rutaID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	paradas := make([]RutaParada, 0)

	for rows.Next() {
		var parada RutaParada

		if err := rows.Scan(
			&parada.ID,
			&parada.PuntoAbordajeID,
			&parada.PuntoNombre,
			&parada.LocalidadNombre,
			&parada.Estado,
			&parada.Orden,
			&parada.PermiteSubir,
			&parada.PermiteBajar,
			&parada.EsObligatoria,
			&parada.CreadoEn,
		); err != nil {
			return nil, err
		}

		paradas = append(paradas, parada)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return paradas, nil
}

// traducirErrorPostgres convierte errores técnicos de
// PostgreSQL en errores conocidos por el dominio.
func traducirErrorPostgres(err error) error {
	var pgError *pgconn.PgError

	if !errors.As(err, &pgError) {
		return err
	}

	switch {
	case pgError.Code == "23505" &&
		pgError.ConstraintName == "uq_rutas_codigo":
		return fmt.Errorf(
			"%w: %s",
			ErrCodigoDuplicado,
			pgError.Detail,
		)

	case pgError.Code == "23503" &&
		pgError.ConstraintName == "fk_ruta_paradas_punto":
		return fmt.Errorf(
			"%w: %s",
			ErrPuntoNoExiste,
			pgError.Detail,
		)

	default:
		return err
	}
}
