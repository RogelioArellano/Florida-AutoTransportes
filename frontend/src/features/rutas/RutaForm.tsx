import {
  useState,
  type FormEvent,
} from 'react'
import type { PuntoAbordaje } from '../puntosAbordaje/types'
import type { CrearRutaInput } from './types'

type RutaFormProps = {
  puntos: PuntoAbordaje[]
  guardando: boolean
  onGuardar: (
    input: CrearRutaInput,
  ) => Promise<void>
}

// Representa una parada mientras se edita en React.
//
// Los valores de los select HTML son string, por eso
// puntoAbordajeId todavía no es number.
type ParadaFormulario = {
  idTemporal: string
  puntoAbordajeId: string
  permiteSubir: boolean
  permiteBajar: boolean
  esObligatoria: boolean
}

// Partial indica que podemos actualizar uno o varios campos.
// Omit impide modificar accidentalmente idTemporal.
type CambiosParada = Partial<
  Omit<ParadaFormulario, 'idTemporal'>
>

function crearParadaVacia(): ParadaFormulario {
  return {
    // Este identificador solo existe en React.
    // No es el ID de la base de datos.
    idTemporal: crypto.randomUUID(),
    puntoAbordajeId: '',
    permiteSubir: true,
    permiteBajar: true,
    esObligatoria: false,
  }
}

function obtenerMensajeError(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }

  return 'Ocurrió un error inesperado'
}

