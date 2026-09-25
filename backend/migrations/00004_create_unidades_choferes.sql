-- +goose Up

CREATE TABLE unidades (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    codigo VARCHAR(30) NOT NULL,
    placas VARCHAR(20),
    marca VARCHAR(80),
    modelo VARCHAR(80),
    anio SMALLINT,
    capacidad_total SMALLINT NOT NULL,
    capacidad_pasajeros SMALLINT NOT NULL,
    activa BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_unidades_codigo_no_vacio
        CHECK (BTRIM(codigo) <> ''),

    CONSTRAINT ck_unidades_placas_no_vacias
        CHECK (
            placas IS NULL
            OR BTRIM(placas) <> ''
        ),

    CONSTRAINT ck_unidades_marca_no_vacia
        CHECK (
            marca IS NULL
            OR BTRIM(marca) <> ''
        ),

    CONSTRAINT ck_unidades_modelo_no_vacio
        CHECK (
            modelo IS NULL
            OR BTRIM(modelo) <> ''
        ),

    CONSTRAINT ck_unidades_anio
        CHECK (
            anio IS NULL
            OR anio BETWEEN 1950 AND 2100
        ),

    CONSTRAINT ck_unidades_capacidad_total
        CHECK (capacidad_total >= 2),

    CONSTRAINT ck_unidades_capacidad_pasajeros
        CHECK (capacidad_pasajeros > 0),

    CONSTRAINT ck_unidades_capacidades
        CHECK (
            capacidad_pasajeros
            < capacidad_total
        )
);

CREATE UNIQUE INDEX uq_unidades_codigo
    ON unidades (
        LOWER(BTRIM(codigo))
    );

CREATE UNIQUE INDEX uq_unidades_placas
    ON unidades (
        UPPER(BTRIM(placas))
    )
    WHERE
        placas IS NOT NULL
        AND BTRIM(placas) <> '';

CREATE TABLE choferes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nombre_completo VARCHAR(150) NOT NULL,
    telefono VARCHAR(20) NOT NULL,
    licencia_numero VARCHAR(50),
    licencia_vigencia DATE,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actualizado_en TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ck_choferes_nombre_no_vacio
        CHECK (
            BTRIM(nombre_completo) <> ''
        ),

    CONSTRAINT ck_choferes_telefono_no_vacio
        CHECK (
            BTRIM(telefono) <> ''
        ),

    CONSTRAINT ck_choferes_licencia_no_vacia
        CHECK (
            licencia_numero IS NULL
            OR BTRIM(licencia_numero) <> ''
        )
);

CREATE INDEX ix_choferes_nombre
    ON choferes (
        LOWER(BTRIM(nombre_completo))
    );

-- +goose Down

DROP TABLE choferes;
DROP TABLE unidades;