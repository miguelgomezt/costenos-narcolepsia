package hospital

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

/*
Doctor es un medico del hospital. Embebe Person y por eso hereda ID y Name,
dos de los tres metodos de la interfaz de Attender.
*/

type Doctor struct {
	Person
	specialty string
	patients  []*Patient
	episodes  []EpisodeRecord
}

//New doctor valida los datos y crea un médico sin pacientes ni episiodos.
func NewDoctor(id, name string, age int, specialty string) (*Doctor, error) {
	person, err := newPerson(id, name, age)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(specialty) == "" {
		return nil, errors.New("doctor: specialty cannot be empty")
	}
	return &Doctor{Person: person, specialty: specialty}, nil
}

//Specialty devuelve la especialidad del medico
func (d *Doctor) Specialty() string { return d.specialty }

//Patients devuelve la copia de la lista de pacientes a cargo del medico
func (d *Doctor) Patients() []*Patient {
	out := make([]*Patient, len(d.patients))
	copy(out, d.patients)
	return out
}

/*
DiagnosePatient deja al paciente a cargo de este médico.
Devuelve error si un paciente es nil o si ya tiene un medico
asignado
*/
func (d *Doctor) DiagnosePatient(p *Patient) error {
	if p == nil {
		return errors.New("doctor: patient cannot be nil")
	}
	if current := p.AssignedDoctor(); current != nil {
		return fmt.Errorf("patient %s already has doctor %s", p.ID(), current.Name())
	}
	d.patients = append(d.patients, p)
	p.setDoctor(d)
	return nil
}

/*
AttendSleepEmergency atiende a un paciente que se durmio, devuelve
error si el paciente esta despierto o es nil. Si el paciente no tenía
médico, este médico queda a cargo.
*/
func (d *Doctor) AttendSleepEmergency(p *Patient) error {
	if p == nil {
		return errors.New("doctor: patient cannot be nil")
	}
	if p.State() == Awake {
		return fmt.Errorf("patient %s is not asleep", p.ID())
	}
	if p.AssignedDoctor() == nil {
		return d.DiagnosePatient(p)
	}
	return nil
}

/*
Attend cumple la interfaz Attender: Atiende la emergencia, crea el registro del
episodio, lo guarda en la lista del propio médico y lo devuelve.
*/
func (d *Doctor) Attend(p *Patient, location string) (EpisodeRecord, error) {
	if err := d.AttendSleepEmergency(p); err != nil {
		return EpisodeRecord{}, err
	}
	if strings.TrimSpace(location) == "" {
		location = p.Location()
	}
	id := fmt.Sprintf("EP-%s-%d", d.ID(), len(d.episodes)+1)
	record := NewEpisodeRecord(id, time.Now(), p, d, location, p.Room())
	d.episodes = append(d.episodes, record)
	return record, nil
}

/*
MyEpisodes devuelve los episodios en lo que intervino este médico.
Devuelve una copia para quien la reciba no puede modificar la lista interna.
*/
func (d *Doctor) MyEpisodes() []EpisodeRecord {
	out := make([]EpisodeRecord, len(d.episodes))
	copy(out, d.episodes)
	return out
}
