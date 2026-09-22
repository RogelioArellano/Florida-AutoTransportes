import {
  useEffect,
  useState,
} from 'react'

import {
  createLocalidad,
  listLocalidades,
} from './api'

import { LocalidadForm } from './localidadForm'
import { LocalidadesTable } from './localidadesTable'

import type {
  CreateLocalidadInput,
  Localidad,
} from './types'

function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message
  }

  return 'Ocurrió un error desconocido.'
}

function compareLocalidades(
  first: Localidad,
  second: Localidad,
): number {
  const estadoResult = first.estado.localeCompare(
    second.estado,
    'es-MX',
    {
      sensitivity: 'base',
    },
  )

  if (estadoResult !== 0) {
    return estadoResult
  }

  return first.nombre.localeCompare(
    second.nombre,
    'es-MX',
    {
      sensitivity: 'base',
    },
  )
}

export default function LocalidadesPage() {
  const [localidades, setLocalidades] =
    useState<Localidad[]>([])

  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  useEffect(() => {
    const controller = new AbortController()

    async function load() {
      try {
        setLoading(true)
        setLoadError('')

        const result = await listLocalidades(
          controller.signal,
        )

        setLocalidades(result)
      } catch (error) {
        if (
          error instanceof DOMException &&
          error.name === 'AbortError'
        ) {
          return
        }

        setLoadError(getErrorMessage(error))
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false)
        }
      }
    }

    void load()

    return () => {
      controller.abort()
    }
  }, [])

  async function handleCreate(
    input: CreateLocalidadInput,
  ): Promise<Localidad> {
    const created = await createLocalidad(input)

    // Usamos la versión funcional de setLocalidades porque
    // el nuevo estado depende del arreglo anterior.
    setLocalidades((currentLocalidades) => {
      const updatedLocalidades = [
        ...currentLocalidades,
        created,
      ]

      return updatedLocalidades.sort(
        compareLocalidades,
      )
    })

    return created
  }

  return (
    <main className="page">
      <header className="page-header">
        <p className="eyebrow">
          Catálogos operativos
        </p>

        <h1>Localidades</h1>

        <p>
          Administra las ciudades o poblaciones que
          posteriormente tendrán puntos de abordaje.
        </p>
      </header>

      <div className="content-grid">
        <LocalidadForm onSubmit={handleCreate} />

        <LocalidadesTable
          localidades={localidades}
          loading={loading}
          error={loadError}
        />
      </div>
    </main>
  )
}