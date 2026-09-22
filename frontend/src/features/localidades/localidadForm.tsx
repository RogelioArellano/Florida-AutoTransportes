import {
  useState,
  type ChangeEvent,
  type FormEvent,
} from 'react'

import type {
  CreateLocalidadInput,
  Localidad,
} from './types'

// Props describe los datos y funciones que el componente
// padre debe proporcionar al formulario.
type LocalidadFormProps = {
  onSubmit: (
    input: CreateLocalidadInput,
  ) => Promise<Localidad>
}

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

export function LocalidadForm({
  onSubmit,
}: LocalidadFormProps) {
  const [form, setForm] =
    useState<CreateLocalidadInput>(initialForm)

  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [successMessage, setSuccessMessage] =
    useState('')

  function handleNombreChange(
    event: ChangeEvent<HTMLInputElement>,
  ) {
    // event.target es el input que originó el evento.
    // event.target.value contiene el texto actual.
    const newValue = event.target.value

    // React necesita un objeto nuevo.
    // ...currentForm copia los valores anteriores.
    setForm((currentForm) => ({
      ...currentForm,
      nombre: newValue,
    }))
  }

  function handleEstadoChange(
    event: ChangeEvent<HTMLInputElement>,
  ) {
    const newValue = event.target.value

    setForm((currentForm) => ({
      ...currentForm,
      estado: newValue,
    }))
  }

  async function handleSubmit(
    event: FormEvent<HTMLFormElement>,
  ) {
    // Un formulario HTML recarga la página por defecto.
    // React evita esa recarga para controlar el proceso.
    event.preventDefault()

    setSaving(true)
    setError('')
    setSuccessMessage('')

    try {
      // onSubmit fue recibido desde el componente padre.
      const created = await onSubmit({
        nombre: form.nombre.trim(),
        estado: form.estado.trim(),
      })

      setForm(initialForm)

      setSuccessMessage(
        `La localidad ${created.nombre} fue registrada.`,
      )
    } catch (submitError) {
      setError(getErrorMessage(submitError))
    } finally {
      // finally se ejecuta tanto si hubo éxito como error.
      setSaving(false)
    }
  }

  const invalidForm =
    form.nombre.trim() === '' ||
    form.estado.trim() === ''

  return (
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
          onChange={handleNombreChange}
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
          onChange={handleEstadoChange}
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
        {error !== '' && (
          <p className="message error">
            {error}
          </p>
        )}

        {successMessage !== '' && (
          <p className="message success">
            {successMessage}
          </p>
        )}
      </div>
    </section>
  )
}