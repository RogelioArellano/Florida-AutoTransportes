package reservas

// AnularReembolsoInput contiene la información necesaria para
// corregir un movimiento de reembolso capturado por error.
//
// La anulación conserva el movimiento original para auditoría.
type AnularReembolsoInput struct {
	ReembolsoID int64  `json:"reembolso_id"`
	Motivo      string `json:"motivo"`
}
