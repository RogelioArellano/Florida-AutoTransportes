package corridas

import "time"

// Estado representa la etapa operativa de una corrida.
type Estado string

const (
	EstadoProgramada Estado = "PROGRAMADA"
	EstadoAbordando  Estado = "ABORDANDO"
	EstadoEnCurso    Estado = "EN_CURSO"
	EstadoCompletada Estado = "COMPLETADA"
	EstadoCancelada  Estado = "CANCELADA"
)

// Corrida representa la ejecución de una programación
// en una fecha concreta.
type Corrida struct {
	ID                 int64           `json:"id"`
	Folio              string          `json:"folio"`
	ProgramacionID     *int64          `json:"programacion_id"`
	ProgramacionCodigo *string         `json:"programacion_codigo"`
	RutaID             int64           `json:"ruta_id"`
	RutaCodigo         string          `json:"ruta_codigo"`
	RutaNombre         string          `json:"ruta_nombre"`
	UnidadID           int64           `json:"unidad_id"`
	UnidadCodigo       string          `json:"unidad_codigo"`
	ChoferID           *int64          `json:"chofer_id"`
	ChoferNombre       *string         `json:"chofer_nombre"`
	FechaServicio      string          `json:"fecha_servicio"`
	SalidaProgramada   time.Time       `json:"salida_programada"`
	LlegadaEstimada    *time.Time      `json:"llegada_estimada"`
	SalidaReal         *time.Time      `json:"salida_real"`
	LlegadaReal        *time.Time      `json:"llegada_real"`
	CapacidadPasajeros int             `json:"capacidad_pasajeros"`
	Estado             Estado          `json:"estado"`
	ReservasAbiertas   bool            `json:"reservas_abiertas"`
	Observaciones      *string         `json:"observaciones"`
	Paradas            []CorridaParada `json:"paradas"`
	CreadoEn           time.Time       `json:"creado_en"`
	ActualizadoEn      time.Time       `json:"actualizado_en"`
}

// CorridaParada es una copia histórica de una parada de ruta.
type CorridaParada struct {
	ID                  int64      `json:"id"`
	CorridaID           int64      `json:"corrida_id"`
	RutaParadaID        int64      `json:"ruta_parada_id"`
	PuntoAbordajeID     int64      `json:"punto_abordaje_id"`
	PuntoNombre         string     `json:"punto_nombre"`
	LocalidadNombre     string     `json:"localidad_nombre"`
	EstadoNombre        string     `json:"estado_nombre"`
	Orden               int        `json:"orden"`
	PermiteSubir        bool       `json:"permite_subir"`
	PermiteBajar        bool       `json:"permite_bajar"`
	EsObligatoria       bool       `json:"es_obligatoria"`
	IncluidaEnRecorrido bool       `json:"incluida_en_recorrido"`
	HoraEstimadaInicio  *time.Time `json:"hora_estimada_inicio"`
	HoraEstimadaFin     *time.Time `json:"hora_estimada_fin"`
	CreadoEn            time.Time  `json:"creado_en"`
	ActualizadoEn       time.Time  `json:"actualizado_en"`
}

// GenerarInput contiene los datos mínimos para generar
// una corrida desde una programación.
type GenerarInput struct {
	ProgramacionID int64   `json:"programacion_id"`
	FechaServicio  string  `json:"fecha_servicio"`
	Observaciones  *string `json:"observaciones"`
}

// ListFilter representa filtros opcionales para consultar
// corridas.
type ListFilter struct {
	FechaDesde *string
	FechaHasta *string
	Estado     *Estado
}
