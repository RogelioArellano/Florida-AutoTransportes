package choferes

import "time"

/*
Chofer representa a las personas autorizadas para conducir
unidades y realizar corridas

licencia_numero y vigencia son opcionales para que se puedan actualizar
posteriorment
*/
type Chofer struct {
	ID               int64     `json:"id"`
	NombreCompleto   string    `json:"nombre_completo"`
	Telefono         string    `json:"telefono"`
	LicenciaNumero   *string   `json:"licencia_numero"`
	LicenciaVigencia *string   `json:"licencia_vigencia"`
	Activo           bool      `json:"activo"`
	CreadoEn         time.Time `json:"creado_en"`
	ActualizadoEn    time.Time `json:"actualizado_en"`
}

// CreateInput representa el JSON necesario para registrar
// un nuevo chofer.
type CreateInput struct {
	NombreCompleto   string  `json:"nombre_completo"`
	Telefono         string  `json:"telefono"`
	LicenciaNumero   *string `json:"licencia_numero"`
	LicenciaVigencia *string `json:"licencia_vigencia"`
}