export default function RutaForm({
  puntos,
  guardando,
  onGuardar,
}: RutaFormProps) {
  const [codigo, setCodigo] = useState('')
  const [nombre, setNombre] = useState('')

  const [paradas, setParadas] = useState<
    ParadaFormulario[]
  >([])

  const [error, setError] = useState('')

  const puntosActivos = puntos.filter(
    (punto) => punto.activo,
  )

  function agregarParada() {
    setError('')

    if (paradas.length >= puntosActivos.length) {
      setError(
        'No hay más puntos de abordaje disponibles',
      )
      return
    }

    setParadas((paradasAnteriores) => [
      ...paradasAnteriores,
      crearParadaVacia(),
    ])
  }

  function eliminarParada(idTemporal: string) {
    setError('')

    setParadas((paradasAnteriores) =>
      paradasAnteriores.filter(
        (parada) =>
          parada.idTemporal !== idTemporal,
      ),
    )
  }

  function actualizarParada(
    idTemporal: string,
    cambios: CambiosParada,
  ) {
    setParadas((paradasAnteriores) =>
      paradasAnteriores.map((parada) => {
        if (parada.idTemporal !== idTemporal) {
          return parada
        }

        return {
          ...parada,
          ...cambios,
        }
      }),
    )
  }

  // direccion solamente puede ser -1 o 1.
  // -1 mueve hacia arriba.
  //  1 mueve hacia abajo.
  function moverParada(
    indiceActual: number,
    direccion: -1 | 1,
  ) {
    setParadas((paradasAnteriores) => {
      const nuevoIndice =
        indiceActual + direccion

      if (
        nuevoIndice < 0 ||
        nuevoIndice >= paradasAnteriores.length
      ) {
        return paradasAnteriores
      }

      // Copiamos el arreglo para no modificar directamente
      // el estado existente de React.
      const nuevasParadas = [
        ...paradasAnteriores,
      ]

      const paradaActual =
        nuevasParadas[indiceActual]

      nuevasParadas[indiceActual] =
        nuevasParadas[nuevoIndice]

      nuevasParadas[nuevoIndice] =
        paradaActual

      return nuevasParadas
    })
  }

  function puntoYaSeleccionado(
    puntoID: number,
    idTemporalActual: string,
  ): boolean {
    return paradas.some(
      (parada) =>
        parada.idTemporal !== idTemporalActual &&
        Number(parada.puntoAbordajeId) === puntoID,
    )
  }

  async function manejarEnvio(
    evento: FormEvent<HTMLFormElement>,
  ) {
    evento.preventDefault()
    setError('')

    const codigoLimpio = codigo.trim()
    const nombreLimpio = nombre.trim()

    if (codigoLimpio === '') {
      setError('Escribe el código de la ruta')
      return
    }

    if (nombreLimpio === '') {
      setError('Escribe el nombre de la ruta')
      return
    }

    if (paradas.length < 2) {
      setError(
        'La ruta debe tener al menos dos paradas',
      )
      return
    }

    const puntoIDs = paradas.map((parada) =>
      Number(parada.puntoAbordajeId),
    )

    if (
      puntoIDs.some(
        (puntoID) =>
          !Number.isInteger(puntoID) ||
          puntoID <= 0,
      )
    ) {
      setError(
        'Selecciona un punto en todas las paradas',
      )
      return
    }

    const puntosUnicos = new Set(puntoIDs)

    if (puntosUnicos.size !== puntoIDs.length) {
      setError(
        'No puedes utilizar dos veces el mismo punto',
      )
      return
    }

    const paradasInput = paradas.map(
      (parada, indice) => {
        const esPrimera = indice === 0
        const esUltima =
          indice === paradas.length - 1

        return {
          punto_abordaje_id:
            Number(parada.puntoAbordajeId),

          // La primera parada siempre permite abordar.
          permite_subir:
            esPrimera || parada.permiteSubir,

          // La última parada siempre permite descender.
          permite_bajar:
            esUltima || parada.permiteBajar,

          // El inicio y final siempre son obligatorios.
          es_obligatoria:
            esPrimera ||
            esUltima ||
            parada.esObligatoria,
        }
      },
    )

    try {
      await onGuardar({
        codigo: codigoLimpio,
        nombre: nombreLimpio,
        paradas: paradasInput,
      })

      setCodigo('')
      setNombre('')
      setParadas([])
    } catch (errorDesconocido) {
      setError(
        obtenerMensajeError(errorDesconocido),
      )
    }
  }

  return (
    <form
      className="ruta-form"
      onSubmit={manejarEnvio}
    >
      <h2>Nueva ruta</h2>

      <div className="ruta-datos-grid">
        <div>
          <label htmlFor="ruta-codigo">
            Código
          </label>

          <input
            id="ruta-codigo"
            type="text"
            value={codigo}
            onChange={(evento) =>
              setCodigo(
                evento.target.value.toUpperCase(),
              )
            }
            placeholder="Ejemplo: MOR-CDMX"
            maxLength={30}
            disabled={guardando}
          />
        </div>

        <div>
          <label htmlFor="ruta-nombre">
            Nombre
          </label>

          <input
            id="ruta-nombre"
            type="text"
            value={nombre}
            onChange={(evento) =>
              setNombre(evento.target.value)
            }
            placeholder="Morelia a Ciudad de México"
            maxLength={150}
            disabled={guardando}
          />
        </div>
      </div>

      <div className="ruta-paradas-encabezado">
        <div>
          <h3>Paradas</h3>

          <small>
            El orden se lee de arriba hacia abajo.
          </small>
        </div>

        <button
          type="button"
          className="boton-secundario"
          onClick={agregarParada}
          disabled={
            guardando ||
            puntosActivos.length === 0
          }
        >
          Agregar parada
        </button>
      </div>

      {puntosActivos.length === 0 && (
        <p className="estado-vacio">
          Primero registra puntos de abordaje activos.
        </p>
      )}

      <div className="ruta-paradas">
        {paradas.map((parada, indice) => {
          const esPrimera = indice === 0
          const esUltima =
            indice === paradas.length - 1

          return (
            <article
              className="ruta-parada"
              key={parada.idTemporal}
            >
              <div className="ruta-parada-titulo">
                <strong>
                  {indice + 1}.{' '}
                  {esPrimera
                    ? 'Origen'
                    : esUltima
                      ? 'Destino'
                      : 'Parada intermedia'}
                </strong>

                <div className="ruta-parada-acciones">
                  <button
                    type="button"
                    title="Mover hacia arriba"
                    aria-label={`Mover parada ${indice + 1} hacia arriba`}
                    onClick={() =>
                      moverParada(indice, -1)
                    }
                    disabled={
                      guardando || esPrimera
                    }
                  >
                    ↑
                  </button>

                  <button
                    type="button"
                    title="Mover hacia abajo"
                    aria-label={`Mover parada ${indice + 1} hacia abajo`}
                    onClick={() =>
                      moverParada(indice, 1)
                    }
                    disabled={
                      guardando || esUltima
                    }
                  >
                    ↓
                  </button>

                  <button
                    type="button"
                    className="boton-eliminar"
                    onClick={() =>
                      eliminarParada(
                        parada.idTemporal,
                      )
                    }
                    disabled={guardando}
                  >
                    Quitar
                  </button>
                </div>
              </div>

              <label
                htmlFor={`punto-${parada.idTemporal}`}
              >
                Punto de abordaje
              </label>

              <select
                id={`punto-${parada.idTemporal}`}
                value={parada.puntoAbordajeId}
                onChange={(evento) =>
                  actualizarParada(
                    parada.idTemporal,
                    {
                      puntoAbordajeId:
                        evento.target.value,
                    },
                  )
                }
                disabled={guardando}
              >
                <option value="">
                  Selecciona un punto
                </option>

                {puntosActivos.map((punto) => (
                  <option
                    key={punto.id}
                    value={String(punto.id)}
                    disabled={puntoYaSeleccionado(
                      punto.id,
                      parada.idTemporal,
                    )}
                  >
                    {punto.nombre} —{' '}
                    {punto.localidad_nombre},{' '}
                    {punto.estado}
                  </option>
                ))}
              </select>

              <div className="ruta-operaciones">
                <label>
                  <input
                    type="checkbox"
                    checked={
                      esPrimera ||
                      parada.permiteSubir
                    }
                    onChange={(evento) =>
                      actualizarParada(
                        parada.idTemporal,
                        {
                          permiteSubir:
                            evento.target.checked,
                        },
                      )
                    }
                    disabled={
                      guardando || esPrimera
                    }
                  />

                  Permite subir
                </label>

                <label>
                  <input
                    type="checkbox"
                    checked={
                      esUltima ||
                      parada.permiteBajar
                    }
                    onChange={(evento) =>
                      actualizarParada(
                        parada.idTemporal,
                        {
                          permiteBajar:
                            evento.target.checked,
                        },
                      )
                    }
                    disabled={
                      guardando || esUltima
                    }
                  />

                  Permite bajar
                </label>

                <label>
                  <input
                    type="checkbox"
                    checked={
                      esPrimera ||
                      esUltima ||
                      parada.esObligatoria
                    }
                    onChange={(evento) =>
                      actualizarParada(
                        parada.idTemporal,
                        {
                          esObligatoria:
                            evento.target.checked,
                        },
                      )
                    }
                    disabled={
                      guardando ||
                      esPrimera ||
                      esUltima
                    }
                  />

                  Parada obligatoria
                </label>
              </div>
            </article>
          )
        })}
      </div>

      {error !== '' && (
        <p role="alert" className="error">
          {error}
        </p>
      )}

      <button
        type="submit"
        disabled={
          guardando || puntosActivos.length < 2
        }
      >
        {guardando
          ? 'Guardando ruta...'
          : 'Guardar ruta'}
      </button>
    </form>
  )
}