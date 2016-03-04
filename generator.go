package smg

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/favclip/genbase"
)

// BuildSource represents source code of assembling..
type BuildSource struct {
	g         *genbase.Generator
	pkg       *genbase.PackageInfo
	typeInfos genbase.TypeInfos

	Structs []*BuildStruct
}

// BuildStruct represents struct of assembling..
type BuildStruct struct {
	parent   *BuildSource
	typeInfo *genbase.TypeInfo

	Fields []*BuildField
}

// BuildField represents field of BuildStruct.
type BuildField struct {
	parent    *BuildStruct
	fieldInfo *genbase.FieldInfo

	Name string
	Tag  *BuildTag
}

// BuildTag represents tag of BuildField.
type BuildTag struct {
	field *BuildField

	Name   string
	ID     bool // e.g. DocID string `search:",id"`
	Ignore bool // e.g. Secret string `search:"-"`
	Ngram  bool // e.g. Description string `search:",ngram"`
	JSON   bool // e.g. Store []*Store `search:",json"`
	Rank   bool // e.g. Stock int `search:",rank"`
	String bool // e.g. Int64String int64 `search:",string"`
}

// Parse construct *BuildSource from package & type information.
func Parse(pkg *genbase.PackageInfo, typeInfos genbase.TypeInfos) (*BuildSource, error) {
	bu := &BuildSource{
		g:         genbase.NewGenerator(pkg),
		pkg:       pkg,
		typeInfos: typeInfos,
	}

	bu.g.AddImport("golang.org/x/net/context", "")
	bu.g.AddImport("google.golang.org/appengine/search", "")
	bu.g.AddImport("google.golang.org/appengine/log", "")
	bu.g.AddImport("bytes", "")
	bu.g.AddImport("github.com/favclip/smg/smgutils", "")
	bu.g.AddImport("errors", "")
	bu.g.AddImport("time", "")

	for _, typeInfo := range typeInfos {
		err := bu.parseStruct(typeInfo)
		if err != nil {
			return nil, err
		}
	}

	for _, st := range bu.Structs {
		if st.HasJSON() {
			bu.g.AddImport("encoding/json", "")
		}
		if id := st.ID(); id != nil && (id.fieldInfo.IsInt() || id.fieldInfo.IsInt64()) {
			bu.g.AddImport("strconv", "")
		}
		if st.HasString() {
			bu.g.AddImport("strconv", "")
		}
		for _, field := range st.Fields {
			if field.fieldInfo.IsTime() {
				bu.g.AddImport("time", "")
			}
		}
	}

	return bu, nil
}

func (b *BuildSource) parseStruct(typeInfo *genbase.TypeInfo) error {
	structType, err := typeInfo.StructType()
	if err != nil {
		return err
	}

	st := &BuildStruct{
		parent:   b,
		typeInfo: typeInfo,
	}

	for _, fieldInfo := range structType.FieldInfos() {
		if len := len(fieldInfo.Names); len == 0 {
			// embedded struct in outer struct or multiply field declarations
			// https://play.golang.org/p/bcxbdiMyP4
			continue
		}

		for _, nameIdent := range fieldInfo.Names {
			err := b.parseField(st, typeInfo, fieldInfo, nameIdent.Name)
			if err != nil {
				return err
			}
		}
	}

	b.Structs = append(b.Structs, st)

	return nil
}

