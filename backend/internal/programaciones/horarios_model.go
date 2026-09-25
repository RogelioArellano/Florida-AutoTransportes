package programaciones

import "time"

// HorarioParada representa la ventana estimada en que
// la unidad pasará por una parada de la ruta.
type HorarioParada struct {
	ProgramacionID           int64     `json:"programacion_id"`
	RutaParadaID             int64     `json:"ruta_parada_id"`
	PuntoAbordajeID          int64     `json:"punto_abordaje_id"`
	PuntoNombre              string    `json:"punto_nombre"`
	Orden                    int       `json:"orden"`
	MinutosDesdeSalidaInicio int       `json:"minutos_desde_salida_inicio"`
	MinutosDesdeSalidaFin    int       `json:"minutos_desde_salida_fin"`
	HoraEstimadaInicio       string    `json:"hora_estimada_inicio"`
	HoraEstimadaFin          string    `json:"hora_estimada_fin"`
	Notas                    *string   `json:"notas"`
	CreadoEn                 time.Time `json:"creado_en"`
	ActualizadoEn            time.Time `json:"actualizado_en"`
}

// ConfigurarHorariosInput contiene toda la configuración
// que reemplazará los horarios anteriores.
type ConfigurarHorariosInput struct {
	ProgramacionID int64                          `json:"programacion_id"`
	Paradas        []ConfigurarHorarioParadaInput `json:"paradas"`
}

// ConfigurarHorarioParadaInput representa la ventana que
// se desea configurar para una parada.
type ConfigurarHorarioParadaInput struct {
	RutaParadaID             int64   `json:"ruta_parada_id"`
	MinutosDesdeSalidaInicio int     `json:"minutos_desde_salida_inicio"`
	MinutosDesdeSalidaFin    int     `json:"minutos_desde_salida_fin"`
	Notas                    *string `json:"notas"`
}
