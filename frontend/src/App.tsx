import { useEffect,useState } from 'react'
import LocalidadesPage from './features/localidades/LocalidadesPage'
import PuntosAbordajePage from './features/puntosAbordaje/PuntosAbordajePage'
import RutasPage from './features/rutas/RutasPage'
import './App.css'

type Seccion =
  | 'localidades'
  | 'puntos-abordaje'
  | 'rutas'

function App() {
  const [seccionActiva, setSeccionActiva] =
    useState<Seccion>('localidades')

    useEffect(() => {
  window.scrollTo({
    top: 0,
    left: 0,
    behavior: 'auto',
  })
}, [seccionActiva])

  function renderizarSeccion() {
    switch (seccionActiva) {
      case 'localidades':
        return <LocalidadesPage />

      case 'puntos-abordaje':
        return <PuntosAbordajePage />

      case 'rutas':
        return <RutasPage />
    }
  }

  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="app-header-contenido">
          <div className="app-identidad">
            <span className="app-logo">FA</span>

            <div>
              <strong>
                Florida Autotransportes
              </strong>

              <small>
                Panel administrativo
              </small>
            </div>
          </div>

          <nav
            className="app-navegacion"
            aria-label="Navegación principal"
          >
            <button
              type="button"
              className={
                seccionActiva === 'localidades'
                  ? 'nav-boton nav-boton-activo'
                  : 'nav-boton'
              }
              aria-pressed={
                seccionActiva === 'localidades'
              }
              onClick={() =>
                setSeccionActiva('localidades')
              }
            >
              Localidades
            </button>

            <button
              type="button"
              className={
                seccionActiva ===
                'puntos-abordaje'
                  ? 'nav-boton nav-boton-activo'
                  : 'nav-boton'
              }
              aria-pressed={
                seccionActiva ===
                'puntos-abordaje'
              }
              onClick={() =>
                setSeccionActiva(
                  'puntos-abordaje',
                )
              }
            >
              Puntos de abordaje
            </button>

            <button
              type="button"
              className={
                seccionActiva === 'rutas'
                  ? 'nav-boton nav-boton-activo'
                  : 'nav-boton'
              }
              aria-pressed={
                seccionActiva === 'rutas'
              }
              onClick={() =>
                setSeccionActiva('rutas')
              }
            >
              Rutas
            </button>
          </nav>
        </div>
      </header>

      {renderizarSeccion()}
    </div>
  )
}

export default App