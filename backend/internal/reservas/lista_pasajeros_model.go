package reservas

import "time"

// ListaPasajerosCorrida representa el resumen operativo y
// financiero de los pasajeros asociados a una corrida.
//
// Esta estructura no modifica información. Se utilizará como
// respuesta del endpoint:
//
//	GET /api/corridas/{corridaID}/pasajeros
type ListaPasajerosCorrida struct {
	// Información general de la corrida.
	CorridaID        int64      `json:"corrida_id"`
	CorridaFolio     string     `json:"corrida_folio"`
	RutaID           int64      `json:"ruta_id"`
	RutaCodigo       string     `json:"ruta_codigo"`
	RutaNombre       string     `json:"ruta_nombre"`
	FechaServicio    string     `json:"fecha_servicio"`
	SalidaProgramada time.Time  `json:"salida_programada"`
	LlegadaEstimada  *time.Time `json:"llegada_estimada"`
	EstadoCorrida    string     `json:"estado_corrida"`

	// Unidad asignada.
	UnidadID     int64   `json:"unidad_id"`
	UnidadCodigo string  `json:"unidad_codigo"`
	UnidadPlacas *string `json:"unidad_placas"`

	// Chofer asignado.
	ChoferID       *int64  `json:"chofer_id"`
	ChoferNombre   *string `json:"chofer_nombre"`
	ChoferTelefono *string `json:"chofer_telefono"`

	// Resumen operativo.
	// TotalPasajerosRegistrados conserva los pasajes de las reservas;
	// OcupacionMaxima refleja el cupo comprometido por segmento.
	CapacidadPasajeros        int `json:"capacidad_pasajeros"`
	TotalReservas             int `json:"total_reservas"`
	TotalPasajerosRegistrados int `json:"total_pasajeros_registrados"`
	OcupacionMaxima           int `json:"ocupacion_maxima"`
	LugaresDisponiblesMinimos int `json:"lugares_disponibles_minimos"`

	// Los totales son NULL si no se puede conocer la cantidad exacta.
	// Las ausencias totales requieren que todas las asistencias estén cerradas.
	// Las reservas históricas desconocidas se cuentan aparte de las abiertas.
	TotalPasajerosAbordados            *int `json:"total_pasajeros_abordados"`
	TotalPasajerosPendientes           *int `json:"total_pasajeros_pendientes"`
	TotalPasajerosNoPresentados        *int `json:"total_pasajeros_no_presentados"`
	TotalReservasAsistenciaAbierta     int  `json:"total_reservas_asistencia_abierta"`
	TotalReservasAsistenciaDesconocida int  `json:"total_reservas_asistencia_desconocida"`

	// Resumen financiero de las reservas no canceladas.
	TotalVendido   Dinero `json:"total_vendido"`
	TotalCobrado   Dinero `json:"total_cobrado"`
	TotalPorCobrar Dinero `json:"total_por_cobrar"`

	// Una reserva puede representar uno o varios pasajeros.
	Reservas []ReservaListaPasajeros `json:"reservas"`
}

// ReservaListaPasajeros representa una reserva dentro de la
// lista operativa de una corrida.
//
// No representa necesariamente a una sola persona:
// CantidadPasajeros puede ser mayor que uno.
type ReservaListaPasajeros struct {
	ReservaID    int64  `json:"reserva_id"`
	ReservaFolio string `json:"reserva_folio"`

	// Pasajero responsable o contacto principal.
	PasajeroID       int64  `json:"pasajero_id"`
	PasajeroNombre   string `json:"pasajero_nombre"`
	PasajeroTelefono string `json:"pasajero_telefono"`

	// Tramo reservado.
	ParadaOrigenID     int64  `json:"corrida_parada_origen_id"`
	ParadaOrigenNombre string `json:"parada_origen_nombre"`
	OrdenOrigen        int    `json:"orden_origen"`

	ParadaDestinoID     int64  `json:"corrida_parada_destino_id"`
	ParadaDestinoNombre string `json:"parada_destino_nombre"`
	OrdenDestino        int    `json:"orden_destino"`

	// Información operativa.
	CantidadPasajeros    int    `json:"cantidad_pasajeros"`
	EstadoReserva        Estado `json:"estado_reserva"`
	RequiereConfirmacion bool   `json:"requiere_confirmacion"`

	DetalleAsistencia

	// Información financiera separada del estado operativo.
	Total          Dinero     `json:"total"`
	MontoPagado    Dinero     `json:"monto_pagado"`
	SaldoPendiente Dinero     `json:"saldo_pendiente"`
	EstadoPago     EstadoPago `json:"estado_pago"`

	Observaciones *string `json:"observaciones"`
}
