-- +goose Up

-- =========================================================
-- PASAJEROS
-- =========================================================

CREATE TABLE pasajeros (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre_completo VARCHAR(150) NOT NULL,
    telefono VARCHAR(30) NOT NULL,
    correo VARCHAR(254),
    notas TEXT,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_pasajeros_nombre_no_vacio
        CHECK (
            BTRIM(nombre_completo) <> ''
        ),

    CONSTRAINT ck_pasajeros_telefono_no_vacio
        CHECK (
            BTRIM(telefono) <> ''
        ),

    CONSTRAINT ck_pasajeros_correo_no_vacio
        CHECK (
            correo IS NULL
            OR BTRIM(correo) <> ''
        ),

    CONSTRAINT ck_pasajeros_notas_no_vacias
        CHECK (
            notas IS NULL
            OR BTRIM(notas) <> ''
        )
);

-- El teléfono no es único porque familiares o acompañantes
-- podrían utilizar el mismo número.
CREATE INDEX ix_pasajeros_telefono
    ON pasajeros (
        telefono
    );

CREATE INDEX ix_pasajeros_nombre
    ON pasajeros (
        LOWER(BTRIM(nombre_completo))
    );

-- =========================================================
-- SOPORTE PARA RELACIONAR LAS PARADAS CON SU CORRIDA
-- =========================================================

-- Esta restricción permite crear llaves foráneas compuestas
-- y garantiza que el origen y destino pertenezcan a la misma
-- corrida registrada en la reserva.
ALTER TABLE corrida_paradas
    ADD CONSTRAINT uq_corrida_paradas_corrida_id_id
    UNIQUE (
        corrida_id,
        id
    );

-- =========================================================
-- FOLIOS DE RESERVA
-- =========================================================

CREATE SEQUENCE reservas_folio_seq
    AS BIGINT
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

-- =========================================================
-- RESERVAS
-- =========================================================

