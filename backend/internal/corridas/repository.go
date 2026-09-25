package corridas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

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

type datosProgramacion struct {
	Codigo                  string
	RutaID                  int64
	RutaCodigo              string
	RutaNombre              string
	RutaActiva              bool
	UnidadID                int64
	UnidadCodigo            string
	CapacidadPasajeros      int
	UnidadActiva            bool
	ChoferID                *int64
	ChoferNombre            *string
	ChoferActivo            bool
	LicenciaVigente         bool
	HoraSalida              string
	DuracionEstimadaMinutos *int
	DentroDeVigencia        bool
	OperaEseDia             bool
}

const consultaCorridaBase = `
	SELECT
		c.id,
		c.folio,
		c.programacion_id,
		c.programacion_codigo,
		c.ruta_id,
		c.ruta_codigo,
		c.ruta_nombre,
		c.unidad_id,
		c.unidad_codigo,
		c.chofer_id,
		c.chofer_nombre,
		TO_CHAR(
			c.fecha_servicio,
			'YYYY-MM-DD'
		),
		c.salida_programada,
		c.llegada_estimada,
		c.salida_real,
		c.llegada_real,
		c.capacidad_pasajeros,
		c.estado,
		c.reservas_abiertas,
		c.observaciones,
		c.creado_en,
		c.actualizado_en
	FROM corridas c
`

// Generate crea una corrida completa a partir de una
// programación recurrente.
func (r *PostgresRepository) Generate(
	ctx context.Context,
	input GenerarInput,
) (Corrida, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Corrida{}, fmt.Errorf(
			"iniciar transacción de corrida: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	datos, err := consultarProgramacion(
		ctx,
		tx,
		input.ProgramacionID,
		input.FechaServicio,
	)
	if err != nil {
		return Corrida{}, err
	}

	if !datos.RutaActiva {
		return Corrida{}, ErrProgramacionNoDisponible
	}

	if !datos.UnidadActiva {
		return Corrida{}, ErrUnidadNoDisponible
	}

	if !datos.DentroDeVigencia ||
		!datos.OperaEseDia {
		return Corrida{}, ErrProgramacionNoOperaFecha
	}

	if datos.ChoferID != nil {
		if !datos.ChoferActivo {
			return Corrida{}, ErrChoferNoDisponible
		}

		if !datos.LicenciaVigente {
			return Corrida{}, ErrLicenciaNoVigente
		}
	}

	cantidadParadas, puntosActivos, err :=
		consultarEstadoParadas(
			ctx,
			tx,
			datos.RutaID,
		)
	if err != nil {
		return Corrida{}, err
	}

	if cantidadParadas == 0 {
		return Corrida{}, ErrProgramacionSinParadas
	}

	if !puntosActivos {
		return Corrida{}, ErrProgramacionNoDisponible
	}

	fechaFolio := strings.ReplaceAll(
		input.FechaServicio,
		"-",
		"",
	)

	folio := datos.Codigo + "-" + fechaFolio

	const insertarCorrida = `
		INSERT INTO corridas (
			folio,
			programacion_id,
			programacion_codigo,
			ruta_id,
			ruta_codigo,
			ruta_nombre,
			unidad_id,
			unidad_codigo,
			chofer_id,
			chofer_nombre,
			fecha_servicio,
			salida_programada,
			llegada_estimada,
			capacidad_pasajeros,
			observaciones
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11::DATE,
			(
				$11::DATE + $12::TIME
			) AT TIME ZONE
				'America/Mexico_City',
			CASE
				WHEN $13::SMALLINT IS NULL
					THEN NULL
				ELSE (
					(
						$11::DATE +
						$12::TIME
					) +
					MAKE_INTERVAL(
						mins => $13::INTEGER
					)
				) AT TIME ZONE
					'America/Mexico_City'
			END,
			$14,
			$15
		)
		RETURNING id;
	`

	var corridaID int64

	err = tx.QueryRow(
		ctx,
		insertarCorrida,
		folio,
		input.ProgramacionID,
		datos.Codigo,
		datos.RutaID,
		datos.RutaCodigo,
		datos.RutaNombre,
		datos.UnidadID,
		datos.UnidadCodigo,
		datos.ChoferID,
		datos.ChoferNombre,
		input.FechaServicio,
		datos.HoraSalida,
		datos.DuracionEstimadaMinutos,
		datos.CapacidadPasajeros,
		input.Observaciones,
	).Scan(&corridaID)
	if err != nil {
		return Corrida{}, traducirErrorPostgres(
			err,
		)
	}

	filasInsertadas, err := copiarParadas(
		ctx,
		tx,
		corridaID,
		input.ProgramacionID,
		datos.RutaID,
		input.FechaServicio,
		datos.HoraSalida,
	)
	if err != nil {
		return Corrida{}, err
	}

	if filasInsertadas != cantidadParadas {
		return Corrida{}, fmt.Errorf(
			"se esperaban %d paradas y se copiaron %d",
			cantidadParadas,
			filasInsertadas,
		)
	}

	corridaCreada, err := obtenerCorridaPorID(
		ctx,
		tx,
		corridaID,
	)
	if err != nil {
		return Corrida{}, fmt.Errorf(
			"consultar corrida creada: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Corrida{}, fmt.Errorf(
			"confirmar transacción de corrida: %w",
			err,
		)
	}

	return corridaCreada, nil
}

