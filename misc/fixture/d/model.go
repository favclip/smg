//go:generate smg -output model_search.go .

package d

// test for struct with tagged comment

// +smg
type Sample struct {
	A string
	B string
}
