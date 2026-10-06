package reservas

import (
	"context"
	"errors"
	"fmt"
)

const (
	// La lista de pasajeros estará disponible para envío
	// desde 60 minutos antes de la salida programada.
	minutosAnticipacionListaPasajeros = 60
)

var (
	// ErrCorridaNoAceptaEnvioLista indica que la corrida no se
	// encuentra en condiciones operativas para enviar su lista.
	ErrCorridaNoAceptaEnvioLista = errors.New(
		"la corrida no acepta el envío de la lista de pasajeros",
	)

	// ErrEnvioListaFueraDeVentana indica que todavía no comienza
	// la ventana de 60 minutos o que la salida ya ocurrió.
	ErrEnvioListaFueraDeVentana = errors.New(
		"la corrida se encuentra fuera de la ventana para enviar la lista de pasajeros",
	)

	// ErrCorridaSinChoferLista indica que no existe un conductor
	// asignado al cual enviar el mensaje.
	ErrCorridaSinChoferLista = errors.New(
		"la corrida no tiene un chofer asignado",
	)

	// ErrCorridaSinReservasLista evita enviar una lista vacía.
	ErrCorridaSinReservasLista = errors.New(
		"la corrida no tiene reservas para enviar",
	)

	// ErrChoferListaPasajerosCambio evita marcar como exitoso un
	// mensaje enviado a un chofer que ya no está asignado.
	ErrChoferListaPasajerosCambio = errors.New(
		"el chofer asignado a la corrida cambió durante el envío",
	)
)

// ListaPasajerosEnvioStore define las operaciones de
// persistencia necesarias para automatizar los envíos.
type ListaPasajerosEnvioStore interface {
	// ListPendingPassengerLists obtiene las corridas que se
	// encuentran dentro de la ventana de envío.
	ListPendingPassengerLists(
		ctx context.Context,
		politica PoliticaEnvioListaPasajerosParams,
	) ([]CorridaPendienteListaPasajeros, error)

	// RegisterPassengerListDelivery registra un envío exitoso.
	//
	// El Repository volverá a validar la corrida, el chofer y
	// la ventana de tiempo dentro de una transacción.
	RegisterPassengerListDelivery(
		ctx context.Context,
		input RegistrarEnvioListaPasajerosInput,
		politica PoliticaEnvioListaPasajerosParams,
	) (EnvioListaPasajeros, error)
}

// ListaPasajerosEnvioService contiene las reglas independientes
// de PostgreSQL para la automatización de listas.
type ListaPasajerosEnvioService struct {
	store ListaPasajerosEnvioStore
}

// NewListaPasajerosEnvioService construye el servicio.
func NewListaPasajerosEnvioService(
	store ListaPasajerosEnvioStore,
) *ListaPasajerosEnvioService {
	return &ListaPasajerosEnvioService{
		store: store,
	}
}

// ListPendingPassengerLists consulta las corridas próximas a
// salir cuya lista debe enviarse o reenviarse.
//
// El reenvío se permite cuando cambió el chofer asignado.
func (s *ListaPasajerosEnvioService) ListPendingPassengerLists(
	ctx context.Context,
) ([]CorridaPendienteListaPasajeros, error) {
	pendientes, err :=
		s.store.ListPendingPassengerLists(
			ctx,
			politicaEnvioListaPasajerosActual(),
		)
	if err != nil {
		return nil, err
	}

	// Evitamos que JSON represente una lista vacía como null.
	if pendientes == nil {
		pendientes = make(
			[]CorridaPendienteListaPasajeros,
			0,
		)
	}

	return pendientes, nil
}

// RegisterPassengerListDelivery registra que el proveedor de
// mensajería confirmó un envío exitoso.
//
// Este método no envía mensajes. Únicamente guarda el resultado
// informado por n8n después de terminar el envío.
func (s *ListaPasajerosEnvioService) RegisterPassengerListDelivery(
	ctx context.Context,
	input RegistrarEnvioListaPasajerosInput,
) (EnvioListaPasajeros, error) {
	if input.CorridaID <= 0 {
		return EnvioListaPasajeros{}, fmt.Errorf(
			"%w: corrida_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	if input.ChoferID <= 0 {
		return EnvioListaPasajeros{}, fmt.Errorf(
			"%w: chofer_id debe ser válido",
			ErrDatosInvalidos,
		)
	}

	return s.store.RegisterPassengerListDelivery(
		ctx,
		input,
		politicaEnvioListaPasajerosActual(),
	)
}

// politicaEnvioListaPasajerosActual centraliza la política.
//
// Posteriormente este valor podrá obtenerse de configuración
// sin cambiar las firmas del Service o del Repository.
func politicaEnvioListaPasajerosActual() PoliticaEnvioListaPasajerosParams {
	return PoliticaEnvioListaPasajerosParams{
		MinutosAnticipacion: minutosAnticipacionListaPasajeros,
	}
}
