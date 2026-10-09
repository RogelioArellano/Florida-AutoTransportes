package reservas

import "time"

// DetalleAsistencia se incorpora en las consultas de reservas y pasajeros.
// NULL conserva las cantidades históricas desconocidas. Las ausencias
// definitivas se calculan solamente después del cierre de asistencia.
type DetalleAsistencia struct {
	CantidadAbordada        *int       `json:"cantidad_abordada"`
	AbordadaEn              *time.Time `json:"abordada_en"`
	AsistenciaCerradaEn     *time.Time `json:"asistencia_cerrada_en"`
	ObservacionesAsistencia *string    `json:"observaciones_asistencia"`
	AsistenciaConocida      bool       `json:"asistencia_conocida"`
	AsistenciaCerrada       bool       `json:"asistencia_cerrada"`
	CantidadPendiente       *int       `json:"cantidad_pendiente"`
	CantidadNoPresentada    *int       `json:"cantidad_no_presentada"`
}

// AsistenciaReserva representa la asistencia de una reserva,
// que puede incluir uno o varios pasajeros.
//
// Los campos de cantidad son punteros para conservar NULL:
// una asistencia histórica desconocida no equivale a cero.
type AsistenciaReserva struct {
	ReservaID    int64  `json:"reserva_id"`
	ReservaFolio string `json:"reserva_folio"`
	CorridaID    int64  `json:"corrida_id"`

	ParadaOrigenID     int64  `json:"corrida_parada_origen_id"`
	ParadaOrigenNombre string `json:"parada_origen_nombre"`

	EstadoReserva     Estado `json:"estado_reserva"`
	CantidadPasajeros int    `json:"cantidad_pasajeros"`
	CantidadAbordada  *int   `json:"cantidad_abordada"`

	AbordadaEn              *time.Time `json:"abordada_en"`
	AsistenciaCerradaEn     *time.Time `json:"asistencia_cerrada_en"`
	ObservacionesAsistencia *string    `json:"observaciones_asistencia"`

	// El Service calcula estos campos a partir de los datos
	// persistidos; el Repository no debe calcularlos.
	AsistenciaConocida   bool `json:"asistencia_conocida"`
	AsistenciaCerrada    bool `json:"asistencia_cerrada"`
	CantidadPendiente    *int `json:"cantidad_pendiente"`
	CantidadNoPresentada *int `json:"cantidad_no_presentada"`
}

// RegistrarAbordajeInput registra la cantidad TOTAL acumulada,
// no una cantidad que deba sumarse a los abordajes anteriores.
//
// Ejemplo: enviar CantidadAbordada=2 dos veces debe conservar
// dos abordajes, no producir cuatro.
type RegistrarAbordajeInput struct {
	ReservaID        int64 `json:"reserva_id"`
	CantidadAbordada int   `json:"cantidad_abordada"`

	// Si se omite o llega vacío, se conservan las notas existentes.
	Observaciones *string `json:"observaciones"`
}

// CerrarAsistenciaInput cierra solamente una reserva.
// El Repository consulta su punto de origen y cantidad actual.
// No se marcan ausentes todas las reservas de la corrida.
type CerrarAsistenciaInput struct {
	ReservaID     int64   `json:"reserva_id"`
	Observaciones *string `json:"observaciones"`
}
