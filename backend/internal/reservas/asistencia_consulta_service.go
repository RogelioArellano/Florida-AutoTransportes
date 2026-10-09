package reservas

// Todas las consultas utilizan las mismas reglas que el endpoint
// específico de asistencia, incluyendo cancelaciones y datos históricos.
func completarDetalleAsistencia(estado Estado, cantidad int, detalle DetalleAsistencia) DetalleAsistencia {
	resumen := completarResumenAsistencia(AsistenciaReserva{
		EstadoReserva:           estado,
		CantidadPasajeros:       cantidad,
		CantidadAbordada:        detalle.CantidadAbordada,
		AbordadaEn:              detalle.AbordadaEn,
		AsistenciaCerradaEn:     detalle.AsistenciaCerradaEn,
		ObservacionesAsistencia: detalle.ObservacionesAsistencia,
	})
	detalle.AsistenciaConocida = resumen.AsistenciaConocida
	detalle.AsistenciaCerrada = resumen.AsistenciaCerrada
	detalle.CantidadPendiente = resumen.CantidadPendiente
	detalle.CantidadNoPresentada = resumen.CantidadNoPresentada
	return detalle
}

func completarReservaConAsistencia(reserva Reserva, err error) (Reserva, error) {
	if err != nil {
		return reserva, err
	}
	reserva.DetalleAsistencia = completarDetalleAsistencia(reserva.Estado, reserva.CantidadPasajeros, reserva.DetalleAsistencia)
	return reserva, nil
}

func completarReservasConAsistencia(reservas []Reserva, err error) ([]Reserva, error) {
	if err != nil || reservas == nil {
		return reservas, err
	}
	// Copiamos el arreglo para no modificar datos compartidos por el Store.
	resultado := make([]Reserva, len(reservas))
	for i, reserva := range reservas {
		resultado[i], _ = completarReservaConAsistencia(reserva, nil)
	}
	return resultado, nil
}

func completarListaConAsistencia(lista ListaPasajerosCorrida) ListaPasajerosCorrida {
	lista.TotalReservasAsistenciaAbierta = 0
	lista.TotalReservasAsistenciaDesconocida = 0
	abordados, pendientes, ausentes := 0, 0, 0
	reservas := make([]ReservaListaPasajeros, len(lista.Reservas))
	for i, reserva := range lista.Reservas {
		reserva.DetalleAsistencia = completarDetalleAsistencia(
			reserva.EstadoReserva, reserva.CantidadPasajeros, reserva.DetalleAsistencia,
		)
		reservas[i] = reserva
		if !reserva.AsistenciaConocida {
			lista.TotalReservasAsistenciaDesconocida++
			continue
		}
		abordados += *reserva.CantidadAbordada
		pendientes += *reserva.CantidadPendiente
		if !reserva.AsistenciaCerrada {
			lista.TotalReservasAsistenciaAbierta++
		} else {
			ausentes += *reserva.CantidadNoPresentada
		}
	}
	lista.Reservas = reservas
	lista.TotalPasajerosAbordados = nil
	lista.TotalPasajerosPendientes = nil
	lista.TotalPasajerosNoPresentados = nil
	if lista.TotalReservasAsistenciaDesconocida == 0 {
		lista.TotalPasajerosAbordados = &abordados
		lista.TotalPasajerosPendientes = &pendientes
		if lista.TotalReservasAsistenciaAbierta == 0 {
			lista.TotalPasajerosNoPresentados = &ausentes
		}
	}
	return lista
}
