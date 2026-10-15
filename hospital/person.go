package hospital

import (
	"errors"
	"strings"
)


type Person struct {
	id   string
	name string
	age  int
}

func newPerson(id, name string, age int) (Person, error) {
	if strings.TrimSpace(id) == "" {
	return Person{}, errors.New("person : id cannot be empty")
}
if strings.TrimSpace(name) == ""{
	return Person{}, errors.New("person: name cannot be empty")
}
if age < 0 || age > 150 {
	return Person{}, errors.New("person: age must be between 0 and 150")
}
return Person{id: id, name: name, age: age}, nil
}

// ID devuelve el identificador de la persona
func (p Person) ID() string{return p.id}

// Name devuelve el nombre de la persona.
func (p Person) Name() string{return p.name}

// Age devuelve la edad de la persona. 
func (p Person) Age() int{return p.age}