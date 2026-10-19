package hospital

import (
	"fmt"
	"sync"
)

/*
RoomState es el estado de una sala. Es un tipo propio con constantes (no
strings sueltos) para que el compilador atrape los errores de escritura.
*/
type RoomState int

const (
	// Available: la sala está vacía.
	Available RoomState = iota
	// Occupied: la sala tiene al menos un paciente.
	Occupied
)

// String permite imprimir el estado de forma legible (fmt lo usa solo).
func (s RoomState) String() string {
	switch s {
	case Available:
		return "Available"
	case Occupied:
		return "Occupied"
	default:
		return fmt.Sprintf("RoomState(%d)", int(s))
	}
}

/*
Room es una sala del hospital con una capacidad fija de pacientes.

Concurrencia (bonus): la sala la leen y modifican varias goroutines a la vez
(la simulación, cada petición HTTP de la API, los tests). Por eso lleva un
sync.RWMutex privado:
  - Lock (exclusivo) en los métodos que ESCRIBEN: Occupy y Release.
  - RLock (compartido) en los que solo LEEN: IsAvailable, State, Occupants.
    Varias lecturas pueden ir en paralelo; una escritura espera a que acaben.

number y capacity se asignan una sola vez en NewRoom y nunca cambian, así que
leerlos no necesita lock: un valor que nadie modifica no puede tener data race.

Orden de bloqueo del proyecto: Hospital.mu -> Room.mu / Patient.mu / Doctor.mu.
Por eso Room NUNCA llama a métodos del Hospital ni bloquea a un Patient
mientras tiene su propio lock: de otro modo dos goroutines podrían esperarse
en círculo (deadlock). Aquí solo se compara el puntero *Patient (identidad),
sin invocar ningún método suyo.
*/
type Room struct {
	number   int
	capacity int

	mu        sync.RWMutex
	state     RoomState
	occupants []*Patient
}

/*
NewRoom crea una sala vacía. Devuelve error si la capacidad es <= 0: una sala
sin cupo no tendría sentido y la rechazamos al crearla, no más tarde.
*/
func NewRoom(number, capacity int) (*Room, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("room %d: capacity must be positive, got %d", number, capacity)
	}
	return &Room{number: number, capacity: capacity, state: Available}, nil
}

// Number devuelve el número de la sala. No usa lock: nunca cambia.
func (r *Room) Number() int { return r.number }

// Capacity devuelve cuántos pacientes caben. No usa lock: nunca cambia.
func (r *Room) Capacity() int { return r.capacity }

// State devuelve Available si la sala está vacía y Occupied si tiene alguien.
func (r *Room) State() RoomState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

/*
IsAvailable dice si todavía hay cupo (ocupantes < capacidad).

Ojo con la diferencia con State: con capacidad 1 coinciden (libre = Available),
pero en una sala de capacidad 3 con un paciente el estado es Occupied y aun así
IsAvailable es true porque quedan dos camas. AssignRoom usa IsAvailable.
*/
func (r *Room) IsAvailable() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.occupants) < r.capacity
}

/*
Occupants devuelve una COPIA de la lista de ocupantes. Si devolviéramos el
slice interno, quien lo reciba podría modificarlo (o leerlo mientras otra
goroutine hace Occupy) saltándose el mutex.
*/
func (r *Room) Occupants() []*Patient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Patient, len(r.occupants))
	copy(out, r.occupants)
	return out
}

/*
Occupy mete al paciente en la sala. Comprobar el cupo y agregar es UNA sola
operación bajo el mismo Lock: sin eso, dos goroutines podrían ver "hay cupo" a
la vez y pasarse de la capacidad (la data race clásica de este trabajo).

Devuelve error si p es nil, si ya está dentro o si la sala está llena.
Ojo: Occupy no cambia el estado del paciente; eso lo hace el hospital con
placeInBed. Room solo conoce sus ocupantes.
*/
func (r *Room) Occupy(p *Patient) error {
	if p == nil {
		return fmt.Errorf("room %d: %w", r.number, ErrNilArgument)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.indexOfLocked(p) >= 0 {
		return fmt.Errorf("room %d, patient %s: %w", r.number, p.ID(), ErrAlreadyInRoom)
	}
	if len(r.occupants) >= r.capacity {
		return fmt.Errorf("room %d: %w", r.number, ErrRoomFull)
	}
	r.occupants = append(r.occupants, p)
	r.state = Occupied
	return nil
}

/*
Release saca al paciente de la sala. Si era el último, la sala vuelve a
Available. Devuelve error si p es nil o no estaba dentro.
*/
func (r *Room) Release(p *Patient) error {
	if p == nil {
		return fmt.Errorf("room %d: %w", r.number, ErrNilArgument)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	i := r.indexOfLocked(p)
	if i < 0 {
		return fmt.Errorf("room %d, patient %s: %w", r.number, p.ID(), ErrNotInRoom)
	}
	// Se conserva el orden de llegada de los demás ocupantes.
	r.occupants = append(r.occupants[:i], r.occupants[i+1:]...)
	if len(r.occupants) == 0 {
		r.state = Available
	}
	return nil
}

/*
indexOfLocked busca al paciente en la lista. El sufijo "Locked" es la
convención del proyecto: significa "quien me llama YA tiene el lock; yo no
bloqueo". sync.Mutex no es reentrante: si esta función volviera a pedir el
lock desde Occupy o Release, la goroutine se quedaría esperándose a sí misma
para siempre (deadlock). Es la única función de Room que usan otras de Room.
*/
func (r *Room) indexOfLocked(p *Patient) int {
	for i, o := range r.occupants {
		if o == p {
			return i
		}
	}
	return -1
}