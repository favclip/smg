package f

import (
	"testing"

	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/search"
)

func TestUsecaseSample(t *testing.T) {
	c, closer, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err.Error())
	}
	defer closer()

	sample := &Sample{
		A: []int64{1, 2, 3},
	}

	index := NewSampleSearch()
	_, err = index.Put(c, sample)
	if err != nil {
		t.Fatal(err.Error())
	}

	index = NewSampleSearch()
	index.A.Match("3")
	iter, err := index.Search(c)
	if err != nil {
		t.Fatal(err)
	}
	for {
		docID, s, err := iter.Next(c)
		if err == search.Done {
			break
		}
		if err != nil {
			t.Fatal(err.Error())
		}
		t.Logf("%s %#v", docID, s)
	}
}
