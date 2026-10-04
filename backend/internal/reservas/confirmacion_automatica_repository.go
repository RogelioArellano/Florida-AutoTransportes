package reservas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var _ ConfirmacionAutomaticaStore = (*PostgresRepository)(nil)

type datosSolicitudConfirmacion struct {
	Estado                   Estado
	RequiereConfirmacion     bool
	ConfirmacionSolicitadaEn sql.NullTime
	ConfirmacionLimiteEn     sql.NullTime

	EstadoCorrida     string
	ReservasAbiertas  bool
	TienePagos        bool
	DentroDeLaVentana bool
}

const consultaConfirmacionPendienteBase = `
	SELECT
		r.id,
		r.folio,

		p.id,
		p.nombre_completo,
		p.telefono,

		c.id,
		c.folio,
		c.salida_programada,

		origen.punto_nombre,
		destino.punto_nombre,
		r.cantidad_pasajeros,

		r.confirmacion_solicitada_en,
		r.confirmacion_limite_en

	FROM reservas r

	INNER JOIN pasajeros p
		ON p.id = r.pasajero_id

	INNER JOIN corridas c
		ON c.id = r.corrida_id

	INNER JOIN corrida_paradas origen
		ON origen.id =
			r.corrida_parada_origen_id
		AND origen.corrida_id =
			r.corrida_id

	INNER JOIN corrida_paradas destino
		ON destino.id =
			r.corrida_parada_destino_id
		AND destino.corrida_id =
			r.corrida_id
`

// ListPendingConfirmations obtiene las reservas sin anticipo
// que ya entraron a la ventana de confirmación y cuyo mensaje
// todavía no ha sido enviado.
//
// La consulta utiliza NOW() de PostgreSQL para que el cálculo
// no dependa del reloj ni de la zona horaria de la API.
func (r *PostgresRepository) ListPendingConfirmations(
	ctx context.Context,
	politica PoliticaConfirmacionParams,
) ([]ConfirmacionPendiente, error) {
	query := consultaConfirmacionPendienteBase + `
		WHERE r.estado = 'APARTADA'
			AND r.requiere_confirmacion = TRUE
			AND r.confirmacion_solicitada_en IS NULL
			AND r.confirmacion_limite_en IS NULL

			AND c.estado = 'PROGRAMADA'
			AND c.reservas_abiertas = TRUE

			AND NOW() >= (
				c.salida_programada
					- (
						$1::INTEGER
						* INTERVAL '1 hour'
					)
			)
			AND NOW() < c.salida_programada

			AND NOT EXISTS (
				SELECT 1
				FROM pagos_reserva pr
				WHERE pr.reserva_id = r.id
					AND pr.estado = 'APLICADO'
			)

		ORDER BY
			c.salida_programada,
			r.id;
	`

	rows, err := r.pool.Query(
		ctx,
		query,
		politica.HorasAnticipacion,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"consultar confirmaciones pendientes: %w",
			err,
		)
	}
	defer rows.Close()

	pendientes := make([]ConfirmacionPendiente, 0)

	for rows.Next() {
		pendiente, err :=
			scanConfirmacionPendiente(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"leer confirmación pendiente: %w",
				err,
			)
		}

		pendientes = append(
			pendientes,
			pendiente,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"recorrer confirmaciones pendientes: %w",
			err,
		)
	}

	return pendientes, nil
}

// RequestConfirmation registra el momento en que el mensaje
// fue enviado correctamente y establece su límite de
// respuesta.
//
// La reserva se bloquea para impedir que un pago, una
// confirmación manual o una cancelación cambien su estado
// mientras se registra la solicitud.
func (r *PostgresRepository) RequestConfirmation(
	ctx context.Context,
	params SolicitarConfirmacionParams,
) (ConfirmacionPendiente, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ConfirmacionPendiente{}, fmt.Errorf(
			"iniciar transacción de solicitud de confirmación: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	datos, err := consultarReservaParaSolicitudConfirmacion(
		ctx,
		tx,
		params.Input.ReservaID,
		params.HorasAnticipacion,
	)
	if err != nil {
		return ConfirmacionPendiente{}, err
	}

	// La operación es idempotente. Una repetición devuelve la
	// solicitud original sin cambiar la fecha ni extender el
	// límite de respuesta.
	if datos.ConfirmacionSolicitadaEn.Valid {
		resultado, err :=
			obtenerConfirmacionPendientePorID(
				ctx,
				tx,
				params.Input.ReservaID,
			)
		if err != nil {
			return ConfirmacionPendiente{}, fmt.Errorf(
				"consultar solicitud de confirmación existente: %w",
				err,
			)
		}

		if err := tx.Commit(ctx); err != nil {
			return ConfirmacionPendiente{}, fmt.Errorf(
				"confirmar consulta de solicitud existente: %w",
				err,
			)
		}

		return resultado, nil
	}

	if datos.Estado != EstadoApartada ||
		!datos.RequiereConfirmacion ||
		datos.EstadoCorrida != "PROGRAMADA" ||
		!datos.ReservasAbiertas ||
		datos.TienePagos {
		return ConfirmacionPendiente{}, fmt.Errorf(
			"%w: estado de reserva %s",
			ErrReservaNoAceptaSolicitudConfirmacion,
			datos.Estado,
		)
	}

	if !datos.DentroDeLaVentana {
		return ConfirmacionPendiente{},
			ErrSolicitudConfirmacionFueraDeVentana
	}

	if err := actualizarSolicitudConfirmacion(
		ctx,
		tx,
		params.Input.ReservaID,
		params.MinutosRespuesta,
	); err != nil {
		return ConfirmacionPendiente{}, err
	}

	resultado, err :=
		obtenerConfirmacionPendientePorID(
			ctx,
			tx,
			params.Input.ReservaID,
		)
	if err != nil {
		return ConfirmacionPendiente{}, fmt.Errorf(
			"consultar solicitud de confirmación registrada: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return ConfirmacionPendiente{}, fmt.Errorf(
			"confirmar solicitud de confirmación: %w",
			err,
		)
	}

	return resultado, nil
}

