package reservas

import "time"

// PoliticaEnvioListaPasajerosParams contiene la anticipación
// utilizada para buscar corridas próximas a salir.
type PoliticaEnvioListaPasajerosParams struct {
	MinutosAnticipacion int
}

// CorridaPendienteListaPasajeros representa una corrida cuya
// lista todavía debe enviarse al chofer.
//
// También incluye los datos del envío anterior para permitir
// un nuevo envío cuando cambia el conductor asignado.
type CorridaPendienteListaPasajeros struct {
	CorridaID        int64     `json:"corrida_id"`
	CorridaFolio     string    `json:"corrida_folio"`
	RutaID           int64     `json:"ruta_id"`
	RutaCodigo       string    `json:"ruta_codigo"`
	RutaNombre       string    `json:"ruta_nombre"`
	FechaServicio    string    `json:"fecha_servicio"`
	SalidaProgramada time.Time `json:"salida_programada"`

	UnidadID     int64   `json:"unidad_id"`
	UnidadCodigo string  `json:"unidad_codigo"`
	UnidadPlacas *string `json:"unidad_placas"`

	ChoferID       int64  `json:"chofer_id"`
	ChoferNombre   string `json:"chofer_nombre"`
	ChoferTelefono string `json:"chofer_telefono"`

	TotalReservas             int `json:"total_reservas"`
	TotalPasajerosRegistrados int `json:"total_pasajeros_registrados"`

	UltimoEnvioEn       *time.Time `json:"ultimo_envio_en"`
	UltimoEnvioChoferID *int64     `json:"ultimo_envio_chofer_id"`
}

// RegistrarEnvioListaPasajerosInput indica a qué corrida y
// chofer se envió correctamente la lista.
//
// ChoferID es obligatorio para evitar registrar el envío si
// el conductor fue cambiado mientras n8n procesaba el mensaje.
type RegistrarEnvioListaPasajerosInput struct {
	CorridaID int64 `json:"corrida_id"`
	ChoferID  int64 `json:"chofer_id"`
}

// EnvioListaPasajeros representa el envío exitoso registrado
// para una corrida.
type EnvioListaPasajeros struct {
	CorridaID    int64  `json:"corrida_id"`
	CorridaFolio string `json:"corrida_folio"`

	ChoferID       int64  `json:"chofer_id"`
	ChoferNombre   string `json:"chofer_nombre"`
	ChoferTelefono string `json:"chofer_telefono"`

	EnviadaEn time.Time `json:"enviada_en"`
}
