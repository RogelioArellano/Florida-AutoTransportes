import type {
  Ruta,
  RutaParada,
} from './types'

type RutasTableProps = {
  rutas: Ruta[]
}

function obtenerOperaciones(
  parada: RutaParada,
): string {
  const operaciones: string[] = []

  if (parada.permite_subir) {
    operaciones.push('Subir')
  }

  if (parada.permite_bajar) {
    operaciones.push('Bajar')
  }

  return operaciones.join(' y ')
}

export default function RutasTable({
  rutas,
}: RutasTableProps) {
  if (rutas.length === 0) {
    return (
      <p className="estado-vacio">
        Todavía no hay rutas registradas.
      </p>
    )
  }

  return (
    <div className="tabla-contenedor">
      <table className="rutas-tabla">
        <thead>
          <tr>
            <th>Código</th>
            <th>Ruta</th>
            <th>Recorrido</th>
            <th>Estatus</th>
          </tr>
        </thead>

        <tbody>
          {rutas.map((ruta) => (
            <tr key={ruta.id}>
              <td>
                <strong>{ruta.codigo}</strong>
              </td>

              <td>{ruta.nombre}</td>

              <td>
                <ol className="ruta-recorrido">
                  {ruta.paradas.map((parada) => (
                    <li key={parada.id}>
                      <span className="parada-orden">
                        {parada.orden}
                      </span>

                      <div className="parada-informacion">
                        <strong>
                          {parada.punto_nombre}
                        </strong>

                        <small>
                          {parada.localidad_nombre},{' '}
                          {parada.estado}
                        </small>

                        <div className="parada-etiquetas">
                          <span className="parada-operacion">
                            {obtenerOperaciones(
                              parada,
                            )}
                          </span>

                          <span
                            className={
                              parada.es_obligatoria
                                ? 'parada-tipo parada-obligatoria'
                                : 'parada-tipo parada-demanda'
                            }
                          >
                            {parada.es_obligatoria
                              ? 'Obligatoria'
                              : 'Bajo demanda'}
                          </span>
                        </div>
                      </div>
                    </li>
                  ))}
                </ol>
              </td>

              <td>
                <span
                  className={
                    ruta.activa
                      ? 'estatus estatus-activo'
                      : 'estatus estatus-inactivo'
                  }
                >
                  {ruta.activa
                    ? 'Activa'
                    : 'Inactiva'}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}