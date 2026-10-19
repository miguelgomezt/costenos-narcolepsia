package hospital

import (
	"errors"
	"fmt"
	"sync"
)

// Hospital es el agregado central del modelo: conoce a los médicos, a los
// pacientes, las salas, el personal que atiende (staff) y el historial de
// episodios.
//
// CONCURRENCIA (bonus):
// Todos los campos de abajo están protegidos por mu. La simulación (una
// goroutine por paciente) y la API web (una goroutine por petición HTTP)
// llaman a los métodos del hospital al mismo tiempo; sin el mutex, dos
// goroutines podrían ver la misma sala libre y ocuparla las dos (data race).
//
// Se usa sync.RWMutex y no sync.Mutex porque la mayoría de las llamadas de la
// API son lecturas (Patients, Rooms, History...). Con RWMutex varias
// lecturas pueden ir en paralelo (RLock) y solo las escrituras (Lock) son
// exclusivas.
//
// ORDEN DE BLOQUEO (para evitar deadlock):
//
//	Hospital.mu  ->  Room.mu / Patient.mu / Doctor.mu
//
// Mientras se tiene h.mu se puede llamar a p.State(), room.Occupy(),
// a.Attend(), etc., porque esos tipos NUNCA llaman de vuelta al Hospital.
// Si alguno lo hiciera, tendríamos un ciclo de locks y podría bloquearse.
//
// REGLA "Locked":
// sync.Mutex NO es reentrante: si un método que ya tiene h.mu llama a otro
// método público que también hace h.mu.Lock(), se queda esperando para
// siempre (deadlock). Por eso cada operación tiene dos versiones:
//   - pública (AssignRoom): toma el lock y llama a la privada.
//   - privada (assignRoomLocked): asume que el que llama YA tiene el lock.
//
// Los métodos privados que terminan en "Locked" solo se llaman con h.mu
// tomado.
type Hospital struct {
	mu sync.RWMutex // protege TODOS los campos de abajo

	name     string
	doctors  []*Doctor
	patients []*Patient
	rooms    []*Room
	staff    []Attender // personal polimórfico: *Doctor, *Orderly, ...
	history  []EpisodeRecord

	// nextStaff es el índice del próximo Attender en la rotación. También
	// lo protege mu: si dos goroutines lo leyeran y sumaran a la vez,
	// ambas podrían despachar a la misma persona (otra data race).
	nextStaff int
}

// NewHospital crea un hospital vacío. No hace falta inicializar mu: el valor
// cero de sync.RWMutex ya es un mutex desbloqueado listo para usar.
func NewHospital(name string) *Hospital {
	return &Hospital{name: name}
}

// Name devuelve el nombre del hospital (la API lo usa en el JSON).
// name nunca cambia después de NewHospital, pero se lee bajo RLock por
// consistencia: así la regla es simple ("todo campo del hospital, con lock").
func (h *Hospital) Name() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.name
}

// ---------------------------------------------------------------------------
// Gestión: admitir pacientes, contratar personal y agregar salas
// ---------------------------------------------------------------------------