// List consulta corridas utilizando filtros opcionales.
func (r *PostgresRepository) List(
	ctx context.Context,
	filter ListFilter,
) ([]Corrida, error) {
	var estado *string

	if filter.Estado != nil {
		estadoTexto := string(*filter.Estado)
		estado = &estadoTexto
	}

	query := consultaCorridaBase + `
		WHERE (
			$1::DATE IS NULL
			OR c.fecha_servicio >= $1::DATE
		)
		AND (
			$2::DATE IS NULL
			OR c.fecha_servicio <= $2::DATE
		)
		AND (
			$3::VARCHAR IS NULL
			OR c.estado = $3::VARCHAR
		)
		ORDER BY
			c.fecha_servicio,
			c.salida_programada,
			c.id;
	`

	rows, err := r.pool.Query(
		ctx,
		query,
		filter.FechaDesde,
		filter.FechaHasta,
		estado,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar corridas: %w",
			err,
		)
	}
	defer rows.Close()

	corridasEncontradas := make(
		[]Corrida,
		0,
	)

	corridaIDs := make([]int64, 0)

	for rows.Next() {
		corrida, err := scanCorrida(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"leer corrida: %w",
				err,
			)
		}

		corrida.Paradas = make(
			[]CorridaParada,
			0,
		)

		corridasEncontradas = append(
			corridasEncontradas,
			corrida,
		)

		corridaIDs = append(
			corridaIDs,
			corrida.ID,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer corridas: %w",
			err,
		)
	}

	if len(corridaIDs) == 0 {
		return corridasEncontradas, nil
	}

	paradasPorCorrida, err := listarParadas(
		ctx,
		r.pool,
		corridaIDs,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar paradas de corridas: %w",
			err,
		)
	}

	for indice := range corridasEncontradas {
		corridaID := corridasEncontradas[indice].ID

		corridasEncontradas[indice].Paradas =
			paradasPorCorrida[corridaID]
	}

	return corridasEncontradas, nil
}

