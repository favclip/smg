package c

// test for struct with inner struct

type Sample struct {
	A string
	B *Sub `search:",json"`
}

type Sub struct {
	C string
}
