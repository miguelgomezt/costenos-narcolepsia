# AI_USAGE

## Alejandro 

**Herramientas:** Claude (Claude Code). Me apoyé en esta herramienta para corregir errores en las clases de salas, registros, errores, tests, API REST. En el bonus lo usé para el mutex de Room, los getters de EpisodeRecord, TestConcurrentAssign, api/server.go y el README.

**Prompt decisivo:** le pasé el contrato del equipo (firmas, orden de bloqueo, Go 1.21, JSON exacto de la web) y le pedí mi parte con comentarios en español que expliquen cada decisión. Luego le pasé el .zip para que siguiera el estilo del equipo.

**Casos donde el código no servía:**
1. Al correr gofmt -l . salieron archivos mal formateados porque tenia espacios o tabulaciones no aceptadas en go, y lo corregí con gofmt -w sobre mis archivos, volviendo a correr gofmt -l . hasta que no listó nada.

**Qué verifiqué yo:** corrí gofmt, go vet y go test -race -count=5, y probé los 4 endpoints con curl.

**Qué aprendí:** por qué sync.Mutex no es reentrante (versiones "Locked"), por qué comprobar cupo y ocupar debe ser una sola operación, qué es una data race, y por qué la API arma sus propios structs JSON.
