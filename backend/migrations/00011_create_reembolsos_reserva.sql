-- +goose Up

-- =========================================================
-- FOLIOS DE REEMBOLSO
-- =========================================================

CREATE SEQUENCE reembolsos_reserva_folio_seq;


-- =========================================================
-- REEMBOLSOS DE RESERVA
-- =========================================================
--
-- Esta tabla representa una salida real de dinero.
--
-- No debemos cambiar el pago original a ANULADO porque:
--
--   1. El pago sí ocurrió.
--   2. El dinero sí ingresó al negocio.
--   3. Posteriormente ocurrió otra operación distinta:
--      la devolución al pasajero.
--
-- Una reserva puede tener más de un reembolso. Esto permite,
-- por ejemplo, devolver $100 primero y $170 posteriormente.
-- =========================================================

CREATE TABLE reembolsos_reserva (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    folio VARCHAR(30) NOT NULL DEFAULT (
        'REE-' ||
        LPAD(
            NEXTVAL('reembolsos_reserva_folio_seq')::TEXT,
            8,
            '0'
        )
    ),

    reserva_id BIGINT NOT NULL,

    monto NUMERIC(12, 2) NOT NULL,

    metodo VARCHAR(20) NOT NULL,

    referencia VARCHAR(150),

    -- APLICADO significa que el dinero fue efectivamente
    -- entregado o transferido al pasajero.
    --
    -- ANULADO se utilizará únicamente para corregir un
    -- registro capturado por error.
    estado VARCHAR(15) NOT NULL DEFAULT 'APLICADO',

    reembolsado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    anulado_en TIMESTAMPTZ,
    motivo_anulacion VARCHAR(500),

    notas TEXT,

    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_reembolsos_reserva_reserva
        FOREIGN KEY (reserva_id)
        REFERENCES reservas (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT uq_reembolsos_reserva_folio
        UNIQUE (folio),

    CONSTRAINT ck_reembolsos_reserva_folio_no_vacio
        CHECK (
            BTRIM(folio) <> ''
        ),

    CONSTRAINT ck_reembolsos_reserva_monto
        CHECK (
            monto > 0
        ),

    CONSTRAINT ck_reembolsos_reserva_metodo
        CHECK (
            metodo IN (
                'EFECTIVO',
                'TRANSFERENCIA',
                'TARJETA',
                'OTRO'
            )
        ),

    CONSTRAINT ck_reembolsos_reserva_referencia
        CHECK (
            referencia IS NULL
            OR BTRIM(referencia) <> ''
        ),

    CONSTRAINT ck_reembolsos_reserva_estado
        CHECK (
            estado IN (
                'APLICADO',
                'ANULADO'
            )
        ),

    CONSTRAINT ck_reembolsos_reserva_anulacion
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

    CONSTRAINT ck_reembolsos_reserva_notas
        CHECK (
            notas IS NULL
            OR BTRIM(notas) <> ''
        )
);


-- Facilita consultar el historial de devoluciones
-- correspondiente a una reserva.
CREATE INDEX ix_reembolsos_reserva_reserva
    ON reembolsos_reserva (
        reserva_id,
        reembolsado_en
    );


-- Facilita sumar únicamente los reembolsos que realmente
-- se encuentran aplicados.
CREATE INDEX ix_reembolsos_reserva_aplicados
    ON reembolsos_reserva (
        reserva_id
    )
    WHERE estado = 'APLICADO';


-- =========================================================
-- RESUMEN DE REEMBOLSOS
-- =========================================================
--
-- La vista calcula:
--
--   monto_reembolsable:
--       Importe autorizado al momento de cancelar.
--
--   monto_reembolsado:
--       Dinero que realmente salió del negocio.
--
--   saldo_por_reembolsar:
--       Importe que todavía debe devolverse.
--
-- No guardamos estos últimos dos importes directamente en
-- reservas porque se pueden calcular desde los movimientos.
-- Así evitamos información duplicada o inconsistente.
-- =========================================================

CREATE VIEW vw_reservas_reembolsos AS
SELECT
    resumen.reserva_id,
    resumen.cancelacion_reembolsable,
    resumen.monto_reembolsable,
    resumen.monto_reembolsado,

    GREATEST(
        resumen.monto_reembolsable
            - resumen.monto_reembolsado,
        0
    ) AS saldo_por_reembolsar,

    CASE
        WHEN resumen.cancelacion_reembolsable = FALSE
            OR resumen.monto_reembolsable = 0
            THEN 'NO_APLICA'

        WHEN resumen.monto_reembolsado = 0
            THEN 'PENDIENTE'

        WHEN resumen.monto_reembolsado
            < resumen.monto_reembolsable
            THEN 'PARCIAL'

        WHEN resumen.monto_reembolsado
            = resumen.monto_reembolsable
            THEN 'REEMBOLSADO'

        ELSE 'EXCEDIDO'
    END AS estado_reembolso

FROM (
    SELECT
        r.id AS reserva_id,
        r.cancelacion_reembolsable,
        r.monto_reembolsable,

        COALESCE(
            SUM(rr.monto) FILTER (
                WHERE rr.estado = 'APLICADO'
            ),
            0
        ) AS monto_reembolsado

    FROM reservas r

    LEFT JOIN reembolsos_reserva rr
        ON rr.reserva_id = r.id

    GROUP BY
        r.id,
        r.cancelacion_reembolsable,
        r.monto_reembolsable
) AS resumen;


-- +goose Down

DROP VIEW vw_reservas_reembolsos;
DROP TABLE reembolsos_reserva;
DROP SEQUENCE reembolsos_reserva_folio_seq;