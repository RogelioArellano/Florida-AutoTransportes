package rutas

import "time"

// Ruta representa una dirección completa de viaje.
//
// La ida y el regreso son rutas independientes porque pueden
// utilizar puntos de abordaje diferentes.
type Ruta struct {
	ID            int64        `json:"id"`
	Codigo        string       `json:"codigo"`
	Nombre        string       `json:"nombre"`
	Activa        bool         `json:"activa"`
	Paradas       []RutaParada `json:"paradas"`
	CreadoEn      time.Time    `json:"creado_en"`
	ActualizadoEn time.Time    `json:"actualizado_en"`
}

// RutaParada representa un punto dentro del recorrido.
// Orden indica la posición en que la unidad visitará el punto.
type RutaParada struct {
	ID              int64     `json:"id"`
	PuntoAbordajeID int64     `json:"punto_abordaje_id"`
	PuntoNombre     string    `json:"punto_nombre"`
	LocalidadNombre string    `json:"localidad_nombre"`
	Estado          string    `json:"estado"`
	Orden           int       `json:"orden"`
	PermiteSubir    bool      `json:"permite_subir"`
	PermiteBajar    bool      `json:"permite_bajar"`
	EsObligatoria   bool      `json:"es_obligatoria"`
	CreadoEn        time.Time `json:"creado_en"`
}

// CreateInput representa el JSON recibido al registrar una ruta.
//
// El orden se obtiene de la posición de cada elemento dentro
// del arreglo Paradas.
type CreateInput struct {
	Codigo  string              `json:"codigo"`
	Nombre  string              `json:"nombre"`
	Paradas []CreateParadaInput `json:"paradas"`
}

// CreateParadaInput contiene la configuración de un punto
// dentro de una nueva ruta.
type CreateParadaInput struct {
	PuntoAbordajeID int64 `json:"punto_abordaje_id"`
	PermiteSubir    bool  `json:"permite_subir"`
	PermiteBajar    bool  `json:"permite_bajar"`
	EsObligatoria   bool  `json:"es_obligatoria"`
}
