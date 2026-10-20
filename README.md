# Hospital de los Costeños con Narcolepsia

Sistema en Go (1.21, solo librería estándar) que gestiona un hospital donde los
pacientes con narcolepsia se duermen de repente en cualquier lugar: sabe dónde
está cada paciente, qué médico lo atiende y registra cada episodio para
asignarle una cama antes de que bloquee un pasillo.

Trabajo de Programación Orientada a Objetos, Universidad EIA.

## Integrantes

| Persona | Parte |
|---|---|
| Miguel | Personas y personal: `person.go`, `patient.go`, `doctor.go`, `attender.go`, interfaz web (`web/`) |
| Alejandro | Salas, registros, errores y calidad: `room.go`, `episode.go`, `errors.go`, `hospital_test.go`, `api/server.go`, README |
| Juan José | Hospital, consultas y escenario: `hospital.go`, `simulation.go`, `main.go` |

## Cómo ejecutar

Desde la raíz del proyecto:

```
go run .          # escenario de demostración obligatorio (sin banderas)
go run . -sim     # BONUS: simulación concurrente (una goroutine por paciente)
go run . -web     # BONUS: interfaz web + API REST
```

Con `-web` el servidor queda en <http://localhost:8080>. **Hay que ejecutarlo
desde la raíz del proyecto**, porque la API sirve la carpeta `web/` por ruta
relativa; si no la encuentra, `api.Start` devuelve un error explicándolo.

Sin banderas el programa ejecuta exactamente el escenario obligatorio: los
bonus solo se activan con `-sim` y `-web`.

## Cómo probar

```
go test ./...                  # tests normales
go test -race -count=5 ./...   # con detector de carreras (los de concurrencia)
go run -race . -sim            # la simulación no debe mostrar "DATA RACE"
gofmt -l .                     # no debe listar nada
go vet ./...                   # no debe reportar nada
```

`-race` necesita cgo y un compilador de C. En Windows: instalar MSYS2,
`pacman -S mingw-w64-ucrt-x86_64-gcc`, agregar `C:\msys64\ucrt64\bin` al PATH y
`go env -w CGO_ENABLED=1`. Basta con que una de las tres máquinas lo verifique.

### Probar la API con curl (`go run . -web` corriendo)

```
curl http://localhost:8080/api/state
curl http://localhost:8080/api/queries
curl -X POST "http://localhost:8080/api/sleep?patient=P-001&location=cafeteria"
curl -X POST "http://localhost:8080/api/wake?patient=P-001"
```

| Endpoint | Éxito | Errores |
|---|---|---|
| `GET /api/state` | 200 con salas, pacientes, médicos y episodios | 405 |
| `GET /api/queries` | 200 con las cuatro consultas (5.1 a 5.4) | 405 |
| `POST /api/sleep?patient=&location=` | 200 `{"message": ...}` | 400 falta parámetro · 404 paciente no existe · 409 sin salas o ya dormido · 405 |
| `POST /api/wake?patient=` | 200 `{"message": ...}` | 400 · 404 · 409 ya despierto · 405 |

Todos los errores salen como `{"error": "mensaje legible"}`.

## Estructura

```
main.go          escenario + banderas -sim y -web
hospital/        modelo completo (no sabe nada de la API ni de la web)
  person.go  patient.go  doctor.go  attender.go   personas y personal
  room.go  episode.go  errors.go                  salas, registros, errores
  hospital.go  simulation.go                      hospital y simulación
  hospital_test.go                                tests
api/server.go    BONUS: API REST con net/http (importa hospital)
web/             BONUS: index.html, app.js, style.css (HTML y JS puro)
```

## Decisiones de diseño

- **Encapsulación:** todos los campos son no exportados; se accede con métodos
  (`Name()`, no `GetName()`). Desde otro paquete el compilador rechaza el acceso
  directo.
- **Composición:** `Person` va embebido por valor en `Patient`, `Doctor` y
  `Orderly`; heredan `ID()` y `Name()` sin duplicar campos.
- **Polimorfismo:** el hospital guarda un `[]Attender` y llama a `Attend` sin
  saber si es `Doctor` u `Orderly`. La interfaz se cumple de forma implícita.
- **Estados como constantes tipadas** (`PatientState`, `NarcolepsyLevel`,
  `RoomState`) con `String()`, no strings sueltos.
- **Errores, no panic:** todo método que puede fallar devuelve `error`. Los
  errores centinela de `errors.go` se envuelven con `%w` y se comprueban con
  `errors.Is`, por ejemplo `errors.Is(err, ErrNoRoomAvailable)`.
- **Copias de slices:** los getters (`MyEpisodes`, `Occupants`, `Rooms`...)
  devuelven copias para que nadie modifique la lista interna.
- **`EpisodeRecord` es un valor inmutable:** se crea completo y no tiene
  métodos que lo cambien, por eso no necesita mutex.
- **Separación modelo/vista:** `hospital/` no cambió al agregar la web. La API
  solo importa `hospital`, usa sus métodos públicos y arma sus propios structs
  JSON (`encoding/json` solo ve campos exportados y los del modelo son privados).

## Nota sobre los mutex (bonus de concurrencia)

El problema: si varias goroutines llaman a `AssignRoom` a la vez, dos pueden ver
la misma sala libre y ocuparla las dos (data race). En la API pasa igual: cada
petición HTTP corre en su propia goroutine.

- **`Hospital.mu` (`sync.Mutex`)** hace que comprobar y ocupar sea una sola
  operación atómica.
- **`Room`** lleva un `sync.RWMutex` propio: `Occupy` y `Release` escriben
  (`Lock`); `IsAvailable`, `State` y `Occupants` solo leen (`RLock`, varias
  lecturas en paralelo). `Patient` y `Doctor` también tienen el suyo, porque la
  web y la simulación los leen desde otras goroutines.
- **Orden de bloqueo:** `Hospital.mu` → `Room.mu` / `Patient.mu` / `Doctor.mu`.
  Nunca al revés: `Room` no llama al hospital ni bloquea a un paciente mientras
  tiene su lock. Un orden fijo evita que dos goroutines se esperen en círculo.
- **Deadlock:** `sync.Mutex` no es reentrante. Un método con el lock que llamara a
  otro método público que también bloquea se quedaría esperando para siempre.
  Por eso las operaciones tienen una versión pública con lock y otra privada
  `...Locked` sin lock (`assignRoomLocked`, `indexOfLocked`).
- **Qué prueba cada test:** `TestRoomConcurrentOccupy` comprueba solo el mutex de
  `Room` (50 goroutines, sala de 3 camas, entran exactamente 3);
  `TestConcurrentAssign` comprueba el conjunto (20 pacientes, 5 salas de una
  cama: exactamente 5 asignaciones, el resto falla con `ErrNoRoomAvailable` y
  sigue en el pasillo).
- **Límite de `-race`:** avisa de las carreras que ocurren durante la ejecución;
  no prueba que no puedan ocurrir. Por eso los tests se repiten con `-count=5`.



