-- +goose Up

ALTER TABLE ruta_paradas
    ADD COLUMN es_obligatoria BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN ruta_paradas.es_obligatoria IS
    'Indica si la corrida debe pasar siempre por esta parada';

-- +goose Down

ALTER TABLE ruta_paradas
    DROP COLUMN es_obligatoria;