// consultarProgramacion obtiene y bloquea la programación
// utilizada para generar la corrida.
func consultarProgramacion(
	ctx context.Context,
	tx pgx.Tx,
	programacionID int64,
	fechaServicio string,
) (datosProgramacion, error) {
	const query = `
		SELECT
			p.codigo,
			p.ruta_id,
			r.codigo,
			r.nombre,
			r.activa,
			p.unidad_id,
			u.codigo,
			u.capacidad_pasajeros,
			u.activa,
			p.chofer_id,
			c.nombre_completo,
			COALESCE(
				c.activo,
				FALSE
			),
			CASE
				WHEN p.chofer_id IS NULL
					THEN TRUE
				ELSE COALESCE(
					c.licencia_numero IS NOT NULL
					AND c.licencia_vigencia IS NOT NULL
					AND c.licencia_vigencia >= $2::DATE,
					FALSE
				)
			END,
			TO_CHAR(
				p.hora_salida,
				'HH24:MI'
			),
			p.duracion_estimada_minutos,
			(
				$2::DATE >= p.vigencia_desde
				AND (
					p.vigencia_hasta IS NULL
					OR $2::DATE <= p.vigencia_hasta
				)
			),
			EXISTS (
				SELECT 1
				FROM programacion_dias pd
				WHERE pd.programacion_id = p.id
					AND pd.dia_semana =
						EXTRACT(
							ISODOW FROM $2::DATE
						)::SMALLINT
			)
		FROM programaciones p
		INNER JOIN rutas r
			ON r.id = p.ruta_id
		INNER JOIN unidades u
			ON u.id = p.unidad_id
		LEFT JOIN choferes c
			ON c.id = p.chofer_id
		WHERE p.id = $1
			AND p.activa = TRUE
		FOR UPDATE OF p;
	`

	var datos datosProgramacion

	var capacidad int16
	var choferID sql.NullInt64
	var choferNombre sql.NullString
	var duracion sql.NullInt16

	err := tx.QueryRow(
		ctx,
		query,
		programacionID,
		fechaServicio,
	).Scan(
		&datos.Codigo,
		&datos.RutaID,
		&datos.RutaCodigo,
		&datos.RutaNombre,
		&datos.RutaActiva,
		&datos.UnidadID,
		&datos.UnidadCodigo,
		&capacidad,
		&datos.UnidadActiva,
		&choferID,
		&choferNombre,
		&datos.ChoferActivo,
		&datos.LicenciaVigente,
		&datos.HoraSalida,
		&duracion,
		&datos.DentroDeVigencia,
		&datos.OperaEseDia,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return datosProgramacion{},
			ErrProgramacionNoDisponible
	}

	if err != nil {
		return datosProgramacion{}, fmt.Errorf(
			"consultar programación de corrida: %w",
			err,
		)
	}

	datos.CapacidadPasajeros = int(capacidad)
	datos.ChoferID = int64DesdeNull(choferID)
	datos.ChoferNombre = stringDesdeNull(
		choferNombre,
	)
	datos.DuracionEstimadaMinutos =
		intDesdeNullInt16(duracion)

	return datos, nil
}

func consultarEstadoParadas(
	ctx context.Context,
	tx pgx.Tx,
	rutaID int64,
) (int64, bool, error) {
	const query = `
		SELECT
			COUNT(*),
			COALESCE(
				BOOL_AND(pa.activo),
				FALSE
			)
		FROM ruta_paradas rp
		INNER JOIN puntos_abordaje pa
			ON pa.id = rp.punto_abordaje_id
		WHERE rp.ruta_id = $1;
	`

	var cantidad int64
	var todasActivas bool

	err := tx.QueryRow(
		ctx,
		query,
		rutaID,
	).Scan(
		&cantidad,
		&todasActivas,
	)
	if err != nil {
		return 0, false, fmt.Errorf(
			"consultar paradas de la ruta: %w",
			err,
		)
	}

	return cantidad, todasActivas, nil
}

