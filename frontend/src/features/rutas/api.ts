import type {
  CrearRutaInput,
  Ruta,
  RutaParada,
} from './types'

const RUTAS_URL = '/api/rutas'

function esObjeto(
  valor: unknown,
): valor is Record<string, unknown> {
  return typeof valor === 'object' && valor !== null
}

function esRutaParada(
  valor: unknown,
): valor is RutaParada {
  if (!esObjeto(valor)) {
    return false
  }

  return (
    typeof valor.id === 'number' &&
    typeof valor.punto_abordaje_id === 'number' &&
    typeof valor.punto_nombre === 'string' &&
    typeof valor.localidad_nombre === 'string' &&
    typeof valor.estado === 'string' &&
    typeof valor.orden === 'number' &&
    typeof valor.permite_subir === 'boolean' &&
    typeof valor.permite_bajar === 'boolean' &&
    typeof valor.es_obligatoria === 'boolean' &&
    typeof valor.creado_en === 'string'
  )
}

function esRuta(valor: unknown): valor is Ruta {
  if (!esObjeto(valor)) {
    return false
  }

  return (
    typeof valor.id === 'number' &&
    typeof valor.codigo === 'string' &&
    typeof valor.nombre === 'string' &&
    typeof valor.activa === 'boolean' &&
    Array.isArray(valor.paradas) &&
    valor.paradas.every(esRutaParada) &&
    typeof valor.creado_en === 'string' &&
    typeof valor.actualizado_en === 'string'
  )
}

function extraerData(valor: unknown): unknown {
  if (esObjeto(valor) && 'data' in valor) {
    return valor.data
  }

  return valor
}

async function obtenerMensajeError(
  respuesta: Response,
): Promise<string> {
  try {
    const cuerpo: unknown = await respuesta.json()

    if (
      esObjeto(cuerpo) &&
      typeof cuerpo.error === 'string'
    ) {
      return cuerpo.error
    }
  } catch {
    // La respuesta podría no contener JSON.
  }

  return `La API respondió con el estado ${respuesta.status}`
}

export async function listarRutas(): Promise<Ruta[]> {
  const respuesta = await fetch(RUTAS_URL)

  if (!respuesta.ok) {
    throw new Error(
      await obtenerMensajeError(respuesta),
    )
  }

  const cuerpo: unknown = await respuesta.json()
  const datos = extraerData(cuerpo)

  if (
    !Array.isArray(datos) ||
    !datos.every(esRuta)
  ) {
    throw new Error(
      'La API devolvió una lista de rutas inválida',
    )
  }

  return datos
}

export async function crearRuta(
  input: CrearRutaInput,
): Promise<Ruta> {
  const respuesta = await fetch(RUTAS_URL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })

  if (!respuesta.ok) {
    throw new Error(
      await obtenerMensajeError(respuesta),
    )
  }

  const cuerpo: unknown = await respuesta.json()
  const datos = extraerData(cuerpo)

  if (!esRuta(datos)) {
    throw new Error(
      'La API devolvió una ruta inválida',
    )
  }

  return datos
}