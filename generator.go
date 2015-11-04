package smg

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/favclip/genbase"
)

// BuildStruct represents source code of assembling..
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
	g.Printf("// for %s\n", st.Name())

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
				func (s *%[1]sSearch) Load(fields []search.Field, metadata *search.DocumentMetadata) error {
					return search.LoadStruct(s, fields)
				}

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
			func (b *%[1]sSearchBuilder) And() *%[1]sSearchBuilder {
				b.currentOp.Children = append(b.currentOp.Children, &smgutils.Op{Type: smgutils.And})
				return b
			}

			func (b *%[1]sSearchBuilder) Or() *%[1]sSearchBuilder {
				b.currentOp.Children = append(b.currentOp.Children, &smgutils.Op{Type: smgutils.Or})
				return b
			}

			func (b *%[1]sSearchBuilder) Group(p func()) *%[1]sSearchBuilder {
				b.StartGroup()
				p()
				b.EndGroup()
				return b
			}

			func (b *%[1]sSearchBuilder) StartGroup() *%[1]sSearchBuilder {
				op := &smgutils.Op{Type: smgutils.Group, Parent: b.currentOp}
				b.currentOp.Children = append(b.currentOp.Children, op)
				b.currentOp = op
				return b
			}

			func (b *%[1]sSearchBuilder) EndGroup() *%[1]sSearchBuilder {
				b.currentOp = b.currentOp.Parent
				return b
			}

			func (b *%[1]sSearchBuilder) Put(c context.Context, src *%[1]s) (string, error) {
				doc, err := src.Searchfy()
				if err != nil {
					return "", err
				}
				return b.PutDocument(c, doc)
			}

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

			func (b *%[1]sSearchBuilder) Delete(c context.Context, src *%[1]s) error {
				doc, err := src.Searchfy()
				if err != nil {
					return err
				}
				return b.DeleteDocument(c, doc)
			}

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

			func (b *%[1]sSearchBuilder) DeleteByDocID(c context.Context, docID string) error {
				index, err := search.Open("%[1]s")
				if err != nil {
					return err
				}

				return index.Delete(c, docID)
			}

			func (b *%[1]sSearchBuilder) Opts() *%[1]sSearchOptions {
				return &%[1]sSearchOptions{b: b}
			}

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

			type %[1]sSearchOptions struct {
				b *%[1]sSearchBuilder
			}

			func (b *%[1]sSearchOptions) Limit(value int) *%[1]sSearchOptions {
				if b.b.opts == nil {
					b.b.opts = &search.SearchOptions{}
				}
				b.b.opts.Limit = value
				return b
			}

			func (b *%[1]sSearchOptions) IDsOnly() *%[1]sSearchOptions {
				if b.b.opts == nil {
					b.b.opts = &search.SearchOptions{}
				}
				b.b.opts.IDsOnly = true
				return b
			}

			type %[1]sSearchIterator struct {
				b    *%[1]sSearchBuilder
				iter *search.Iterator
			}

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

			type %[1]sSearchStringPropertyInfo struct {
				Name string
				b    *%[1]sSearchBuilder
			}

			func (p *%[1]sSearchStringPropertyInfo) Match(value string) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Match, Value: value})
				return p.b
			}

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

			type %[1]sSearchNgramStringPropertyInfo struct {
				%[1]sSearchStringPropertyInfo
			}

			func (p *%[1]sSearchNgramStringPropertyInfo) NgramMatch(value string) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.NgramMatch, Value: value})
				return p.b
			}

			type %[1]sSearchNumberPropertyInfo struct {
				Name string
				b    *%[1]sSearchBuilder
			}

			func (p *%[1]sSearchNumberPropertyInfo) IntGreaterThanOrEqual(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.GtEq, Value: value})
				return p.b
			}

			func (p *%[1]sSearchNumberPropertyInfo) IntGreaterThan(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Gt, Value: value})
				return p.b
			}

			func (p *%[1]sSearchNumberPropertyInfo) IntLessThanOrEqual(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.LtEq, Value: value})
				return p.b
			}

			func (p *%[1]sSearchNumberPropertyInfo) IntLessThan(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Lt, Value: value})
				return p.b
			}

			func (p *%[1]sSearchNumberPropertyInfo) IntEqual(value int) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Eq, Value: value})
				return p.b
			}

			func (p *%[1]sSearchNumberPropertyInfo) Int64GreaterThanOrEqual(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.GtEq, Value: value})
				return p.b
			}

			func (p *%[1]sSearchNumberPropertyInfo) Int64GreaterThan(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Gt, Value: value})
				return p.b
			}

			func (p *%[1]sSearchNumberPropertyInfo) Int64LessThanOrEqual(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.LtEq, Value: value})
				return p.b
			}

			func (p *%[1]sSearchNumberPropertyInfo) Int64LessThan(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Lt, Value: value})
				return p.b
			}

			func (p *%[1]sSearchNumberPropertyInfo) Int64Equal(value int64) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Eq, Value: value})
				return p.b
			}

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

			type %[1]sSearchBoolPropertyInfo struct {
				Name string
				b    *%[1]sSearchBuilder
			}

			func (p *%[1]sSearchNumberPropertyInfo) Equal(value bool) *%[1]sSearchBuilder {
				p.b.currentOp.Children = append(p.b.currentOp.Children, &smgutils.Op{FieldName: p.Name, Type: smgutils.Eq, Value: value})
				return p.b
			}

			type %[1]sSearchTimePropertyInfo struct {
				Name string
				b    *%[1]sSearchBuilder
			}

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

func (st *BuildStruct) Name() string {
	return st.typeInfo.Name()
}

func (st *BuildStruct) Rank() *BuildField {
	for _, field := range st.Fields {
		if field.Tag.Rank {
			return field
		}
	}
	return nil
}

func (st *BuildStruct) ID() *BuildField {
	for _, field := range st.Fields {
		if field.Tag.ID {
			return field
		}
	}
	return nil
}

func (st *BuildStruct) HasJSON() bool {
	for _, field := range st.Fields {
		if field.Tag.JSON {
			return true
		}
	}
	return false
}

func (st *BuildStruct) HasID() bool {
	for _, field := range st.Fields {
		if field.Tag.ID {
			return true
		}
	}
	return false
}

func (st *BuildStruct) HasNgram() bool {
	for _, field := range st.Fields {
		if field.Tag.Ngram {
			return true
		}
	}
	return false
}

func (st *BuildStruct) HasString() bool {
	for _, field := range st.Fields {
		if field.Tag.String {
			return true
		}
	}
	return false
}
