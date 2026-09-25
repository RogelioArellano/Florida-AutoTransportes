-- +goose Up

CREATE TABLE programaciones (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo VARCHAR(30) NOT NULL,
    nombre VARCHAR(150) NOT NULL,
    ruta_id BIGINT NOT NULL,
    unidad_id BIGINT NOT NULL,
    chofer_id BIGINT,
    hora_salida TIME NOT NULL,
    duracion_estimada_minutos SMALLINT,
    vigencia_desde DATE NOT NULL,
    vigencia_hasta DATE,
    activa BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_programaciones_ruta
        FOREIGN KEY (ruta_id)
        REFERENCES rutas (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_programaciones_unidad
        FOREIGN KEY (unidad_id)
        REFERENCES unidades (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_programaciones_chofer
        FOREIGN KEY (chofer_id)
        REFERENCES choferes (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT ck_programaciones_codigo_no_vacio
        CHECK (
            BTRIM(codigo) <> ''
        ),

    CONSTRAINT ck_programaciones_nombre_no_vacio
        CHECK (
            BTRIM(nombre) <> ''
        ),

    CONSTRAINT ck_programaciones_duracion
        CHECK (
            duracion_estimada_minutos IS NULL
            OR duracion_estimada_minutos > 0
        ),

    CONSTRAINT ck_programaciones_vigencia
        CHECK (
            vigencia_hasta IS NULL
            OR vigencia_hasta >= vigencia_desde
        )
);

CREATE UNIQUE INDEX uq_programaciones_codigo
    ON programaciones (
        LOWER(BTRIM(codigo))
    );

CREATE INDEX ix_programaciones_ruta
    ON programaciones (
        ruta_id
    );

CREATE INDEX ix_programaciones_unidad
    ON programaciones (
        unidad_id
    );

CREATE INDEX ix_programaciones_chofer
    ON programaciones (
        chofer_id
    )
    WHERE chofer_id IS NOT NULL;

CREATE INDEX ix_programaciones_vigencia
    ON programaciones (
        vigencia_desde,
        vigencia_hasta
    )
    WHERE activa = TRUE;

CREATE TABLE programacion_dias (
    programacion_id BIGINT NOT NULL,
    dia_semana SMALLINT NOT NULL,

    CONSTRAINT pk_programacion_dias
        PRIMARY KEY (
            programacion_id,
            dia_semana
        ),

    CONSTRAINT fk_programacion_dias_programacion
        FOREIGN KEY (programacion_id)
        REFERENCES programaciones (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT ck_programacion_dias_dia_semana
        CHECK (
            dia_semana BETWEEN 1 AND 7
        )
);

CREATE INDEX ix_programacion_dias_dia_semana
    ON programacion_dias (
        dia_semana,
        programacion_id
    );

-- +goose Down

DROP TABLE programacion_dias;
DROP TABLE programaciones;