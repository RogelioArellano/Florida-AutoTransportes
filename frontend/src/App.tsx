import { useState } from 'react'
import LocalidadesPage from './features/localidades/LocalidadesPage'
import PuntosAbordajePage from './features/puntosAbordaje/PuntosAbordajePage'
import './App.css'

// Este tipo solo permite dos valores.
// TypeScript marcará error si intentamos usar una sección inexistente.
type Seccion =
  | 'localidades'
  | 'puntos-abordaje'

function App() {
  const [seccionActiva, setSeccionActiva] =
    useState<Seccion>('localidades')

  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="app-header-contenido">
          <div className="app-identidad">
            <span className="app-logo">FA</span>

            <div>
              <strong>Florida Autotransportes</strong>
              <small>Panel administrativo</small>
            </div>
          </div>

          <nav
            className="app-navegacion"
            aria-label="Catálogos operativos"
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
                seccionActiva === 'puntos-abordaje'
                  ? 'nav-boton nav-boton-activo'
                  : 'nav-boton'
              }
              aria-pressed={
                seccionActiva === 'puntos-abordaje'
              }
              onClick={() =>
                setSeccionActiva(
                  'puntos-abordaje',
                )
              }
            >
              Puntos de abordaje
            </button>
          </nav>
        </div>
      </header>

      {seccionActiva === 'localidades' ? (
        <LocalidadesPage />
      ) : (
        <PuntosAbordajePage />
      )}
    </div>
  )
}

export default App

