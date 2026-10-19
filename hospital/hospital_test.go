package hospital

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

/* ---------- ayudas ---------- */

// newTestHospital crea un hospital con `rooms` salas de la capacidad dada.
// t.Helper() hace que, si algo falla aquí, Go señale la línea del test que
// llamó a la ayuda y no la de la ayuda.
func newTestHospital(t *testing.T, rooms, capacity int) *Hospital {
	t.Helper()
	h := NewHospital("Hospital de pruebas")
	for i := 1; i <= rooms; i++ {
		r, err := NewRoom(i, capacity)
		if err != nil {
			t.Fatalf("NewRoom(%d, %d): %v", i, capacity, err)
		}
		if err := h.AddRoom(r); err != nil {
			t.Fatalf("AddRoom(%d): %v", i, err)
		}
	}
	return h
}

// newTestPatient crea un paciente despierto y lo admite en el hospital.
func newTestPatient(t *testing.T, h *Hospital, n int, level NarcolepsyLevel) *Patient {
	t.Helper()
	p, err := NewPatient(fmt.Sprintf("P-%03d", n), fmt.Sprintf("Paciente %d", n), 30+n, level)
	if err != nil {
		t.Fatalf("NewPatient(%d): %v", n, err)
	}
	if err := h.AdmitPatient(p); err != nil {
		t.Fatalf("AdmitPatient(%d): %v", n, err)
	}
	return p
}

/* ---------- tests base ---------- */

// 1) Asignación con sala libre: sala -> Occupied, paciente -> AsleepInBed.
func TestAssignRoomWithFreeRoom(t *testing.T) {
	h := newTestHospital(t, 1, 1)
	p := newTestPatient(t, h, 1, Severe)
	p.SufferSleepAttack("cafetería")

	room, err := h.AssignRoom(p)
	if err != nil {
		t.Fatalf("AssignRoom devolvió error inesperado: %v", err)
	}
	if room == nil || room.Number() != 1 {
		t.Fatalf("se esperaba la sala 1, llegó %v", room)
	}
	if p.State() != AsleepInBed {
		t.Errorf("estado del paciente = %v, se esperaba AsleepInBed", p.State())
	}
	if p.Room() != room {
		t.Errorf("p.Room() no es la sala asignada")
	}
	if room.State() != Occupied || room.IsAvailable() {
		t.Errorf("la sala debería quedar Occupied y sin cupo (estado %v)", room.State())
	}
}

// 2) Sin sala libre: error ErrNoRoomAvailable y el paciente sigue en el pasillo.
func TestAssignRoomNoRoomAvailable(t *testing.T) {
	h := newTestHospital(t, 1, 1)
	first := newTestPatient(t, h, 1, Mild)
	second := newTestPatient(t, h, 2, Severe)
	first.SufferSleepAttack("recepción")
	second.SufferSleepAttack("pasillo 2")

	if _, err := h.AssignRoom(first); err != nil {
		t.Fatalf("la primera asignación debía funcionar: %v", err)
	}
	room, err := h.AssignRoom(second)
	if !errors.Is(err, ErrNoRoomAvailable) {
		t.Fatalf("err = %v, se esperaba ErrNoRoomAvailable", err)
	}
	if room != nil {
		t.Errorf("sin sala libre no se debe inventar una, llegó %v", room)
	}
	if second.State() != AsleepInHallway {
		t.Errorf("estado = %v, el paciente debía seguir en AsleepInHallway", second.State())
	}
	if second.Room() != nil {
		t.Errorf("el paciente no debería tener sala")
	}
}

// 3) Una consulta: PatientsInHallway devuelve solo a los dormidos en el pasillo.
func TestPatientsInHallway(t *testing.T) {
	h := newTestHospital(t, 1, 1)
	inBed := newTestPatient(t, h, 1, Mild)
	inHall := newTestPatient(t, h, 2, Moderate)
	awake := newTestPatient(t, h, 3, Severe)
	inBed.SufferSleepAttack("cafetería")
	inHall.SufferSleepAttack("pasillo 1")
	if _, err := h.AssignRoom(inBed); err != nil {
		t.Fatalf("AssignRoom: %v", err)
	}

	got := h.PatientsInHallway()
	if len(got) != 1 || got[0] != inHall {
		t.Fatalf("PatientsInHallway = %v, se esperaba solo %s", got, inHall.ID())
	}
	if awake.State() != Awake {
		t.Errorf("el paciente despierto no debía cambiar")
	}
}

