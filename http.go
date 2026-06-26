package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"zgo.at/jfmt"
	"zgo.at/zli"
)

var client = http.Client{Timeout: 10 * time.Second}

type ElasticError struct {
	Status int `json:"status"`
	Error  struct {
		Reason    string `json:"reason"`
		Type      string `json:"type"`
		RootCause []struct {
			Reason string `json:"reason"`
			Type   string `json:"type"`
		} `json:"root_cause"`
	} `json:"error"`
}

func (e ElasticError) ESError() string {
	if e.Error.Reason == "" {
		return ""
	}
	if len(e.Error.RootCause) > 0 {
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("status %d", e.Status))
		for _, rc := range e.Error.RootCause {
			msg.WriteString(fmt.Sprintf(": %s", rc.Reason))
		}
		return msg.String()
	}
	return fmt.Sprintf("status %d: %s", e.Status, e.Error.Reason)
}

func get(path string, scan any) []byte               { return do("GET", path, nil, scan) }
func post(path string, body []byte, scan any) []byte { return do("POST", path, body, scan) }
func del(path string, body []byte, scan any) []byte  { return do("DELETE", path, body, scan) }

func do(verb, path string, body []byte, scan any) []byte {
	u := addr + path
	if strings.Contains(path, "?") {
		u += "&format=json"
	} else {
		u += "?format=json"
	}

	var rb io.Reader
	if body != nil {
		rb = bytes.NewReader(body)
	}
	r, err := http.NewRequest(verb, u, rb)
	zli.F(err)
	r.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(r)
	zli.F(err)
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	zli.F(err)

	if verbose {
		fmt.Fprintf(os.Stderr, "%-6s %s\n", verb, u)
		fmt.Fprint(os.Stderr, "BODY   ")
		if body == nil {
			fmt.Println("<nil>")
		} else {
			jfmt.NewFormatter(120, "       ", "    ").Format(os.Stderr, bytes.NewReader(body))
		}
		fmt.Fprint(os.Stderr, "RESP   ", resp.Status, "\n       ")
		jfmt.NewFormatter(120, "       ", "    ").Format(os.Stderr, bytes.NewReader(b))
	}

	if scan != nil {
		zli.F(json.Unmarshal(b, scan))
		if eserr, ok := scan.(interface{ ESError() string }); ok {
			if m := eserr.ESError(); m != "" {
				zli.Fatalf(m)
			}
		}
	}
	return b
}
