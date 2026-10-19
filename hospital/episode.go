package hospital

import (
	"fmt"
	"time"
)

/*
EpisodeRecord es el registro de UN ataque de sueño: cuándo ocurrió, quién se
durmió, quién lo atendió, dónde y en qué sala terminó (nil si no hubo sala).

Es un valor inmutable: todos sus campos se asignan en NewEpisodeRecord y no hay
ningún método que los cambie. Como nadie escribe después de crearlo, varias
goroutines pueden leerlo a la vez sin mutex (no hay data race si no hay
escrituras). Por eso Doctor y Orderly guardan copias del valor sin problema.

Los campos van en minúscula (no exportados): desde otro paquete, como api o
main, solo se llega a ellos con los getters de abajo.
*/
type EpisodeRecord struct {
	id       string
	when     time.Time
	patient  *Patient
	attender Attender
	location string
	room     *Room
}

/*
NewEpisodeRecord crea el registro. No devuelve error porque el contrato del
equipo así lo fija y porque no hay nada que validar que no se pueda leer
después con los getters; Summary tolera paciente o atendedor nil.
*/
func NewEpisodeRecord(id string, when time.Time, p *Patient, by Attender, location string, room *Room) EpisodeRecord {
	return EpisodeRecord{
		id:       id,
		when:     when,
		patient:  p,
		attender: by,
		location: location,
		room:     room,
	}
}

// ID devuelve el identificador del episodio, por ejemplo "EP-D-1-1".
func (e EpisodeRecord) ID() string { return e.id }

// When devuelve la fecha y hora del ataque (time.Time es un valor, se copia).
func (e EpisodeRecord) When() time.Time { return e.when }

// Patient devuelve el paciente que se durmió.
func (e EpisodeRecord) Patient() *Patient { return e.patient }

// Attender devuelve a quien atendió (Doctor u Orderly, sin saber cuál).
func (e EpisodeRecord) Attender() Attender { return e.attender }

// Location devuelve el lugar donde se durmió el paciente.
func (e EpisodeRecord) Location() string { return e.location }

// Room devuelve la sala asignada, o nil si el paciente se quedó en el pasillo.
func (e EpisodeRecord) Room() *Room { return e.room }

/*
Summary devuelve una línea legible del episodio. Solo usa datos que no cambian
(nombre, número de sala), así que no necesita bloquear a Patient ni a Room.
*/
func (e EpisodeRecord) Summary() string {
	patient := "paciente desconocido"
	if e.patient != nil {
		patient = e.patient.Name()
	}
	attender := "nadie"
	if e.attender != nil {
		attender = e.attender.Name()
	}
	room := "sin sala"
	if e.room != nil {
		room = fmt.Sprintf("sala %d", e.room.Number())
	}
	return fmt.Sprintf("[%s] %s: %s se durmió en %s, atendido por %s (%s)",
		e.when.Format("15:04:05"), e.id, patient, e.location, attender, room)
}
