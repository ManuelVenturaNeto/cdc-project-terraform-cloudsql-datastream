package gen

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"
)

var firstNames = []string{
	"Ana", "Bruno", "Carla", "Diego", "Elisa", "Felipe", "Gabriela", "Hugo",
	"Isabela", "Joao", "Karen", "Lucas", "Marina", "Nina", "Otavio", "Paula",
	"Rafael", "Sofia", "Thiago", "Vera",
}

var lastNames = []string{
	"Almeida", "Barbosa", "Cardoso", "Duarte", "Ferreira", "Gomes", "Lima",
	"Martins", "Nunes", "Oliveira", "Pereira", "Ribeiro", "Santos", "Silva", "Souza",
}

var streets = []string{
	"Rua das Acacias", "Avenida Brasil", "Rua Sete de Setembro", "Avenida Atlantica",
	"Rua dos Girassois", "Travessa do Comercio", "Alameda dos Ipes", "Rua Marechal Deodoro",
	"Avenida Getulio Vargas", "Rua Voluntarios da Patria",
}

var districts = []string{
	"Centro", "Jardim America", "Vila Nova", "Boa Vista", "Santa Cecilia",
	"Alto da Boa Vista", "Parque Industrial", "Bela Vista", "Sao Jose", "Cidade Jardim",
}

var complements = []string{
	"", "Apto 101", "Apto 302", "Bloco B", "Casa 2", "Sala 45", "Fundos", "Cobertura",
}

var addressLabels = []string{"home", "work", "billing", "pickup"}

type city struct {
	name      string
	state     string
	areaCode  int
	latitude  float64
	longitude float64
}

var cities = []city{
	{"Sao Paulo", "SP", 11, -23.550520, -46.633308},
	{"Rio de Janeiro", "RJ", 21, -22.906847, -43.172896},
	{"Belo Horizonte", "MG", 31, -19.916681, -43.934493},
	{"Curitiba", "PR", 41, -25.428356, -49.273252},
	{"Porto Alegre", "RS", 51, -30.034647, -51.217658},
	{"Salvador", "BA", 71, -12.977749, -38.501629},
	{"Recife", "PE", 81, -8.047562, -34.877000},
	{"Fortaleza", "CE", 85, -3.731862, -38.526670},
	{"Brasilia", "DF", 61, -15.793889, -47.882778},
	{"Manaus", "AM", 92, -3.119027, -60.021731},
}

var orderStatuses = []string{"placed", "paid", "shipped", "in_transit", "delivered"}

var trackableStatuses = []string{"shipped", "in_transit"}

type Address struct {
	Street     string
	Number     string
	Complement string
	District   string
	City       string
	State      string
	ZipCode    string
	Country    string
}

func pick[Item any](list []Item) Item {
	return list[rand.IntN(len(list))]
}

func Name() string {
	return pick(firstNames) + " " + pick(lastNames)
}

func Email(name string) string {
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "."))
	return fmt.Sprintf("%s.%d@example.com", slug, rand.IntN(1_000_000_000))
}

func Phone() string {
	return fmt.Sprintf("+55 %d 9%04d-%04d", pick(cities).areaCode, rand.IntN(10000), rand.IntN(10000))
}

func NewAddress() Address {
	chosenCity := pick(cities)
	return Address{
		Street:     pick(streets),
		Number:     fmt.Sprintf("%d", 1+rand.IntN(4000)),
		Complement: Complement(),
		District:   District(),
		City:       chosenCity.name,
		State:      chosenCity.state,
		ZipCode:    ZipCode(),
		Country:    "BR",
	}
}

func Complement() string {
	return pick(complements)
}

func District() string {
	return pick(districts)
}

func ZipCode() string {
	return fmt.Sprintf("%05d-%03d", rand.IntN(100000), rand.IntN(1000))
}

func AddressLabel() string {
	return pick(addressLabels)
}

func OrderAmount() float64 {
	return math.Round((15+rand.Float64()*2000)*100) / 100
}

func PlacedAt() time.Time {
	return time.Now().Add(-time.Duration(rand.IntN(30*24)) * time.Hour)
}

func InitialStatus() string {
	return orderStatuses[0]
}

func FinalStatus() string {
	return orderStatuses[len(orderStatuses)-1]
}

func RandomStatus() string {
	return pick(orderStatuses)
}

func TrackableStatuses() []string {
	return trackableStatuses
}

func IsTrackable(status string) bool {
	for _, trackable := range trackableStatuses {
		if trackable == status {
			return true
		}
	}
	return false
}

func StatusAdvanceCase(column string) string {
	var statement strings.Builder
	statement.WriteString("CASE " + column)
	for position := 0; position+1 < len(orderStatuses); position++ {
		fmt.Fprintf(&statement, " WHEN '%s' THEN '%s'", orderStatuses[position], orderStatuses[position+1])
	}
	statement.WriteString(" ELSE " + column + " END")
	return statement.String()
}

func Coordinate() (latitude, longitude float64) {
	chosenCity := pick(cities)
	return round6(chosenCity.latitude + jitter(0.08)), round6(chosenCity.longitude + jitter(0.08))
}

func CoordinateStep() (latitude, longitude float64) {
	return round6(jitter(0.01)), round6(jitter(0.01))
}

func jitter(spread float64) float64 {
	return (rand.Float64()*2 - 1) * spread
}

func round6(value float64) float64 {
	return math.Round(value*1e6) / 1e6
}
