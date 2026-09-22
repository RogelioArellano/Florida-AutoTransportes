// Representa un punto de abordaje que ya existe en el sistema.
// Su estructura debe coincidir con el JSON enviado por la API de Go.
export type PuntoAbordaje = {
  id: number
  localidad_id: number
  localidad_nombre: string
  estado: string
  nombre: string
  referencia: string | null
  latitud: number | null
  longitud: number | null
  activo: boolean
  creado_en: string
  actualizado_en: string
}

// Representa los datos que React enviará al backend
// cuando el usuario registre un nuevo punto.
export type CrearPuntoAbordajeInput = {
  localidad_id: number
  nombre: string
  referencia: string
  latitud: number | null
  longitud: number | null
}