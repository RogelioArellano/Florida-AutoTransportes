-- +goose Up

-- Registra el último envío exitoso de la lista de pasajeros.
--
-- Estos campos se actualizan solamente después de que el
-- proveedor de mensajería confirme que el mensaje fue enviado.
ALTER TABLE corridas
    ADD COLUMN lista_pasajeros_enviada_en TIMESTAMPTZ,
    ADD COLUMN lista_pasajeros_enviada_chofer_id BIGINT;

-- Conservamos el chofer al que se realizó el último envío.
-- Si posteriormente cambia el chofer de la corrida, la
-- automatización podrá detectar que debe enviar nuevamente
-- la lista al nuevo conductor.
ALTER TABLE corridas
    ADD CONSTRAINT fk_corridas_lista_pasajeros_chofer
    FOREIGN KEY (
        lista_pasajeros_enviada_chofer_id
    )
    REFERENCES choferes (id)
    ON UPDATE RESTRICT
    ON DELETE RESTRICT;

-- Los dos datos del envío deben existir juntos:
--
--   - ambos NULL: la lista todavía no ha sido enviada;
--   - ambos con valor: existe un envío exitoso registrado.
ALTER TABLE corridas
    ADD CONSTRAINT ck_corridas_lista_pasajeros_envio
    CHECK (
        (
            lista_pasajeros_enviada_en IS NULL
            AND lista_pasajeros_enviada_chofer_id IS NULL
        )
        OR
        (
            lista_pasajeros_enviada_en IS NOT NULL
            AND lista_pasajeros_enviada_chofer_id IS NOT NULL
        )
    );

-- Facilita la búsqueda periódica de corridas programadas
-- próximas a salir y con un chofer asignado.
CREATE INDEX ix_corridas_lista_pasajeros_pendiente
    ON corridas (
        salida_programada
    )
    WHERE estado = 'PROGRAMADA'
        AND chofer_id IS NOT NULL;

-- +goose Down

DROP INDEX ix_corridas_lista_pasajeros_pendiente;

ALTER TABLE corridas
    DROP CONSTRAINT ck_corridas_lista_pasajeros_envio,
    DROP CONSTRAINT fk_corridas_lista_pasajeros_chofer;

ALTER TABLE corridas
    DROP COLUMN lista_pasajeros_enviada_chofer_id,
    DROP COLUMN lista_pasajeros_enviada_en;