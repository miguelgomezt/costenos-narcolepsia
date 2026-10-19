// Programa de demostración del Hospital de los Costeños con Narcolepsia.
//
// main SOLO construye objetos e imprime resultados. Toda la lógica de negocio
// (asignar salas, despachar personal, contar episodios) vive en el paquete
// hospital; aquí solo se llaman sus métodos públicos.
//
// Modos de ejecución (paquete flag):
//
//	go run .            escenario obligatorio de la sección 6 del enunciado
//	go run . -sim       simulación concurrente (bonus); -dur cambia la duración
//	go run -race . -sim la misma simulación con el detector de carreras
//	go run . -web       interfaz web en http://localhost:8080 (bonus)
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"costenos-narcolepsia/hospital"
	"costenos-narcolepsia/api"
)

// demo agrupa lo que devuelve buildHospital. main necesita los médicos y
// los pacientes por separado (para imprimir 5.2 y para elegir quién se
// duerme), además del hospital.
type demo struct {
	h        *hospital.Hospital
	doctors  []*hospital.Doctor
	patients []*hospital.Patient
}

// buildHospital arma el MISMO hospital para los tres modos (escenario, -sim
// y -web): 2 médicos, 1 camillero, 3 salas de capacidad 1 y 5 pacientes, dos
// de ellos Severe. Así no se repite código y los tres modos parten igual.
//
// Cada constructor puede fallar y devuelve error; aquí se propaga el error
// hacia main en vez de usar panic o de ignorarlo con _.
func buildHospital() (*demo, error) {
	h := hospital.NewHospital("Hospital de los Costeños con Narcolepsia")
	d := &demo{h: h}

	// --- Médicos ---
	doctorData := []struct {
		id, name, specialty string
		age                 int
	}{
		{"D-001", "Karen Ospina", "Neurología", 41},
		{"D-002", "Rafael Mendoza", "Medicina del sueño", 52},
	}
	for _, x := range doctorData {
		doc, err := hospital.NewDoctor(x.id, x.name, x.age, x.specialty)
		if err != nil {
			return nil, fmt.Errorf("crear médico %s: %w", x.id, err)
		}
		if err := h.HireDoctor(doc); err != nil {
			return nil, err
		}
		d.doctors = append(d.doctors, doc)
	}

	// --- Camillero (segundo tipo concreto que cumple Attender) ---
	orderly, err := hospital.NewOrderly("O-001", "Chucho Barraza", 35)
	if err != nil {
		return nil, fmt.Errorf("crear camillero: %w", err)
	}
	if err := h.HireOrderly(orderly); err != nil {
		return nil, err
	}

	// --- Salas: 3 de capacidad 1 ---
	for _, number := range []int{101, 102, 103} {
		room, err := hospital.NewRoom(number, 1)
		if err != nil {
			return nil, fmt.Errorf("crear sala %d: %w", number, err)
		}
		if err := h.AddRoom(room); err != nil {
			return nil, err
		}
	}

	// --- Pacientes: 5, dos con nivel Severe (P-002 y P-004) ---
	patientData := []struct {
		id, name string
		age      int
		level    hospital.NarcolepsyLevel
	}{
		{"P-001", "Mario Arrieta", 34, hospital.Moderate},
		{"P-002", "Yeimy Padilla", 27, hospital.Severe},
		{"P-003", "Luis Mercado", 45, hospital.Mild},
		{"P-004", "Wilfrido Berrío", 61, hospital.Severe},
		{"P-005", "Ana Cassiani", 30, hospital.Mild},
	}
	for _, x := range patientData {
		p, err := hospital.NewPatient(x.id, x.name, x.age, x.level)
		if err != nil {
			return nil, fmt.Errorf("crear paciente %s: %w", x.id, err)
		}
		if err := h.AdmitPatient(p); err != nil {
			return nil, err
		}
		d.patients = append(d.patients, p)
	}

	return d, nil
}

func main() {
	// Banderas. flag.Bool devuelve un *bool; se lee con *sim después de
	// flag.Parse().
	sim := flag.Bool("sim", false, "ejecuta la simulación concurrente (bonus)")
	web := flag.Bool("web", false, "inicia la interfaz web en http://localhost:8080 (bonus)")
	dur := flag.Duration("dur", 5*time.Second, "duración de la simulación con -sim (por ejemplo 3s)")
	flag.Parse()

	if *sim && *web {
		fmt.Fprintln(os.Stderr, "error: usa -sim o -web, no las dos a la vez")
		os.Exit(2)
	}

	d, err := buildHospital()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error al armar el hospital:", err)
		os.Exit(1)
	}

	switch {
	case *web:
		runWeb(d)
	case *sim:
		runSim(d, *dur)
	default:
		runScenario(d)
	}
}

