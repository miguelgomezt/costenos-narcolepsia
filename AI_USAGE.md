# AI_USAGE

## Miguel
# Uso de IA — Miguel Gómez (Persona 1)

## Herramienta
Claude, en conversación durante todo el trabajo.

## Para qué lo usé
- **Organización y entorno:** plan de trabajo en Word, reparto entre los tres, guía para instalar Go, VS Code y Git y crear el repositorio, y los prompts que les pasé a Alejandro y a Juan José para su bonus.
- **Mis archivos de lógica** (`person.go`, `patient.go`, `doctor.go`, `attender.go`): Claude me explicó los conceptos y me dio versiones de referencia. Yo las escribí en mi equipo, las compilé y corregí los errores que Claude me señaló.
- **Mutex (bonus de concurrencia):** Claude me indicó qué métodos de `Patient`, `Doctor` y `Orderly` proteger con `sync.RWMutex` y yo lo apliqué.
- **Interfaz web (bonus):** `web/index.html`, `style.css` y `app.js` los generó Claude. Yo los copié, los probé en el navegador con la API de Alejandro y corregí un nombre de archivo.

**Qué hice yo**
- Instalé el entorno, creé el repositorio y escribí mis archivos.
- Compilé con `gofmt`, `go vet` y `go build` y corregí los errores.
- Probé el programa completo: `go run .`, `go run . -sim` y `go run . -web`.

**Cómo verifiqué**
- `gofmt -l .` sin salida, `go vet ./...` limpio, `go build ./...` y `go test ./...` en verde.
- La simulación terminó con números coherentes (ataques = con sala + sin sala; todas las salas libres al final).

**Lo que puedo explicar**
- El hospital tiene una lista de `Attender`. Tanto `Doctor` como `Orderly` tienen los métodos que pide la interfaz, así que el hospital llama a `Attend` sin saber cuál de los dos es, y cada uno actúa a su manera.
- `sync.Mutex` no es reentrante: si una goroutine ya tiene el lock y lo pide otra vez, se queda esperando a sí misma para siempre. Eso es un deadlock.

**Limitaciones**
- El código de la interfaz web es de la IA: lo revisé y lo probé, pero no lo escribí yo.



## Alejandro 

**Herramientas:** Claude (Claude Code). Me apoyé en esta herramienta para corregir errores en las clases de salas, registros, errores, tests, API REST. En el bonus lo usé para el mutex de Room, los getters de EpisodeRecord, TestConcurrentAssign, api/server.go y el README.

**Prompt decisivo:** le pasé el contrato del equipo (firmas, orden de bloqueo, Go 1.21, JSON exacto de la web) y le pedí mi parte con comentarios en español que expliquen cada decisión. Luego le pasé el .zip para que siguiera el estilo del equipo.

**Casos donde el código no servía:**
1. Al correr gofmt -l . salieron archivos mal formateados porque tenia espacios o tabulaciones no aceptadas en go, y lo corregí con gofmt -w sobre mis archivos, volviendo a correr gofmt -l . hasta que no listó nada.

**Qué verifiqué yo:** corrí gofmt, go vet y go test -race -count=5, y probé los 4 endpoints con curl.

**Qué aprendí:** por qué sync.Mutex no es reentrante (versiones "Locked"), por qué comprobar cupo y ocupar debe ser una sola operación, qué es una data race, y por qué la API arma sus propios structs JSON.
