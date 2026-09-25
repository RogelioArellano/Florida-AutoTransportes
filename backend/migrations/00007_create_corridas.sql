-- +goose Up

CREATE TABLE corridas (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    folio VARCHAR(50) NOT NULL,
    programacion_id BIGINT,
    programacion_codigo VARCHAR(30),
    ruta_id BIGINT NOT NULL,
    ruta_codigo VARCHAR(30) NOT NULL,
    ruta_nombre VARCHAR(150) NOT NULL,
    unidad_id BIGINT NOT NULL,
    unidad_codigo VARCHAR(30) NOT NULL,
    chofer_id BIGINT,
    chofer_nombre VARCHAR(150),
    fecha_servicio DATE NOT NULL,
    salida_programada TIMESTAMPTZ NOT NULL,
    llegada_estimada TIMESTAMPTZ,
    salida_real TIMESTAMPTZ,
    llegada_real TIMESTAMPTZ,
    capacidad_pasajeros SMALLINT NOT NULL,
    estado VARCHAR(20) NOT NULL DEFAULT 'PROGRAMADA',
    reservas_abiertas BOOLEAN NOT NULL DEFAULT TRUE,
    observaciones TEXT,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_corridas_programacion
        FOREIGN KEY (programacion_id)
        REFERENCES programaciones (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_corridas_ruta
        FOREIGN KEY (ruta_id)
        REFERENCES rutas (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_corridas_unidad
        FOREIGN KEY (unidad_id)
        REFERENCES unidades (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_corridas_chofer
        FOREIGN KEY (chofer_id)
        REFERENCES choferes (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT ck_corridas_folio_no_vacio
        CHECK (
            BTRIM(folio) <> ''
        ),

    CONSTRAINT ck_corridas_programacion_codigo
        CHECK (
            programacion_codigo IS NULL
            OR BTRIM(programacion_codigo) <> ''
        ),

    CONSTRAINT ck_corridas_ruta_codigo_no_vacio
        CHECK (
            BTRIM(ruta_codigo) <> ''
        ),

    CONSTRAINT ck_corridas_ruta_nombre_no_vacio
        CHECK (
            BTRIM(ruta_nombre) <> ''
        ),

    CONSTRAINT ck_corridas_unidad_codigo_no_vacio
        CHECK (
            BTRIM(unidad_codigo) <> ''
        ),

    CONSTRAINT ck_corridas_chofer_nombre
        CHECK (
            chofer_nombre IS NULL
            OR BTRIM(chofer_nombre) <> ''
        ),

    CONSTRAINT ck_corridas_capacidad
        CHECK (
            capacidad_pasajeros > 0
        ),

    CONSTRAINT ck_corridas_estado
        CHECK (
            estado IN (
                'PROGRAMADA',
                'ABORDANDO',
                'EN_CURSO',
                'COMPLETADA',
                'CANCELADA'
            )
        ),

    CONSTRAINT ck_corridas_llegada_estimada
        CHECK (
            llegada_estimada IS NULL
            OR llegada_estimada >= salida_programada
        ),

    CONSTRAINT ck_corridas_salida_real
        CHECK (
            salida_real IS NULL
            OR estado IN (
                'EN_CURSO',
                'COMPLETADA'
            )
        ),

    CONSTRAINT ck_corridas_llegada_real
        CHECK (
            llegada_real IS NULL
            OR (
                salida_real IS NOT NULL
                AND llegada_real >= salida_real
            )
        ),

    CONSTRAINT ck_corridas_cancelada_sin_reservas_abiertas
        CHECK (
            estado <> 'CANCELADA'
            OR reservas_abiertas = FALSE
        )
);

CREATE UNIQUE INDEX uq_corridas_folio
    ON corridas (
        LOWER(BTRIM(folio))
    );

CREATE UNIQUE INDEX uq_corridas_programacion_fecha
    ON corridas (
        programacion_id,
        fecha_servicio
    )
    WHERE programacion_id IS NOT NULL;

CREATE INDEX ix_corridas_fecha_estado
    ON corridas (
        fecha_servicio,
        estado
    );

CREATE INDEX ix_corridas_unidad_fecha
    ON corridas (
        unidad_id,
        fecha_servicio
    );

CREATE INDEX ix_corridas_chofer_fecha
    ON corridas (
        chofer_id,
        fecha_servicio
    )
    WHERE chofer_id IS NOT NULL;

CREATE TABLE corrida_paradas (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    corrida_id BIGINT NOT NULL,
    ruta_parada_id BIGINT NOT NULL,
    punto_abordaje_id BIGINT NOT NULL,
    punto_nombre VARCHAR(150) NOT NULL,
    localidad_nombre VARCHAR(100) NOT NULL,
    estado_nombre VARCHAR(100) NOT NULL,
    orden SMALLINT NOT NULL,
    permite_subir BOOLEAN NOT NULL,
    permite_bajar BOOLEAN NOT NULL,
    es_obligatoria BOOLEAN NOT NULL,
    incluida_en_recorrido BOOLEAN NOT NULL,
    hora_estimada_inicio TIMESTAMPTZ,
    hora_estimada_fin TIMESTAMPTZ,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_corrida_paradas_corrida
        FOREIGN KEY (corrida_id)
        REFERENCES corridas (id)
        ON UPDATE RESTRICT
        ON DELETE CASCADE,

    CONSTRAINT fk_corrida_paradas_ruta_parada
        FOREIGN KEY (ruta_parada_id)
        REFERENCES ruta_paradas (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_corrida_paradas_punto
        FOREIGN KEY (punto_abordaje_id)
        REFERENCES puntos_abordaje (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT ck_corrida_paradas_punto_nombre
        CHECK (
            BTRIM(punto_nombre) <> ''
        ),

    CONSTRAINT ck_corrida_paradas_localidad
        CHECK (
            BTRIM(localidad_nombre) <> ''
        ),

    CONSTRAINT ck_corrida_paradas_estado_nombre
        CHECK (
            BTRIM(estado_nombre) <> ''
        ),

    CONSTRAINT ck_corrida_paradas_orden
        CHECK (
            orden > 0
        ),

    CONSTRAINT ck_corrida_paradas_operacion
        CHECK (
            permite_subir
            OR permite_bajar
        ),

    CONSTRAINT ck_corrida_paradas_horas_completas
        CHECK (
            (
                hora_estimada_inicio IS NULL
                AND hora_estimada_fin IS NULL
            )
            OR
            (
                hora_estimada_inicio IS NOT NULL
                AND hora_estimada_fin IS NOT NULL
            )
        ),

    CONSTRAINT ck_corrida_paradas_ventana
        CHECK (
            hora_estimada_inicio IS NULL
            OR hora_estimada_fin >= hora_estimada_inicio
        ),

    CONSTRAINT uq_corrida_paradas_orden
        UNIQUE (
            corrida_id,
            orden
        ),

    CONSTRAINT uq_corrida_paradas_ruta_parada
        UNIQUE (
            corrida_id,
            ruta_parada_id
        )
);

CREATE INDEX ix_corrida_paradas_punto
    ON corrida_paradas (
        punto_abordaje_id,
        corrida_id
    );

CREATE INDEX ix_corrida_paradas_recorrido
    ON corrida_paradas (
        corrida_id,
        incluida_en_recorrido,
        orden
    );

-- +goose Down

DROP TABLE corrida_paradas;
DROP TABLE corridas;