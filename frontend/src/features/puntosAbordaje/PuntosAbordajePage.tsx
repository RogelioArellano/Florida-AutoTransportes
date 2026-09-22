import { useEffect, useState } from 'react'
import { listLocalidades } from '../localidades/api'
import type { Localidad } from '../localidades/types'
import {
  crearPuntoAbordaje,
  listarPuntosAbordaje,
} from './api'
import PuntoAbordajeForm from './PuntoAbordajeForm'
import PuntosAbordajeTable from './PuntosAbordajeTable'
import type {
  CrearPuntoAbordajeInput,
  PuntoAbordaje,
} from './types'

function obtenerMensajeError(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }

  return 'Ocurrió un error inesperado'
}

export default function PuntosAbordajePage() {
  const [localidades, setLocalidades] = useState<
    Localidad[]
  >([])

  const [puntos, setPuntos] = useState<
    PuntoAbordaje[]
  >([])

  const [cargando, setCargando] = useState(true)
  const [guardando, setGuardando] = useState(false)
  const [errorCarga, setErrorCarga] = useState('')
  const [mensaje, setMensaje] = useState('')

  useEffect(() => {
    async function cargarDatos() {
      setCargando(true)
      setErrorCarga('')

      try {
        // Ambas peticiones se ejecutan al mismo tiempo.
        const [
          localidadesObtenidas,
          puntosObtenidos,
        ] = await Promise.all([
          listLocalidades(),
          listarPuntosAbordaje(),
        ])

        setLocalidades(localidadesObtenidas)
        setPuntos(puntosObtenidos)
      } catch (error) {
        setErrorCarga(obtenerMensajeError(error))
      } finally {
        setCargando(false)
      }
    }

    void cargarDatos()
  }, [])

  async function manejarGuardado(
    input: CrearPuntoAbordajeInput,
  ) {
    setGuardando(true)
    setMensaje('')

    try {
      const puntoCreado =
        await crearPuntoAbordaje(input)

      // No necesitamos volver a consultar toda la lista.
      // Agregamos el nuevo punto al estado existente.
      setPuntos((puntosAnteriores) => [
        puntoCreado,
        ...puntosAnteriores,
      ])

      setMensaje(
        `El punto "${puntoCreado.nombre}" fue guardado correctamente.`,
      )
    } finally {
      // finally se ejecuta tanto si funciona como si falla.
      setGuardando(false)
    }
  }

  return (
    <main className="puntos-page">
      <header className="puntos-encabezado">
        <p className="etiqueta">
          Catálogos operativos
        </p>

        <h1>Puntos de abordaje</h1>

        <p>
          Administra los lugares donde los pasajeros
          pueden subir o bajar de la unidad.
        </p>
      </header>

      {errorCarga !== '' && (
        <p role="alert" className="error">
          {errorCarga}
        </p>
      )}

      {mensaje !== '' && (
        <p role="status" className="success">
          {mensaje}
        </p>
      )}

      {cargando ? (
        <p role="status">Cargando información...</p>
      ) : (
        <section className="puntos-layout">
          <div className="puntos-panel">
            <PuntoAbordajeForm
              localidades={localidades}
              guardando={guardando}
              onGuardar={manejarGuardado}
            />
          </div>

          <div className="puntos-panel puntos-listado">
            <div className="puntos-listado-titulo">
              <h2>Puntos registrados</h2>

              <span className="contador">
                {puntos.length}
              </span>
            </div>

            <PuntosAbordajeTable puntos={puntos} />
          </div>
        </section>
      )}
    </main>
  )
}