-- +goose Up

ALTER TABLE localidades
    ALTER COLUMN creado_en
        TYPE TIMESTAMPTZ
        USING creado_en AT TIME ZONE CURRENT_SETTING('TimeZone'),
    ALTER COLUMN actualizado_en
        TYPE TIMESTAMPTZ
        USING actualizado_en AT TIME ZONE CURRENT_SETTING('TimeZone');

-- +goose Down

ALTER TABLE localidades
    ALTER COLUMN creado_en
        TYPE TIMESTAMP
        USING creado_en AT TIME ZONE CURRENT_SETTING('TimeZone'),
    ALTER COLUMN actualizado_en
        TYPE TIMESTAMP
        USING actualizado_en AT TIME ZONE CURRENT_SETTING('TimeZone');