func copiarParadas(
	ctx context.Context,
	tx pgx.Tx,
	corridaID int64,
	programacionID int64,
	rutaID int64,
	fechaServicio string,
	horaSalida string,
) (int64, error) {
	const query = `
		INSERT INTO corrida_paradas (
			corrida_id,
			ruta_parada_id,
			punto_abordaje_id,
			punto_nombre,
			localidad_nombre,
			estado_nombre,
			orden,
			permite_subir,
			permite_bajar,
			es_obligatoria,
			incluida_en_recorrido,
			hora_estimada_inicio,
			hora_estimada_fin
		)
		SELECT
			$1,
			rp.id,
			pa.id,
			pa.nombre,
			l.nombre,
			l.estado,
			rp.orden,
			rp.permite_subir,
			rp.permite_bajar,
			rp.es_obligatoria,
			rp.es_obligatoria,
			CASE
				WHEN h.ruta_parada_id IS NULL
					THEN NULL
				ELSE (
					(
						$4::DATE +
						$5::TIME
					) +
					MAKE_INTERVAL(
						mins =>
							h.minutos_desde_salida_inicio::INTEGER
					)
				) AT TIME ZONE
					'America/Mexico_City'
			END,
			CASE
				WHEN h.ruta_parada_id IS NULL
					THEN NULL
				ELSE (
					(
						$4::DATE +
						$5::TIME
					) +
					MAKE_INTERVAL(
						mins =>
							h.minutos_desde_salida_fin::INTEGER
					)
				) AT TIME ZONE
					'America/Mexico_City'
			END
		FROM ruta_paradas rp
		INNER JOIN puntos_abordaje pa
			ON pa.id = rp.punto_abordaje_id
		INNER JOIN localidades l
			ON l.id = pa.localidad_id
		LEFT JOIN programacion_parada_horarios h
			ON h.programacion_id = $2
			AND h.ruta_id = rp.ruta_id
			AND h.ruta_parada_id = rp.id
		WHERE rp.ruta_id = $3
		ORDER BY rp.orden;
	`

	resultado, err := tx.Exec(
		ctx,
		query,
		corridaID,
		programacionID,
		rutaID,
		fechaServicio,
		horaSalida,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"copiar paradas de la corrida: %w",
			err,
		)
	}

	return resultado.RowsAffected(), nil
}

type querier interface {
	Query(
		ctx context.Context,
		sql string,
		args ...any,
	) (pgx.Rows, error)

	QueryRow(
		ctx context.Context,
		sql string,
		args ...any,
	) pgx.Row
}

func obtenerCorridaPorID(
	ctx context.Context,
	querier querier,
	corridaID int64,
) (Corrida, error) {
	query := consultaCorridaBase + `
		WHERE c.id = $1;
	`

	corrida, err := scanCorrida(
		querier.QueryRow(
			ctx,
			query,
			corridaID,
		),
	)
	if err != nil {
		return Corrida{}, err
	}

	paradasPorCorrida, err := listarParadas(
		ctx,
		querier,
		[]int64{corridaID},
	)
	if err != nil {
		return Corrida{}, err
	}

	corrida.Paradas =
		paradasPorCorrida[corridaID]

	return corrida, nil
}