func (b *BuildSource) parseField(st *BuildStruct, typeInfo *genbase.TypeInfo, fieldInfo *genbase.FieldInfo, name string) error {
	field := &BuildField{
		parent:    st,
		fieldInfo: fieldInfo,
		Name:      name,
	}
	st.Fields = append(st.Fields, field)

	tag := &BuildTag{
		field: field,
		Name:  name,
	}
	field.Tag = tag

	if fieldInfo.Tag != nil {
		// remove back quote
		tagBody := fieldInfo.Tag.Value[1 : len(fieldInfo.Tag.Value)-1]
		structTag := reflect.StructTag(tagBody)

		searchTag := structTag.Get("search")
		if searchTag == "-" {
			tag.Ignore = true
		} else if idx := strings.Index(searchTag, ","); idx == -1 {
			// nothing to do
		} else {
			for idx != -1 || searchTag != "" {
				value := searchTag
				if idx != -1 {
					value = searchTag[:idx]
					searchTag = searchTag[idx+1:]
				} else {
					searchTag = searchTag[len(value):]
				}
				idx = strings.Index(searchTag, ",")

				switch value {
				case "id":
					tag.ID = true
				case "ngram":
					tag.Ngram = true
				case "json":
					tag.JSON = true
				case "rank":
					tag.Rank = true
				case "string":
					tag.String = true
				}
			}
		}
	}

	return nil
}

// Emit generate wrapper code.
func (b *BuildSource) Emit(args *[]string) ([]byte, error) {
	b.g.PrintHeader("smg", args)

	for _, st := range b.Structs {
		err := st.emit(b.g)
		if err != nil {
			return nil, err
		}
	}

	return b.g.Format()
}

