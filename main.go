package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"zgo.at/acidtab"
	"zgo.at/jfmt"
	"zgo.at/zli"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/zmap"
	"zgo.at/zstd/zslice"
)

var (
	addr              string
	verbose, expanded bool
)

func main() {
	f := zli.NewFlags(os.Args)
	var (
		a    = f.String("http://elastic:elastic@127.0.0.1:9200", "es")
		v    = f.Bool(false, "v")
		x    = f.Bool(false, "x")
		help = f.Bool(false, "h", "help")
	)
	zli.F(f.Parse(zli.AllowUnknown()))
	if help.Bool() || slices.Contains(f.Args, "help") {
		fmt.Print(usage)
		return
	}
	addr, verbose, expanded = a.String(), v.Bool(), x.Bool()

	cmd, index := f.Shift(), f.Shift()
	if strings.HasPrefix(index, "-") {
		f.Args, index = append([]string{index}, f.Args...), ""
	}
	// Accept both "index-name cmd" and "cmd index-name"
	commands := []string{"ls", "list", "describe", "drop", "select", "delete"}
	if slices.Contains(commands, index) {
		cmd, index = index, cmd
	}
	if cmd == "" {
		zli.F(f.Parse())
		listIndexes()
		return
	}
	if !slices.Contains(commands, cmd) {
		index, cmd = cmd, "ls"
	}
	if index == "" {
		zli.Fatalf("no index name given")
	}

	switch cmd {
	case "ls", "list", "select":
		var (
			selekt = f.String("*", "s", "select")
			where  = f.String("", "w", "where")
			order  = f.String("", "o", "order")
			limit  = f.Int(100, "l", "limit")
		)
		zli.F(f.Parse())
		listIndex(index, selekt.String(), where.String(), order.String(), limit.Int())
	case "delete":
		var (
			where = f.String("", "w", "where")
		)
		zli.F(f.Parse())
		deleteRows(index, where.String())

	case "describe":
		zli.F(f.Parse())
		var j json.RawMessage
		r := get("/"+index, &j)

		jfmt.NewFormatter(120, "", "  ").Format(os.Stdout, bytes.NewReader(r))

	// TODO: no option for this; need to delete+recreate.
	// es «index» truncate           Truncate index.
	// case "truncate":

	case "drop":
		zli.F(f.Parse())

		var list []struct {
			Index string `json:"index"`
		}
		get("/_cat/indices", &list)

		matches := make([]string, 0, 4)
		for ind := range strings.SplitSeq(index, ",") {
			ind = strings.TrimSpace(ind)

			found := false
			for _, l := range list {
				if ok, _ := filepath.Match(ind, l.Index); ok {
					matches, found = append(matches, l.Index), true
				}
			}
			if !found {
				zli.Fatalf("no indexes matched %q", ind)
			}
		}

		var j struct {
			ElasticError
			Acknowledged bool `json:"acknowledged"`
		}
		del("/"+strings.Join(matches, ","), nil, &j)
	}
}

// https://www.elastic.co/guide/en/elasticsearch/reference/8.19/cat-indices.html
func listIndexes() {
	var list []struct {
		UUID         string `json:"uuid"`           // "zoYQqEScQlewe6DEjGBOYA"
		Index        string `json:"index"`          // "aleph-entity-event-v1",
		Status       string `json:"status"`         // "open",
		Health       string `json:"health"`         // "green",
		DocsCount    string `json:"docs.count"`     // "0",
		DocsDeleted  string `json:"docs.deleted"`   // "0",
		Pri          string `json:"pri"`            // "5",
		PriStoreSize string `json:"pri.store.size"` // "1.1kb",
		Rep          string `json:"rep"`            // "0",
		StoreSize    string `json:"store.size"`     // "1.1kb",
	}
	get("/_cat/indices?s=index", &list)

	t := acidtab.New("Name", "Status", "Health", "# docs", "# shard").
		AlignCol(4, acidtab.Right).AlignCol(5, acidtab.Right)
	for _, l := range list {
		t = t.Rows(l.Index, l.Status, l.Health, l.DocsCount, l.Pri)
	}
	printTable(t)
}

type Hit struct {
	ID     string         `json:"_id"`
	Index  string         `json:"_index"`
	Score  float64        `json:"_score"`
	Source map[string]any `json:"_source"`
	//Type   string         `json:"_type"`
}

