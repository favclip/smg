package smgutils

import (
	"bytes"
	"testing"
	"time"
)

type UserSearch struct {
	ID          string
	Name        string
	NameUnigram string
	NameBigram  string
	Age         float64
	CreatedAt   time.Time
}

func TestOpQueryMatch(t *testing.T) {
	op := &Op{FieldName: "N", Value: "name", Type: Match}
	bf := &bytes.Buffer{}
	err := op.Query(bf)
	if err != nil {
		t.Fatal(err)
	}

	query := bf.String()
	if query != ` N: "name" ` {
		t.Errorf("unexpected: %s", query)
	}
}

func TestOpQueryNgramMatch(t *testing.T) {
	op := &Op{FieldName: "N", Value: "name", Type: NgramMatch}
	bf := &bytes.Buffer{}
	err := op.Query(bf)
	if err != nil {
		t.Fatal(err)
	}

	query := bf.String()
	if query != `(NBigram: "na" AND NBigram: "am" AND NBigram: "me")` {
		t.Errorf("unexpected: %s", query)
	}
}

func TestOpQueryAnd(t *testing.T) {
	op := &Op{Type: And}
	bf := &bytes.Buffer{}
	err := op.Query(bf)
	if err != nil {
		t.Fatal(err)
	}

	query := bf.String()
	if query != ` AND ` {
		t.Errorf("unexpected: %s", query)
	}
}

func TestOpQueryOr(t *testing.T) {
	op := &Op{Type: Or}
	bf := &bytes.Buffer{}
	err := op.Query(bf)
	if err != nil {
		t.Fatal(err)
	}

	query := bf.String()
	if query != ` OR ` {
		t.Errorf("unexpected: %s", query)
	}
}

func TestOpQueryComparisonInt(t *testing.T) {
	op := &Op{FieldName: "N", Value: 3, Type: Gt}
	bf := &bytes.Buffer{}
	err := op.Query(bf)
	if err != nil {
		t.Fatal(err)
	}

	query := bf.String()
	if query != ` N > 3 ` {
		t.Errorf("unexpected: %s", query)
	}
}

func TestOpQueryComparisonInt64(t *testing.T) {
	op := &Op{FieldName: "N", Value: int64(3), Type: Lt}
	bf := &bytes.Buffer{}
	err := op.Query(bf)
	if err != nil {
		t.Fatal(err)
	}

	query := bf.String()
	if query != ` N < 3 ` {
		t.Errorf("unexpected: %s", query)
	}
}

func TestOpQueryComparisonGroup(t *testing.T) {
	op := &Op{Type: Group, Children: []*Op{&Op{FieldName: "N", Value: "name", Type: Match}}}
	bf := &bytes.Buffer{}
	err := op.Query(bf)
	if err != nil {
		t.Fatal(err)
	}

	query := bf.String()
	if query != `( N: "name" )` {
		t.Errorf("unexpected: %s", query)
	}
}

func TestUnigram(t *testing.T) {
	ss := Unigram("test")

	if v := len(ss); v != 4 {
		t.Fatalf("unexpected: %d", v)
	}

	if ss[0] != "t" || ss[1] != "e" || ss[2] != "s" || ss[3] != "t" {
		t.Fatalf("unexpected: %#v", ss)
	}
}

func TestBigram(t *testing.T) {
	ss := Bigram("test")

	if v := len(ss); v != 3 {
		t.Fatalf("unexpected: %d", v)
	}

	if ss[0] != "te" || ss[1] != "es" || ss[2] != "st" {
		t.Fatalf("unexpected: %#v", ss)
	}
}

func TestUnigramForSearch(t *testing.T) {
	s, err := UnigramForSearch("test")
	if err != nil {
		t.Fatal(err)
	}
	if s != `{"Value":["t","e","s","t"]}` {
		t.Errorf("unexpected: %s", s)
	}
}

func TestBigramForSearch(t *testing.T) {
	s, err := BigramForSearch("test")
	if err != nil {
		t.Fatal(err)
	}
	if s != `{"Value":["te","es","st"]}` {
		t.Errorf("unexpected: %s", s)
	}
}

func TestSanitize(t *testing.T) {
	s := Sanitize(`例えば"や\が紛れていても`)
	if s != `例えば\"や\\が紛れていても` {
		t.Errorf("unexpected: %s", s)
	}
}

func TestUnix(t *testing.T) {
	utc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf(err.Error())
	}
	unixtime := Unix(time.Date(1970, 1, 1, 0, 0, 0, 0, utc))
	if unixtime != 0 {
		t.Errorf("unexpected: %d", unixtime)
	}
}

func TestUnixMax(t *testing.T) {
	utc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Errorf(err.Error())
	}
	unixtime := Unix(time.Date(2050, 1, 1, 0, 0, 0, 0, utc))
	if unixtime != 2147483647 {
		t.Errorf("unexpected: %d", unixtime)
	}
}

func TestUnixMin(t *testing.T) {
	unixtime := Unix(time.Time{})
	if unixtime != -2147483647 {
		t.Errorf("unexpected: %d", unixtime)
	}
}