func (st *BuildStruct) emit(g *genbase.Generator) error {
	g.Printf("// %[1]sSearch best match Search API wrapper for %[1]s.\n", st.Name())

	// generate FooJson struct from Foo struct
	g.Printf("type %sSearch struct {\n", st.Name())
	g.Printf("src *%s\n\n", st.Name())
	for _, field := range st.Fields {
		if field.Tag.Ignore {
			continue
		}
		if field.Tag.ID {
			g.Printf("%[1]s string\n", field.Name)
		} else if field.Tag.Ngram {
			g.Printf("%[1]s string\n", field.Name, st.Name())
			g.Printf("%[1]sUnigram string\n", field.Name, st.Name())
			g.Printf("%[1]sBigram string\n", field.Name, st.Name())
		} else if field.Tag.String {
			g.Printf("%[1]s string\n", field.Name, st.Name())
		} else if field.fieldInfo.IsNumber() {
			g.Printf("%[1]s float64\n", field.Name, st.Name())
		} else if field.fieldInfo.IsBool() {
			g.Printf("%[1]s float64 // 1(true) or 0(false)\n", field.Name, st.Name())
		} else if field.fieldInfo.IsTime() {
			g.Printf("%[1]s time.Time\n", field.Name, st.Name())
		} else if field.fieldInfo.IsString() || field.Tag.JSON {
			g.Printf("%[1]s string\n", field.Name, st.Name())
		} else {
			return fmt.Errorf("%s: unknown field type in %s", field.Name, st.Name())
		}
	}
	g.Printf("}\n\n")

	// implement DocumentMetadata
	rank := st.Rank()
	if rank != nil {
		g.Printf(`
				// Load by search.LoadStruct.
				func (s *%[1]sSearch) Load(fields []search.Field, metadata *search.DocumentMetadata) error {
					return search.LoadStruct(s, fields)
				}

				// Save with search.DocumentMetadata#Rank.
				func (s *%[1]sSearch) Save() ([]search.Field, *search.DocumentMetadata, error) {
					fields, err := search.SaveStruct(s)
					if err != nil {
						return nil, nil, err
					}
					// https://cloud.google.com/appengine/docs/go/search/reference#DocumentMetadata
					// 0 origin value can not be correctly sorted.
					// Typically, the data is assumed to be 0's origin, 1 added every time.
					metadata := &search.DocumentMetadata{Rank: int(s.%[2]s) + 1}

					return fields, metadata, nil
				}
			`, st.Name(), rank.Name)
	}

	// implement Searchfy method
	g.Printf(`
			// Searchfy converts *%[1]s to *%[1]sSearch.
			func (src *%[1]s) Searchfy() (*%[1]sSearch, error) {
				if src == nil {
					return nil, nil
				}
				dest := &%[1]sSearch{}
				dest.src = src
		`, st.Name())

	if st.HasJSON() {
		g.Printf("var err error\n")
		g.Printf("var b []byte\n")
	} else if st.HasNgram() {
		g.Printf("var err error\n")
	}

	for _, field := range st.Fields {
		if field.Tag.Ignore {
			continue
		}
		if field.Tag.ID || field.Tag.String {
			if field.fieldInfo.IsInt64() {
				g.Printf("dest.%[1]s = strconv.FormatInt(src.%[1]s, 10)\n", field.Name)
			} else if field.fieldInfo.IsInt() {
				g.Printf("dest.%[1]s = strconv.Itoa(src.%[1]s)\n", field.Name)
			} else if field.fieldInfo.IsString() {
				g.Printf("dest.%[1]s = src.%[1]s\n", field.Name)
			} else {
				// TODO support float32, float64
				return fmt.Errorf("%s: id field should be int64, int, or string", field.Name)
			}

		} else if field.Tag.Ngram {
			if field.fieldInfo.IsString() {
				g.Printf(`
						dest.%[1]s = src.%[1]s
						dest.%[1]sUnigram, err = smgutils.UnigramForSearch(src.%[1]s)
						if err != nil {
							return nil, err
						}
						dest.%[1]sBigram, err = smgutils.BigramForSearch(src.%[1]s)
						if err != nil {
							return nil, err
						}
					`, field.Name)
			} else {
				return fmt.Errorf("%s: ngram field should be string", field.Name)
			}
		} else if field.Tag.JSON {
			g.Printf(`
					b, err = json.Marshal(src.%[1]s)
					if err != nil {
						return nil, err
					}
					if str := string(b); str != "" && str != "\"\"" {
						dest.%[1]s = str
					}
				`, field.Name)
		} else {
			// TODO implement invalid type detection
			if field.fieldInfo.IsInt64() || field.fieldInfo.IsInt() || field.fieldInfo.IsFloat32() {
				g.Printf("dest.%[1]s = float64(src.%[1]s)\n", field.Name)
			} else if field.fieldInfo.IsBool() {
				g.Printf(`if src.%[1]s {
							dest.%[1]s = 1
						} else {
							dest.%[1]s = 0
						}
					`, field.Name)
			} else if field.fieldInfo.IsString() || field.fieldInfo.IsFloat64() || field.fieldInfo.IsTime() {
				g.Printf("dest.%[1]s = src.%[1]s\n", field.Name)
			} else {
				return fmt.Errorf("%s: unknown field type in %s", field.Name, st.Name())
			}
		}
	}
	g.Printf("return dest, nil\n}\n\n")

	// implement New*Search method
	g.Printf(`
			// New%[1]sSearch create new *%[1]sSearchBuilder.
			func New%[1]sSearch() *%[1]sSearchBuilder {
				op := &smgutils.Op{}
				b := &%[1]sSearchBuilder{
					rootOp:    op,
					currentOp: op,
				}
		`, st.Name())

	for _, field := range st.Fields {
		if field.Tag.ID || field.Tag.Ignore {
			continue
		} else if field.Tag.Ngram {
			g.Printf("b.%[1]s = &%[2]sSearchNgramStringPropertyInfo{%[2]sSearchStringPropertyInfo{\"%[1]s\", b}}\n", field.Name, st.Name())
		} else if field.fieldInfo.IsString() || field.Tag.JSON || field.Tag.String {
			g.Printf("b.%[1]s = &%[2]sSearchStringPropertyInfo{\"%[1]s\", b}\n", field.Name, st.Name())
		} else if field.fieldInfo.IsNumber() || field.fieldInfo.IsBool() {
			// special support to bool
			g.Printf("b.%[1]s = &%[2]sSearchNumberPropertyInfo{\"%[1]s\", b}\n", field.Name, st.Name())
		} else if field.fieldInfo.IsTime() {
			g.Printf("b.%[1]s = &%[2]sSearchTimePropertyInfo{\"%[1]s\", b}\n", field.Name, st.Name())
		} else {
			return fmt.Errorf("%s: unknown field type in %s", field.Name, st.Name())
		}
	}
	g.Printf(`
				return b
			}
		`)

	// implement *SearchBuilder struct
	g.Printf(`
			// %[1]sSearchBuilder builds Search API query.
			type %[1]sSearchBuilder struct {
				rootOp      *smgutils.Op
				currentOp   *smgutils.Op // for grouping
				opts        *search.SearchOptions
				query       string
				index       *search.Index
		`, st.Name())
	for _, field := range st.Fields {
		if field.Tag.ID || field.Tag.Ignore {
			continue
		} else if field.Tag.Ngram {
			g.Printf("%[1]s *%[2]sSearchNgramStringPropertyInfo\n", field.Name, st.Name())
		} else if field.fieldInfo.IsString() || field.Tag.JSON || field.Tag.String {
			g.Printf("%[1]s *%[2]sSearchStringPropertyInfo\n", field.Name, st.Name())
		} else if field.fieldInfo.IsNumber() || field.fieldInfo.IsBool() {
			// TODO special support to bool
			g.Printf("%[1]s *%[2]sSearchNumberPropertyInfo\n", field.Name, st.Name())
		} else if field.fieldInfo.IsTime() {
			g.Printf("%[1]s *%[2]sSearchTimePropertyInfo\n", field.Name, st.Name())
		} else {
			return fmt.Errorf("%s: unknown field type in %s", field.Name, st.Name())
		}
	}
	g.Printf("}\n\n")

	// implement SearchBuilder methods and others
	g.Printf(`
			// And append new operant to query.
			func (b *%[1]sSearchBuilder) And() *%[1]sSearchBuilder {
				b.currentOp.Children = append(b.currentOp.Children, &smgutils.Op{Type: smgutils.And})
				return b
			}

			// Or append new operant to query.
			func (b *%[1]sSearchBuilder) Or() *%[1]sSearchBuilder {
				b.currentOp.Children = append(b.currentOp.Children, &smgutils.Op{Type: smgutils.Or})
				return b
			}

			// Group append new operant to query.
			func (b *%[1]sSearchBuilder) Group(p func()) *%[1]sSearchBuilder {
				b.StartGroup()
				p()
				b.EndGroup()
				return b
			}

			// StartGroup append new operant to query.
			func (b *%[1]sSearchBuilder) StartGroup() *%[1]sSearchBuilder {
				op := &smgutils.Op{Type: smgutils.Group, Parent: b.currentOp}
				b.currentOp.Children = append(b.currentOp.Children, op)
				b.currentOp = op
				return b
			}

			// EndGroup append new operant to query.
			func (b *%[1]sSearchBuilder) EndGroup() *%[1]sSearchBuilder {
				b.currentOp = b.currentOp.Parent
				return b
			}

			// Put document to Index.
			func (b *%[1]sSearchBuilder) Put(c context.Context, src *%[1]s) (string, error) {
				doc, err := src.Searchfy()
				if err != nil {
					return "", err
				}
				return b.PutDocument(c, doc)
			}

			// PutDocument to Index.
			func (b *%[1]sSearchBuilder) PutDocument(c context.Context, src *%[1]sSearch) (string, error) {
				index, err := search.Open("%[1]s")
				if err != nil {
					return "", err
				}

				docID := ""
				if v, ok := interface{}(src).(smgutils.DocIDer); ok { // TODO can I shorten this cond expression?
					docID, err = v.DocID(c)
					if err != nil {
						return "", err
					}
			`, st.Name(), "%")
	if st.HasID() {
		g.Printf("src.ID = docID")
	}
	g.Printf(`
				}

				log.Debugf(c, "id: %[2]s#v, payload: %[2]s#v", docID, src)

				docID, err = index.Put(c, docID, src)
				if err != nil {
					return "", err
				}

			`, st.Name(), "%")
	if st.HasID() {
		g.Printf("src.ID = docID")
	}
	g.Printf(`

				return docID, nil
			}

			// Delete document from Index.
			func (b *%[1]sSearchBuilder) Delete(c context.Context, src *%[1]s) error {
				doc, err := src.Searchfy()
				if err != nil {
					return err
				}
				return b.DeleteDocument(c, doc)
			}

			// DeleteDocument from Index.
			func (b *%[1]sSearchBuilder) DeleteDocument(c context.Context, src *%[1]sSearch) error {
				if v, ok := interface{}(src).(smgutils.DocIDer); ok { // TODO can I shorten this cond expression?
					docID, err := v.DocID(c)
					if err != nil {
						return err
					}
					return b.DeleteByDocID(c, docID)
				}

				return errors.New("src is not implemented DocIDer interface")
			}

			// DeleteByDocID from Index.
			func (b *%[1]sSearchBuilder) DeleteByDocID(c context.Context, docID string) error {
				index, err := search.Open("%[1]s")
				if err != nil {
					return err
				}

				return index.Delete(c, docID)
			}

			// Opts returns *%[1]sSearchOptions.
			func (b *%[1]sSearchBuilder) Opts() *%[1]sSearchOptions {
				return &%[1]sSearchOptions{b: b}
			}

			// Search returns *%[1]sSearchIterator, It is result from Index.
			func (b *%[1]sSearchBuilder) Search(c context.Context) (*%[1]sSearchIterator, error) {
				index, err := search.Open("%[1]s")
				if err != nil {
					return nil, err
				}
				b.index = index

				buffer := &bytes.Buffer{}
				err = b.rootOp.Query(buffer)
				if err != nil {
					return nil, err
				}
				b.query = buffer.String()
				log.Debugf(c, "query: '%[2]ss', opts: %[2]s#v", b.query, b.opts)
				iter := b.index.Search(c, b.query, b.opts)

				return &%[1]sSearchIterator{b, iter}, nil
			}

			// %[1]sSearchOptions construct *search.SearchOptions.
			type %[1]sSearchOptions struct {
				b *%[1]sSearchBuilder
			}

			// Limit setup opts.
			func (b *%[1]sSearchOptions) Limit(value int) *%[1]sSearchOptions {
				if b.b.opts == nil {
					b.b.opts = &search.SearchOptions{}
				}
				b.b.opts.Limit = value
				return b
			}

			// IDsOnly setup opts.
			func (b *%[1]sSearchOptions) IDsOnly() *%[1]sSearchOptions {
				if b.b.opts == nil {
					b.b.opts = &search.SearchOptions{}
				}
				b.b.opts.IDsOnly = true
				return b
			}

			// Cursor setup opts.
			func (b *%[1]sSearchOptions) Cursor(cursor search.Cursor) *%[1]sSearchOptions {
				b.b.opts.Cursor = cursor
				return b
			}

			// Offset setup opts.
			func (b *%[1]sSearchOptions) Offset(value int) *%[1]sSearchOptions {
				b.b.opts.Offset = value
				return b
			}

			// %[1]sSearchIterator can access to search result.
			type %[1]sSearchIterator struct {
				b    *%[1]sSearchBuilder
				iter *search.Iterator
			}
			// Next returns next document from iter.
			func (b *%[1]sSearchIterator) Next(c context.Context) (string, *%[1]sSearch, error) {
				var s *%[1]sSearch
				if b.b.opts == nil || b.b.opts.IDsOnly != true {
					s = &%[1]sSearch{}
				}

				docID, err := b.iter.Next(s)
				if err != nil {
					return "", nil, err
				}
							`, st.Name(), "%")
	if st.HasID() {
		g.Printf("if s != nil {\n")
		g.Printf("s.ID = docID\n")
		g.Printf("}\n")
	}
	g.Printf(`
				return docID, s, err
			}

			// Cursor returns cursor of search.
			func (b *%[1]sSearchIterator) Cursor() search.Cursor {
				return b.iter.Cursor()
			}

			// %[1]sSearchStringPropertyInfo hold property info.
			type %[1]sSearchStringPropertyInfo struct {
				Name string
				b    *%[1]sSearchBuilder
			}

			// Match add query operand.
			func (p *%[1]sSearchStringPropertyInfo) Match(value string) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Match, Value: value})
				return p.b
			}

			// Asc add query operand.
			func (p *%[1]sSearchStringPropertyInfo) Asc() *%[1]sSearchBuilder {
				if p.b.opts == nil {
					p.b.opts = &search.SearchOptions{}
				}
				if p.b.opts.Sort == nil {
					p.b.opts.Sort = &search.SortOptions{}
				}
				p.b.opts.Sort.Expressions = append(p.b.opts.Sort.Expressions, search.SortExpression{
					Expr:    p.Name,
					Reverse: true,
				})

				return p.b
			}

			// Desc add query operand.
			func (p *%[1]sSearchStringPropertyInfo) Desc() *%[1]sSearchBuilder {
				if p.b.opts == nil {
					p.b.opts = &search.SearchOptions{}
				}
				if p.b.opts.Sort == nil {
					p.b.opts.Sort = &search.SortOptions{}
				}
				p.b.opts.Sort.Expressions = append(p.b.opts.Sort.Expressions, search.SortExpression{
					Expr:    p.Name,
					Reverse: false,
				})

				return p.b
			}

			// %[1]sSearchNgramStringPropertyInfo hold property info.
			type %[1]sSearchNgramStringPropertyInfo struct {
				%[1]sSearchStringPropertyInfo
			}

			// NgramMatch add query operand.
			func (p *%[1]sSearchNgramStringPropertyInfo) NgramMatch(value string) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.NgramMatch, Value: value})
				return p.b
			}

			// %[1]sSearchNumberPropertyInfo hold property info.
			type %[1]sSearchNumberPropertyInfo struct {
				Name string
				b    *%[1]sSearchBuilder
			}

			// IntGreaterThanOrEqual add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) IntGreaterThanOrEqual(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.GtEq, Value: value})
				return p.b
			}

			// IntGreaterThan add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) IntGreaterThan(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Gt, Value: value})
				return p.b
			}

			// IntLessThanOrEqual add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) IntLessThanOrEqual(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.LtEq, Value: value})
				return p.b
			}

			// IntLessThan add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) IntLessThan(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Lt, Value: value})
				return p.b
			}

			// IntEqual add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) IntEqual(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Eq, Value: value})
				return p.b
			}

			// Int64GreaterThanOrEqual add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) Int64GreaterThanOrEqual(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.GtEq, Value: value})
				return p.b
			}

			// Int64GreaterThan add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) Int64GreaterThan(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Gt, Value: value})
				return p.b
			}

			// Int64LessThanOrEqual add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) Int64LessThanOrEqual(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.LtEq, Value: value})
				return p.b
			}

			// Int64LessThan add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) Int64LessThan(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Lt, Value: value})
				return p.b
			}

			// Int64Equal add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) Int64Equal(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Eq, Value: value})
				return p.b
			}

			// Asc add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) Asc() *%[1]sSearchBuilder {
				if p.b.opts == nil {
					p.b.opts = &search.SearchOptions{}
				}
				if p.b.opts.Sort == nil {
					p.b.opts.Sort = &search.SortOptions{}
				}
				p.b.opts.Sort.Expressions = append(p.b.opts.Sort.Expressions, search.SortExpression{
					Expr:    p.Name,
					Reverse: true,
				})

				return p.b
			}

			// Desc add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) Desc() *%[1]sSearchBuilder {
				if p.b.opts == nil {
					p.b.opts = &search.SearchOptions{}
				}
				if p.b.opts.Sort == nil {
					p.b.opts.Sort = &search.SortOptions{}
				}
				p.b.opts.Sort.Expressions = append(p.b.opts.Sort.Expressions, search.SortExpression{
					Expr:    p.Name,
					Reverse: false,
				})

				return p.b
			}

			// %[1]sSearchBoolPropertyInfo hold property info.
			type %[1]sSearchBoolPropertyInfo struct {
				Name string
				b    *%[1]sSearchBuilder
			}

			// Equal add query operand.
			func (p *%[1]sSearchNumberPropertyInfo) Equal(value bool) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Eq, Value: value})
				return p.b
			}

			// %[1]sSearchTimePropertyInfo hold property info.
			type %[1]sSearchTimePropertyInfo struct {
				Name string
				b    *%[1]sSearchBuilder
			}

			// query spec for time.Time.
			// https://cloud.google.com/appengine/docs/go/search/query_strings#Go_Queries_on_date_fields
			// It using date, not datetime.

			// GreaterThanOrEqual add query operand.
			func (p *%[1]sSearchTimePropertyInfo) GreaterThanOrEqual(value time.Time) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.GtEq, Value: value.UTC().Format("2006-01-02")})
				return p.b
			}

			// GreaterThan add query operand.
			func (p *%[1]sSearchTimePropertyInfo) GreaterThan(value time.Time) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Gt, Value: value.UTC().Format("2006-01-02")})
				return p.b
			}

			// LessThanOrEqual add query operand.
			func (p *%[1]sSearchTimePropertyInfo) LessThanOrEqual(value time.Time) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.LtEq, Value: value.UTC().Format("2006-01-02")})
				return p.b
			}

			// LessThan add query operand.
			func (p *%[1]sSearchTimePropertyInfo) LessThan(value time.Time) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Lt, Value: value.UTC().Format("2006-01-02")})
				return p.b
			}

			// Equal add query operand.
			func (p *%[1]sSearchTimePropertyInfo) Equal(value time.Time) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Eq, Value: value.UTC().Format("2006-01-02")})
				return p.b
			}

			// Asc add query operand.
			func (p *%[1]sSearchTimePropertyInfo) Asc() *%[1]sSearchBuilder {
				if p.b.opts == nil {
					p.b.opts = &search.SearchOptions{}
				}
				if p.b.opts.Sort == nil {
					p.b.opts.Sort = &search.SortOptions{}
				}
				p.b.opts.Sort.Expressions = append(p.b.opts.Sort.Expressions, search.SortExpression{
					Expr:    p.Name,
					Reverse: true,
				})

				return p.b
			}

			// Desc add query operand.
			func (p *%[1]sSearchTimePropertyInfo) Desc() *%[1]sSearchBuilder {
				if p.b.opts == nil {
					p.b.opts = &search.SearchOptions{}
				}
				if p.b.opts.Sort == nil {
					p.b.opts.Sort = &search.SortOptions{}
				}
				p.b.opts.Sort.Expressions = append(p.b.opts.Sort.Expressions, search.SortExpression{
					Expr:    p.Name,
					Reverse: false,
				})

				return p.b
			}
		`, st.Name(), "%")

	g.Printf("\n\n")

	return nil
}

