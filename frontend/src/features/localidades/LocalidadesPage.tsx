import {
  useEffect,
  useState,
  type ChangeEvent,
  type FormEvent,
} from 'react'

import {
  createLocalidad,
  listLocalidades,
} from './api'

import type {
  CreateLocalidadInput,
  Localidad,
} from './types'

const initialForm: CreateLocalidadInput = {
  nombre: '',
  estado: '',
}

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

export function LocalidadesPage() {
  const [form, setForm] =
    useState<CreateLocalidadInput>(initialForm)

  const [localidades, setLocalidades] =
    useState<Localidad[]>([])

  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const [loadError, setLoadError] = useState('')
  const [formError, setFormError] = useState('')
  const [successMessage, setSuccessMessage] =
    useState('')

  useEffect(() => {
    // AbortController permite cancelar la petición si el
    // componente desaparece antes de recibir la respuesta.
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

    // React ejecuta esta limpieza cuando el componente
    // deja de estar activo.
    return () => {
      controller.abort()
    }
  }, [])

  function handleInputChange(
    event: ChangeEvent<HTMLInputElement>,
  ) {
    const field =
      event.target.name as keyof CreateLocalidadInput

    const value = event.target.value

    // La función recibe el estado anterior y genera uno nuevo.
    // No modificamos directamente el objeto existente.
    setForm((currentForm) => ({
      ...currentForm,
      [field]: value,
    }))
  }

  async function handleSubmit(
    event: FormEvent<HTMLFormElement>,
  ) {
    // Evita que el navegador recargue toda la página.
    event.preventDefault()

    setSaving(true)
    setFormError('')
    setSuccessMessage('')

    try {
      const created = await createLocalidad({
        nombre: form.nombre.trim(),
        estado: form.estado.trim(),
      })

      // Creamos un arreglo nuevo para que React detecte
      // el cambio y vuelva a dibujar la tabla.
      setLocalidades((currentLocalidades) => {
        return [...currentLocalidades, created].sort(
          compareLocalidades,
        )
      })

      setForm(initialForm)
      setSuccessMessage(
        `La localidad ${created.nombre} fue registrada.`,
      )
    } catch (error) {
      setFormError(getErrorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  const invalidForm =
    form.nombre.trim() === '' ||
    form.estado.trim() === ''

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
        <section className="card">
          <h2>Nueva localidad</h2>

          <form
            className="form"
            onSubmit={handleSubmit}
          >
            <label htmlFor="nombre">
              Localidad
            </label>

            <input
              id="nombre"
              name="nombre"
              type="text"
              value={form.nombre}
              onChange={handleInputChange}
              maxLength={100}
              autoComplete="off"
              placeholder="Ejemplo: Morelia"
              disabled={saving}
              required
            />

            <label htmlFor="estado">
              Estado
            </label>

            <input
              id="estado"
              name="estado"
              type="text"
              value={form.estado}
              onChange={handleInputChange}
              maxLength={100}
              autoComplete="off"
              placeholder="Ejemplo: Michoacán"
              disabled={saving}
              required
            />

            <button
              type="submit"
              disabled={saving || invalidForm}
            >
              {saving
                ? 'Guardando...'
                : 'Guardar localidad'}
            </button>
          </form>

          <div
            className="message-area"
            aria-live="polite"
          >
            {formError !== '' && (
              <p className="message error">
                {formError}
              </p>
            )}

            {successMessage !== '' && (
              <p className="message success">
                {successMessage}
              </p>
            )}
          </div>
        </section>

        <section className="card">
          <div className="section-heading">
            <h2>Localidades registradas</h2>

            <span className="counter">
              {localidades.length}
            </span>
          </div>

          {loading && (
            <p className="muted">
              Consultando localidades...
            </p>
          )}

          {!loading && loadError !== '' && (
            <p className="message error">
              {loadError}
            </p>
          )}

          {!loading &&
            loadError === '' &&
            localidades.length === 0 && (
              <p className="muted">
                Todavía no hay localidades registradas.
              </p>
            )}

          {!loading &&
            loadError === '' &&
            localidades.length > 0 && (
              <div className="table-container">
                <table>
                  <thead>
                    <tr>
                      <th>ID</th>
                      <th>Localidad</th>
                      <th>Estado</th>
                      <th>Estatus</th>
                    </tr>
                  </thead>

                  <tbody>
                    {localidades.map((localidad) => (
                      <tr key={localidad.id}>
                        <td>{localidad.id}</td>
                        <td>{localidad.nombre}</td>
                        <td>{localidad.estado}</td>
                        <td>
                          <span
                            className={
                              localidad.activa
                                ? 'status active'
                                : 'status inactive'
                            }
                          >
                            {localidad.activa
                              ? 'Activa'
                              : 'Inactiva'}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
        </section>
      </div>
    </main>
  )
}