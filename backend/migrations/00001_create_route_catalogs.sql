-- +goose Up

CREATE TABLE localidades(
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre VARCHAR(100) NOT NULL,
    estado VARCHAR(100) NOT NULL,
    activa BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMP NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_localidades_nombre_no_vacio
        CHECK (BTRIM(nombre) <> ''),

    CONSTRAINT ck_localidades_estado_no_vacio
        CHECK (BTRIM(estado) <> '')
);

CREATE UNIQUE INDEX uq_localidades_nombre_estado
    ON localidades(
        LOWER(BTRIM(nombre)), 
        LOWER(BTRIM(estado))
);

CREATE TABLE puntos_abordaje (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    localidad_id BIGINT NOT NULL,
    nombre VARCHAR(150) NOT NULL,
    referencia TEXT,
    latitud NUMERIC(9, 6),
    longitud NUMERIC(9, 6),
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_puntos_abordaje_localidad
        FOREIGN KEY (localidad_id)
        REFERENCES localidades (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT ck_puntos_abordaje_nombre_no_vacio
        CHECK (BTRIM(nombre) <> ''),

    CONSTRAINT ck_puntos_abordaje_coordenadas_completas
        CHECK (
            (latitud IS NULL AND longitud IS NULL)
            OR
            (latitud IS NOT NULL AND longitud IS NOT NULL)
        ),

    CONSTRAINT ck_puntos_abordaje_latitud
        CHECK (
            latitud IS NULL
            OR latitud BETWEEN -90 AND 90
        ),

    CONSTRAINT ck_puntos_abordaje_longitud
        CHECK (
            longitud IS NULL
            OR longitud BETWEEN -180 AND 180
        )
);

CREATE UNIQUE INDEX uq_puntos_abordaje_localidad_nombre
    ON puntos_abordaje (
        localidad_id,
        LOWER(BTRIM(nombre))
    );

CREATE TABLE rutas (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo VARCHAR(30) NOT NULL,
    nombre VARCHAR(150) NOT NULL,
    activa BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_rutas_codigo_no_vacio
        CHECK (BTRIM(codigo) <> ''),

    CONSTRAINT ck_rutas_nombre_no_vacio
        CHECK (BTRIM(nombre) <> '')
);

CREATE UNIQUE INDEX uq_rutas_codigo
    ON rutas (LOWER(BTRIM(codigo)));

CREATE TABLE ruta_paradas (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ruta_id BIGINT NOT NULL,
    punto_abordaje_id BIGINT NOT NULL,
    orden SMALLINT NOT NULL,
    permite_subir BOOLEAN NOT NULL DEFAULT TRUE,
    permite_bajar BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_ruta_paradas_ruta
        FOREIGN KEY (ruta_id)
        REFERENCES rutas (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT fk_ruta_paradas_punto
        FOREIGN KEY (punto_abordaje_id)
        REFERENCES puntos_abordaje (id)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT,

    CONSTRAINT ck_ruta_paradas_orden_positivo
        CHECK (orden > 0),

    CONSTRAINT ck_ruta_paradas_operacion
        CHECK (permite_subir OR permite_bajar),

    CONSTRAINT uq_ruta_paradas_orden
        UNIQUE (ruta_id, orden),

    CONSTRAINT uq_ruta_paradas_punto
        UNIQUE (ruta_id, punto_abordaje_id)
);

-- +goose Down

DROP TABLE ruta_paradas;
DROP TABLE rutas;
DROP TABLE puntos_abordaje;
DROP TABLE localidades;