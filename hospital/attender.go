package hospital

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

/*
Attender es cualquiera que pueda atender a un paciente dormido.
El hospital puede guardar un []Attender y llamar a Attend sin saber quién es cada uno.
(Polimorfismo)
*/

type Attender interface {
	ID() string
	Name() string
	Attend(p *Patient, location string) (EpisodeRecord, error)
}

/*
Hacemos comprobaciones, si un tipo deja de cumplir,
no compila.
*/
var (
	_ Attender = (*Doctor)(nil)
	_ Attender = (*Orderly)(nil)
)

/*
Orderly es un camillero. Embebe Person (Hereda ID y Name) y a diferencia del
médico, no diagnostica. Solo lleva al paciente y deja el registro del episodio.
*/

type Orderly struct {
	Person
	mu       sync.RWMutex
	episodes []EpisodeRecord
}

/*
NewOrderly valida los datos y crea un camillero
sin episodios.
*/

func NewOrderly(id, name string, age int) (*Orderly, error) {
	person, err := newPerson(id, name, age)
	if err != nil {
		return nil, err
	}
	return &Orderly{Person: person}, nil
}

/*
Attend cumple con la interfaz Attender. Devuelve error si el paciente es nil
o está despierto. Crea el registro, lo guarda y lo devuelve.
*/
func (o *Orderly) Attend(p *Patient, location string) (EpisodeRecord, error) {
	if p == nil {
		return EpisodeRecord{}, errors.New("orderly: Patient cannot be nil")
	}
	if p.State() == Awake {
		return EpisodeRecord{}, fmt.Errorf("patient %s is not asleep", p.ID())
	}
	if strings.TrimSpace(location) == "" {
		location = p.Location()
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	id := fmt.Sprintf("EP-%s-%d", o.ID(), len(o.episodes)+1)
	record := NewEpisodeRecord(id, time.Now(), p, o, location, p.Room())
	o.episodes = append(o.episodes, record)
	return record, nil
}

//MyEpisodes devuelve una copia de los episodios que atendio el camillero.
func (o *Orderly) MyEpisodes() []EpisodeRecord {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]EpisodeRecord, len(o.episodes))
	copy(out, o.episodes)
	return out
}