// AdmitPatient registra un paciente. Devuelve error si es nil o si ya hay
// otro paciente con el mismo ID.
func (h *Hospital) AdmitPatient(p *Patient) error {
	if p == nil {
		return fmt.Errorf("patient: %w", ErrNilArgument)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, existing := range h.patients {
		if existing.ID() == p.ID() {
			return fmt.Errorf("admit patient %s: %w", p.ID(), ErrDuplicateID)
		}
	}
	h.patients = append(h.patients, p)
	return nil
}

// HireDoctor contrata un médico: lo guarda en doctors y TAMBIÉN en staff,
// porque *Doctor cumple la interfaz Attender.
func (h *Hospital) HireDoctor(d *Doctor) error {
	if d == nil {
		return fmt.Errorf("doctor: %w", ErrNilArgument)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.staffHasIDLocked(d.ID()) {
		return fmt.Errorf("hire doctor %s: %w", d.ID(), ErrDuplicateID)
	}
	h.doctors = append(h.doctors, d)
	h.staff = append(h.staff, d)
	return nil
}

// HireOrderly contrata un camillero. Solo va al staff: no es médico.
func (h *Hospital) HireOrderly(o *Orderly) error {
	if o == nil {
		return fmt.Errorf("orderly: %w", ErrNilArgument)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.staffHasIDLocked(o.ID()) {
		return fmt.Errorf("hire orderly %s: %w", o.ID(), ErrDuplicateID)
	}
	h.staff = append(h.staff, o)
	return nil
}

// staffHasIDLocked dice si ya hay alguien en el staff con ese ID.
// Requiere h.mu tomado (lo llaman HireDoctor y HireOrderly).
func (h *Hospital) staffHasIDLocked(id string) bool {
	for _, a := range h.staff {
		if a.ID() == id {
			return true
		}
	}
	return false
}

// AddRoom registra una sala. Error si es nil o si el número ya existe.
func (h *Hospital) AddRoom(r *Room) error {
	if r == nil {
		return fmt.Errorf("room: %w", ErrNilArgument)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, existing := range h.rooms {
		if existing.Number() == r.Number() {
			return fmt.Errorf("add room %d: %w", r.Number(), ErrDuplicateID)
		}
	}
	h.rooms = append(h.rooms, r)
	return nil
}

// ---------------------------------------------------------------------------
// Consulta 5.3: asignar sala
// ---------------------------------------------------------------------------

// AssignRoom asigna la primera sala disponible al paciente.
// Versión pública: toma el lock y delega en assignRoomLocked.
func (h *Hospital) AssignRoom(p *Patient) (*Room, error) {
	if p == nil {
		return nil, fmt.Errorf("patient: %w", ErrNilArgument)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.assignRoomLocked(p)
}

// assignRoomLocked es el corazón del bonus de concurrencia.
// Requiere h.mu tomado.
//
// "Comprobar si la sala está libre" (IsAvailable) y "ocuparla" (Occupy) son
// dos pasos. Si otra goroutine pudiera meterse entre los dos, ambas verían la
// sala libre y la ocuparían a la vez. Como todo este método corre con h.mu
// tomado, ninguna otra goroutine puede asignar sala al mismo tiempo: los dos
// pasos se vuelven una sola operación atómica desde el punto de vista del
// resto del programa.
func (h *Hospital) assignRoomLocked(p *Patient) (*Room, error) {
	for _, room := range h.rooms {
		if !room.IsAvailable() {
			continue
		}
		if err := room.Occupy(p); err != nil {
			// No debería pasar (acabamos de ver que estaba libre y tenemos
			// el lock), pero si pasa probamos con la siguiente sala en vez
			// de fallar: nunca panic.
			continue
		}
		if err := p.placeInBed(room); err != nil {
			// El paciente no aceptó la cama (por ejemplo, estaba despierto).
			// Deshacemos la ocupación para no dejar una sala "ocupada" por
			// alguien que no está en ella.
			_ = room.Release(p)
			return nil, fmt.Errorf("place patient %s in room %d: %w", p.ID(), room.Number(), err)
		}
		return room, nil
	}
	// No hay sala: no inventamos ninguna. El paciente sigue en
	// AsleepInHallway porque no tocamos su estado.
	return nil, ErrNoRoomAvailable
}

// ---------------------------------------------------------------------------
// Registro de episodios y flujo completo de un ataque
// ---------------------------------------------------------------------------

// RegisterEpisode agrega un episodio al historial.
func (h *Hospital) RegisterEpisode(rec EpisodeRecord) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.registerEpisodeLocked(rec)
}

// registerEpisodeLocked requiere h.mu tomado.
func (h *Hospital) registerEpisodeLocked(rec EpisodeRecord) error {
	if rec.Patient() == nil {
		return errors.New("episode has no patient")
	}
	h.history = append(h.history, rec)
	return nil
}

// nextAttenderLocked elige a quién despachar rotando por el staff
// (round robin). Aquí está el polimorfismo: devuelve un Attender sin saber si
// es *Doctor u *Orderly. Requiere h.mu tomado porque modifica nextStaff.
func (h *Hospital) nextAttenderLocked() (Attender, error) {
	if len(h.staff) == 0 {
		return nil, errors.New("no staff available to attend the patient")
	}
	a := h.staff[h.nextStaff%len(h.staff)]
	h.nextStaff++
	return a, nil
}

// HandleSleepAttack ejecuta el flujo completo de un ataque de sueño:
//
//  1. p.SufferSleepAttack(location): el paciente queda en AsleepInHallway.
//  2. assignRoomLocked: busca sala. Si falla, se GUARDA el error y el
//     flujo CONTINÚA (el paciente igual debe ser atendido y registrado).
//  3. Se elige un Attender rotando y se llama a.Attend(p, location).
//  4. registerEpisodeLocked agrega el episodio al historial.
//  5. Se devuelve el error de sala (si lo hubo) para que main lo imprima.
//
// Todo el flujo corre con UN solo h.mu.Lock(). Por eso adentro se usan las
// versiones "Locked": si llamara a h.AssignRoom (pública), intentaría tomar
// h.mu otra vez y se quedaría bloqueado para siempre.
func (h *Hospital) HandleSleepAttack(p *Patient, location string) error {
	if p == nil {
		return fmt.Errorf("patient: %w", ErrNilArgument)
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	// Un paciente que ya está dormido (en cama o en el pasillo) no puede
	// tener otro ataque. Sin esta guarda, si alguien pulsa dos veces
	// "Ataque de sueño" en la web, el paciente ocuparía DOS salas.
	if p.State() != Awake {
		return fmt.Errorf("patient %s is already asleep", p.ID())
	}

	p.SufferSleepAttack(location)

	_, roomErr := h.assignRoomLocked(p) // si falla, se guarda y se sigue

	a, err := h.nextAttenderLocked()
	if err != nil {
		return err
	}
	rec, err := a.Attend(p, location) // despacho polimórfico
	if err != nil {
		return fmt.Errorf("attend patient %s: %w", p.ID(), err)
	}
	if err := h.registerEpisodeLocked(rec); err != nil {
		return err
	}
	return roomErr // nil si consiguió sala; ErrNoRoomAvailable si no
}

// WakePatient despierta al paciente y libera su sala (paso 3 del escenario).
func (h *Hospital) WakePatient(p *Patient) error {
	if p == nil {
		return fmt.Errorf("patient: %w", ErrNilArgument)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.wakePatientLocked(p)
}

// wakePatientLocked requiere h.mu tomado.
//
// El orden importa: primero se guarda la sala y se libera, y DESPUÉS se llama
// a WakeUp, porque WakeUp pone la sala del paciente en nil y ya no sabríamos
// cuál liberar. Como todo ocurre bajo h.mu, nadie puede asignar esa sala a
// otro paciente "a mitad" de este proceso.
func (h *Hospital) wakePatientLocked(p *Patient) error {
	if r := p.Room(); r != nil {
		if err := r.Release(p); err != nil {
			return fmt.Errorf("release room %d: %w", r.Number(), err)
		}
	}
	// Si ya estaba despierto, WakeUp devuelve su propio error
	// ("patient ... is already awake") y lo pasamos tal cual.
	return p.WakeUp()
}

// ---------------------------------------------------------------------------
// Lecturas: devuelven COPIAS bajo RLock
// ---------------------------------------------------------------------------
//
// ¿Por qué copias? Si devolviéramos h.patients directamente, quien lo reciba
// (main, la API) lo leería SIN el lock mientras otra goroutine hace append:
// eso es una data race y además rompe el encapsulamiento (podrían modificar
// el slice interno). La copia se toma con el lock y luego es solo de quien
// la pidió.
//
// Ojo: se copian los slices, no los objetos. Los *Patient siguen siendo
// compartidos, pero eso es seguro porque Patient tiene su propio mutex.

// Doctors devuelve una copia de la lista de médicos.
func (h *Hospital) Doctors() []*Doctor {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]*Doctor(nil), h.doctors...)
}

// Patients devuelve una copia de la lista de pacientes.
func (h *Hospital) Patients() []*Patient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]*Patient(nil), h.patients...)
}

// Rooms devuelve una copia de la lista de salas.
func (h *Hospital) Rooms() []*Room {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]*Room(nil), h.rooms...)
}

