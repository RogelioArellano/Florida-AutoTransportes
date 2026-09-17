// Representa la estructura que devuelve la API.
export type Localidad = {
  id: number
  nombre: string
  estado: string
  activa: boolean
  creado_en: string
  actualizado_en: string
}

// Representa únicamente los datos necesarios para crear una localidad.
export type CreateLocalidadInput = {
  nombre: string
  estado: string
}