// Name returns struct type name.
func (st *BuildStruct) Name() string {
	return st.typeInfo.Name()
}

// Rank returns field with rank annotation.
func (st *BuildStruct) Rank() *BuildField {
	for _, field := range st.Fields {
		if field.Tag.Rank {
			return field
		}
	}
	return nil
}

// ID returns field with id annotation.
func (st *BuildStruct) ID() *BuildField {
	for _, field := range st.Fields {
		if field.Tag.ID {
			return field
		}
	}
	return nil
}

// HasJSON returns struct has json annotated field.
func (st *BuildStruct) HasJSON() bool {
	for _, field := range st.Fields {
		if field.Tag.JSON {
			return true
		}
	}
	return false
}

// HasID returns struct has id annotated field.
func (st *BuildStruct) HasID() bool {
	for _, field := range st.Fields {
		if field.Tag.ID {
			return true
		}
	}
	return false
}

// HasNgram returns struct has ngram annotated field.
func (st *BuildStruct) HasNgram() bool {
	for _, field := range st.Fields {
		if field.Tag.Ngram {
			return true
		}
	}
	return false
}

// HasString returns struct has string annotated field.
func (st *BuildStruct) HasString() bool {
	for _, field := range st.Fields {
		if field.Tag.String {
			return true
		}
	}
	return false
}