// https://www.elastic.co/guide/en/elasticsearch/reference/8.19/indices-get-index.html
func listIndex(index, selekt, where, order string, limit int) {
	var s struct {
		ElasticError
		Hits struct {
			Hits []Hit `json:"hits"`
		} `json:"hits"`
	}

	params := make(url.Values)
	params.Set("size", strconv.Itoa(limit))
	if order != "" {
		params.Set("sort", order)
	}

	var body []byte
	if where != "" {
		// https://www.elastic.co/guide/en/elasticsearch/reference/8.19/query-dsl-query-string-query.html
		body = zjson.MustMarshal(map[string]any{
			"query": map[string]any{
				"query_string": map[string]any{"query": where},
			},
		})
	}

	// get("/"+index+"/_search?"+params.Encode(), &s)
	post("/"+index+"/_search?"+params.Encode(), body, &s)
	printRows(s.Hits.Hits, index, selekt)
}

// DELETE /<index>/_doc/<_id>
// POST /my-index-000001/_delete_by_query
func deleteRows(index, where string) {
	if strings.TrimSpace(where) == "" {
		zli.Fatalf("-where is empty or not given")
	}

	var s struct {
		ElasticError
		Deleted int `json:"deleted"`
	}

	body := zjson.MustMarshal(map[string]any{
		"query": map[string]any{
			"query_string": map[string]any{"query": where},
		},
	})

	post("/"+index+"/_delete_by_query", body, &s)
	fmt.Printf("(deleted %d)\n", s.Deleted)
}

func printRows(hits []Hit, index string, selekt string) {
	if len(hits) == 0 {
		fmt.Println("(no rows)")
		return
	}

	var headers []string
	all := append(slices.Collect(maps.Keys(hits[0].Source)), "_id")
	selekt = strings.TrimSpace(selekt)
	switch {
	case selekt == "*":
		// Only include headers with at least one non-empty (nil, empty string,
		// or 0-length array) value.
		headers = make([]string, 0, 16)
		headers = append(headers, "_id")
		for _, h := range hits {
			for k, v := range h.Source {
				empty := v == nil
				switch vv := v.(type) {
				case string:
					empty = vv == ""
				case []any:
					empty = len(vv) == 0
				}
				if !empty && !slices.Contains(headers, k) {
					headers = append(headers, k)
				}
			}
		}
	case selekt == "*.all":
		headers = all
	default:
		headers = strings.Split(selekt, ",")
		for i := range headers {
			headers[i] = strings.TrimSpace(headers[i])
			if !slices.Contains(all, headers[i]) {
				zli.Fatalf("unknown column for index %q: %q", index, headers[i])
			}
		}
	}

	slices.Sort(headers)

	var t *acidtab.Table
	if expanded {
		var (
			headersType = headerTypes(index, expanded)
			l           = zslice.Longest(headers) + zslice.Longest(zmap.Values(headersType))
			headers2    = make([]string, len(headers))
		)
		for i := range headers {
			t := headersType[headers[i]]
			headers2[i] = fmt.Sprintf("%s  %s%s", headers[i], strings.Repeat(" ", l-len(headers[i])-len(t)), t)
		}
		t = acidtab.New(headers2...)
	} else {
		t = acidtab.New(headers...)
	}
	for _, h := range hits {
		vals := make([]any, 0, len(headers))
		for _, header := range headers {
			v := h.Source[header]
			if header == "_id" {
				v = h.ID
			}
			if v == nil {
				v = "\x1b[38;5;250m<nil>\x1b[0m"
			} else {
				j, err := json.Marshal(v)
				if err == nil {
					v = string(j)
				}
			}
			vals = append(vals, v)
		}
		t = t.Row(vals...)
	}

	printTable(t)
}

// Get list of all header types.
func headerTypes(index string, expanded bool) map[string]string {
	type prop struct {
		Type       string          `json:"type"`
		CopyTo     []string        `json:"copy_to"`
		Index      zbool.Bool      `json:"index"`
		Dynamic    zbool.Bool      `json:"dynamic"`
		Analyzer   string          `json:"analyzer"`
		Store      zbool.Bool      `json:"store"`
		TermVector string          `json:"term_vector"`
		Fields     map[string]any  `json:"fields"`
		Properties map[string]prop `json:"properties"`
	}
	var l map[string]struct {
		Aliases  any `json:"aliases"`
		Settings any `json:"settings"`
		Mappings struct {
			Properties map[string]prop `json:"properties"`
		} `json:"mappings"`
	}

	get("/"+index, &l)

	h := map[string]string{"_id": "keyword"}
	for _, k := range zmap.KeysOrdered(l[index].Mappings.Properties) {
		p := l[index].Mappings.Properties[k]
		h[k] = p.Type
	}
	return h
}

func printTable(t *acidtab.Table) {
	if expanded {
		// TODO: Prefix("") doesn't work?
		t.Prefix("").Close(acidtab.CloseTop).Vertical(os.Stdout)
	} else {
		t.Horizontal(os.Stdout)
	}
}
