package hospital

import (
	"errors"
	"strings"
)

/**Person guarda los datos de identidad que comparten paciente y médicos.
Se embede (composición) en Patient, Doctor y Orderly, asi que esos tipos
no repiten los campos ni los metodos.
**/

type Person struct {
	id   string
	name string
	age  int
}

/**
newPerson valida los datos y contruye una persona. Es privada(Minuscula)
porque solo la usan los contructores de Patient, Doctor y Orderly, que estan
en un mismo paquete. Asi la validación esta en un solo lugar.
**/
func newPerson(id, name string, age int) (Person, error) {
	if strings.TrimSpace(id) == "" {
		return Person{}, errors.New("person : id cannot be empty")
	}
	if strings.TrimSpace(name) == "" {
		return Person{}, errors.New("person: name cannot be empty")
	}
	if age < 0 || age > 150 {
		return Person{}, errors.New("person: age must be between 0 and 150")
	}
	return Person{id: id, name: name, age: age}, nil
}

// ID devuelve el identificador de la persona
func (p Person) ID() string { return p.id }

// Name devuelve el nombre de la persona.
func (p Person) Name() string { return p.name }

// Age devuelve la edad de la persona.
func (p Person) Age() int { return p.age }