// ---------------------------------------------------------------------------
// Modo por defecto: escenario obligatorio (sección 6 del enunciado)
// ---------------------------------------------------------------------------

func runScenario(d *demo) {
	h := d.h
	p := d.patients // alias corto: p[0] es P-001, p[3] es P-004, etc.

	fmt.Println("==", h.Name(), "==")
	fmt.Printf("Personal: %d médicos y 1 camillero | Salas: %d | Pacientes: %d\n",
		len(h.Doctors()), len(h.Rooms()), len(h.Patients()))

	// Paso 2: cuatro pacientes se duermen en lugares distintos. Hay 3 salas,
	// así que los tres primeros consiguen sala y el cuarto (P-004) no.
	fmt.Println()
	fmt.Println("== Paso 2: ataques de sueño ==")
	attacks := []struct {
		patient  *hospital.Patient
		location string
	}{
		{p[0], "cafetería"},
		{p[1], "fila de radiología"},
		{p[2], "parqueadero"},
		{p[3], "pasillo 2, segundo piso"},
	}
	for _, a := range attacks {
		err := h.HandleSleepAttack(a.patient, a.location)
		printAttack(h, a.patient, a.location, err)
	}
	// Se muestra la consulta 5.1 aquí también, que es cuando de verdad hay
	// alguien en el pasillo (al final del escenario ya no habrá nadie).
	fmt.Println("  Pacientes en el pasillo ahora (consulta 5.1):")
	printHallway(h)

	// Paso 3: P-001 despierta y libera su sala; luego se le asigna sala al
	// paciente que quedó en el pasillo.
	fmt.Println()
	fmt.Println("== Paso 3: un paciente despierta y se reasigna la sala ==")
	freed := p[0].Room() // se guarda ANTES de despertar: WakeUp pone la sala en nil
	if err := h.WakePatient(p[0]); err != nil {
		fmt.Printf("  %s no pudo despertar: %v\n", p[0].ID(), err)
	} else if freed != nil {
		fmt.Printf("  %s %s despierta y libera la sala %d\n", p[0].ID(), p[0].Name(), freed.Number())
	}

	// Paso 4: las cuatro consultas.
	fmt.Println()
	fmt.Println("== Paso 4: consultas ==")

	// 5.3 Disponibilidad de salas: se le asigna sala al que estaba en el
	// pasillo. Esta es la misma llamada que falló en el paso 2 (dentro de
	// HandleSleepAttack); ahora hay una sala libre.
	fmt.Println()
	fmt.Printf("-- 5.3 Asignando sala a %s %s --\n", p[3].ID(), p[3].Name())
	if room, err := h.AssignRoom(p[3]); err != nil {
		fmt.Println("  " + readableError(err))
	} else {
		fmt.Printf("  asignado a la sala %d (estado del paciente: %v)\n", room.Number(), p[3].State())
	}
	fmt.Println("  Salas:")
	for _, r := range h.Rooms() {
		status := "ocupada"
		if r.IsAvailable() {
			status = "libre"
		}
		fmt.Printf("    sala %d: %s\n", r.Number(), status)
	}

	// 5.1 Pacientes dormidos en el pasillo al final (ya nadie, porque P-004
	// consiguió sala en 5.3).
	fmt.Println()
	fmt.Println("-- 5.1 Pacientes dormidos en el pasillo (al final) --")
	printHallway(h)

	// 5.2 Historial por médico: lo responde el propio Doctor, sin recibir el
	// hospital.
	for _, doc := range d.doctors {
		fmt.Println()
		fmt.Printf("-- 5.2 Episodios atendidos por Dr(a). %s --\n", doc.Name())
		episodes := doc.MyEpisodes()
		if len(episodes) == 0 {
			fmt.Println("  (ninguno)")
		}
		for _, e := range episodes {
			fmt.Println("  " + e.Summary())
		}
	}

	// 5.4 Reporte de severidad. Un map no tiene orden fijo en Go (cada
	// ejecución puede recorrerlo distinto), así que se sacan las claves a un
	// slice y se ordenan por ID para que la salida sea siempre la misma.
	fmt.Println()
	fmt.Println("-- 5.4 Reporte de severidad (pacientes Severe) --")
	report := h.SevereReport()
	severe := make([]*hospital.Patient, 0, len(report))
	for patient := range report {
		severe = append(severe, patient)
	}
	sort.Slice(severe, func(i, j int) bool { return severe[i].ID() < severe[j].ID() })
	for _, patient := range severe {
		fmt.Printf("  %s %-18s %d episodio(s) hoy\n", patient.ID(), patient.Name(), report[patient])
	}
}

