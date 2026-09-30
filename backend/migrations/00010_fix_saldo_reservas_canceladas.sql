-- +goose Up

-- =========================================================
-- CORRECCIÓN DEL SALDO DE RESERVAS CANCELADAS
-- =========================================================
--
-- Una reserva cancelada conserva:
--
--   - Su total original.
--   - Los pagos que realmente recibió.
--   - Su estado de pago histórico.
--   - El importe que puede ser reembolsado.
--
-- Sin embargo, ya no debe aparecer como una cuenta por
-- cobrar. Por eso su saldo pendiente debe ser cero.
-- =========================================================

CREATE OR REPLACE VIEW vw_reservas_saldos AS
SELECT
    resumen.reserva_id,
    resumen.total,
    resumen.monto_pagado,

    CASE
        WHEN resumen.estado = 'CANCELADA'
            THEN 0

        ELSE GREATEST(
            resumen.total - resumen.monto_pagado,
            0
        )
    END AS saldo_pendiente,

    -- El estado de pago se conserva como información
    -- histórica. El estado CANCELADA se encuentra en la
    -- tabla reservas y no debe mezclarse con estado_pago.
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
        r.estado,

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
        r.total,
        r.estado
) AS resumen;


-- +goose Down

-- =========================================================
-- RESTAURAR EL COMPORTAMIENTO ANTERIOR
-- =========================================================
--
-- Antes de esta migración, una reserva cancelada conservaba
-- total - monto_pagado como saldo pendiente.
-- =========================================================

CREATE OR REPLACE VIEW vw_reservas_saldos AS
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