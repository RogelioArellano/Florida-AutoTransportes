import { useState } from 'react'

export default function App() {
  const [mensaje, setMensaje] = useState('Conexión pendiente de comprobar.')
  const [consultando, setConsultando] = useState(false)
  const [hayError, setHayError] = useState(false)

  async function probarConexion() {
    setConsultando(true)
    setHayError(false)
    setMensaje('Consultando la API...')

    const controller = new AbortController()
    const timeout = window.setTimeout(() => controller.abort(), 5000)

    try {
      const response = await fetch('/api/healthz', {
        signal: controller.signal,
        cache: 'no-store',
      })

      if (!response.ok) {
        throw new Error(`La API respondió con HTTP ${response.status}.`)
      }

      const data: unknown = await response.json()
      if (
        typeof data !== 'object' ||
        data === null ||
        !('status' in data) ||
        data.status !== 'ok' ||
        !('service' in data) ||
        typeof data.service !== 'string'
      ) {
        throw new Error('La respuesta de la API tiene un formato inesperado.')
      }

      setMensaje(`Conexión correcta con ${data.service}.`)
    } catch (error) {
      setHayError(true)
      if (controller.signal.aborted) {
        setMensaje('La consulta superó los 5 segundos de espera.')
      } else {
        const detalle = error instanceof Error ? error.message : 'Error desconocido.'
        setMensaje(`No se pudo comprobar la conexión. ${detalle}`)
      }
    } finally {
      window.clearTimeout(timeout)
      setConsultando(false)
    }
  }

  return (
    <main className="panel">
      <p className="etiqueta">Primera integración</p>
      <h1>Florida Autotransportes</h1>
      <p>Conexión entre la interfaz React y la API Go.</p>
      <button onClick={probarConexion} disabled={consultando}>
        {consultando ? 'Consultando...' : 'Probar conexión'}
      </button>
      <p
        role="status"
        aria-live="polite"
        className={hayError ? 'mensaje error' : 'mensaje'}
      >
        {mensaje}
      </p>
    </main>
  )
}