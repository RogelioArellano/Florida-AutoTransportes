import type {
  CrearPuntoAbordajeInput,
  PuntoAbordaje,
} from './types'

const PUNTOS_ABORDAJE_URL = '/api/puntos-abordaje'

// Record<string, unknown> representa un objeto cuya estructura
// todavía no conocemos con seguridad.
function esObjeto(valor: unknown): valor is Record<string, unknown> {
  return typeof valor === 'object' && valor !== null
}

function esNumeroONull(valor: unknown): valor is number | null {
  return valor === null || typeof valor === 'number'
}

// Aunque TypeScript conoce el tipo que esperamos, la información
// recibida por internet puede contener cualquier cosa.
//
// Esta función revisa el JSON durante la ejecución.
function esPuntoAbordaje(valor: unknown): valor is PuntoAbordaje {
  if (!esObjeto(valor)) {
    return false
  }

  return (
    typeof valor.id === 'number' &&
    typeof valor.localidad_id === 'number' &&
    typeof valor.localidad_nombre === 'string' &&
    typeof valor.estado === 'string' &&
    typeof valor.nombre === 'string' &&
    (valor.referencia === null ||
      typeof valor.referencia === 'string') &&
    esNumeroONull(valor.latitud) &&
    esNumeroONull(valor.longitud) &&
    typeof valor.activo === 'boolean' &&
    typeof valor.creado_en === 'string' &&
    typeof valor.actualizado_en === 'string'
  )
}

// Admite tanto:
//
// [ ... ]
//
// como:
//
// { "data": [ ... ] }
//
// Esto hace que la función sea compatible con las dos formas
// comunes de responder desde una API.
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
    // Algunas respuestas de error pueden no contener JSON.
  }

  return `La API respondió con el estado ${respuesta.status}`
}

export async function listarPuntosAbordaje(): Promise<
  PuntoAbordaje[]
> {
  const respuesta = await fetch(PUNTOS_ABORDAJE_URL)

  if (!respuesta.ok) {
    throw new Error(await obtenerMensajeError(respuesta))
  }

  const cuerpo: unknown = await respuesta.json()
  const datos = extraerData(cuerpo)

  if (
    !Array.isArray(datos) ||
    !datos.every(esPuntoAbordaje)
  ) {
    throw new Error(
      'La API devolvió una lista de puntos de abordaje inválida',
    )
  }

  return datos
}

export async function crearPuntoAbordaje(
  input: CrearPuntoAbordajeInput,
): Promise<PuntoAbordaje> {
  const respuesta = await fetch(PUNTOS_ABORDAJE_URL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })

  if (!respuesta.ok) {
    throw new Error(await obtenerMensajeError(respuesta))
  }

  const cuerpo: unknown = await respuesta.json()
  const datos = extraerData(cuerpo)

  if (!esPuntoAbordaje(datos)) {
    throw new Error(
      'La API devolvió un punto de abordaje inválido',
    )
  }

  return datos
}