package rdf

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// write.go has the three formats.
//
// Provenance is the thing that makes them differ, because a statement about a
// statement is the one thing RDF has never had a single obvious spelling for.
// N-Triples and Turtle get the quoted-triple form, `<< s p o >> prov:wasDerivedFrom
// <url>`, which costs one line per claim. JSON-LD gets named graphs, one per
// source URL, because JSON-LD has had those since 1.0 and has no quoted triples
// at all. Both are lossless and both say the same thing; there is no third
// option where every format spells it the same way.

// Formats is what --format takes.
var Formats = []string{"nt", "turtle", "jsonld"}

// Write emits the triples in the named format.
func Write(w io.Writer, ts []Triple, format string) error {
	switch format {
	case "", "nt":
		return writeNT(w, ts)
	case "turtle", "ttl":
		return writeTurtle(w, ts)
	case "jsonld", "json-ld":
		return writeJSONLD(w, ts)
	}
	return fmt.Errorf("unknown rdf format %q: one of %s", format, strings.Join(Formats, ", "))
}

// writeNT streams. Nothing is held in memory beyond the line being written,
// which is the reason n-triples is the default: a crawl's worth of claims should
// not have to fit anywhere.
func writeNT(w io.Writer, ts []Triple) error {
	for _, t := range ts {
		if !t.Repeat {
			if _, err := fmt.Fprintf(w, "%s %s %s .\n", ntTerm(t.Subject, false), ntTerm(t.Predicate, false), ntTerm(t.Object, t.ObjectIsLiteral)); err != nil {
				return err
			}
		}
		if t.Derived == "" {
			continue
		}
		_, err := fmt.Fprintf(w, "<< %s %s %s >> <%swasDerivedFrom> %s .\n",
			ntTerm(t.Subject, false), ntTerm(t.Predicate, false), ntTerm(t.Object, t.ObjectIsLiteral),
			NSProv, ntTerm(t.Derived, false))
		if err != nil {
			return err
		}
	}
	return nil
}

// writeTurtle groups by subject, which is the only reason to prefer Turtle over
// n-triples: it is the format somebody reads rather than the one a machine
// loads.
func writeTurtle(w io.Writer, ts []Triple) error {
	prefixes := Prefixes(ts)
	for _, name := range prefixNames(prefixes) {
		if _, err := fmt.Fprintf(w, "@prefix %s: <%s> .\n", name, prefixes[name]); err != nil {
			return err
		}
	}
	if len(prefixes) > 0 {
		fmt.Fprintln(w)
	}
	// Subjects in first-seen order, so a diff between two runs shows what
	// changed rather than what moved.
	var order []string
	bySubject := map[string][]Triple{}
	// A claim two pages agree on is one line in the subject's block and one
	// annotation per page, so the block stays readable and the sources all
	// survive.
	var annotated []Triple
	for _, t := range ts {
		if t.Derived != "" {
			annotated = append(annotated, t)
		}
		if t.Repeat {
			continue
		}
		if _, ok := bySubject[t.Subject]; !ok {
			order = append(order, t.Subject)
		}
		bySubject[t.Subject] = append(bySubject[t.Subject], t)
	}
	for _, s := range order {
		group := bySubject[s]
		if _, err := fmt.Fprintf(w, "%s\n", ntTerm(s, false)); err != nil {
			return err
		}
		for i, t := range group {
			end := ";"
			if i == len(group)-1 {
				end = "."
			}
			p := shorten(t.Predicate, prefixes)
			o := t.Object
			if !t.ObjectIsLiteral {
				o = shorten(t.Object, prefixes)
			} else {
				o = ntTerm(o, true)
			}
			if _, err := fmt.Fprintf(w, "    %s %s %s\n", p, o, end); err != nil {
				return err
			}
		}
		fmt.Fprintln(w)
	}
	for _, t := range annotated {
		o := t.Object
		if !t.ObjectIsLiteral {
			o = shorten(o, prefixes)
		} else {
			o = ntTerm(o, true)
		}
		if _, err := fmt.Fprintf(w, "<< %s %s %s >> prov:wasDerivedFrom %s .\n",
			shorten(t.Subject, prefixes), shorten(t.Predicate, prefixes), o, ntTerm(t.Derived, false)); err != nil {
			return err
		}
	}
	return nil
}

