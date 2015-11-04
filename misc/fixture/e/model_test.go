package e

import (
	"testing"
	"time"

	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/search"
)

func TestUsecaseSample(t *testing.T) {
	c, closer, err := aetest.NewContext()
	if err != nil {
		t.Fatal(err.Error())
	}
	defer closer()

	func() {
		var inv *Inventory
		index := NewInventorySearch()

		inv = &Inventory{
			ID:          111,
			ProductName: "カップヌードル",
			Description: "お湯を入れたらすぐヌードル！",
			Stock:       3,
			Price:       150,
			Barcode:     4902425613642,
			AdminNames:  []string{"Mr.A"},
			Shops:       []*Shop{&Shop{Name: "0-24", Address: "Hongo street"}},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		_, err = index.Put(c, inv)
		if err != nil {
			t.Fatal(err)
		}

		inv = &Inventory{
			ID:          222,
			ProductName: "おせんべい",
			Description: "囓ってパリッ！",
			Stock:       10,
			Price:       100,
			Barcode:     4902425613643,
			AdminNames:  []string{"Mr.B"},
			Shops:       []*Shop{&Shop{Name: "HighSon", Address: "Hongo Bldg"}},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		_, err = index.Put(c, inv)
		if err != nil {
			t.Fatal(err)
		}
	}()

	func() {
		t.Logf("#1")

		index := NewInventorySearch()
		// (ProductName: カップヌードル AND Shop: Hongo AND Stock >= 1) OR (ProductName: カップヌードル AND Stock > 10) OR (ProductNameBigram: カッ AND ProductNameBigram: ップ AND ProductNameBigram: プヌ AND ProductNameBigram: ヌー AND ProductNameBigram: ード AND ProductNameBigram: ドル)
		index.Group(func() {
			index.ProductName.Match("カップヌードル").And().Shops.Match("Hongo").And().Stock.IntGreaterThanOrEqual(1)
		})
		index.Or().StartGroup().ProductName.Match("カップヌードル").And().Stock.IntGreaterThan(10).EndGroup()
		index.Or().Description.NgramMatch("お湯を")

		index.Stock.Desc()
		index.Opts().Limit(10)

		iter, err := index.Search(c)
		if err != nil {
			t.Fatal(err)
		}
		for {
			docID, s, err := iter.Next(c)
			if err == search.Done {
				t.Log("#1 done!")
				break
			}
			if err != nil {
				t.Fatal(err.Error())
			}
			t.Logf("%s %#v", docID, s)
		}
	}()

	func() {
		t.Logf("#2")

		index := NewInventorySearch()
		index.Description.NgramMatch("囓って")

		index.Stock.Desc()
		index.Opts().Limit(10)

		iter, err := index.Search(c)
		if err != nil {
			t.Log("#2 done!")
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
	}()

	func() {
		t.Logf("#3")

		index := NewInventorySearch()
		index.Price.Match("150")
		index.Barcode.Match("4902425613642")

		index.Stock.Desc()
		index.Opts().Limit(10)

		iter, err := index.Search(c)
		if err != nil {
			t.Log("#3 done!")
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
	}()
}
