package smgutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/context"
)

type OpType int

const (
	Unknown OpType = iota
	Match
	NgramMatch
	And
	Or
	Gt
	GtEq
	Lt
	LtEq
	Eq
	Group
)

type Op struct {
	FieldName string
	Type      OpType
	Value     interface{}
	Parent    *Op
	Children  []*Op
}

func (op *Op) Query(buffer *bytes.Buffer) error {
	var err error
	switch op.Type {
	case Match:
		if str, ok := op.Value.(string); ok {
			_, err = buffer.WriteString(fmt.Sprintf(` %s: "%s" `, op.FieldName, Sanitize(str)))
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("%#v is not string", op.Value)
		}

	case NgramMatch:
		if str, ok := op.Value.(string); ok {
			_, err = buffer.WriteString(StringPropQuery(op.FieldName, str))
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("%#v is not string", op.Value)
		}

	case And:
		_, err = buffer.WriteString(" AND ")
		if err != nil {
			return err
		}

	case Or:
		_, err = buffer.WriteString(" OR ")
		if err != nil {
			return err
		}

	case Gt, GtEq, Lt, LtEq, Eq:
		expr := ""
		switch op.Type {
		case Gt:
			expr = ">"
		case GtEq:
			expr = ">="
		case Lt:
			expr = "<"
		case LtEq:
			expr = "<="
		case Eq:
			expr = "="
		}
		if num, ok := op.Value.(int); ok {
			_, err = buffer.WriteString(fmt.Sprintf(` %s %s %d `, op.FieldName, expr, num))
			if err != nil {
				return err
			}
		} else if num, ok := op.Value.(int64); ok {
			_, err = buffer.WriteString(fmt.Sprintf(` %s %s %d `, op.FieldName, expr, num))
			if err != nil {
				return err
			}
		} else if b, ok := op.Value.(bool); ok {
			num := 0
			if b {
				num = 1
			}
			_, err = buffer.WriteString(fmt.Sprintf(` %s %s %d `, op.FieldName, expr, num))
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("%#v is not int or int64", op.Value)
		}

	case Group:
		_, err = buffer.WriteString("(")
		if err != nil {
			return err
		}
		for _, ch := range op.Children {
			err = ch.Query(buffer)
			if err != nil {
				return err
			}
		}
		_, err = buffer.WriteString(")")
		if err != nil {
			return err
		}

	case Unknown: // unknown == root
		for _, ch := range op.Children {
			err = ch.Query(buffer)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type DocIDer interface {
	DocID(c context.Context) (string, error)
}

func Unigram(str string) []string {
	result := make([]string, 0, len(str))
	for _, r := range str {
		result = append(result, string([]rune{r}))
	}

	return result
}

func Bigram(str string) []string {
	result := make([]string, 0, len(str))
	var prev rune
	for i, r := range str {
		if i != 0 {
			result = append(result, string([]rune{prev, r}))
		}
		prev = r
	}

	return result
}

func UnigramForSearch(str string) (string, error) {
	result := make([]string, 0, len(str))
	for _, r := range str {
		result = append(result, string([]rune{r}))
	}

	uni, err := json.Marshal(struct{ Value []string }{result})
	if err != nil {
		return "", err
	}

	return string(uni), nil
}

func BigramForSearch(str string) (string, error) {
	result := make([]string, 0, len(str))
	var prev rune
	for i, r := range str {
		if i != 0 {
			result = append(result, string([]rune{prev, r}))
		}
		prev = r
	}

	bi, err := json.Marshal(struct{ Value []string }{result})
	if err != nil {
		return "", err
	}

	return string(bi), nil
}

func StringPropQuery(propName string, value string) string {
	if l := utf8.RuneCountInString(value); l == 0 {
		return ""
	} else if l == 1 {
		return fmt.Sprintf(`%sUnigram: "%s"`, propName, Sanitize(value))
	}

	scattered := make([]string, 0, len(value))
	bi := Bigram(value)
	for _, s := range bi {
		scattered = append(scattered, fmt.Sprintf(`%sBigram: "%s"`, propName, Sanitize(s)))
	}

	return "(" + strings.Join(scattered, " AND ") + ")"
}

func Sanitize(value string) string {
	value = strings.Replace(value, `\`, `\\`, -1)
	value = strings.Replace(value, `"`, `\"`, -1)
	return value
}
