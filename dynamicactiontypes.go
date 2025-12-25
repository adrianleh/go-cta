package cta

import "fmt"

const (
	DynamicActionTypeNone               = 0
	DynamicActionTypeCanceled           = 1
	DynamicActionTypeReassigned         = 2
	DynamicActionTypeShifted            = 3
	DynamicActionTypeExpressed          = 4
	DynamicActionTypeStopsAffected      = 6
	DynamicActionTypeNewTrip            = 8
	DynamicActionTypePartialTrip        = 9
	DynamicActionTypePartialTripNew     = 10
	DynamicActionTypeDelayedCancel      = 12
	DynamicActionTypeAddedStop          = 13
	DynamicActionTypeUnknownDelay       = 14
	DynamicActionTypeUnknownDelayNew    = 15
	DynamicActionTypeInvalidatedTrip    = 16
	DynamicActionTypeInvalidatedTripNew = 17
	DynamicActionTypeCancelledTripNew   = 18
	DynamicActionTypeStopsAffectedNew   = 19
)

func DynamicActionTypeName(id int) string {
	switch id {
	case DynamicActionTypeNone:
		return "None"
	case DynamicActionTypeCanceled:
		return "Canceled"
	case DynamicActionTypeReassigned:
		return "Reassigned"
	case DynamicActionTypeShifted:
		return "Shifted"
	case DynamicActionTypeExpressed:
		return "Expressed"
	case DynamicActionTypeStopsAffected:
		return "Stops Affected"
	case DynamicActionTypeNewTrip:
		return "New Trip"
	case DynamicActionTypePartialTrip:
		return "Partial Trip"
	case DynamicActionTypePartialTripNew:
		return "Partial Trip New"
	case DynamicActionTypeDelayedCancel:
		return "Delayed Cancel"
	case DynamicActionTypeAddedStop:
		return "Added Stop"
	case DynamicActionTypeUnknownDelay:
		return "Unknown Delay"
	case DynamicActionTypeUnknownDelayNew:
		return "Unknown Delay New"
	case DynamicActionTypeInvalidatedTrip:
		return "Invalidated Trip"
	case DynamicActionTypeInvalidatedTripNew:
		return "Invalidated Trip New"
	case DynamicActionTypeCancelledTripNew:
		return "Cancelled Trip New"
	case DynamicActionTypeStopsAffectedNew:
		return "Stops Affected New"
	default:
		return fmt.Sprintf("Unknown (%d)", id)
	}
}

func DynamicActionTypeDescription(id int) string {
	switch id {
	case DynamicActionTypeNone:
		return "No change."
	case DynamicActionTypeCanceled:
		return "The event or trip has been canceled."
	case DynamicActionTypeReassigned:
		return "The event or trip has been moved to a different work (to be handled by a different vehicle or operator)."
	case DynamicActionTypeShifted:
		return "The time of this event, or the entire trip, has been moved."
	case DynamicActionTypeExpressed:
		return "The event is \"drop-off only\" and will not stop to pick up passengers."
	case DynamicActionTypeStopsAffected:
		return "This trip has events that are affected by Disruption Management changes, but the trip itself is not affected."
	case DynamicActionTypeNewTrip:
		return "This trip was created dynamically and does not appear in the TA schedule."
	case DynamicActionTypePartialTrip:
		return "This trip has been split, and this part of the split is using the original trip identifier(s).\n-or-\nThe trip has been short-turned leading to the removal of shortturned stops from the trip resulting in the trip being partial."
	case DynamicActionTypePartialTripNew:
		return "This trip has been split, and this part of the split has been assigned a new trip identifier(s)."
	case DynamicActionTypeDelayedCancel:
		return "This event or trip has been marked as canceled, but the cancellation should not be shown to the public."
	case DynamicActionTypeAddedStop:
		return "This event has been added to the trip. It was not originally scheduled."
	case DynamicActionTypeUnknownDelay:
		return "This trip has been affected by a delay."
	case DynamicActionTypeUnknownDelayNew:
		return "This trip, which was created dynamically, has been affected by a delay."
	case DynamicActionTypeInvalidatedTrip:
		return "This trip has been invalidated. Predictions for it should not be shown to the public."
	case DynamicActionTypeInvalidatedTripNew:
		return "This trip, which was created dynamically, has been invalidated. Predictions for it should not be shown to the public."
	case DynamicActionTypeCancelledTripNew:
		return "This trip, which was created dynamically, has been canceled."
	case DynamicActionTypeStopsAffectedNew:
		return "This trip, which was created dynamically, has events that are affected by Disruption Management changes, but the trip itself is not affected."
	default:
		return fmt.Sprintf("Unknown dynamic action type %d", id)
	}
}

