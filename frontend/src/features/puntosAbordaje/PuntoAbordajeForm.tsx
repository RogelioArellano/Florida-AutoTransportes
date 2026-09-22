import { useState, type FormEvent } from 'react'
import type { Localidad } from '../localidades/types'
import type { CrearPuntoAbordajeInput } from './types'

// Estas son las propiedades que el componente padre
// deberá proporcionar al formulario.
type PuntoAbordajeFormProps = {
  localidades: Localidad[]
  guardando: boolean
  onGuardar: (
    input: CrearPuntoAbordajeInput,
  ) => Promise<void>
}

// Los campos HTML siempre manejan inicialmente texto,
// incluso cuando el input tiene type="number".
type FormularioState = {
  localidadId: string
  nombre: string
  referencia: string
  latitud: string
  longitud: string
}

const formularioInicial: FormularioState = {
  localidadId: '',
  nombre: '',
  referencia: '',
  latitud: '',
  longitud: '',
}

function convertirNumeroOpcional(valor: string): number | null {
  const valorLimpio = valor.trim()

  if (valorLimpio === '') {
    return null
  }

  return Number(valorLimpio)
}

function obtenerMensajeError(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }

  return 'Ocurrió un error inesperado'
}

export default function PuntoAbordajeForm({
  localidades,
  guardando,
  onGuardar,
}: PuntoAbordajeFormProps) {
  const [formulario, setFormulario] =
    useState<FormularioState>(formularioInicial)

  const [error, setError] = useState('')

  // keyof FormularioState limita "campo" a:
  // localidadId, nombre, referencia, latitud o longitud.
  function actualizarCampo(
    campo: keyof FormularioState,
    valor: string,
  ) {
    setFormulario((formularioAnterior) => ({
      ...formularioAnterior,
      [campo]: valor,
    }))
  }

  async function manejarEnvio(
    evento: FormEvent<HTMLFormElement>,
  ) {
    // Evita que el navegador recargue la página.
    evento.preventDefault()

    setError('')

    const localidadId = Number(formulario.localidadId)
    const nombre = formulario.nombre.trim()
    const referencia = formulario.referencia.trim()

    const latitud = convertirNumeroOpcional(
      formulario.latitud,
    )

    const longitud = convertirNumeroOpcional(
      formulario.longitud,
    )

    if (!Number.isInteger(localidadId) || localidadId <= 0) {
      setError('Selecciona una localidad')
      return
    }

    if (nombre === '') {
      setError('Escribe el nombre del punto de abordaje')
      return
    }

    // Ambos campos deben capturarse juntos.
    if (
      (latitud === null && longitud !== null) ||
      (latitud !== null && longitud === null)
    ) {
      setError(
        'Captura tanto la latitud como la longitud',
      )
      return
    }

    if (
      latitud !== null &&
      !Number.isFinite(latitud)
    ) {
      setError('La latitud no es un número válido')
      return
    }

    if (
      longitud !== null &&
      !Number.isFinite(longitud)
    ) {
      setError('La longitud no es un número válido')
      return
    }

    if (
      latitud !== null &&
      (latitud < -90 || latitud > 90)
    ) {
      setError('La latitud debe estar entre -90 y 90')
      return
    }

    if (
      longitud !== null &&
      (longitud < -180 || longitud > 180)
    ) {
      setError(
        'La longitud debe estar entre -180 y 180',
      )
      return
    }

    const input: CrearPuntoAbordajeInput = {
      localidad_id: localidadId,
      nombre,
      referencia,
      latitud,
      longitud,
    }

    try {
      await onGuardar(input)

      // Solo limpiamos el formulario cuando el backend
      // confirma que el registro fue guardado.
      setFormulario(formularioInicial)
    } catch (errorDesconocido) {
      setError(obtenerMensajeError(errorDesconocido))
    }
  }

  return (
    <form
      className="punto-abordaje-form"
      onSubmit={manejarEnvio}
    >
      <h2>Nuevo punto de abordaje</h2>

      <label htmlFor="localidad">
        Localidad
      </label>

      <select
        id="localidad"
        value={formulario.localidadId}
        onChange={(evento) =>
          actualizarCampo(
            'localidadId',
            evento.target.value,
          )
        }
        disabled={guardando}
      >
        <option value="">
          Selecciona una localidad
        </option>

        {localidades
          .filter((localidad) => localidad.activa)
          .map((localidad) => (
            <option
              key={localidad.id}
              value={localidad.id}
            >
              {localidad.nombre}, {localidad.estado}
            </option>
          ))}
      </select>

      <label htmlFor="nombre-punto">
        Nombre del punto
      </label>

      <input
        id="nombre-punto"
        type="text"
        value={formulario.nombre}
        onChange={(evento) =>
          actualizarCampo(
            'nombre',
            evento.target.value,
          )
        }
        placeholder="Ejemplo: Pabellón Don Vasco"
        maxLength={150}
        disabled={guardando}
      />

      <label htmlFor="referencia">
        Referencia
      </label>

      <textarea
        id="referencia"
        value={formulario.referencia}
        onChange={(evento) =>
          actualizarCampo(
            'referencia',
            evento.target.value,
          )
        }
        placeholder="Ejemplo: Frente a la entrada principal"
        maxLength={500}
        rows={3}
        disabled={guardando}
      />

      <div className="coordenadas-grid">
        <div>
          <label htmlFor="latitud">
            Latitud
          </label>

          <input
            id="latitud"
            type="number"
            step="any"
            value={formulario.latitud}
            onChange={(evento) =>
              actualizarCampo(
                'latitud',
                evento.target.value,
              )
            }
            placeholder="19.702600"
            disabled={guardando}
          />
        </div>

        <div>
          <label htmlFor="longitud">
            Longitud
          </label>

          <input
            id="longitud"
            type="number"
            step="any"
            value={formulario.longitud}
            onChange={(evento) =>
              actualizarCampo(
                'longitud',
                evento.target.value,
              )
            }
            placeholder="-101.192000"
            disabled={guardando}
          />
        </div>
      </div>

      <small>
        Las coordenadas son opcionales, pero deben
        capturarse juntas.
      </small>

      {error !== '' && (
        <p role="alert" className="error">
          {error}
        </p>
      )}

      <button type="submit" disabled={guardando}>
        {guardando
          ? 'Guardando...'
          : 'Guardar punto de abordaje'}
      </button>
    </form>
  )
}