// consultarReservaParaSolicitudConfirmacion bloquea la
// reserva y obtiene todas las condiciones necesarias para
// aplicar la política de confirmación.
func consultarReservaParaSolicitudConfirmacion(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
	horasAnticipacion int,
) (datosSolicitudConfirmacion, error) {
	const query = `
		SELECT
			r.estado,
			r.requiere_confirmacion,
			r.confirmacion_solicitada_en,
			r.confirmacion_limite_en,

			c.estado,
			c.reservas_abiertas,

			EXISTS (
				SELECT 1
				FROM pagos_reserva pr
				WHERE pr.reserva_id = r.id
					AND pr.estado = 'APLICADO'
			),

			NOW() >= (
				c.salida_programada
					- (
						$2::INTEGER
						* INTERVAL '1 hour'
					)
			)
			AND NOW() < c.salida_programada

		FROM reservas r

		INNER JOIN corridas c
			ON c.id = r.corrida_id

		WHERE r.id = $1

		FOR UPDATE OF r;
	`

	var datos datosSolicitudConfirmacion
	var estadoReserva string

	err := tx.QueryRow(
		ctx,
		query,
		reservaID,
		horasAnticipacion,
	).Scan(
		&estadoReserva,
		&datos.RequiereConfirmacion,
		&datos.ConfirmacionSolicitadaEn,
		&datos.ConfirmacionLimiteEn,
		&datos.EstadoCorrida,
		&datos.ReservasAbiertas,
		&datos.TienePagos,
		&datos.DentroDeLaVentana,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return datosSolicitudConfirmacion{},
			ErrReservaNoEncontrada
	}

	if err != nil {
		return datosSolicitudConfirmacion{}, fmt.Errorf(
			"consultar reserva para solicitar confirmación: %w",
			err,
		)
	}

	datos.Estado = Estado(estadoReserva)

	return datos, nil
}

func actualizarSolicitudConfirmacion(
	ctx context.Context,
	tx pgx.Tx,
	reservaID int64,
	minutosRespuesta int,
) error {
	const query = `
		UPDATE reservas
		SET
			confirmacion_solicitada_en = NOW(),
			confirmacion_limite_en =
				NOW()
					+ (
						$2::INTEGER
						* INTERVAL '1 minute'
					),
			actualizado_en = NOW()
		WHERE id = $1;
	`

	_, err := tx.Exec(
		ctx,
		query,
		reservaID,
		minutosRespuesta,
	)
	if err != nil {
		return fmt.Errorf(
			"actualizar solicitud de confirmación: %w",
			err,
		)
	}

	return nil
}

func obtenerConfirmacionPendientePorID(
	ctx context.Context,
	querier querier,
	reservaID int64,
) (ConfirmacionPendiente, error) {
	query := consultaConfirmacionPendienteBase + `
		WHERE r.id = $1;
	`

	resultado, err := scanConfirmacionPendiente(
		querier.QueryRow(
			ctx,
			query,
			reservaID,
		),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConfirmacionPendiente{},
			ErrReservaNoEncontrada
	}

	if err != nil {
		return ConfirmacionPendiente{}, err
	}

	return resultado, nil
}

func scanConfirmacionPendiente(
	fila scanner,
) (ConfirmacionPendiente, error) {
	var pendiente ConfirmacionPendiente
	var cantidadPasajeros int16
	var confirmacionSolicitadaEn sql.NullTime
	var confirmacionLimiteEn sql.NullTime

	err := fila.Scan(
		&pendiente.ReservaID,
		&pendiente.ReservaFolio,
		&pendiente.PasajeroID,
		&pendiente.PasajeroNombre,
		&pendiente.PasajeroTelefono,
		&pendiente.CorridaID,
		&pendiente.CorridaFolio,
		&pendiente.SalidaProgramada,
		&pendiente.ParadaOrigenNombre,
		&pendiente.ParadaDestinoNombre,
		&cantidadPasajeros,
		&confirmacionSolicitadaEn,
		&confirmacionLimiteEn,
	)
	if err != nil {
		return ConfirmacionPendiente{}, err
	}

	pendiente.CantidadPasajeros =
		int(cantidadPasajeros)

	pendiente.ConfirmacionSolicitadaEn =
		timeDesdeNull(confirmacionSolicitadaEn)

	pendiente.ConfirmacionLimiteEn =
		timeDesdeNull(confirmacionLimiteEn)

	return pendiente, nil
}
