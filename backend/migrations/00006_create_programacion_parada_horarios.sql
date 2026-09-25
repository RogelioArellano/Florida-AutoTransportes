-- +goose Up

-- Estas restricciones compuestas permiten garantizar que
-- una parada configurada pertenezca a la misma ruta que
-- utiliza la programación.
ALTER TABLE programaciones
    ADD CONSTRAINT uq_programaciones_id_ruta
    UNIQUE (
        id,
        ruta_id
    );

ALTER TABLE ruta_paradas
    ADD CONSTRAINT uq_ruta_paradas_ruta_id_id
    UNIQUE (
        ruta_id,
        id
    );

CREATE TABLE programacion_parada_horarios (
    programacion_id BIGINT NOT NULL,
    ruta_id BIGINT NOT NULL,
    ruta_parada_id BIGINT NOT NULL,
    minutos_desde_salida_inicio SMALLINT NOT NULL,
    minutos_desde_salida_fin SMALLINT NOT NULL,
    notas VARCHAR(250),
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_programacion_parada_horarios
        PRIMARY KEY (
            programacion_id,
            ruta_parada_id
        ),

    CONSTRAINT fk_programacion_parada_horarios_programacion
        FOREIGN KEY (
            programacion_id,
            ruta_id
        )
        REFERENCES programaciones (
            id,
            ruta_id
        )
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT fk_programacion_parada_horarios_ruta_parada
        FOREIGN KEY (
            ruta_id,
            ruta_parada_id
        )
        REFERENCES ruta_paradas (
            ruta_id,
            id
        )
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT ck_programacion_parada_horarios_inicio
        CHECK (
            minutos_desde_salida_inicio >= 0
        ),

    CONSTRAINT ck_programacion_parada_horarios_fin
        CHECK (
            minutos_desde_salida_fin >=
            minutos_desde_salida_inicio
        ),

    CONSTRAINT ck_programacion_parada_horarios_notas
        CHECK (
            notas IS NULL
            OR BTRIM(notas) <> ''
        )
);

CREATE INDEX ix_programacion_parada_horarios_ruta
    ON programacion_parada_horarios (
        ruta_id,
        ruta_parada_id
    );

-- +goose Down

DROP TABLE programacion_parada_horarios;

ALTER TABLE ruta_paradas
    DROP CONSTRAINT uq_ruta_paradas_ruta_id_id;

ALTER TABLE programaciones
    DROP CONSTRAINT uq_programaciones_id_ruta;