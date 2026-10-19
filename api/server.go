// Package api expone el hospital por HTTP con JSON, para la interfaz web.
//
// Decisión de diseño clave: este paquete IMPORTA a hospital, pero hospital no
// sabe que la API existe. Así se puede cambiar la vista (web, consola, otra
// API) sin tocar el modelo. La API tampoco guarda estado propio: cada petición
// lee del hospital en el momento.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"costenos-narcolepsia/hospital"
)

// webDir es la carpeta con index.html, app.js y style.css. Es relativa: por
// eso hay que ejecutar "go run . -web" desde la raíz del proyecto.
const webDir = "web"

/*
Start levanta el servidor HTTP en addr (por ejemplo ":8080") y se queda
bloqueado hasta que falle. Devuelve error en vez de hacer panic o log.Fatal,
como en el resto del proyecto: main decide qué hacer con él.

Concurrencia: net/http atiende cada petición en SU PROPIA goroutine, así que
dos clientes pueden pedir sala a la vez igual que en la simulación. La API es
segura porque solo llama a métodos públicos del hospital, que ya llevan mutex.
*/
func Start(h *hospital.Hospital, addr string) error {
	if h == nil {
		return errors.New("api: hospital cannot be nil")
	}
	// Mejor fallar ahora con un mensaje claro que servir 404 en el navegador.
	if info, err := os.Stat(webDir); err != nil || !info.IsDir() {
		return fmt.Errorf("api: no se encontró la carpeta %q: ejecuta el programa desde la raíz del proyecto", webDir)
	}
	srv := &http.Server{
		Addr:    addr,
		Handler: newMux(h),
		// Sin este límite un cliente lento podría dejar conexiones abiertas
		// para siempre (go vet y los linters lo avisan).
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("api: escuchando en %s", addr)
	return srv.ListenAndServe()
}

// server solo guarda el puntero al hospital: nada de estado propio.
type server struct {
	h *hospital.Hospital
}

/*
newMux registra las rutas. Es Go 1.21: el patrón "POST /api/sleep" (con método)
llegó en Go 1.22, así que registramos solo la ruta y cada handler comprueba
r.Method por dentro (responde 405 si no corresponde).
*/
func newMux(h *hospital.Hospital) *http.ServeMux {
	s := &server{h: h}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/queries", s.handleQueries)
	mux.HandleFunc("/api/sleep", s.handleSleep)
	mux.HandleFunc("/api/wake", s.handleWake)
	// Cualquier otra ruta /api/... responde JSON (si no, caería en el
	// FileServer y devolvería un 404 de texto plano).
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "ruta de la API desconocida: "+r.URL.Path)
	})
	// Misma origen para página y API: el navegador no necesita CORS.
	mux.Handle("/", http.FileServer(http.Dir(webDir)))
	return mux
}

/* ---------- structs de respuesta ----------

encoding/json solo serializa campos EXPORTADOS, y los del modelo (Patient,
Room...) son privados a propósito. Por eso armamos structs propios, con las
etiquetas json que la página espera, y los llenamos con los getters.
*/

type roomJSON struct {
	ID          string `json:"id"`
	Occupied    bool   `json:"occupied"`
	PatientID   string `json:"patientId,omitempty"`   // omitempty: se omite si la sala está libre
	PatientName string `json:"patientName,omitempty"` // idem
}

type patientJSON struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Age        int    `json:"age"`
	Level      string `json:"level"`
	State      string `json:"state"`
	Location   string `json:"location"`
	RoomID     string `json:"roomId"`
	DoctorID   string `json:"doctorId"`
	DoctorName string `json:"doctorName"`
}

type doctorJSON struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Specialty    string   `json:"specialty"`
	PatientIDs   []string `json:"patientIds"`
	EpisodeCount int      `json:"episodeCount"`
}

type episodeJSON struct {
	ID           string `json:"id"`
	Time         string `json:"time"`
	PatientID    string `json:"patientId"`
	PatientName  string `json:"patientName"`
	AttenderID   string `json:"attenderId"`
	AttenderName string `json:"attenderName"`
	Location     string `json:"location"`
	RoomID       string `json:"roomId"`
}

type stateJSON struct {
	Hospital string        `json:"hospital"`
	Rooms    []roomJSON    `json:"rooms"`
	Patients []patientJSON `json:"patients"`
	Doctors  []doctorJSON  `json:"doctors"`
	Episodes []episodeJSON `json:"episodes"`
}

type hallwayJSON struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
}

type doctorEpisodesJSON struct {
	DoctorID   string   `json:"doctorId"`
	DoctorName string   `json:"doctorName"`
	Episodes   []string `json:"episodes"`
}

type severeJSON struct {
	PatientID   string `json:"patientId"`
	PatientName string `json:"patientName"`
	Episodes    int    `json:"episodes"`
}

