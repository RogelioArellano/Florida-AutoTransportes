-- +goose Up

-- La asistencia pertenece a la reserva y admite grupos.
-- No modifica precios, pagos, descuentos ni reembolsos.
ALTER TABLE reservas
    ADD COLUMN cantidad_abordada SMALLINT,
    ADD COLUMN abordada_en TIMESTAMPTZ,
    ADD COLUMN asistencia_cerrada_en TIMESTAMPTZ,
    ADD COLUMN observaciones_asistencia TEXT;

-- Las reservas que todavía no abordaron comienzan en cero.
-- Para estados históricos ABORDADA / NO_PRESENTADA dejamos
-- NULL: el sistema anterior no guardaba cantidades ni fechas.
UPDATE reservas
SET cantidad_abordada = 0
WHERE estado IN ('APARTADA', 'CONFIRMADA', 'CANCELADA');

-- Las reservas nuevas comienzan con cero abordajes.
ALTER TABLE reservas
    ALTER COLUMN cantidad_abordada SET DEFAULT 0;

ALTER TABLE reservas
    ADD CONSTRAINT ck_reservas_asistencia_cantidad
    CHECK (
        cantidad_abordada IS NULL
        OR cantidad_abordada BETWEEN 0 AND cantidad_pasajeros
    ),

    ADD CONSTRAINT ck_reservas_asistencia_estado
    CHECK (
        -- Compatibilidad con asistencia histórica desconocida.
        (
            cantidad_abordada IS NULL
            AND estado IN ('ABORDADA', 'NO_PRESENTADA')
            AND abordada_en IS NULL
            AND asistencia_cerrada_en IS NULL
            AND observaciones_asistencia IS NULL
        )
        OR
        (
            cantidad_abordada IS NOT NULL
            AND (
                -- Sin abordajes ni cierre de asistencia.
                (
                    estado IN ('APARTADA', 'CONFIRMADA', 'CANCELADA')
                    AND cantidad_abordada = 0
                    AND abordada_en IS NULL
                    AND asistencia_cerrada_en IS NULL
                )
                OR
                -- Al menos una persona abordó. El cierre es opcional.
                (
                    estado = 'ABORDADA'
                    AND cantidad_abordada > 0
                    AND abordada_en IS NOT NULL
                )
                OR
                -- Ninguna persona abordó y la asistencia ya se cerró.
                (
                    estado = 'NO_PRESENTADA'
                    AND cantidad_abordada = 0
                    AND abordada_en IS NULL
                    AND asistencia_cerrada_en IS NOT NULL
                )
            )
        )
    ),

    ADD CONSTRAINT ck_reservas_asistencia_fechas
    CHECK (
        asistencia_cerrada_en IS NULL
        OR abordada_en IS NULL
        OR asistencia_cerrada_en >= abordada_en
    ),

    ADD CONSTRAINT ck_reservas_asistencia_observaciones
    CHECK (
        observaciones_asistencia IS NULL
        OR BTRIM(observaciones_asistencia) <> ''
    );

COMMENT ON COLUMN reservas.cantidad_abordada IS
    'Personas que abordaron; NULL indica asistencia histórica desconocida.';

COMMENT ON COLUMN reservas.abordada_en IS
    'Fecha del primer abordaje de la reserva.';

COMMENT ON COLUMN reservas.asistencia_cerrada_en IS
    'Al cerrar, las ausencias son cantidad_pasajeros menos cantidad_abordada.';

-- +goose Down

ALTER TABLE reservas
    DROP CONSTRAINT ck_reservas_asistencia_observaciones,
    DROP CONSTRAINT ck_reservas_asistencia_fechas,
    DROP CONSTRAINT ck_reservas_asistencia_estado,
    DROP CONSTRAINT ck_reservas_asistencia_cantidad;

ALTER TABLE reservas
    DROP COLUMN observaciones_asistencia,
    DROP COLUMN asistencia_cerrada_en,
    DROP COLUMN abordada_en,
    DROP COLUMN cantidad_abordada;
