package unidades

import "time"

/*
Unidad representa los vehiculos del negocio utilizados para realizar los viajes

capacidad_total incluye todos los lugares fisicos de la unidad
capacidad_pasajeros representa solo los lugares disponibles para venta
*/
type Unidad struct {
	ID                 int64     `json:"id"`
	Codigo             string    `json:"codigo"`
	Placas             *string   `json:"placas"`
	Marca              *string   `json:"marca"`
	Modelo             *string   `json:"modelo"`
	Anio               *int      `json:"anio"`
	CapacidadTotal     int       `json:"capacidad_total"`
	CapacidadPasajeros int       `json:"capacidad_pasajeros"`
	Activa             bool      `json:"activa"`
	CreadoEn           time.Time `json:"creado_en"`
	ActualizadoEn      time.Time `json:"actualizado_en"`
}

/*
CreateInput representa los datos que el cliente debe enviar
para registrar una unidad.

No se incluye el ID, activa ni las fechas porque esos valores serán
generados por la BD
*/

type CreateInput struct {
	Codigo             string  `json:"codigo"`
	Placas             *string `json:"placas"`
	Marca              *string `json:"marca"`
	Modelo             *string `json:"modelo"`
	Anio               *int    `json:"anio"`
	CapacidadTotal     int     `json:"capacidad_total"`
	CapacidadPasajeros int     `json:"capacidad_pasajeros"`
}
