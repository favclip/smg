package e

import (
	"strconv"
	"time"

	"github.com/mjibson/goon"
	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"
)

// +smg
type Inventory struct {
	ParentKey   *datastore.Key `json:"-" datastore:"-" goon:"parent" search:"-"`
	ID          int64          `json:"-" datastore:"-" goon:"id" json:",string" search:",id"` // search:"id" もサポートしたい
	ProductName string
	Description string    `search:",ngram"`
	Stock       int       `search:",rank"`
	Price       int       `search:",string"`
	Barcode     int64     `search:",string"`
	AdminNames  []string  `search:",json"`
	Shops       []*Shop   `search:",json"`
	CreatedAt   time.Time `datastore:",noindex"`
	UpdatedAt   time.Time `datastore:",noindex" search:"-"`
}

type Shop struct {
	Name    string
	Address string
}

func (doc *InventorySearch) DocID(c context.Context) (string, error) {
	g := goon.FromContext(c)
	id, err := strconv.ParseInt(doc.ID, 10, 0)
	if err != nil {
		return "", err
	}
	key, err := g.KeyError(&Inventory{ID: id})
	if err != nil {
		return "", err
	}

	return key.Encode(), nil
}