// History devuelve una copia del historial de episodios.
func (h *Hospital) History() []EpisodeRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]EpisodeRecord(nil), h.history...)
}

// ---------------------------------------------------------------------------
// Consultas 5.1 y 5.4
// ---------------------------------------------------------------------------

// PatientsInHallway (5.1) devuelve los pacientes dormidos en el pasillo.
// Se hace bajo RLock para recorrer h.patients sin que nadie lo modifique, y
// porque así ningún paciente cambia de sala "a mitad" del recorrido (los
// cambios de sala solo ocurren con h.mu en modo escritura).
func (h *Hospital) PatientsInHallway() []*Patient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []*Patient
	for _, p := range h.patients {
		if p.State() == AsleepInHallway {
			result = append(result, p)
		}
	}
	return result
}

// SevereReport (5.4) cruza los pacientes de nivel Severe con cuántos
// episodios tuvieron en el historial. Se usa map[*Patient]int porque la
// clave es el propio paciente y el valor su conteo. El orden de un map no es
// determinista, por eso main ordena por ID antes de imprimir.
func (h *Hospital) SevereReport() map[*Patient]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	report := make(map[*Patient]int)
	for _, p := range h.patients {
		if p.Level() == Severe {
			report[p] = 0 // aparece aunque tenga 0 episodios
		}
	}
	for _, rec := range h.history {
		if _, isSevere := report[rec.Patient()]; isSevere {
			report[rec.Patient()]++
		}
	}
	return report
}
