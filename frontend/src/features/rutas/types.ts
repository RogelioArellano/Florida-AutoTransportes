// Representa una parada perteneciente a una ruta
// que ya fue guardada en la base de datos.
export type RutaParada = {
  id: number
  punto_abordaje_id: number
  punto_nombre: string
  localidad_nombre: string
  estado: string
  orden: number
  permite_subir: boolean
  permite_bajar: boolean
  es_obligatoria: boolean
  creado_en: string
}

// Representa una ruta completa obtenida desde la API.
export type Ruta = {
  id: number
  codigo: string
  nombre: string
  activa: boolean
  paradas: RutaParada[]
  creado_en: string
  actualizado_en: string
}

// Representa una parada que todavía no ha sido guardada.
//
// No contiene id, orden, nombres ni fechas porque esos
// valores serán generados u obtenidos por el backend.
export type CrearRutaParadaInput = {
  punto_abordaje_id: number
  permite_subir: boolean
  permite_bajar: boolean
  es_obligatoria: boolean
}

// Representa el cuerpo del POST /api/rutas.
export type CrearRutaInput = {
  codigo: string
  nombre: string
  paradas: CrearRutaParadaInput[]
}