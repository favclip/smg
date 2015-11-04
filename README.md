# smg

`Search Model Generator`.

## Description

`smg` can generate model & query builder for appengine Search API.

If you use string literal for building query, You shoots your foot when you do typo.
smg generate appengine Search API wrapper code. It is type safe. Your mistake will be found by go compiler.

```
type User struct {
	Name	string
}
```

```
user := &User {
	"go-chan",
}
index := NewUserSearch()
docID, err = index.Put(c, user)
```

```
user := &User {
	"go-chan",
}
index := NewUserSearch()
docID, err = index.Put(c, user)
```

### Example

[from](https://github.com/favclip/smg/blob/master/misc/fixture/a/model.go):

```
package a

// test for basic struct definition

type Sample struct {
	Foo string
}
```

[to](https://github.com/favclip/smg/blob/master/misc/fixture/a/model_search.go):

```
// generated!
type SampleSearch struct {
	src *Sample

	Foo string
}

func (src *Sample) Searchfy() (*SampleSearch, error) {
	if src == nil {
		return nil, nil
	}
	dest := &SampleSearch{}
	dest.src = src
	dest.Foo = src.Foo
	return dest, nil
}

func NewSampleSearch() *SampleSearchBuilder {
	op := &smgutils.Op{}
	b := &SampleSearchBuilder{
		rootOp:    op,
		currentOp: op,
	}
	b.Foo = &SampleSearchStringPropertyInfo{"Foo", b}

	return b
}

type SampleSearchBuilder struct {
	rootOp    *smgutils.Op
	currentOp *smgutils.Op // for grouping
	opts      *search.SearchOptions
	query     string
	index     *search.Index
	Foo       *SampleSearchStringPropertyInfo
}

...
```

usage:

```
src := &Sample{"Foo!"}

index := NewSampleSearch() // generated!
docID, err = index.Put(c, user)
```

[other example](https://github.com/favclip/smg/blob/master/usage_test.go).

### With `go generate`

```
$ ls -la .
total 8
drwxr-xr-x@ 3 vvakame  staff  102 10 15 14:14 .
drwxr-xr-x@ 7 vvakame  staff  238  8 14 18:24 ..
-rw-r--r--@ 1 vvakame  staff  147  8 14 18:24 model.go
$ cat model.go
//go:generate smg -output model_search.go .

package d

// test for struct with tagged comment

// +smg
type Sample struct {
	A string
	B string
}
$ go generate
$ ls -la .
total 32
drwxr-xr-x@ 4 vvakame  staff   136 10 15 14:15 .
drwxr-xr-x@ 7 vvakame  staff   238  8 14 18:24 ..
-rw-r--r--@ 1 vvakame  staff   147  8 14 18:24 model.go
-rw-r--r--  1 vvakame  staff  9993 10 15 14:15 model_search.go
```

## Installation

```
$ go get github.com/favclip/smg/cmd/smg
$ smg
Usage of smg:
	smg [flags] [directory]
	smg [flags] files... # Must be a single package
Flags:
  -output="": output file name; default srcdir/<type>_string.go
  -type="": comma-separated list of type names; must be set
```

## Command sample

Model with type specific option.

```
$ cat misc/fixture/a/model.go
package a

// test for basic struct definition

type Sample struct {
	Foo string
}
$ smg -type Sample -output misc/fixture/a/model_search.go misc/fixture/a
```

Model with tagged comment.

```
$ cat misc/fixture/d/model.go
//go:generate smg -output model_search.go .

package d

// test for struct with tagged comment

// +smg
type Sample struct {
	A string
	B string
}
$ smg -output misc/fixture/d/model_search.go misc/fixture/d
```
