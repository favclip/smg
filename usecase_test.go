package smg

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/favclip/smg/smgutils"
	"github.com/mjibson/goon"
	"golang.org/x/net/context"
	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/search"
)

/// user defined

type Inventory struct {
	ParentKey   *datastore.Key `json:"-" datastore:"-" goon:"parent" search:"-"`
	ID          int64          `json:"-" datastore:"-" goon:"id" json:",string" search:",id"`
	ProductName string
	Description string    `search:",ngram"`
	Stock       int       `search:",rank"`
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

/// code example

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
}

/// generated code

type InventorySearch struct {
	ID                 string
	ProductName        string
	Description        string
	DescriptionUnigram string
	DescriptionBigram  string
	Stock              float64
	AdminNames         string
	Shops              string
	CreatedAt          time.Time
}

func (doc *InventorySearch) Load(fields []search.Field, metadata *search.DocumentMetadata) error {
	return search.LoadStruct(doc, fields)
}

func (doc *InventorySearch) Save() ([]search.Field, *search.DocumentMetadata, error) {
	fields, err := search.SaveStruct(doc)
	if err != nil {
		return nil, nil, err
	}
	metadata := &search.DocumentMetadata{Rank: int(doc.Stock)}

	return fields, metadata, nil
}

func (doc *Inventory) Searchfy() (*InventorySearch, error) {
	if doc == nil {
		return nil, nil
	}
	dest := &InventorySearch{}

	var err error
	var b []byte

	dest.ID = strconv.FormatInt(doc.ID, 10)
	dest.ProductName = doc.ProductName
	dest.Description = doc.Description
	dest.DescriptionUnigram, err = smgutils.UnigramForSearch(doc.Description)
	if err != nil {
		return nil, err
	}
	dest.DescriptionBigram, err = smgutils.BigramForSearch(doc.Description)
	if err != nil {
		return nil, err
	}
	dest.Stock = float64(doc.Stock)
	b, err = json.Marshal(dest.AdminNames)
	if err != nil {
		return nil, err
	}
	if str := string(b); str != "" && str != `""` {
		dest.AdminNames = str
	}
	b, err = json.Marshal(dest.Shops)
	if err != nil {
		return nil, err
	}
	if str := string(b); str != "" && str != `""` {
		dest.Shops = str
	}
	dest.CreatedAt = doc.CreatedAt

	return dest, nil
}

func NewInventorySearch() *InventorySearchBuilder {
	var _ search.FieldLoadSaver = &InventorySearch{}

	op := &smgutils.Op{}
	b := &InventorySearchBuilder{
		rootOp:    op,
		currentOp: op,
		opts: &search.SearchOptions{
			Sort: &search.SortOptions{},
		},
	}
	b.ProductName = &InventorySearchStringPropertyInfo{"ProductName", b}
	b.Description = &InventorySearchNgramStringPropertyInfo{InventorySearchStringPropertyInfo{"Description", b}}
	b.Stock = &InventorySearchNumberPropertyInfo{"Stock", b}
	b.AdminNames = &InventorySearchStringPropertyInfo{"AdminNames", b}
	b.Shops = &InventorySearchStringPropertyInfo{"Shops", b}
	b.CreatedAt = &InventorySearchTimePropertyInfo{"CreatedAt", b}

	return b
}

var _ smgutils.SearchBuilder = &InventorySearchBuilder{}

type InventorySearchBuilder struct {
	rootOp      *smgutils.Op
	currentOp   *smgutils.Op // groupの扱い
	opts        *search.SearchOptions
	query       string
	index       *search.Index
	ProductName *InventorySearchStringPropertyInfo
	Description *InventorySearchNgramStringPropertyInfo
	Stock       *InventorySearchNumberPropertyInfo
	AdminNames  *InventorySearchStringPropertyInfo
	Shops       *InventorySearchStringPropertyInfo
	CreatedAt   *InventorySearchTimePropertyInfo
}

func (b *InventorySearchBuilder) IndexName() string {
	return "Inventory"
}

