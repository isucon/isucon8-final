package randnameja

import "math/rand"

var (
	lastnameLen  int
	firstnameLen int
	Separator    string = " "
)

func init() {
	lastnameLen = len(lastnames)
	firstnameLen = len(firstnames)
}

func Generate() string {
	return lastnames[rand.Intn(lastnameLen)] + Separator + firstnames[rand.Intn(firstnameLen)]
}
