package smg

import (
	"testing"

	"github.com/favclip/smg/misc/fixture/a"
	"github.com/favclip/smg/misc/fixture/e"
	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/search"
)

func TestBasicUsage1(t *testing.T) {
	c, closer, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer closer()

	src := &a.Sample{"Foo!"}
	index := a.NewSampleSearch()
	_, err = index.Put(c, src)
	if err != nil {
		t.Fatal(err)
	}

	index.Foo.Match("Foo")
	index.Opts().Limit(3)
	iter, err := index.Search(c)
	if err != nil {
		t.Fatal(err)
	}

	for {
		_, doc, err := iter.Next(c)
		if err == search.Done {
			break
		}
		t.Logf("%#v", doc)
	}
}

func TestBasicUsage2(t *testing.T) {
	c, closer, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer closer()

	src := &e.Inventory{
		ProductName: "go-chan",
		Description: "Hi, go-chan!",
		Stock:       3,
		Price:       1050,
	}
	key, err := datastore.Put(c, datastore.NewIncompleteKey(c, "Inventory", nil), src)
	if err != nil {
		t.Fatal(err)
	}
	src.ID = key.IntID()

	index := e.NewInventorySearch()
	_, err = index.Put(c, src)
	if err != nil {
		t.Fatal(err)
	}
	index.Opts().IDsOnly()

	iter, err := index.Search(c)
	if err != nil {
		t.Fatal(err)
	}

	for {
		docID, _, err := iter.Next(c)
		if err == search.Done {
			break
		}

		key, err := datastore.DecodeKey(docID)
		if err != nil {
			t.Fatal(err)
		}

		src := &e.Inventory{}
		err = datastore.Get(c, key, src)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("%#v", src)
	}
}

func TestBasicUsage3(t *testing.T) {
	c, closer, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer closer()

	src := &e.Inventory{
		ProductName: "go-chan",
		Description: "Hi, go-chan!",
		Stock:       3,
		Price:       1050,
	}

	index := e.NewInventorySearch()
	_, err = index.Put(c, src)
	if err != nil {
		t.Fatal(err)
	}

	index.Group(func() {
		index.ProductName.Match("go-chan").Or().Description.NgramMatch("go-chan")
	}).Or()
	index.Stock.IntLessThan(5)

	iter, err := index.Search(c)
	if err != nil {
		t.Fatal(err)
	}

	for {
		_, doc, err := iter.Next(c)
		if err == search.Done {
			break
		}

		t.Logf("%#v", doc)
	}
}

func TestBasicUsage4(t *testing.T) {
	c, closer, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer closer()

	src := &e.Inventory{
		ProductName: "go-chan",
		Description: "Hi, go-chan!",
		Stock:       3,
		Price:       1050,
	}

	index := e.NewInventorySearch()
	_, err = index.Put(c, src)
	if err != nil {
		t.Fatal(err)
	}

	index.Group(func() {
		index.ProductName.Match("go-chan").Or().Description.NgramMatch("go-chan")
	}).Or()
	index.Stock.IntLessThan(5)

	iter, err := index.Search(c)
	if err != nil {
		t.Fatal(err)
	}

	for {
		_, doc, err := iter.Next(c)
		if err == search.Done {
			break
		}

		t.Logf("%#v, %#v", doc, iter.Cursor())
	}
}