type queriesJSON struct {
	Hallway        []hallwayJSON        `json:"hallway"`
	DoctorEpisodes []doctorEpisodesJSON `json:"doctorEpisodes"`
	FreeRooms      []string             `json:"freeRooms"`
	SevereReport   []severeJSON         `json:"severeReport"`
}

/* ---------- utilidades ---------- */

// roomID arma el id de sala del JSON ("R-1"). Con nil devuelve "" (sin sala).
func roomID(r *hospital.Room) string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("R-%d", r.Number())
}

/*
writeJSON serializa PRIMERO y escribe DESPUÉS. Si json.Marshal falla, todavía
podemos responder 500; si codificáramos directo sobre w, el 200 ya estaría
enviado y no habría vuelta atrás.
*/
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		status = http.StatusInternalServerError
		body = []byte(`{"error":"no se pudo codificar la respuesta"}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		log.Printf("api: error escribiendo la respuesta: %v", err)
	}
}

// writeError responde siempre con la forma {"error":"mensaje legible"}.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// requireMethod responde 405 y devuelve false si el método no es el esperado.
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, "método no permitido: usa "+method)
	return false
}

/*
findPatient busca por id recorriendo h.Patients() (el hospital no tiene un
método de búsqueda y no podemos tocarlo). Patients() devuelve una copia, así
que recorrerla no interfiere con otras goroutines.
*/
func (s *server) findPatient(id string) *hospital.Patient {
	for _, p := range s.h.Patients() {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

/*
patientFromQuery valida el parámetro ?patient= y busca al paciente. Si algo
falla ya escribió la respuesta (400 si falta, 404 si no existe) y devuelve nil.
*/
func (s *server) patientFromQuery(w http.ResponseWriter, r *http.Request) *hospital.Patient {
	id := strings.TrimSpace(r.URL.Query().Get("patient"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "falta el parámetro patient")
		return nil
	}
	p := s.findPatient(id)
	if p == nil {
		writeError(w, http.StatusNotFound, "no existe el paciente "+id)
		return nil
	}
	return p
}

/* ---------- GET /api/state ---------- */

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.buildState())
}

/*
buildState arma la foto del hospital. Todos los slices se crean con make para
que, si están vacíos, salgan como [] y no como null (un slice nil se
serializa como null y la página haría .map sobre null y fallaría).

Cada getter del hospital es seguro por sí solo, pero la foto completa se arma
en varias llamadas: si alguien despierta a un paciente a mitad de camino, esta
respuesta puede mezclar un instante con otro. La página se refresca cada 2 s,
así que se corrige sola; para una foto exacta haría falta un método
Hospital.Snapshot() bajo un solo lock, que está fuera de mi parte.
*/
func (s *server) buildState() stateJSON {
	rooms := s.h.Rooms()
	patients := s.h.Patients()
	doctors := s.h.Doctors()
	history := s.h.History()

	st := stateJSON{
		Hospital: s.h.Name(),
		Rooms:    make([]roomJSON, 0, len(rooms)),
		Patients: make([]patientJSON, 0, len(patients)),
		Doctors:  make([]doctorJSON, 0, len(doctors)),
		Episodes: make([]episodeJSON, 0, len(history)),
	}

	for _, room := range rooms {
		item := roomJSON{ID: roomID(room)}
		// Occupants() es una copia tomada bajo el lock de la sala: es la fuente
		// de verdad de quién está dentro (más directa que recorrer pacientes).
		if occ := room.Occupants(); len(occ) > 0 {
			item.Occupied = true
			item.PatientID = occ[0].ID()
			item.PatientName = occ[0].Name()
		}
		st.Rooms = append(st.Rooms, item)
	}

	for _, p := range patients {
		item := patientJSON{
			ID:       p.ID(),
			Name:     p.Name(),
			Age:      p.Age(),
			Level:    p.Level().String(),
			State:    p.State().String(),
			Location: p.Location(),
			RoomID:   roomID(p.Room()),
		}
		if d := p.AssignedDoctor(); d != nil {
			item.DoctorID = d.ID()
			item.DoctorName = d.Name()
		}
		st.Patients = append(st.Patients, item)
	}

	for _, d := range doctors {
		ids := make([]string, 0)
		for _, p := range d.Patients() {
			ids = append(ids, p.ID())
		}
		st.Doctors = append(st.Doctors, doctorJSON{
			ID:           d.ID(),
			Name:         d.Name(),
			Specialty:    d.Specialty(),
			PatientIDs:   ids,
			EpisodeCount: len(d.MyEpisodes()),
		})
	}

	// h.History() ya viene en orden cronológico: no se reordena.
	for _, rec := range history {
		st.Episodes = append(st.Episodes, episodeToJSON(rec))
	}
	return st
}

// episodeToJSON tolera paciente o atendedor nil (un registro vacío no debe
// tumbar toda la respuesta).
func episodeToJSON(rec hospital.EpisodeRecord) episodeJSON {
	e := episodeJSON{
		ID:       rec.ID(),
		Time:     rec.When().Format(time.RFC3339),
		Location: rec.Location(),
		RoomID:   roomID(rec.Room()),
	}
	if p := rec.Patient(); p != nil {
		e.PatientID = p.ID()
		e.PatientName = p.Name()
	}
	if a := rec.Attender(); a != nil {
		e.AttenderID = a.ID()
		e.AttenderName = a.Name()
	}
	return e
}

/* ---------- GET /api/queries ---------- */

func (s *server) handleQueries(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.buildQueries())
}

// buildQueries ejecuta las cuatro consultas de la sección 5 del enunciado.
func (s *server) buildQueries() queriesJSON {
	q := queriesJSON{
		Hallway:        make([]hallwayJSON, 0),
		DoctorEpisodes: make([]doctorEpisodesJSON, 0),
		FreeRooms:      make([]string, 0),
		SevereReport:   make([]severeJSON, 0),
	}

	// 5.1 pacientes dormidos en el pasillo.
	for _, p := range s.h.PatientsInHallway() {
		q.Hallway = append(q.Hallway, hallwayJSON{ID: p.ID(), Name: p.Name(), Location: p.Location()})
	}

	// 5.2 episodios de cada médico (MyEpisodes vive dentro del propio Doctor).
	for _, d := range s.h.Doctors() {
		ids := make([]string, 0)
		for _, rec := range d.MyEpisodes() {
			ids = append(ids, rec.ID())
		}
		q.DoctorEpisodes = append(q.DoctorEpisodes, doctorEpisodesJSON{DoctorID: d.ID(), DoctorName: d.Name(), Episodes: ids})
	}

	// 5.3 salas con cupo.
	for _, room := range s.h.Rooms() {
		if room.IsAvailable() {
			q.FreeRooms = append(q.FreeRooms, roomID(room))
		}
	}

	// 5.4 reporte de casos severos. SevereReport devuelve un map y el orden de
	// iteración de un map en Go es ALEATORIO a propósito, así que se pasa a
	// lista y se ordena por id: la página no debe "saltar" en cada refresco.
	for p, count := range s.h.SevereReport() {
		q.SevereReport = append(q.SevereReport, severeJSON{PatientID: p.ID(), PatientName: p.Name(), Episodes: count})
	}
	sort.Slice(q.SevereReport, func(i, j int) bool {
		return q.SevereReport[i].PatientID < q.SevereReport[j].PatientID
	})
	return q
}

/* ---------- POST /api/sleep y /api/wake ---------- */

/*
handleSleep: 400 si falta patient o location, 404 si el paciente no existe,
409 (conflicto con el estado actual) si HandleSleepAttack devuelve error.

El paciente se busca ANTES de llamar al hospital: así distinguimos "no existe"
(404) de "existe pero no se puede" (409).
*/
func (s *server) handleSleep(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	location := strings.TrimSpace(r.URL.Query().Get("location"))
	if location == "" {
		writeError(w, http.StatusBadRequest, "falta el parámetro location")
		return
	}
	p := s.patientFromQuery(w, r)
	if p == nil {
		return
	}

	if err := s.h.HandleSleepAttack(p, location); err != nil {
		writeError(w, http.StatusConflict, humanError(p, err))
		return
	}
	// Sin error el paciente debería tener sala; si por algún motivo no la tiene,
	// se dice con claridad en vez de afirmar algo falso.
	if room := p.Room(); room != nil {
		writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("%s fue llevado a %s", p.Name(), roomID(room))})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("%s se durmió en %s y sigue en el pasillo", p.Name(), location)})
}

// handleWake despierta al paciente y libera su sala (lo hace el hospital).
func (s *server) handleWake(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	p := s.patientFromQuery(w, r)
	if p == nil {
		return
	}
	if err := s.h.WakePatient(p); err != nil {
		writeError(w, http.StatusConflict, humanError(p, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": p.Name() + " despertó"})
}

/*
humanError convierte un error del hospital en un mensaje para la pantalla.
Se identifica con errors.Is (no comparando texto): funciona aunque el hospital
envuelva el error con fmt.Errorf("...: %w", err). Los demás errores se muestran
tal cual; ya son legibles.
*/
func humanError(p *hospital.Patient, err error) string {
	switch {
	case errors.Is(err, hospital.ErrNoRoomAvailable):
		return "no hay salas libres: " + p.Name() + " sigue en el pasillo"
	case errors.Is(err, hospital.ErrNotAsleep):
		return p.Name() + " no está dormido"
	default:
		return err.Error()
	}
}
