import type {
  CreateLocalidadInput,
  Localidad,
} from './types'

// Esta funcion comprueba que un valor recibido realmente tenga
// la estructura esperada de una localidad
function isLocalidad(value: unknown): value is Localidad{
    if (typeof value !== 'object' || value == null) {
        return false
    }

    const localidad = value as Record<string, unknown>

    return (
        typeof localidad.id === 'number' &&
        Number.isSafeInteger(localidad.id) &&
        typeof localidad.nombre === 'string' &&
        typeof localidad.estado === 'string' &&
        typeof localidad.activa === 'boolean' &&
        typeof localidad.creado_en === 'string' &&
        typeof localidad.actualizado_en === 'string'
    )
}

// Intenta recuperar el mensaje de error enviado por la API
// Si la respuesta no tiene ese formato, devolverà un mensaje http general
async function readAPIError(
    response: Response,    
): Promise<string> {
    try{
        const data: unknown = await response.json()

        if (
            typeof data === 'object' &&
            data !== null &&
            'error' in data &&
            typeof data.error === 'string'
        ) {
            return data.error
        }
    } catch {
        //la respuesta no contenìa JSON valido
    }

    return `La API respondió con HTTP ${response.status}`
}

export async function listLocalidades(
    signal?: AbortSignal,    
): Promise<Localidad[]> {
    
    const response = await fetch('api/localidades', {
        method: 'GET',
        headers: {
            Accept: 'application/json',
        },
        cache: 'no-store',
        signal,
    })

    if (!response.ok) {
    throw new Error(await readAPIError(response))
  }

  const data: unknown = await response.json()

  if (
    !Array.isArray(data) ||
    !data.every(isLocalidad)
  ) {
    throw new Error(
      'La API devolvió un listado de localidades inválido.',
    )
  }

  return data
}

export async function createLocalidad(
  input: CreateLocalidadInput,
): Promise<Localidad> {
  const response = await fetch('/api/localidades', {
    method: 'POST',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  })

  if (!response.ok) {
    throw new Error(await readAPIError(response))
  }

  const data: unknown = await response.json()

  if (!isLocalidad(data)) {
    throw new Error(
      'La API devolvió una localidad inválida.',
    )
  }

  return data
}