// writeJSONLD emits the context inline, so a consumer needs nothing from the
// network to read the file. A context that has to be dereferenced is a context
// that stops working the day the host does.
func writeJSONLD(w io.Writer, ts []Triple) error {
	ctx := map[string]any{
		"schema": NSSchema,
		"fb":     NSFB,
		"prov":   NSProv,
	}
	prefixes := Prefixes(ts)
	if _, ok := prefixes["prov"]; !ok {
		delete(ctx, "prov")
	}

	// With provenance on, each source is a named graph holding the claims it
	// asserted, which is how two surfaces disagreeing stays visible: the
	// disagreement is two graphs, not one overwritten fact.
	bySource := map[string][]Triple{}
	var sources []string
	for _, t := range ts {
		if _, ok := bySource[t.Derived]; !ok {
			sources = append(sources, t.Derived)
		}
		bySource[t.Derived] = append(bySource[t.Derived], t)
	}

	var graphs []any
	for _, src := range sources {
		nodes := nodesOf(bySource[src])
		if src == "" {
			graphs = append(graphs, nodes...)
			continue
		}
		// The graph gets its own IRI rather than borrowing the page's. A page
		// and the set of claims read off it are two things, and naming them
		// both `src` turns the provenance into `src was derived from src`,
		// which is true and useless.
		graphs = append(graphs, map[string]any{
			"@id":                 src + "#claims",
			"@type":               "prov:Entity",
			"prov:wasDerivedFrom": map[string]string{"@id": src},
			"@graph":              nodes,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(map[string]any{"@context": ctx, "@graph": graphs})
}

// nodesOf collects triples into one object per subject, in first-seen order.
func nodesOf(ts []Triple) []any {
	var order []string
	props := map[string]map[string][]any{}
	for _, t := range ts {
		if _, ok := props[t.Subject]; !ok {
			order = append(order, t.Subject)
			props[t.Subject] = map[string][]any{}
		}
		key := compact(t.Predicate)
		if t.ObjectIsLiteral {
			props[t.Subject][key] = append(props[t.Subject][key], t.Object)
			continue
		}
		props[t.Subject][key] = append(props[t.Subject][key], map[string]string{"@id": t.Object})
	}
	out := make([]any, 0, len(order))
	for _, s := range order {
		node := map[string]any{"@id": s}
		keys := make([]string, 0, len(props[s]))
		for k := range props[s] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			vs := props[s][k]
			if len(vs) == 1 {
				node[k] = vs[0]
				continue
			}
			node[k] = vs
		}
		out = append(out, node)
	}
	return out
}

// compact turns a full IRI into its prefixed form for JSON-LD, and rdf:type into
// @type, which is the spelling every JSON-LD reader expects to see.
func compact(iri string) string {
	switch {
	case iri == NSRDF+"type":
		return "@type"
	case strings.HasPrefix(iri, NSSchema):
		return "schema:" + strings.TrimPrefix(iri, NSSchema)
	case strings.HasPrefix(iri, NSFB):
		return "fb:" + strings.TrimPrefix(iri, NSFB)
	}
	return iri
}

// shorten writes an IRI in prefixed form where a prefix covers it.
func shorten(iri string, prefixes map[string]string) string {
	for _, name := range prefixNames(prefixes) {
		base := prefixes[name]
		if rest, ok := strings.CutPrefix(iri, base); ok && rest != "" && !strings.ContainsAny(rest, "/#:") {
			return name + ":" + rest
		}
	}
	return ntTerm(iri, false)
}

// ntTerm writes an IRI in angle brackets or a literal in quotes.
//
// fb:// URIs are IRIs by every rule that matters here: a scheme, an authority
// and a path, and no consumer has to know what the scheme means to treat two of
// them as the same node.
func ntTerm(s string, literal bool) string {
	if literal {
		return `"` + escapeLiteral(s) + `"`
	}
	return "<" + escapeIRI(s) + ">"
}

func escapeLiteral(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return r.Replace(s)
}

// escapeIRI takes out the characters n-triples will not have inside angle
// brackets. Facebook's own URLs carry spaces and quotes often enough that
// leaving them would produce a file no parser accepts.
func escapeIRI(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '<', '>', '"', '{', '}', '|', '^', '`', '\\', ' ':
			fmt.Fprintf(&b, "%%%02X", r)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, "%%%02X", r)
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