/* ---------- tests de Room ---------- */

func TestNewRoomRejectsInvalidCapacity(t *testing.T) {
	for _, c := range []int{0, -1} {
		if r, err := NewRoom(1, c); err == nil || r != nil {
			t.Errorf("NewRoom(1, %d) debía fallar, devolvió (%v, %v)", c, r, err)
		}
	}
}

func TestRoomOccupyAndRelease(t *testing.T) {
	r, err := NewRoom(7, 2)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := NewPatient("P-A", "Ana", 20, Mild)
	b, _ := NewPatient("P-B", "Beto", 21, Mild)
	c, _ := NewPatient("P-C", "Caro", 22, Mild)

	if r.State() != Available || !r.IsAvailable() {
		t.Fatalf("una sala nueva debe estar Available")
	}
	if err := r.Occupy(a); err != nil {
		t.Fatalf("Occupy(a): %v", err)
	}
	if err := r.Occupy(a); !errors.Is(err, ErrAlreadyInRoom) {
		t.Errorf("repetir al mismo paciente: err = %v, se esperaba ErrAlreadyInRoom", err)
	}
	// Con un ocupante de dos: estado Occupied pero todavía hay cupo.
	if r.State() != Occupied || !r.IsAvailable() {
		t.Errorf("con 1 de 2 camas: State=%v IsAvailable=%v", r.State(), r.IsAvailable())
	}
	if err := r.Occupy(b); err != nil {
		t.Fatalf("Occupy(b): %v", err)
	}
	if err := r.Occupy(c); !errors.Is(err, ErrRoomFull) {
		t.Errorf("sala llena: err = %v, se esperaba ErrRoomFull", err)
	}
	if err := r.Occupy(nil); !errors.Is(err, ErrNilArgument) {
		t.Errorf("Occupy(nil): err = %v, se esperaba ErrNilArgument", err)
	}

	if err := r.Release(c); !errors.Is(err, ErrNotInRoom) {
		t.Errorf("Release de alguien ausente: err = %v, se esperaba ErrNotInRoom", err)
	}
	if err := r.Release(a); err != nil {
		t.Fatalf("Release(a): %v", err)
	}
	if err := r.Release(b); err != nil {
		t.Fatalf("Release(b): %v", err)
	}
	if r.State() != Available || !r.IsAvailable() {
		t.Errorf("al quedar vacía debe volver a Available (estado %v)", r.State())
	}
}

// Occupants devuelve una copia: modificarla no debe afectar a la sala.
func TestRoomOccupantsReturnsCopy(t *testing.T) {
	r, _ := NewRoom(1, 2)
	a, _ := NewPatient("P-A", "Ana", 20, Mild)
	if err := r.Occupy(a); err != nil {
		t.Fatal(err)
	}
	got := r.Occupants()
	got[0] = nil
	if again := r.Occupants(); again[0] != a {
		t.Errorf("modificar la copia alteró la lista interna de la sala")
	}
}

/* ---------- tests de concurrencia (correr con -race) ---------- */

/*
TestRoomConcurrentOccupy prueba SOLO el mutex de Room (no depende del
Hospital): 50 goroutines intentan entrar a una sala de capacidad 3 a la vez
mientras otras leen IsAvailable y Occupants. Deben entrar exactamente 3 y el
detector de carreras no debe avisar nada.
*/
func TestRoomConcurrentOccupy(t *testing.T) {
	const (
		patients = 50
		capacity = 3
	)
	room, err := NewRoom(1, capacity)
	if err != nil {
		t.Fatal(err)
	}

	people := make([]*Patient, patients)
	for i := range people {
		p, err := NewPatient(fmt.Sprintf("P-%03d", i), fmt.Sprintf("Paciente %d", i), 30, Mild)
		if err != nil {
			t.Fatal(err)
		}
		people[i] = p
	}

	errs := make([]error, patients) // cada goroutine escribe SOLO en su posición: sin carrera
	start := make(chan struct{})    // "pistoletazo": todas arrancan juntas
	var wg sync.WaitGroup
	for i, p := range people {
		wg.Add(2)
		// Go 1.21: la variable del for se comparte entre iteraciones, así que
		// se pasa como parámetro para que cada goroutine tenga la suya.
		go func(i int, p *Patient) {
			defer wg.Done()
			<-start
			errs[i] = room.Occupy(p)
		}(i, p)
		go func() { // lector que compite con los escritores
			defer wg.Done()
			<-start
			_ = room.IsAvailable()
			_ = room.Occupants()
		}()
	}
	close(start)
	wg.Wait()

	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, ErrRoomFull):
			t.Errorf("error inesperado: %v", err)
		}
	}
	if ok != capacity {
		t.Errorf("entraron %d pacientes, se esperaban exactamente %d", ok, capacity)
	}
	if n := len(room.Occupants()); n != capacity {
		t.Errorf("la sala tiene %d ocupantes, capacidad %d", n, capacity)
	}
}