// printAttack imprime el resultado de un ataque de sueño. Quién atendió se
// lee del último episodio del historial: HandleSleepAttack acaba de
// registrarlo.
func printAttack(h *hospital.Hospital, p *hospital.Patient, location string, err error) {
	attendedBy := ""
	if history := h.History(); len(history) > 0 {
		last := history[len(history)-1]
		if last.Patient() == p && last.Attender() != nil {
			attendedBy = " | atendió: " + last.Attender().Name()
		}
	}
	if err != nil {
		fmt.Printf("  %s %-18s se durmió en %-25s -> %s%s\n",
			p.ID(), p.Name(), location, readableError(err), attendedBy)
		return
	}
	room := "sin sala"
	if r := p.Room(); r != nil {
		room = fmt.Sprintf("sala %d", r.Number())
	}
	fmt.Printf("  %s %-18s se durmió en %-25s -> %s%s\n", p.ID(), p.Name(), location, room, attendedBy)
}

// readableError traduce los errores conocidos a un mensaje legible para el
// usuario. Se usa errors.Is (y no comparar el texto) porque el error puede
// venir envuelto con fmt.Errorf("...: %w", err).
func readableError(err error) string {
	if errors.Is(err, hospital.ErrNoRoomAvailable) {
		return "ERROR: no hay sala disponible, el paciente se queda en el pasillo"
	}
	return "ERROR: " + err.Error()
}

func printHallway(h *hospital.Hospital) {
	hallway := h.PatientsInHallway()
	if len(hallway) == 0 {
		fmt.Println("  (ninguno)")
		return
	}
	for _, p := range hallway {
		fmt.Printf("  %s %-18s (%s)\n", p.ID(), p.Name(), p.Location())
	}
}

// ---------------------------------------------------------------------------
// -sim: simulación concurrente (bonus)
// ---------------------------------------------------------------------------

func runSim(d *demo, duration time.Duration) {
	fmt.Printf("== Simulación concurrente: %d pacientes, %d salas, %v ==\n",
		len(d.patients), len(d.h.Rooms()), duration)

	// RunSimulation bloquea hasta que terminan todas las goroutines y
	// devuelve los números; hospital/ no imprime nada.
	s := hospital.RunSimulation(d.h, d.patients, duration)

	fmt.Printf("  Ataques de sueño:        %d\n", s.Attempts)
	fmt.Printf("  Con sala asignada:       %d\n", s.Successes)
	fmt.Printf("  Sin sala (al pasillo):   %d\n", s.NoRoomFailures)
	fmt.Printf("  Despertares:             %d\n", s.WakeUps)
	fmt.Printf("  Otros errores:           %d\n", s.OtherErrors)
	fmt.Printf("  Episodios en historial:  %d\n", len(d.h.History()))

	// Comprobación final: todos despertaron, así que ninguna sala debe
	// quedar ocupada ni nadie en el pasillo.
	free := 0
	for _, r := range d.h.Rooms() {
		if r.IsAvailable() {
			free++
		}
	}
	fmt.Printf("  Al terminar: %d de %d salas libres, %d pacientes en el pasillo\n",
		free, len(d.h.Rooms()), len(d.h.PatientsInHallway()))
}

// ---------------------------------------------------------------------------
// -web: interfaz web (bonus)
// ---------------------------------------------------------------------------

// runWeb arma el hospital SIN ejecutar los ataques del escenario, para que
// se disparen desde la página. Hay que correrlo desde la raíz del proyecto
// para que el servidor encuentre la carpeta web/.
func runWeb(d *demo) {
	fmt.Println("Servidor en http://localhost:8080")
	// api.Start bloquea mientras el servidor esté vivo; solo vuelve si hay
	// un error (por ejemplo, el puerto 8080 ya está en uso).
	if err := api.Start(d.h, ":8080"); err != nil {
		fmt.Fprintln(os.Stderr, "error del servidor:", err)
		os.Exit(1)
	}
}
