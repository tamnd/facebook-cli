package fb

import (
	"reflect"
	"sort"
	"strings"
	"time"
)

// fields.go is the census: every field of every record kind, and how many of the
// committed fixtures actually had something in it.
//
// It exists because of the rule in spec 3004 doc 00: a field Facebook ships and
// fb drops is a defect. The trouble with that rule is that the failure is
// silent. A parser that stops finding `talking_about_count` because Facebook
// renamed the key produces a record with a zero in it, and a zero is what a Page
// nobody is talking about looks like too.
//
// So the census is generated from the fixtures and committed, and a test
// regenerates it and fails when it differs. A field that stops arriving stops
// being a null nobody notices and becomes a diff in fields_gen.go.
//
// What it cannot tell you is whether an empty field is empty because Facebook
// did not ship it or because nothing in the fixture set has one. Twelve captures
// is twelve pages, not the whole of facebook.com, so `filled 0 of 3` is a
// question rather than a verdict, and the census prints the numbers instead of a
// pass and a fail for exactly that reason.

// Field is one field of one record kind as measured against the fixtures.
type Field struct {
	Kind string `json:"kind"`
	// Name is the dotted JSON path, so `owner.name` rather than `Owner.Name`.
	// The JSON is what people write jq against.
	Name string `json:"name"`
	Type string `json:"type"`
	// Filled is how many fixtures had this field non-empty, out of Fixtures.
	Filled   int `json:"filled"`
	Fixtures int `json:"fixtures"`
	// Seen names the fixtures it was filled in, which is the difference between
	// "no page has this" and "the one page that has it is the one I forgot".
	Seen []string `json:"seen,omitempty"`
}

// FieldKinds is every kind the census covers, in the order fields prints them.
func FieldKinds() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range Census {
		if !seen[f.Kind] {
			seen[f.Kind] = true
			out = append(out, f.Kind)
		}
	}
	sort.Strings(out)
	return out
}

// FieldsOf returns the census for one kind.
func FieldsOf(kind string) ([]Field, error) {
	k := strings.ToLower(strings.TrimSpace(kind))
	var out []Field
	for _, f := range Census {
		if strings.ToLower(f.Kind) == k {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil, usage("there is no census for %q: fb fields covers %s", kind, strings.Join(FieldKinds(), ", "))
	}
	return out, nil
}

// walkFields lists every leaf path on a record type, and says for a given value
// which of those paths is filled.
//
// Structs are walked through, slices and maps are leaves. Recursing into a slice
// element would give paths like `comments[].author.profile_picture.uri`, which
// is a census of Ref and Image over again under three hundred names. The element
// types are kinds of their own and are censused as themselves.
func walkFields(v reflect.Value, prefix string, seen map[reflect.Type]bool, add func(path, typ string, filled bool)) {
	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		name, opts, _ := strings.Cut(sf.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(sf.Name)
		}
		fv := v.Field(i)
		// The envelope is fb's own bookkeeping and the engine fills it, not the
		// parsers, so counting it here would put `tier: filled 0 of 2` at the
		// top of every kind and teach people to skim past the zeros.
		if sf.Anonymous && sf.Type == reflect.TypeOf(Envelope{}) {
			continue
		}
		// Any other embedded struct with no JSON name is inlined by
		// encoding/json, so its keys sit at the top level and the census puts
		// them there too.
		if sf.Anonymous && sf.Tag.Get("json") == "" {
			walkFields(fv, prefix, seen, add)
			continue
		}
		_ = opts
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		inner, ok := structUnder(fv)
		if !ok || seen[inner.Type()] {
			// A leaf, or a type already on this path: Photo.Next is a Ref and a
			// Ref that pointed back would walk forever.
			add(path, typeName(sf.Type), !isEmpty(fv))
			continue
		}
		add(path, typeName(sf.Type), !isEmpty(fv))
		seen[inner.Type()] = true
		walkFields(inner, path, seen, add)
		delete(seen, inner.Type())
	}
}

// structUnder returns the struct behind a field, following one pointer, and
// says no for time.Time, which is a struct with unexported fields and is a leaf
// to everyone who reads it.
//
// A nil pointer is a leaf too. NASA's captured post is not a share, so walking
// into its nil shared_post would put twenty-five rows of `filled 0` in the
// census for one field that is absent, and bury the fields that are absent for a
// reason worth looking at. One row saying `shared_post: filled 0 of 1` is the
// honest version: no fixture has a share, so nothing under it was measured.
func structUnder(v reflect.Value) (reflect.Value, bool) {
	t := v.Type()
	if t.Kind() == reflect.Pointer {
		if v.IsNil() || t.Elem().Kind() != reflect.Struct {
			return reflect.Value{}, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct || v.Type() == reflect.TypeOf(time.Time{}) {
		return reflect.Value{}, false
	}
	return v, true
}

// isEmpty is the same question encoding/json's omitempty asks, plus the zero
// time and the zero struct, because a record's Counts is present in the JSON and
// still says nothing when every count is zero.
func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Pointer, reflect.Interface:
		return v.IsNil()
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Struct:
		if t, ok := v.Interface().(time.Time); ok {
			return t.IsZero()
		}
		return v.IsZero()
	}
	return false
}

// typeName is the Go type as somebody reading the census would write it, with
// the package qualifier off: every type in here is an fb type or a builtin.
func typeName(t reflect.Type) string {
	return strings.ReplaceAll(t.String(), "fb.", "")
}
