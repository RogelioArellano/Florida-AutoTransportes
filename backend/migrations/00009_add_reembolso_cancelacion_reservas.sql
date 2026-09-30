-- +goose Up

-- Estos campos guardan el resultado de la política de
-- reembolso aplicada al momento de cancelar una reserva.
--
-- No representan que el dinero ya fue devuelto.
-- El movimiento financiero del reembolso se implementará
-- posteriormente en una tabla independiente.

ALTER TABLE reservas
    ADD COLUMN cancelacion_reembolsable BOOLEAN
        NOT NULL DEFAULT FALSE,

    ADD COLUMN monto_reembolsable NUMERIC(12, 2)
        NOT NULL DEFAULT 0,

    ADD COLUMN limite_reembolso_en TIMESTAMPTZ;

ALTER TABLE reservas
    ADD CONSTRAINT ck_reservas_monto_reembolsable_no_negativo
        CHECK (monto_reembolsable >= 0);

ALTER TABLE reservas
    ADD CONSTRAINT ck_reservas_reembolso_consistente
        CHECK (
            (
                cancelacion_reembolsable = FALSE
                AND monto_reembolsable = 0
            )
            OR
            (
                cancelacion_reembolsable = TRUE
                AND monto_reembolsable > 0
                AND limite_reembolso_en IS NOT NULL
                AND estado = 'CANCELADA'
            )
        );

COMMENT ON COLUMN reservas.cancelacion_reembolsable IS
    'Indica si la cancelación ocurrió dentro del límite permitido para solicitar un reembolso.';

COMMENT ON COLUMN reservas.monto_reembolsable IS
    'Importe pagado que puede devolverse al pasajero. No significa que el reembolso ya fue realizado.';

COMMENT ON COLUMN reservas.limite_reembolso_en IS
    'Fecha y hora límite calculada según la política vigente al cancelar la reserva.';

-- +goose Down

ALTER TABLE reservas
    DROP CONSTRAINT ck_reservas_reembolso_consistente;

ALTER TABLE reservas
    DROP CONSTRAINT ck_reservas_monto_reembolsable_no_negativo;

ALTER TABLE reservas
    DROP COLUMN limite_reembolso_en;

ALTER TABLE reservas
    DROP COLUMN monto_reembolsable;

ALTER TABLE reservas
    DROP COLUMN cancelacion_reembolsable;