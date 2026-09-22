import type { PuntoAbordaje } from "./types";

type PuntosAbordajeTableProps = {
    puntos: PuntoAbordaje[]
}

function formatearCoordeneadas(
    latitud: number | null,
    longitud: number | null,
): string {

    if (latitud === null || longitud === null) {
        return 'Sin coordenadas'
    }

    return `${latitud.toFixed(6)}, ${longitud.toFixed(6)}`
}

export default function PuntosAbordajeTable({
    puntos,
}: PuntosAbordajeTableProps) {
    if (puntos.length === 0) {
        return (
            <p className="estado-vacio">
                Todavia no hay puntos de abordaje registrados
            </p>
        )
    }

    return (
        <div className="tabla-contenedor">
            <table className="puntos-tabla">
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Punto</th>
                        <th>Localidad</th>
                        <th>Referencia</th>
                        <th>Coordenadas</th>
                        <th>Estatus</th>
                    </tr>
                </thead>

                <tbody>
                    {puntos.map((punto) =>(
                        <tr key={punto.id}>
                            <td>{punto.id}</td>

                            <td>{punto.nombre}</td>
                            
                            <td>
                                {punto.localidad_nombre}, {punto.estado}
                            </td>

                            <td>
                                {punto.referencia || 'Sin referencia'}
                            </td>

                            <td>
                                {formatearCoordeneadas(
                                    punto.latitud,
                                    punto.longitud,
                                )}
                            </td>

                            <td>
                                <span
                                    className={
                                        punto.activo
                                        ? 'estatus estatus-activo'
                                        : 'estatus estatus-inactivo'
                                    }
                                >
                                    {punto.activo ? 'Activo' : 'Inactivo'}
                                </span>
                            </td>

                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    )
}
