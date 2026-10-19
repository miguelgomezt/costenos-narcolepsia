package hospital

import "errors"

/*
Errores centinela del paquete hospital.

Un error centinela es una variable global creada con errors.New. Quien llama
no compara textos: pregunta errors.Is(err, ErrNoRoomAvailable). Así el mensaje
puede cambiar o venir envuelto con fmt.Errorf("...: %w", err) y la comprobación
sigue funcionando. La API REST usa estos errores para elegir el código HTTP.

Los mensajes van en minúscula y sin punto final (convención de Go) porque casi
siempre se concatenan dentro de otro mensaje.
*/
var (
	// ErrNoRoomAvailable: ninguna sala tiene cupo. El paciente sigue en el pasillo.
	ErrNoRoomAvailable = errors.New("no room available")

	// ErrDuplicateID: ya existe una entidad con ese id (paciente, médico, sala...).
	ErrDuplicateID = errors.New("duplicate id")

	// ErrNotAsleep: se intentó una operación de sueño con un paciente despierto.
	ErrNotAsleep = errors.New("patient is not asleep")

	// ErrNilArgument: se pasó nil donde se esperaba un valor real.
	ErrNilArgument = errors.New("argument cannot be nil")

	// ErrRoomFull: la sala ya alcanzó su capacidad (Room.Occupy).
	ErrRoomFull = errors.New("room is full")

	// ErrAlreadyInRoom: el paciente ya está dentro de esa sala (Room.Occupy).
	ErrAlreadyInRoom = errors.New("patient is already in the room")

	// ErrNotInRoom: el paciente no está en esa sala (Room.Release).
	ErrNotInRoom = errors.New("patient is not in the room")
)