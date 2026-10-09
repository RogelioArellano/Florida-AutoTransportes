package corridas

// OperacionInput identifica la corrida. El endpoint determina la acción;
// no acepta estados arbitrarios ni fechas reales proporcionadas por el cliente.
type OperacionInput struct {
	CorridaID int64 `json:"corrida_id"`
}
