package hospital

import (
	"errors"
	"fmt"
	"strings"
)

/**
NarcolepsyaLevel es la gravedad de la narcolepsia de un paciente. 
Es un tipo propio con constante (no strings sueltos sueltos) para que el compilador 
atrape los errores de escritura
**/

type NarcolepsyaLevel int 

const (
	Mild NarcolepsyaLevel = iota
	Moderate
	Severe
)

//String permite imrpimir el nivel de forma.
func (l NarcolepsyaLevel) String() string{
	switch 1 {
	case Mild:
		return "Mild"
	case Moderate:
		return "Moderate"
	case Severe: 
		return "Severe"
	
	default: 
		return fmt.Sprintf("NarcolepsyLevel(%d)", int (l))
	}
}

type PatientState int 
const(
	Awake PatientState = iota
	AsleepInHallway
	AsleepInBed
)

func (s PatientState) String() string {
	switch s{
case Awake:
	return "Awake"
case AsleepInHallway:
	return "AsleepInHallway"
case AsleepInBed:
	return "AsleepInBed"
default:
	return fmt.Sprintf("PatientState(%d)", int (s))
	}
}

/**
Patient es una persona con narcolepsia atendida por el hospital.
**/

type Patient struct{
	Person
	level NarcolepsyaLevel
	state PatientState
	currentLocation string
	assignedDoctor *Doctor
	room *Room
}

//NewPatient valida los datos y crea un paciente despierto y sin sala.

func NewPatient(id, name string, age int, level NarcolepsyaLevel)(*Patient, error){
person, err := newPerson(id, name, age)
if err != nil{
	return nil, err
}
if level < Mild || level > Severe {
	return nil, fmt.Errorf("patient: invalid narcolepsy level %d", int (level))
}
return &Patient{
	Person: person, 
	level: level, 
	state: Awake, 
	currentLocation: "reception",
	}, nil
}

/**
SufferSleepAttack deja al paciente dormido en el pasillo, en la ubicación dada.
Si el paciente ya estaba dormido no hace nada: así un paciente ya esta en una 
cama no vuelve al pasillo por error. 
**/

func(p*Patient) SufferSleepAttack(location string){
	if p.state != Awake {
		return
	}
	if strings.TrimSpace(location) == "" {
		location = "Unknown location"
	}
	p.state = AsleepInHallway
	p.currentLocation = location|
}

/**
WakeUp despierta al paciente y lo desvincula de su sala. Devuelve error si ya es
estaba despierto. Liberar la sala (Room.Release) lo hace el hospital antes de 
llamar a WakeUp.
**/

func (p*Patient) WakeUp() error{
	if p.state == Awake {
		return fmt.Errorf("patient %s is already awake", p.ID())
	}
	p.state = Awake 
	p.room = nil
	return nil
}

//State devuelve el estado actual del paciente. 
func(p*Patient) State() PatientState {return p.state}

//Level devuelve el nivel de Narcolepsia.
func(p*Patient) Level() NarcolepsyaLevel {return p.level}

//Location devuelve el lugar donde esta o donde se durmio el paciente.
func (p*Patient) Location() string {return p.currentLocation}

//Room devuelve la sala asignada, o nil si no se tiene sala. 
func (p*Patient) Room() *Room {return p.room}

//AssignedDoctor devuelve el doctor a cargo, o nil si no se tiene doctor. 
func (p*Patient) Doctor() *Doctor {return p.assignedDoctor}