/*
TestConcurrentAssign: 20 pacientes dormidos en el pasillo piden sala a la vez
en un hospital con 5 salas de capacidad 1. Sin el mutex de Hospital, dos
goroutines verían la misma sala libre y la ocuparían las dos.

Se comprueba:
  - exactamente M asignaciones exitosas;
  - todas las demás fallan con ErrNoRoomAvailable;
  - ninguna sala supera su capacidad y no se repite sala entre pacientes;
  - los que se quedaron sin sala siguen en AsleepInHallway.

Correr con: go test -race -count=5 ./...
*/
func TestConcurrentAssign(t *testing.T) {
	const (
		numPatients = 20
		numRooms    = 5
	)
	h := newTestHospital(t, numRooms, 1)

	patients := make([]*Patient, numPatients)
	for i := range patients {
		p := newTestPatient(t, h, i+1, Moderate)
		p.SufferSleepAttack("pasillo") // ya dormido en el pasillo, listo para pedir sala
		patients[i] = p
	}

	// Cada goroutine guarda su resultado en SU posición del slice: así no hace
	// falta otro mutex y no hay carrera. Los t.Errorf se hacen después del
	// Wait, en la goroutine del test (t.Fatal no se puede llamar desde otra).
	rooms := make([]*Room, numPatients)
	errs := make([]error, numPatients)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, p := range patients {
		wg.Add(1)
		go func(i int, p *Patient) { // i y p como parámetros (Go 1.21)
			defer wg.Done()
			<-start // todas esperan aquí para chocar en el mismo instante
			rooms[i], errs[i] = h.AssignRoom(p)
		}(i, p)
	}
	close(start)
	wg.Wait()

	assigned := 0
	used := make(map[*Room]*Patient) // sala -> paciente al que se le dio
	for i, p := range patients {
		if errs[i] == nil {
			assigned++
			room := rooms[i]
			if room == nil {
				t.Errorf("%s: sin error pero sin sala", p.ID())
				continue
			}
			if other, dup := used[room]; dup {
				t.Errorf("la sala %d se asignó a %s y a %s", room.Number(), other.ID(), p.ID())
			}
			used[room] = p
			if p.State() != AsleepInBed || p.Room() != room {
				t.Errorf("%s: estado %v, sala %v; se esperaba AsleepInBed en la sala asignada", p.ID(), p.State(), p.Room())
			}
			continue
		}
		if !errors.Is(errs[i], ErrNoRoomAvailable) {
			t.Errorf("%s: error %v, se esperaba ErrNoRoomAvailable", p.ID(), errs[i])
		}
		if rooms[i] != nil {
			t.Errorf("%s: falló pero llegó la sala %d", p.ID(), rooms[i].Number())
		}
		if p.State() != AsleepInHallway || p.Room() != nil {
			t.Errorf("%s: estado %v; sin sala debía seguir en AsleepInHallway", p.ID(), p.State())
		}
	}
	if assigned != numRooms {
		t.Errorf("asignaciones exitosas = %d, se esperaban exactamente %d", assigned, numRooms)
	}
	for _, room := range h.Rooms() {
		if n := len(room.Occupants()); n > room.Capacity() {
			t.Errorf("sala %d con %d ocupantes y capacidad %d", room.Number(), n, room.Capacity())
		}
	}
}
