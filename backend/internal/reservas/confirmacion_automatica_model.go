package reservas

import "time"

// ConfirmacionPendiente contiene la información necesaria para
// que atención al cliente o n8n soliciten confirmación al
// pasajero responsable de una reserva sin anticipo.
type ConfirmacionPendiente struct {
	ReservaID    int64  `json:"reserva_id"`
	ReservaFolio string `json:"reserva_folio"`

	PasajeroID       int64  `json:"pasajero_id"`
	PasajeroNombre   string `json:"pasajero_nombre"`
	PasajeroTelefono string `json:"pasajero_telefono"`

	CorridaID    int64  `json:"corrida_id"`
	CorridaFolio string `json:"corrida_folio"`

	SalidaProgramada time.Time `json:"salida_programada"`

	ParadaOrigenNombre  string `json:"parada_origen_nombre"`
	ParadaDestinoNombre string `json:"parada_destino_nombre"`
	CantidadPasajeros   int    `json:"cantidad_pasajeros"`

	ConfirmacionSolicitadaEn *time.Time `json:"confirmacion_solicitada_en"`
	ConfirmacionLimiteEn     *time.Time `json:"confirmacion_limite_en"`
}

// SolicitarConfirmacionInput identifica la reserva a la que ya
// se envió correctamente la solicitud de confirmación.
//
// El endpoint se invocará después de que n8n o atención al
// cliente hayan enviado el mensaje al pasajero.
type SolicitarConfirmacionInput struct {
	ReservaID int64 `json:"reserva_id"`
}

// PoliticaConfirmacionParams contiene la política que el
// Service entrega al Repository.
//
// Los cálculos de fecha se realizarán con NOW() de PostgreSQL.
type PoliticaConfirmacionParams struct {
	HorasAnticipacion int
	MinutosRespuesta  int
}

// SolicitarConfirmacionParams combina la solicitud validada
// con la política vigente.
type SolicitarConfirmacionParams struct {
	Input SolicitarConfirmacionInput
	PoliticaConfirmacionParams
}
