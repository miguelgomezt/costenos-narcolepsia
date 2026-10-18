/*
 * Interfaz del Hospital de los Costeños.
 *
 * Esta página no conoce Go: solo habla con la API REST de api/server.go:
 *   GET  /api/state                       -> salas, pacientes, médicos, historial
 *   GET  /api/queries                     -> resultado de las consultas 5.1 a 5.4
 *   POST /api/sleep?patient=ID&location=X -> ataque de sueño (409 si no hay sala)
 *   POST /api/wake?patient=ID             -> despertar al paciente
 *
 * Abre /?mock=1 para probar la página con datos de ejemplo, sin servidor Go.
 */
"use strict";

const REFRESH_MS = 2000;
const MOCK = new URLSearchParams(location.search).has("mock");

/* ---------- utilidades ---------- */

// Crea un elemento. Siempre usamos textContent (nunca innerHTML) para que un
// nombre raro que venga del servidor no pueda inyectar HTML.
function el(tag, props, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(props || {})) {
    if (key === "class") node.className = value;
    else if (key === "text") node.textContent = value;
    else if (key.startsWith("data-")) node.setAttribute(key, value);
    else node[key] = value;
  }
  for (const child of children) if (child) node.append(child);
  return node;
}

const STATE_LABEL = {
  Awake: ["Despierto", "awake"],
  AsleepInHallway: ["Dormido en pasillo", "hall"],
  AsleepInBed: ["Dormido en cama", "bed"],
};