func (b *InventorySearchBuilder) QueryString() (string, error) {
	buffer := &bytes.Buffer{}
	err := b.rootOp.Query(buffer)
	if err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (b *InventorySearchBuilder) SearchOptions() *search.SearchOptions {
	return b.opts
}

func (b *InventorySearchBuilder) And() *InventorySearchBuilder {
	b.currentOp.Children = append(b.currentOp.Children, &smgutils.Op{Type: smgutils.And})
	return b
}

func (b *InventorySearchBuilder) Or() *InventorySearchBuilder {
	b.currentOp.Children = append(b.currentOp.Children, &smgutils.Op{Type: smgutils.Or})
	return b
}

func (b *InventorySearchBuilder) Group(p func()) *InventorySearchBuilder {
	b.StartGroup()
	p()
	b.EndGroup()
	return b
}

func (b *InventorySearchBuilder) StartGroup() *InventorySearchBuilder {
	op := &smgutils.Op{Type: smgutils.Group, Parent: b.currentOp}
	b.currentOp.Children = append(b.currentOp.Children, op)
	b.currentOp = op
	return b
}

func (b *InventorySearchBuilder) EndGroup() *InventorySearchBuilder {
	b.currentOp = b.currentOp.Parent
	return b
}

func (b *InventorySearchBuilder) Put(c context.Context, src *Inventory) (string, error) {
	doc, err := src.Searchfy()
	if err != nil {
		return "", err
	}
	return b.PutDocument(c, doc)
}

func (b *InventorySearchBuilder) PutDocument(c context.Context, src *InventorySearch) (string, error) {
	index, err := search.Open(b.IndexName())
	if err != nil {
		return "", err
	}

	docID := ""
	if v, ok := interface{}(src).(smgutils.DocIDer); ok { // TODO can I shorten this cond expression?
		docID, err = v.DocID(c)
		if err != nil {
			return "", err
		}
		src.ID = docID
	}

	log.Debugf(c, "id: %#v, payload: %#v", docID, src)

	docID, err = index.Put(c, docID, src)
	if err != nil {
		return "", err
	}

	src.ID = docID

	return docID, nil
}

func (b *InventorySearchBuilder) Opts() *InventorySearchOptions {
	return &InventorySearchOptions{b: b}
}

func (b *InventorySearchBuilder) Search(c context.Context) (*InventorySearchIterator, error) {
	index, err := search.Open(b.IndexName())
	if err != nil {
		return nil, err
	}
	b.index = index

	query, err := b.QueryString()
	if err != nil {
		return nil, err
	}
	b.query = query
	log.Debugf(c, "query: `%s`, opts: %#v", b.query, b.opts)
	iter := b.index.Search(c, b.query, b.opts)

	return &InventorySearchIterator{b, iter}, nil
}

type InventorySearchOptions struct {
	b *InventorySearchBuilder
}

func (b *InventorySearchOptions) Limit(value int) *InventorySearchOptions {
	b.b.opts.Limit = value
	return b
}

func (b *InventorySearchOptions) IDsOnly() *InventorySearchOptions {
	b.b.opts.IDsOnly = true
	return b
}

func (b *InventorySearchOptions) Cursor(cursor search.Cursor) *InventorySearchOptions {
	b.b.opts.Cursor = cursor
	return b
}

func (b *InventorySearchOptions) Offset(value int) *InventorySearchOptions {
	b.b.opts.Offset = value
	return b
}

type InventorySearchIterator struct {
	b    *InventorySearchBuilder
	iter *search.Iterator
}

func (b *InventorySearchIterator) Next(c context.Context) (string, *InventorySearch, error) {
	var s *InventorySearch
	if b.b.opts.IDsOnly == false {
		s = &InventorySearch{}
	}

	docID, err := b.iter.Next(s)
	if err != nil {
		return "", nil, err
	}
	if s != nil {
		s.ID = docID
	}
	return docID, s, err
}

func (b *InventorySearchIterator) Cursor() search.Cursor {
	return b.iter.Cursor()
}

type InventorySearchStringPropertyInfo struct {
	Name string
	b    *InventorySearchBuilder
}

func (p *InventorySearchStringPropertyInfo) Match(value string) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Match, Value: value})
	return p.b
}

type InventorySearchNgramStringPropertyInfo struct {
	InventorySearchStringPropertyInfo
}

func (p *InventorySearchNgramStringPropertyInfo) NgramMatch(value string) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.NgramMatch, Value: value})
	return p.b
}

type InventorySearchNumberPropertyInfo struct {
	Name string
	b    *InventorySearchBuilder
}

func (p *InventorySearchNumberPropertyInfo) IntGreaterThanOrEqual(value int) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.GtEq, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) IntGreaterThan(value int) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Gt, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) IntLessThanOrEqual(value int) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.LtEq, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) IntLessThan(value int) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Lt, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) IntEqual(value int) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Eq, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) Int64GreaterThanOrEqual(value int64) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.GtEq, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) Int64GreaterThan(value int64) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Gt, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) Int64LessThanOrEqual(value int64) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.LtEq, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) Int64LessThan(value int64) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Lt, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) Int64Equal(value int64) *InventorySearchBuilder {
	p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Eq, Value: value})
	return p.b
}

func (p *InventorySearchNumberPropertyInfo) Asc() *InventorySearchBuilder {
	p.b.opts.Sort.Expressions = append(p.b.opts.Sort.Expressions, search.SortExpression{
		Expr:    p.Name,
		Reverse: true,
	})

	return p.b
}

func (p *InventorySearchNumberPropertyInfo) Desc() *InventorySearchBuilder {
	p.b.opts.Sort.Expressions = append(p.b.opts.Sort.Expressions, search.SortExpression{
		Expr:    p.Name,
		Reverse: false,
	})

	return p.b
}

type InventorySearchTimePropertyInfo struct {
	Name string
	b    *InventorySearchBuilder
}
