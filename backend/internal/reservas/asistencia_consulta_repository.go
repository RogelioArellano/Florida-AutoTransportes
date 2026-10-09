package reservas

import "database/sql"

// Comparte la lectura nullable entre el detalle y la lista de pasajeros.
// Los campos calculados se completan posteriormente en el Service.
func detalleAsistenciaDesdeNull(
	cantidad sql.NullInt64,
	abordada, cierre sql.NullTime,
	observaciones sql.NullString,
) DetalleAsistencia {
	detalle := DetalleAsistencia{
		AbordadaEn:              timeDesdeNull(abordada),
		AsistenciaCerradaEn:     timeDesdeNull(cierre),
		ObservacionesAsistencia: stringDesdeNull(observaciones),
	}
	if cantidad.Valid {
		valor := int(cantidad.Int64)
		detalle.CantidadAbordada = &valor
	}
	return detalle
}
