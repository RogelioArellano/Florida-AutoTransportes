package reservas

// VencerConfirmacionInput identifica la reserva cuya solicitud
// de confirmación ya superó el límite de respuesta.
type VencerConfirmacionInput struct {
	ReservaID int64 `json:"reserva_id"`
}

// VencerConfirmacionParams combina la solicitud validada con
// el motivo de cancelación definido por el sistema.
//
// El motivo no se recibe desde HTTP para evitar que una
// automatización registre textos diferentes para el mismo
// proceso.
type VencerConfirmacionParams struct {
	Input             VencerConfirmacionInput
	MotivoCancelacion string
}