// Spanish translations
func DynamicActionTypeNameES(id int) string {
	switch id {
	case DynamicActionTypeNone:
		return "Ninguno"
	case DynamicActionTypeCanceled:
		return "Cancelado"
	case DynamicActionTypeReassigned:
		return "Reasignado"
	case DynamicActionTypeShifted:
		return "Desplazado"
	case DynamicActionTypeExpressed:
		return "Solo bajada"
	case DynamicActionTypeStopsAffected:
		return "Paradas afectadas"
	case DynamicActionTypeNewTrip:
		return "Viaje nuevo"
	case DynamicActionTypePartialTrip:
		return "Viaje parcial"
	case DynamicActionTypePartialTripNew:
		return "Viaje parcial (nuevo)"
	case DynamicActionTypeDelayedCancel:
		return "Cancelación diferida"
	case DynamicActionTypeAddedStop:
		return "Parada añadida"
	case DynamicActionTypeUnknownDelay:
		return "Retraso desconocido"
	case DynamicActionTypeUnknownDelayNew:
		return "Retraso desconocido (nuevo)"
	case DynamicActionTypeInvalidatedTrip:
		return "Viaje invalidado"
	case DynamicActionTypeInvalidatedTripNew:
		return "Viaje invalidado (nuevo)"
	case DynamicActionTypeCancelledTripNew:
		return "Viaje cancelado (nuevo)"
	case DynamicActionTypeStopsAffectedNew:
		return "Paradas afectadas (nuevo)"
	default:
		return fmt.Sprintf("Desconocido (%d)", id)
	}
}

func DynamicActionTypeDescriptionES(id int) string {
	switch id {
	case DynamicActionTypeNone:
		return "Sin cambios."
	case DynamicActionTypeCanceled:
		return "El evento o viaje ha sido cancelado."
	case DynamicActionTypeReassigned:
		return "El evento o viaje se ha asignado a otro trabajo (será manejado por otro vehículo u operador)."
	case DynamicActionTypeShifted:
		return "La hora de este evento, o del viaje completo, ha sido cambiada."
	case DynamicActionTypeExpressed:
		return "El evento es 'solo bajada' y no recogerá pasajeros."
	case DynamicActionTypeStopsAffected:
		return "Este viaje tiene eventos que se ven afectados por cambios de Gestión de Disrupciones, pero el viaje en sí no está afectado."
	case DynamicActionTypeNewTrip:
		return "Este viaje se creó dinámicamente y no aparece en el horario TA."
	case DynamicActionTypePartialTrip:
		return "Este viaje ha sido dividido; esta parte usa los identificadores de viaje originales.\n-o-\nEl viaje se ha acortado, resultando en la eliminación de paradas acortadas y haciendo el viaje parcial."
	case DynamicActionTypePartialTripNew:
		return "Este viaje ha sido dividido; esta parte tiene un nuevo identificador de viaje."
	case DynamicActionTypeDelayedCancel:
		return "Este evento o viaje ha sido marcado como cancelado, pero la cancelación no debe mostrarse al público."
	case DynamicActionTypeAddedStop:
		return "Este evento se ha añadido al viaje. No estaba originalmente programado."
	case DynamicActionTypeUnknownDelay:
		return "Este viaje se ha visto afectado por un retraso."
	case DynamicActionTypeUnknownDelayNew:
		return "Este viaje, creado dinámicamente, se ha visto afectado por un retraso."
	case DynamicActionTypeInvalidatedTrip:
		return "Este viaje ha sido invalidado. No se deben mostrar predicciones para él al público."
	case DynamicActionTypeInvalidatedTripNew:
		return "Este viaje, creado dinámicamente, ha sido invalidado. No se deben mostrar predicciones para él al público."
	case DynamicActionTypeCancelledTripNew:
		return "Este viaje, creado dinámicamente, ha sido cancelado."
	case DynamicActionTypeStopsAffectedNew:
		return "Este viaje, creado dinámicamente, tiene eventos afectados por cambios de Gestión de Disrupciones, pero el viaje en sí no está afectado."
	default:
		return fmt.Sprintf("Tipo de acción dinámica desconocido %d", id)
	}
}
