package reservas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var _ ListaPasajerosEnvioStore = (*PostgresRepository)(nil)

// ListPendingPassengerLists obtiene corridas que:
//
//   - están PROGRAMADAS;
//   - tienen un chofer activo;
//   - tienen reservaciones activas;
//   - se encuentran dentro de los 60 minutos previos;
//   - no fueron enviadas al chofer actual.
func (r *PostgresRepository) ListPendingPassengerLists(
	ctx context.Context,
	politica PoliticaEnvioListaPasajerosParams,
) ([]CorridaPendienteListaPasajeros, error) {
	const query = `
		SELECT
			c.id,
			c.folio,
			c.ruta_id,
			c.ruta_codigo,
			c.ruta_nombre,
			TO_CHAR(
				c.fecha_servicio,
				'YYYY-MM-DD'
			),
			c.salida_programada,

			c.unidad_id,
			c.unidad_codigo,
			u.placas,

			c.chofer_id,
			c.chofer_nombre,
			ch.telefono,

			COUNT(r.id)::BIGINT,
			COALESCE(
				SUM(r.cantidad_pasajeros),
				0
			)::BIGINT,

			c.lista_pasajeros_enviada_en,
			c.lista_pasajeros_enviada_chofer_id

		FROM corridas c

		INNER JOIN unidades u
			ON u.id = c.unidad_id

		INNER JOIN choferes ch
			ON ch.id = c.chofer_id
			AND ch.activo = TRUE

		INNER JOIN reservas r
			ON r.corrida_id = c.id
			AND r.estado IN (
				'APARTADA',
				'CONFIRMADA',
				'ABORDADA'
			)

		WHERE c.estado = 'PROGRAMADA'
			AND c.chofer_id IS NOT NULL

			AND NOW() >= (
				c.salida_programada
					- (
						$1::INTEGER
						* INTERVAL '1 minute'
					)
			)

			AND NOW() < c.salida_programada

			-- NULL IS DISTINCT FROM un ID produce TRUE.
			-- También produce TRUE cuando cambió el chofer.
			AND
				c.lista_pasajeros_enviada_chofer_id
				IS DISTINCT FROM c.chofer_id

		GROUP BY
			c.id,
			c.folio,
			c.ruta_id,
			c.ruta_codigo,
			c.ruta_nombre,
			c.fecha_servicio,
			c.salida_programada,

			c.unidad_id,
			c.unidad_codigo,
			u.placas,

			c.chofer_id,
			c.chofer_nombre,
			ch.telefono,

			c.lista_pasajeros_enviada_en,
			c.lista_pasajeros_enviada_chofer_id

		ORDER BY
			c.salida_programada,
			c.id;
	`

	rows, err := r.pool.Query(
		ctx,
		query,
		politica.MinutosAnticipacion,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar listas de pasajeros pendientes: %w",
			err,
		)
	}
	defer rows.Close()

	pendientes := make(
		[]CorridaPendienteListaPasajeros,
		0,
	)

	for rows.Next() {
		var pendiente CorridaPendienteListaPasajeros

		var placas sql.NullString
		var totalReservas int64
		var totalPasajeros int64

		var ultimoEnvioEn sql.NullTime
		var ultimoEnvioChoferID sql.NullInt64

		err := rows.Scan(
			&pendiente.CorridaID,
			&pendiente.CorridaFolio,
			&pendiente.RutaID,
			&pendiente.RutaCodigo,
			&pendiente.RutaNombre,
			&pendiente.FechaServicio,
			&pendiente.SalidaProgramada,

			&pendiente.UnidadID,
			&pendiente.UnidadCodigo,
			&placas,

			&pendiente.ChoferID,
			&pendiente.ChoferNombre,
			&pendiente.ChoferTelefono,

			&totalReservas,
			&totalPasajeros,

			&ultimoEnvioEn,
			&ultimoEnvioChoferID,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"leer lista de pasajeros pendiente: %w",
				err,
			)
		}

		pendiente.UnidadPlacas =
			stringDesdeNull(placas)

		pendiente.TotalReservas =
			int(totalReservas)

		pendiente.TotalPasajerosRegistrados =
			int(totalPasajeros)

		pendiente.UltimoEnvioEn =
			timeDesdeNull(ultimoEnvioEn)

		if ultimoEnvioChoferID.Valid {
			valor := ultimoEnvioChoferID.Int64

			pendiente.UltimoEnvioChoferID =
				&valor
		}

		pendientes = append(
			pendientes,
			pendiente,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer listas de pasajeros pendientes: %w",
			err,
		)
	}

	return pendientes, nil
}

// datosRegistroEnvioLista contiene los datos consultados bajo
// bloqueo antes de registrar un envío.
type datosRegistroEnvioLista struct {
	CorridaFolio string
	Estado       string

	ChoferID       sql.NullInt64
	ChoferNombre   sql.NullString
	ChoferTelefono sql.NullString
	ChoferActivo   bool

	UltimoEnvioEn       sql.NullTime
	UltimoEnvioChoferID sql.NullInt64

	DentroDeVentana bool
	TieneReservas   bool
}

