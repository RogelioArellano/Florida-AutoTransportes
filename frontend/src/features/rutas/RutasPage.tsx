import {
  useEffect,
  useState,
} from 'react'
import { listarPuntosAbordaje } from '../puntosAbordaje/api'
import type { PuntoAbordaje } from '../puntosAbordaje/types'
import {
  crearRuta,
  listarRutas,
} from './api'
import RutaForm from './RutaForm'
import RutasTable from './RutasTable'
import type {
  CrearRutaInput,
  Ruta,
} from './types'

function obtenerMensajeError(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }

  return 'Ocurrió un error inesperado'
}

export default function RutasPage() {
  const [puntos, setPuntos] = useState<
    PuntoAbordaje[]
  >([])

  const [rutas, setRutas] = useState<Ruta[]>([])
  const [cargando, setCargando] = useState(true)
  const [guardando, setGuardando] =
    useState(false)

  const [errorCarga, setErrorCarga] =
    useState('')

  const [mensaje, setMensaje] = useState('')

  useEffect(() => {
    async function cargarDatos() {
      setCargando(true)
      setErrorCarga('')

      try {
        const [
          puntosObtenidos,
          rutasObtenidas,
        ] = await Promise.all([
          listarPuntosAbordaje(),
          listarRutas(),
        ])

        setPuntos(puntosObtenidos)
        setRutas(rutasObtenidas)
      } catch (error) {
        setErrorCarga(
          obtenerMensajeError(error),
        )
      } finally {
        setCargando(false)
      }
    }

    void cargarDatos()
  }, [])

  async function manejarGuardado(
    input: CrearRutaInput,
  ) {
    setGuardando(true)
    setMensaje('')

    try {
      const rutaCreada = await crearRuta(input)

      setRutas((rutasAnteriores) =>
        [...rutasAnteriores, rutaCreada].sort(
          (primeraRuta, segundaRuta) =>
            primeraRuta.nombre.localeCompare(
              segundaRuta.nombre,
              'es',
            ),
        ),
      )

      setMensaje(
        `La ruta "${rutaCreada.nombre}" fue guardada correctamente.`,
      )
    } finally {
      // Si crearRuta genera un error, se propagará hacia
      // RutaForm, que será responsable de mostrarlo.
      setGuardando(false)
    }
  }

  return (
    <main className="rutas-page">
      <header className="rutas-encabezado">
        <p className="etiqueta">
          Operación de viajes
        </p>

        <h1>Rutas</h1>

        <p>
          Define el orden de los puntos que recorrerá
          la unidad en cada dirección.
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
        <p role="status">
          Cargando rutas y puntos de abordaje...
        </p>
      ) : (
        <section className="rutas-layout">
          <div className="rutas-panel">
            <RutaForm
              puntos={puntos}
              guardando={guardando}
              onGuardar={manejarGuardado}
            />
          </div>

          <div className="rutas-panel rutas-listado">
            <div className="rutas-listado-titulo">
              <h2>Rutas registradas</h2>

              <span className="contador">
                {rutas.length}
              </span>
            </div>

            <RutasTable rutas={rutas} />
          </div>
        </section>
      )}
    </main>
  )
}