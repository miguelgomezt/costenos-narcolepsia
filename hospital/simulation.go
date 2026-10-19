package hospital

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// SimulationSummary es el resultado de RunSimulation. El paquete hospital no
// imprime nada: devuelve estos números y main decide cómo mostrarlos.
type SimulationSummary struct {
	Attempts       int // ataques de sueño intentados (llamadas a HandleSleepAttack)
	Successes      int // ataques en que el paciente consiguió sala
	NoRoomFailures int // ataques en que no había sala (ErrNoRoomAvailable)
	OtherErrors    int // cualquier otro error (no debería haber ninguno)
	WakeUps        int // veces que un paciente se despertó con éxito
}

// Intervalos de la simulación. Son cortos para que en pocos segundos haya
// muchos ataques y muchas goroutines compitiendo por las mismas salas, que
// es justo lo que queremos poner a prueba con -race.
const (
	minAwake = 20 * time.Millisecond  // mínimo despierto antes de dormirse
	maxAwake = 120 * time.Millisecond // máximo despierto antes de dormirse
	minSleep = 30 * time.Millisecond  // mínimo dormido antes de despertar
	maxSleep = 150 * time.Millisecond // máximo dormido antes de despertar
)

// simLocations son los lugares donde un paciente puede dormirse. Es un
// arreglo que solo se LEE desde las goroutines, nunca se escribe, así que
// compartirlo es seguro sin mutex (una data race necesita al menos una
// escritura).
var simLocations = [...]string{
	"cafetería",
	"fila de radiología",
	"pasillo 2, segundo piso",
	"parqueadero",
	"sala de espera",
	"farmacia",
	"entrada principal",
}

// randomBetween devuelve una duración aleatoria en [lo, hi). Los parámetros
// no se llaman min y max porque desde Go 1.21 esos son funciones integradas
// del lenguaje y las estaríamos tapando dentro de la función.
func randomBetween(rng *rand.Rand, lo, hi time.Duration) time.Duration {
	return lo + time.Duration(rng.Int63n(int64(hi-lo)))
}

// RunSimulation lanza UNA goroutine por paciente. Cada goroutine, hasta que
// pase duration, repite: despierto un rato -> ataque de sueño -> dormido un
// rato -> despertar. Al final espera a todas y devuelve el resumen.
//
// Decisiones de concurrencia:
//   - sync.WaitGroup: wg.Add(1) ANTES de lanzar cada goroutine y wg.Done()
//     con defer al salir; wg.Wait() bloquea hasta que todas terminan. Si el
//     Add estuviera dentro de la goroutine, Wait podría correr antes y
//     devolver sin esperar.
//   - Contadores con sync/atomic: muchas goroutines suman al mismo contador;
//     un "attempts++" normal es leer-sumar-escribir y dos goroutines podrían
//     pisarse (data race). atomic.Int64.Add lo hace en un solo paso.
//   - Un *rand.Rand propio por goroutine: rand.Rand NO es seguro para
//     concurrencia, así que compartir uno sería una data race. Cada goroutine
//     crea el suyo con una semilla distinta.
//   - Go 1.21: la variable del for es UNA sola para todo el bucle. Si la
//     goroutine usara p directamente, todas podrían ver el último paciente.
//     Por eso p e i se pasan como parámetros: cada goroutine recibe su copia.
//   - Las goroutines no tocan los campos del hospital: solo llaman a sus
//     métodos públicos, que ya toman h.mu por dentro.
func RunSimulation(h *Hospital, patients []*Patient, duration time.Duration) SimulationSummary {
	if h == nil || duration <= 0 {
		return SimulationSummary{}
	}

	var (
		wg                                 sync.WaitGroup
		attempts, successes, noRoom, other atomic.Int64
		wakeUps                            atomic.Int64
		deadline                           = time.Now().Add(duration)
		baseSeed                           = time.Now().UnixNano()
	)

	for i, p := range patients {
		if p == nil {
			continue
		}
		wg.Add(1)
		go func(i int, p *Patient) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(baseSeed + int64(i)))

			for time.Now().Before(deadline) {
				// 1. Despierto un rato.
				time.Sleep(randomBetween(rng, minAwake, maxAwake))
				if !time.Now().Before(deadline) {
					return // se acabó el tiempo mientras estaba despierto
				}

				// 2. Ataque de sueño en un lugar aleatorio.
				location := simLocations[rng.Intn(len(simLocations))]
				err := h.HandleSleepAttack(p, location)
				attempts.Add(1)
				switch {
				case err == nil:
					successes.Add(1)
				case errors.Is(err, ErrNoRoomAvailable):
					// No hay sala: el paciente queda en el pasillo. No es
					// un fallo del programa, es un caso previsto.
					noRoom.Add(1)
				default:
					other.Add(1)
				}

				// 3. Dormido un rato y luego despierta (libera la sala si
				// tenía). Se despierta SIEMPRE, también después del plazo,
				// para que al terminar ningún paciente quede ocupando sala.
				time.Sleep(randomBetween(rng, minSleep, maxSleep))
				if err := h.WakePatient(p); err != nil {
					other.Add(1)
				} else {
					wakeUps.Add(1)
				}
			}
		}(i, p) // se pasan como parámetros por la regla de Go 1.21
	}

	wg.Wait() // espera a que TODAS las goroutines terminen

	// Aquí ya no queda ninguna goroutine escribiendo, así que leer los
	// contadores es seguro.
	return SimulationSummary{
		Attempts:       int(attempts.Load()),
		Successes:      int(successes.Load()),
		NoRoomFailures: int(noRoom.Load()),
		OtherErrors:    int(other.Load()),
		WakeUps:        int(wakeUps.Load()),
	}
}