// RegisterPassengerListDelivery registra un envío exitoso.
//
// La corrida se bloquea para impedir que cambie el chofer
// entre la validación y la actualización.
func (r *PostgresRepository) RegisterPassengerListDelivery(
	ctx context.Context,
	input RegistrarEnvioListaPasajerosInput,
	politica PoliticaEnvioListaPasajerosParams,
) (EnvioListaPasajeros, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return EnvioListaPasajeros{}, fmt.Errorf(
			"iniciar registro de envío de lista: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	datos, err := consultarCorridaParaRegistrarEnvioLista(
		ctx,
		tx,
		input.CorridaID,
		politica.MinutosAnticipacion,
	)
	if err != nil {
		return EnvioListaPasajeros{}, err
	}

	if !datos.ChoferID.Valid {
		return EnvioListaPasajeros{},
			ErrCorridaSinChoferLista
	}

	// Es importante comparar el chofer enviado por n8n contra
	// el que actualmente tiene asignado la corrida.
	if datos.ChoferID.Int64 != input.ChoferID {
		return EnvioListaPasajeros{},
			ErrChoferListaPasajerosCambio
	}

	if !datos.ChoferActivo ||
		!datos.ChoferNombre.Valid ||
		!datos.ChoferTelefono.Valid {
		return EnvioListaPasajeros{},
			ErrCorridaNoAceptaEnvioLista
	}

	// La operación es idempotente. Si n8n repite el registro
	// para el mismo chofer, conservamos la fecha original.
	if datos.UltimoEnvioChoferID.Valid &&
		datos.UltimoEnvioChoferID.Int64 ==
			input.ChoferID &&
		datos.UltimoEnvioEn.Valid {
		resultado := construirEnvioListaPasajeros(
			input.CorridaID,
			datos,
			datos.UltimoEnvioEn.Time,
		)

		if err := tx.Commit(ctx); err != nil {
			return EnvioListaPasajeros{}, fmt.Errorf(
				"finalizar consulta de envío existente: %w",
				err,
			)
		}

		return resultado, nil
	}

	if datos.Estado != "PROGRAMADA" {
		return EnvioListaPasajeros{},
			ErrCorridaNoAceptaEnvioLista
	}

	if !datos.DentroDeVentana {
		return EnvioListaPasajeros{},
			ErrEnvioListaFueraDeVentana
	}

	if !datos.TieneReservas {
		return EnvioListaPasajeros{},
			ErrCorridaSinReservasLista
	}

	const actualizar = `
		UPDATE corridas
		SET
			lista_pasajeros_enviada_en = NOW(),
			lista_pasajeros_enviada_chofer_id = $2,
			actualizado_en = NOW()
		WHERE id = $1
		RETURNING lista_pasajeros_enviada_en;
	`

	var enviadaEn sql.NullTime

	err = tx.QueryRow(
		ctx,
		actualizar,
		input.CorridaID,
		input.ChoferID,
	).Scan(&enviadaEn)
	if err != nil {
		return EnvioListaPasajeros{}, fmt.Errorf(
			"registrar envío de lista de pasajeros: %w",
			err,
		)
	}

	if !enviadaEn.Valid {
		return EnvioListaPasajeros{}, errors.New(
			"la fecha del envío no fue registrada",
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return EnvioListaPasajeros{}, fmt.Errorf(
			"confirmar registro de envío de lista: %w",
			err,
		)
	}

	return construirEnvioListaPasajeros(
		input.CorridaID,
		datos,
		enviadaEn.Time,
	), nil
}

// consultarCorridaParaRegistrarEnvioLista bloquea la corrida y
// calcula las condiciones utilizando el reloj de PostgreSQL.
func consultarCorridaParaRegistrarEnvioLista(
	ctx context.Context,
	tx pgx.Tx,
	corridaID int64,
	minutosAnticipacion int,
) (datosRegistroEnvioLista, error) {
	const query = `
		SELECT
			c.folio,
			c.estado,

			c.chofer_id,
			c.chofer_nombre,
			ch.telefono,
			COALESCE(ch.activo, FALSE),

			c.lista_pasajeros_enviada_en,
			c.lista_pasajeros_enviada_chofer_id,

			(
				NOW() >= (
					c.salida_programada
						- (
							$2::INTEGER
							* INTERVAL '1 minute'
						)
				)
				AND NOW() < c.salida_programada
			),

			EXISTS (
				SELECT 1
				FROM reservas r
				WHERE r.corrida_id = c.id
					AND r.estado IN (
						'APARTADA',
						'CONFIRMADA',
						'ABORDADA'
					)
			)

		FROM corridas c

		LEFT JOIN choferes ch
			ON ch.id = c.chofer_id

		WHERE c.id = $1

		FOR UPDATE OF c;
	`

	var datos datosRegistroEnvioLista

	err := tx.QueryRow(
		ctx,
		query,
		corridaID,
		minutosAnticipacion,
	).Scan(
		&datos.CorridaFolio,
		&datos.Estado,

		&datos.ChoferID,
		&datos.ChoferNombre,
		&datos.ChoferTelefono,
		&datos.ChoferActivo,

		&datos.UltimoEnvioEn,
		&datos.UltimoEnvioChoferID,

		&datos.DentroDeVentana,
		&datos.TieneReservas,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return datosRegistroEnvioLista{},
			ErrCorridaNoEncontrada
	}

	if err != nil {
		return datosRegistroEnvioLista{}, fmt.Errorf(
			"consultar corrida para registrar envío de lista: %w",
			err,
		)
	}

	return datos, nil
}

func construirEnvioListaPasajeros(
	corridaID int64,
	datos datosRegistroEnvioLista,
	enviadaEn time.Time,
) EnvioListaPasajeros {
	return EnvioListaPasajeros{
		CorridaID:    corridaID,
		CorridaFolio: datos.CorridaFolio,

		ChoferID:       datos.ChoferID.Int64,
		ChoferNombre:   datos.ChoferNombre.String,
		ChoferTelefono: datos.ChoferTelefono.String,

		EnviadaEn: enviadaEn,
	}
}
