package puntosabordaje

import "time"

type PuntoAbordaje struct {
	ID              int64     `json:"id"`
	LocalidadID     int64     `json:"localidad_id"`
	LocalidadNombre string    `json:"localidad_nombre"`
	Estado          string    `json:"estado"`
	Nombre          string    `json:"nombre"`
	Referencia      string    `json:"referencia"`
	Latitud         *float64  `json:"latitud"`
	Longitud        *float64  `json:"longitud"`
	Activo          bool      `json:"activo"`
	CreadoEn        time.Time `json:"creado_en"`
	ActualizadoEn   time.Time `json:"actualizado_en"`
}

type CreateInput struct {
	LocalidadID int64    `json:"localidad_id"`
	Nombre      string   `json:"nombre"`
	Referencia  string   `json:"referencia"`
	Latitud     *float64 `json:"latitud"`
	Longitud    *float64 `json:"longitud"`
}
