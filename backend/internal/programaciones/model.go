package programaciones

import "time"

/*
Programacion representa una regla recurrente de operacion

No es una corrida concreta, solo describe que ruta debe
operar, a que hora y en que días
*/

type Programacion struct {
	ID                      int64     `json:"id"`
	Codigo                  string    `json:"codigo"`
	Nombre                  string    `json:"nombre"`
	RutaID                  int64     `json:"ruta_id"`
	RutaCodigo              string    `json:"ruta_codigo"`
	RutaNombre              string    `json:"ruta_nombre"`
	UnidadID                int64     `json:"unidad_id"`
	UnidadCodigo            string    `json:"unidad_codigo"`
	ChoferID                *int64    `json:"chofer_id"`
	ChoferNombre            *string   `json:"chofer_nombre"`
	HoraSalida              string    `json:"hora_salida"`
	DuracionEstimadaMinutos *int      `json:"duracion_estimada_minutos"`
	VigenciaDesde           string    `json:"vigencia_desde"`
	VigenciaHasta           *string   `json:"vigencia_hasta"`
	DiasSemana              []int     `json:"dias_semana"`
	Activa                  bool      `json:"activa"`
	CreadoEn                time.Time `json:"creado_en"`
	ActualizadoEn           time.Time `json:"actualizado_en"`
}

/*
CreateInput representa los datos recibidos al registrar
una programación.

Los días utilizan la numeración ISO:
1 lunes, 2 martes, ..., 7 domingo.
*/
type CreateInput struct {
	Codigo                  string  `json:"codigo"`
	Nombre                  string  `json:"nombre"`
	RutaID                  int64   `json:"ruta_id"`
	UnidadID                int64   `json:"unidad_id"`
	ChoferID                *int64  `json:"chofer_id"`
	HoraSalida              string  `json:"hora_salida"`
	DuracionEstimadaMinutos *int    `json:"duracion_estimada_minutos"`
	VigenciaDesde           string  `json:"vigencia_desde"`
	VigenciaHasta           *string `json:"vigencia_hasta"`
	DiasSemana              []int   `json:"dias_semana"`
}