func listarParadas(
	ctx context.Context,
	querier querier,
	corridaIDs []int64,
) (map[int64][]CorridaParada, error) {
	const query = `
		SELECT
			cp.id,
			cp.corrida_id,
			cp.ruta_parada_id,
			cp.punto_abordaje_id,
			cp.punto_nombre,
			cp.localidad_nombre,
			cp.estado_nombre,
			cp.orden,
			cp.permite_subir,
			cp.permite_bajar,
			cp.es_obligatoria,
			cp.incluida_en_recorrido,
			cp.hora_estimada_inicio,
			cp.hora_estimada_fin,
			cp.creado_en,
			cp.actualizado_en
		FROM corrida_paradas cp
		WHERE cp.corrida_id =
			ANY($1::BIGINT[])
		ORDER BY
			cp.corrida_id,
			cp.orden;
	`

	rows, err := querier.Query(
		ctx,
		query,
		corridaIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resultado := make(
		map[int64][]CorridaParada,
	)

	for rows.Next() {
		parada, err := scanCorridaParada(rows)
		if err != nil {
			return nil, err
		}

		resultado[parada.CorridaID] = append(
			resultado[parada.CorridaID],
			parada,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return resultado, nil
}

type scanner interface {
	Scan(destinos ...any) error
}

func scanCorrida(
	fila scanner,
) (Corrida, error) {
	var corrida Corrida

	var programacionID sql.NullInt64
	var programacionCodigo sql.NullString
	var choferID sql.NullInt64
	var choferNombre sql.NullString
	var llegadaEstimada sql.NullTime
	var salidaReal sql.NullTime
	var llegadaReal sql.NullTime
	var capacidad int16
	var estado string
	var observaciones sql.NullString

	err := fila.Scan(
		&corrida.ID,
		&corrida.Folio,
		&programacionID,
		&programacionCodigo,
		&corrida.RutaID,
		&corrida.RutaCodigo,
		&corrida.RutaNombre,
		&corrida.UnidadID,
		&corrida.UnidadCodigo,
		&choferID,
		&choferNombre,
		&corrida.FechaServicio,
		&corrida.SalidaProgramada,
		&llegadaEstimada,
		&salidaReal,
		&llegadaReal,
		&capacidad,
		&estado,
		&corrida.ReservasAbiertas,
		&observaciones,
		&corrida.CreadoEn,
		&corrida.ActualizadoEn,
	)
	if err != nil {
		return Corrida{}, err
	}

	corrida.ProgramacionID =
		int64DesdeNull(programacionID)

	corrida.ProgramacionCodigo =
		stringDesdeNull(programacionCodigo)

	corrida.ChoferID =
		int64DesdeNull(choferID)

	corrida.ChoferNombre =
		stringDesdeNull(choferNombre)

	corrida.LlegadaEstimada =
		timeDesdeNull(llegadaEstimada)

	corrida.SalidaReal =
		timeDesdeNull(salidaReal)

	corrida.LlegadaReal =
		timeDesdeNull(llegadaReal)

	corrida.CapacidadPasajeros =
		int(capacidad)

	corrida.Estado = Estado(estado)

	corrida.Observaciones =
		stringDesdeNull(observaciones)

	return corrida, nil
}

func scanCorridaParada(
	fila scanner,
) (CorridaParada, error) {
	var parada CorridaParada

	var orden int16
	var horaInicio sql.NullTime
	var horaFin sql.NullTime

	err := fila.Scan(
		&parada.ID,
		&parada.CorridaID,
		&parada.RutaParadaID,
		&parada.PuntoAbordajeID,
		&parada.PuntoNombre,
		&parada.LocalidadNombre,
		&parada.EstadoNombre,
		&orden,
		&parada.PermiteSubir,
		&parada.PermiteBajar,
		&parada.EsObligatoria,
		&parada.IncluidaEnRecorrido,
		&horaInicio,
		&horaFin,
		&parada.CreadoEn,
		&parada.ActualizadoEn,
	)
	if err != nil {
		return CorridaParada{}, err
	}

	parada.Orden = int(orden)

	parada.HoraEstimadaInicio =
		timeDesdeNull(horaInicio)

	parada.HoraEstimadaFin =
		timeDesdeNull(horaFin)

	return parada, nil
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

func timeDesdeNull(
	valor sql.NullTime,
) *time.Time {
	if !valor.Valid {
		return nil
	}

	resultado := valor.Time

	return &resultado
}

func traducirErrorPostgres(
	err error,
) error {
	var postgresError *pgconn.PgError

	if !errors.As(err, &postgresError) {
		return fmt.Errorf(
			"insertar corrida: %w",
			err,
		)
	}

	switch {
	case postgresError.Code == "23505" &&
		postgresError.ConstraintName ==
			"uq_corridas_folio":
		return ErrCorridaDuplicada

	case postgresError.Code == "23505" &&
		postgresError.ConstraintName ==
			"uq_corridas_programacion_fecha":
		return ErrCorridaDuplicada

	default:
		return fmt.Errorf(
			"insertar corrida: %w",
			err,
		)
	}
}
