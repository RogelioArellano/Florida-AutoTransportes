import type { Localidad } from './types'

type LocalidadesTableProps = {
  localidades: Localidad[]
  loading: boolean
  error: string
}

export function LocalidadesTable({
  localidades,
  loading,
  error,
}: LocalidadesTableProps) {
  return (
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

      {!loading && error !== '' && (
        <p className="message error">
          {error}
        </p>
      )}

      {!loading &&
        error === '' &&
        localidades.length === 0 && (
          <p className="muted">
            Todavía no hay localidades registradas.
          </p>
        )}

      {!loading &&
        error === '' &&
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
  )
}