CREATE TABLE reservas (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    folio VARCHAR(30) NOT NULL DEFAULT (
        'RES-' ||
        LPAD(
            NEXTVAL('reservas_folio_seq')::TEXT,
            8,
            '0'
        )
    ),

    corrida_id BIGINT NOT NULL,
    pasajero_id BIGINT NOT NULL,

    -- Son IDs de corrida_paradas, no de puntos_abordaje.
    -- De esa forma conservamos la parada específica de esta
    -- ejecución y no solamente el catálogo general.
    corrida_parada_origen_id BIGINT NOT NULL,
    corrida_parada_destino_id BIGINT NOT NULL,

    cantidad_pasajeros SMALLINT NOT NULL,
    precio_unitario NUMERIC(12, 2) NOT NULL,
    subtotal NUMERIC(12, 2) NOT NULL,

    -- El descuento es opcional.
    --
    -- PORCENTAJE:
    -- valor_descuento guarda, por ejemplo, 10.00.
    --
    -- MONTO_FIJO:
    -- valor_descuento guarda directamente el importe.
    tipo_descuento VARCHAR(20),
    valor_descuento NUMERIC(12, 2) NOT NULL DEFAULT 0,

    -- Indica a cuántos pasajes se aplica el descuento.
    --
    -- Esto permite que un descuento de lealtad se aplique
    -- solamente al pasaje de quien presenta la tarjeta,
    -- aunque la reserva incluya más personas.
    cantidad_pasajes_descuento SMALLINT NOT NULL DEFAULT 0,

    monto_descuento NUMERIC(12, 2) NOT NULL DEFAULT 0,
    descripcion_descuento VARCHAR(250),
    total NUMERIC(12, 2) NOT NULL,

    estado VARCHAR(30) NOT NULL DEFAULT 'APARTADA',

    -- Una reserva sin anticipo necesitará confirmación.
    -- Una reserva con pago podrá marcarse como confirmada
    -- automáticamente desde el Service/Repository.
    requiere_confirmacion BOOLEAN NOT NULL DEFAULT TRUE,
    confirmada_en TIMESTAMPTZ,
    confirmacion_solicitada_en TIMESTAMPTZ,
    confirmacion_limite_en TIMESTAMPTZ,

    cancelada_en TIMESTAMPTZ,
    motivo_cancelacion VARCHAR(500),

    observaciones TEXT,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_reservas_corrida
        FOREIGN KEY (corrida_id)
        REFERENCES corridas (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_reservas_pasajero
        FOREIGN KEY (pasajero_id)
        REFERENCES pasajeros (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_reservas_origen_corrida
        FOREIGN KEY (
            corrida_id,
            corrida_parada_origen_id
        )
        REFERENCES corrida_paradas (
            corrida_id,
            id
        )
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_reservas_destino_corrida
        FOREIGN KEY (
            corrida_id,
            corrida_parada_destino_id
        )
        REFERENCES corrida_paradas (
            corrida_id,
            id
        )
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT ck_reservas_folio_no_vacio
        CHECK (
            BTRIM(folio) <> ''
        ),

    CONSTRAINT ck_reservas_cantidad_pasajeros
        CHECK (
            cantidad_pasajeros > 0
        ),

    CONSTRAINT ck_reservas_precio_unitario
        CHECK (
            precio_unitario > 0
        ),

    CONSTRAINT ck_reservas_subtotal
        CHECK (
            subtotal =
                precio_unitario * cantidad_pasajeros
        ),

    CONSTRAINT ck_reservas_tipo_descuento
        CHECK (
            tipo_descuento IS NULL
            OR tipo_descuento IN (
                'PORCENTAJE',
                'MONTO_FIJO'
            )
        ),

    CONSTRAINT ck_reservas_descuento_completo
        CHECK (
            (
                tipo_descuento IS NULL
                AND valor_descuento = 0
                AND cantidad_pasajes_descuento = 0
                AND monto_descuento = 0
                AND descripcion_descuento IS NULL
            )
            OR
            (
                tipo_descuento IS NOT NULL
                AND valor_descuento > 0
                AND cantidad_pasajes_descuento > 0
                AND cantidad_pasajes_descuento <=
                    cantidad_pasajeros
                AND monto_descuento > 0
                AND monto_descuento <= subtotal
                AND descripcion_descuento IS NOT NULL
                AND BTRIM(descripcion_descuento) <> ''
            )
        ),

    CONSTRAINT ck_reservas_descuento_porcentaje
        CHECK (
            tipo_descuento <> 'PORCENTAJE'
            OR (
                valor_descuento <= 100
                AND monto_descuento = ROUND(
                    (
                        precio_unitario *
                        cantidad_pasajes_descuento *
                        valor_descuento
                    ) / 100,
                    2
                )
            )
        ),

    CONSTRAINT ck_reservas_descuento_monto_fijo
        CHECK (
            tipo_descuento <> 'MONTO_FIJO'
            OR (
                monto_descuento = valor_descuento
                AND monto_descuento <=
                    (
                        precio_unitario *
                        cantidad_pasajes_descuento
                    )
            )
        ),

    CONSTRAINT ck_reservas_total
        CHECK (
            total = subtotal - monto_descuento
            AND total >= 0
        ),

    CONSTRAINT ck_reservas_estado
        CHECK (
            estado IN (
                'APARTADA',
                'CONFIRMADA',
                'CANCELADA',
                'ABORDADA',
                'NO_PRESENTADA'
            )
        ),

    CONSTRAINT ck_reservas_confirmacion_solicitud
        CHECK (
            (
                confirmacion_solicitada_en IS NULL
                AND confirmacion_limite_en IS NULL
            )
            OR
            (
                requiere_confirmacion = TRUE
                AND confirmacion_solicitada_en IS NOT NULL
                AND confirmacion_limite_en IS NOT NULL
                AND confirmacion_limite_en >
                    confirmacion_solicitada_en
            )
        ),

    CONSTRAINT ck_reservas_confirmada
        CHECK (
            estado <> 'CONFIRMADA'
            OR (
                confirmada_en IS NOT NULL
                AND requiere_confirmacion = FALSE
            )
        ),

    CONSTRAINT ck_reservas_cancelacion
        CHECK (
            (
                estado = 'CANCELADA'
                AND cancelada_en IS NOT NULL
                AND motivo_cancelacion IS NOT NULL
                AND BTRIM(motivo_cancelacion) <> ''
            )
            OR
            (
                estado <> 'CANCELADA'
                AND cancelada_en IS NULL
                AND motivo_cancelacion IS NULL
            )
        ),

    CONSTRAINT ck_reservas_observaciones
        CHECK (
            observaciones IS NULL
            OR BTRIM(observaciones) <> ''
        )
);

CREATE UNIQUE INDEX uq_reservas_folio
    ON reservas (
        LOWER(BTRIM(folio))
    );

CREATE INDEX ix_reservas_corrida_estado
    ON reservas (
        corrida_id,
        estado
    );

CREATE INDEX ix_reservas_pasajero
    ON reservas (
        pasajero_id,
        creado_en DESC
    );

CREATE INDEX ix_reservas_tramo
    ON reservas (
        corrida_id,
        corrida_parada_origen_id,
        corrida_parada_destino_id
    );

CREATE INDEX ix_reservas_confirmacion_pendiente
    ON reservas (
        confirmacion_limite_en
    )
    WHERE requiere_confirmacion = TRUE
        AND estado = 'APARTADA';

-- =========================================================
-- PAGOS DE RESERVA
-- =========================================================

CREATE TABLE pagos_reserva (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    reserva_id BIGINT NOT NULL,
    monto NUMERIC(12, 2) NOT NULL,
    metodo VARCHAR(20) NOT NULL,
    referencia VARCHAR(150),
    estado VARCHAR(15) NOT NULL DEFAULT 'APLICADO',
    pagado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    anulado_en TIMESTAMPTZ,
    motivo_anulacion VARCHAR(500),
    notas TEXT,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_pagos_reserva_reserva
        FOREIGN KEY (reserva_id)
        REFERENCES reservas (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT ck_pagos_reserva_monto
        CHECK (
            monto > 0
        ),

    CONSTRAINT ck_pagos_reserva_metodo
        CHECK (
            metodo IN (
                'EFECTIVO',
                'TRANSFERENCIA',
                'TARJETA',
                'OTRO'
            )
        ),

    CONSTRAINT ck_pagos_reserva_referencia
        CHECK (
            referencia IS NULL
            OR BTRIM(referencia) <> ''
        ),

    CONSTRAINT ck_pagos_reserva_estado
        CHECK (
            estado IN (
                'APLICADO',
                'ANULADO'
            )
        ),

    CONSTRAINT ck_pagos_reserva_anulacion
        CHECK (
            (
                estado = 'APLICADO'
                AND anulado_en IS NULL
                AND motivo_anulacion IS NULL
            )
            OR
            (
                estado = 'ANULADO'
                AND anulado_en IS NOT NULL
                AND motivo_anulacion IS NOT NULL
                AND BTRIM(motivo_anulacion) <> ''
            )
        ),

    CONSTRAINT ck_pagos_reserva_notas
        CHECK (
            notas IS NULL
            OR BTRIM(notas) <> ''
        )
);

CREATE INDEX ix_pagos_reserva_reserva
    ON pagos_reserva (
        reserva_id,
        pagado_en
    );

CREATE INDEX ix_pagos_reserva_aplicados
    ON pagos_reserva (
        reserva_id
    )
    WHERE estado = 'APLICADO';

-- =========================================================
-- VISTA DE SALDOS
-- =========================================================

CREATE VIEW vw_reservas_saldos AS
SELECT
    resumen.reserva_id,
    resumen.total,
    resumen.monto_pagado,
    GREATEST(
        resumen.total - resumen.monto_pagado,
        0
    ) AS saldo_pendiente,

    CASE
        WHEN resumen.monto_pagado = 0
            THEN 'SIN_PAGO'

        WHEN resumen.monto_pagado < resumen.total
            THEN 'PAGO_PARCIAL'

        WHEN resumen.monto_pagado = resumen.total
            THEN 'PAGADA'

        ELSE 'SALDO_A_FAVOR'
    END AS estado_pago

FROM (
    SELECT
        r.id AS reserva_id,
        r.total,

        COALESCE(
            SUM(p.monto) FILTER (
                WHERE p.estado = 'APLICADO'
            ),
            0
        ) AS monto_pagado

    FROM reservas r

    LEFT JOIN pagos_reserva p
        ON p.reserva_id = r.id

    GROUP BY
        r.id,
        r.total
) AS resumen;

-- +goose Down

DROP VIEW vw_reservas_saldos;
DROP TABLE pagos_reserva;
DROP TABLE reservas;
DROP SEQUENCE reservas_folio_seq;
DROP TABLE pasajeros;

ALTER TABLE corrida_paradas
    DROP CONSTRAINT uq_corrida_paradas_corrida_id_id;