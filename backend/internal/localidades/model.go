package localidades

import "time"

type Localidad struct {
	ID            int64     `json:"id"`
	Nombre        string    `json:"nombre"`
	Estado        string    `json:"estado"`
	Activa        bool      `json:"activa"`
	CreadoEn      time.Time `json:"creado_en"`
	ActualizadoEn time.Time `json:"actualizado_en"`
}

type CreateInput struct {
	Nombre string `json:"nombre"`
	Estado string `json:"estado"`
}