function formatTime(iso) {
  const d = new Date(iso);
  return isNaN(d) ? iso : d.toLocaleTimeString("es-CO", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

let toastTimer = null;
function toast(message, isError) {
  const box = document.getElementById("toast");
  box.textContent = message;
  box.className = "toast" + (isError ? " error" : "");
  box.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { box.hidden = true; }, 3500);
}

function setConnection(kind, text) {
  const box = document.getElementById("conn");
  box.className = "conn " + kind;
  box.textContent = text;
}

/* ---------- datos de ejemplo (?mock=1) ---------- */

const mockData = {
  state: {
    hospital: "Hospital de los Costeños",
    rooms: [
      { id: "R-1", occupied: true, patientId: "P-002", patientName: "Luis Pérez" },
      { id: "R-2", occupied: false },
      { id: "R-3", occupied: false },
    ],
    patients: [
      { id: "P-001", name: "Carlos Mendoza", age: 45, level: "Severe", state: "Awake", location: "reception", roomId: "", doctorId: "D-1", doctorName: "Dra. Valdés" },
      { id: "P-002", name: "Luis Pérez", age: 32, level: "Moderate", state: "AsleepInBed", location: "cafetería", roomId: "R-1", doctorId: "D-1", doctorName: "Dra. Valdés" },
      { id: "P-003", name: "María Ríos", age: 58, level: "Mild", state: "AsleepInHallway", location: "pasillo 2", roomId: "", doctorId: "", doctorName: "" },
    ],
    doctors: [{ id: "D-1", name: "Dra. Valdés", specialty: "Neurología", patientIds: ["P-001", "P-002"], episodeCount: 1 }],
    episodes: [
      { id: "EP-D-1-1", time: new Date().toISOString(), patientId: "P-002", patientName: "Luis Pérez", attenderId: "D-1", attenderName: "Dra. Valdés", location: "cafetería", roomId: "R-1" },
    ],
  },
};

// En modo mock, "dormir" y "despertar" cambian los datos locales, igual que haría Go.
function mockQueries() {
  const s = mockData.state;
  const count = (id) => s.episodes.filter((e) => e.patientId === id).length;
  return {
    hallway: s.patients.filter((p) => p.state === "AsleepInHallway").map((p) => ({ id: p.id, name: p.name, location: p.location })),
    doctorEpisodes: s.doctors.map((d) => ({
      doctorId: d.id,
      doctorName: d.name,
      episodes: s.episodes.filter((e) => e.attenderId === d.id).map((e) => e.id),
    })),
    freeRooms: s.rooms.filter((r) => !r.occupied).map((r) => r.id),
    severeReport: s.patients.filter((p) => p.level === "Severe").map((p) => ({ patientId: p.id, patientName: p.name, episodes: count(p.id) })),
  };
}

function mockSleep(id, loc) {
  const s = mockData.state;
  const p = s.patients.find((x) => x.id === id);
  if (!p) throw new Error("paciente desconocido");
  if (p.state !== "Awake") throw new Error(p.name + " ya está dormido");
  p.state = "AsleepInHallway";
  p.location = loc;
  const room = s.rooms.find((r) => !r.occupied);
  if (!room) throw new Error("no hay salas libres: " + p.name + " sigue en el pasillo");
  room.occupied = true;
  room.patientId = p.id;
  room.patientName = p.name;
  p.state = "AsleepInBed";
  p.roomId = room.id;
  s.episodes.push({ id: "EP-D-1-" + (s.episodes.length + 1), time: new Date().toISOString(), patientId: p.id, patientName: p.name, attenderId: "D-1", attenderName: "Dra. Valdés", location: loc, roomId: room.id });
  return p.name + " fue llevado a " + room.id;
}

function mockWake(id) {
  const s = mockData.state;
  const p = s.patients.find((x) => x.id === id);
  if (!p || p.state === "Awake") throw new Error("el paciente ya está despierto");
  const room = s.rooms.find((r) => r.patientId === id);
  if (room) { room.occupied = false; delete room.patientId; delete room.patientName; }
  p.state = "Awake";
  p.roomId = "";
  return p.name + " despertó";
}

/* ---------- llamadas a la API ---------- */

async function getJSON(path) {
  const res = await fetch(path, { cache: "no-store" });
  if (!res.ok) throw new Error("HTTP " + res.status);
  return res.json();
}

async function post(path) {
  const res = await fetch(path, { method: "POST" });
  let body = {};
  try { body = await res.json(); } catch (_) { /* respuesta sin JSON */ }
  if (!res.ok) throw new Error(body.error || "Error " + res.status);
  return body.message || "Listo";
}

async function act(kind, patientId, location) {
  try {
    let message;
    if (MOCK) {
      message = kind === "sleep" ? mockSleep(patientId, location) : mockWake(patientId);
    } else if (kind === "sleep") {
      message = await post("/api/sleep?patient=" + encodeURIComponent(patientId) + "&location=" + encodeURIComponent(location));
    } else {
      message = await post("/api/wake?patient=" + encodeURIComponent(patientId));
    }
    toast(message, false);
  } catch (err) {
    toast(err.message, true);
  }
  refresh();
}

/* ---------- dibujo ---------- */

function renderStats(state) {
  const awake = state.patients.filter((p) => p.state === "Awake").length;
  const hall = state.patients.filter((p) => p.state === "AsleepInHallway").length;
  const bed = state.patients.filter((p) => p.state === "AsleepInBed").length;
  const free = state.rooms.filter((r) => !r.occupied).length;
  const items = [
    [awake, "Despiertos"],
    [hall, "En el pasillo"],
    [bed, "En cama"],
    [free + " / " + state.rooms.length, "Salas libres"],
    [state.episodes.length, "Episodios"],
  ];
  document.getElementById("stats").replaceChildren(
    ...items.map(([n, label]) => el("div", { class: "stat" }, el("b", { text: String(n) }), el("span", { text: label })))
  );
}

function renderRooms(state) {
  document.getElementById("rooms").replaceChildren(
    ...state.rooms.map((r) =>
      el("div", { class: "room " + (r.occupied ? "busy" : "free") },
        el("div", { class: "rid", text: r.id }),
        el("div", { class: "rstate", text: r.occupied ? "Ocupada por " + (r.patientName || r.patientId) : "Libre" }))
    )
  );
}

function renderPatients(state) {
  const box = document.getElementById("patients");
  if (state.patients.length === 0) {
    box.replaceChildren(el("p", { class: "empty", text: "No hay pacientes registrados." }));
    return;
  }
  box.replaceChildren(
    ...state.patients.map((p) => {
      const [label, cls] = STATE_LABEL[p.state] || [p.state, "lvl"];
      const awake = p.state === "Awake";
      const button = awake
        ? el("button", { class: "btn primary", text: "Ataque de sueño", "data-act": "sleep", "data-id": p.id, "data-name": p.name })
        : el("button", { class: "btn", text: "Despertar", "data-act": "wake", "data-id": p.id, "data-name": p.name });
      const meta = [p.id, p.age + " años", "en " + (p.location || "—")];
      if (p.roomId) meta.push("sala " + p.roomId);
      meta.push(p.doctorName ? "médico: " + p.doctorName : "sin médico");
      return el("div", { class: "patient" },
        el("div", { class: "pname" },
          document.createTextNode(p.name),
          el("span", { class: "badge " + cls, text: label }),
          el("span", { class: "badge lvl" + (p.level === "Severe" ? " severe" : ""), text: p.level })),
        el("div", { class: "pmeta", text: meta.join(" · ") }),
        el("div", { class: "pact" }, button));
    })
  );
}

function queryCard(title, desc, items) {
  const body = items.length
    ? el("ul", {}, ...items.map((t) => el("li", { text: t })))
    : el("p", { class: "empty", text: "Sin resultados" });
  return el("div", { class: "qcard" }, el("h3", { text: title }), el("p", { class: "qdesc", text: desc }), body);
}

function renderQueries(q) {
  document.getElementById("queries").replaceChildren(
    queryCard("5.1 Pacientes en el pasillo", "Esperan un camillero", (q.hallway || []).map((p) => p.name + " (" + p.location + ")")),
    queryCard("5.2 Episodios por médico", "MyEpisodes de cada médico", (q.doctorEpisodes || []).map((d) => d.doctorName + ": " + d.episodes.length)),
    queryCard("5.3 Salas libres", "Candidatas para AssignRoom", q.freeRooms || []),
    queryCard("5.4 Reporte de casos severos", "Episodios por paciente Severe", (q.severeReport || []).map((r) => r.patientName + ": " + r.episodes))
  );
}

function renderHistory(state) {
  const box = document.getElementById("history");
  if (state.episodes.length === 0) {
    box.replaceChildren(el("p", { class: "empty", text: "Todavía no hay episodios." }));
    return;
  }
  const newestFirst = state.episodes.slice().reverse();
  box.replaceChildren(
    ...newestFirst.map((e) =>
      el("div", { class: "episode" },
        el("time", { text: formatTime(e.time) }),
        el("span", { text: e.patientName + " se durmió en " + e.location + " · atendido por " + e.attenderName + (e.roomId ? " · sala " + e.roomId : " · sin sala") }))
    )
  );
}

/* ---------- ciclo de actualización ---------- */

async function refresh() {
  try {
    const state = MOCK ? mockData.state : await getJSON("/api/state");
    const queries = MOCK ? mockQueries() : await getJSON("/api/queries");
    document.getElementById("hospital-name").textContent = state.hospital || "Hospital de los Costeños";
    document.title = state.hospital || "Hospital de los Costeños";
    renderStats(state);
    renderRooms(state);
    renderPatients(state);
    renderQueries(queries);
    renderHistory(state);
    setConnection(MOCK ? "mock" : "ok", MOCK ? "Datos de ejemplo" : "Conectado");
  } catch (err) {
    setConnection("bad", "Sin conexión con la API");
  }
}

/* ---------- eventos ---------- */

const dialog = document.getElementById("sleep-dialog");
const locationInput = document.getElementById("sleep-location");
let pendingPatient = null;

// Un solo listener para todos los botones (los botones se recrean en cada refresh).
document.getElementById("patients").addEventListener("click", (event) => {
  const btn = event.target.closest("button[data-act]");
  if (!btn) return;
  const id = btn.getAttribute("data-id");
  if (btn.getAttribute("data-act") === "wake") {
    act("wake", id);
    return;
  }
  pendingPatient = id;
  document.getElementById("sleep-who").textContent = btn.getAttribute("data-name") + " (" + id + ")";
  locationInput.value = "";
  dialog.showModal();
  locationInput.focus();
});

document.getElementById("sleep-cancel").addEventListener("click", () => dialog.close());

document.getElementById("sleep-form").addEventListener("submit", () => {
  const where = locationInput.value.trim();
  if (pendingPatient && where) act("sleep", pendingPatient, where);
  pendingPatient = null;
});

refresh();
setInterval(refresh, REFRESH_MS);