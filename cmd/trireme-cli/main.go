// Command trireme-cli is how seats and the XO reach a Trireme server's shared state.
// It reads TRIREME_URL and TRIREME_TOKEN from the environment.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const usage = `trireme-cli: reach the Trireme shared state.

  trireme-cli fleets
  trireme-cli journal [-project P] [-limit N]
  trireme-cli blockers [-all]
  trireme-cli blocker open -project P -seat S -summary TEXT [-detail TEXT] [-option CHOICE ...]
  trireme-cli consult -project P -from SEAT -to SEAT -body TEXT
  trireme-cli answer -project P -from SEAT -to SEAT -body TEXT
  trireme-cli directive -project P -to SEAT -body TEXT
  trireme-cli kick -project P -seat SEAT [-body TEXT]
`

// client talks to one server.
type client struct {
	base  string
	token string
	http  *http.Client
	out   io.Writer
}

func main() {
	c := &client{base: os.Getenv("TRIREME_URL"), token: os.Getenv("TRIREME_TOKEN"),
		http: &http.Client{Timeout: 30 * time.Second}, out: os.Stdout}
	if err := c.run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "trireme-cli:", err)
		os.Exit(1)
	}
}

func (c *client) run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" {
		_, err := io.WriteString(c.out, usage)
		return err
	}
	if c.base == "" {
		return fmt.Errorf("TRIREME_URL is not set")
	}
	commands := map[string]func([]string) error{
		"fleets": c.fleets, "journal": c.journal, "blockers": c.blockers, "blocker": c.blocker,
		"consult": c.inboxCmd("consult"), "answer": c.inboxCmd("answer"),
		"directive": c.inboxCmd("directive"), "kick": c.inboxCmd("kick"),
	}
	cmd, ok := commands[args[0]]
	if !ok {
		return fmt.Errorf("unknown command %q (try: trireme-cli help)", args[0])
	}
	return cmd(args[1:])
}

func (c *client) fleets(args []string) error {
	var out struct {
		Fleets []struct {
			ProjectID string  `json:"project_id"`
			State     string  `json:"state"`
			Reason    string  `json:"reason"`
			SpentUSD  float64 `json:"spent_usd"`
		} `json:"fleets"`
		Seats map[string]string `json:"seats"`
	}
	if err := c.get("/api/fleets", nil, &out); err != nil {
		return err
	}
	for _, f := range out.Fleets {
		fmt.Fprintf(c.out, "%s: %s %s, $%.2f spent\n", f.ProjectID, f.State, f.Reason, f.SpentUSD)
	}
	for k, v := range out.Seats {
		fmt.Fprintf(c.out, "  %s: %s\n", k, v)
	}
	return nil
}

func (c *client) journal(args []string) error {
	fs := flag.NewFlagSet("journal", flag.ContinueOnError)
	project := fs.String("project", "", "only this project")
	limit := fs.Int("limit", 50, "how many events")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var out struct {
		Events []map[string]any `json:"events"`
	}
	q := url.Values{"project": {*project}, "limit": {fmt.Sprint(*limit)}}
	if err := c.get("/api/journal", q, &out); err != nil {
		return err
	}
	for _, e := range out.Events {
		payload, _ := json.Marshal(e["payload"]) // why: it came from JSON, so it re-encodes
		fmt.Fprintf(c.out, "%v %v %v %v %s\n", e["ts"], e["project_id"], e["seat"], e["type"], payload)
	}
	return nil
}

func (c *client) blockers(args []string) error {
	fs := flag.NewFlagSet("blockers", flag.ContinueOnError)
	all := fs.Bool("all", false, "include answered blockers")
	if err := fs.Parse(args); err != nil {
		return err
	}
	q := url.Values{}
	if !*all {
		q.Set("open", "1")
	}
	var out struct {
		Blockers []map[string]any `json:"blockers"`
	}
	if err := c.get("/api/blockers", q, &out); err != nil {
		return err
	}
	for _, b := range out.Blockers {
		fmt.Fprintf(c.out, "#%v [%v] %v (from %v) %v\n", b["id"], b["project_id"], b["summary"], b["seat"], answerOf(b))
	}
	return nil
}

func answerOf(b map[string]any) string {
	if b["answered_at"] == nil {
		return "OPEN"
	}
	return fmt.Sprintf("ANSWERED: %v %v", b["choice"], b["text"])
}

type options []string

func (o *options) String() string     { return strings.Join(*o, ",") }
func (o *options) Set(v string) error { *o = append(*o, v); return nil }

func (c *client) blocker(args []string) error {
	if len(args) == 0 || args[0] != "open" {
		return fmt.Errorf("usage: trireme-cli blocker open -project P -seat S -summary TEXT")
	}
	fs := flag.NewFlagSet("blocker open", flag.ContinueOnError)
	project, seat := fs.String("project", "", "project id"), fs.String("seat", "", "your seat id")
	summary, detail := fs.String("summary", "", "one plain sentence"), fs.String("detail", "", "the full detail")
	var opts options
	fs.Var(&opts, "option", "a button choice (repeatable)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	body := map[string]any{"project_id": *project, "seat": *seat, "summary": *summary, "detail": *detail, "options": []string(opts)}
	var out map[string]any
	if err := c.post("/api/blockers", body, &out); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "opened blocker #%v\n", out["id"])
	return nil
}

func (c *client) inboxCmd(kind string) func([]string) error {
	return func(args []string) error {
		fs := flag.NewFlagSet(kind, flag.ContinueOnError)
		project, body := fs.String("project", "", "project id"), fs.String("body", "", "the message")
		from, to := fs.String("from", "", "your seat id"), fs.String("to", "", "the seat it is for")
		seat := fs.String("seat", "", "the seat to wake (kick only)")
		if err := fs.Parse(args); err != nil {
			return err
		}
		if kind == "directive" {
			*from = "xo"
		}
		if kind == "kick" {
			*from, *to = "user", *seat
		}
		item := map[string]any{"project_id": *project, "from_seat": *from, "to_seat": *to, "kind": kind, "body": *body}
		var out map[string]any
		if err := c.post("/api/inbox", item, &out); err != nil {
			return err
		}
		fmt.Fprintf(c.out, "posted %s #%v to %s\n", kind, out["id"], *to)
		return nil
	}
}

func (c *client) get(path string, q url.Values, out any) error {
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *client) post(path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.base+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *client) do(req *http.Request, out any) error {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }() // why: the body is fully read below
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: %d %s", req.Method, req.URL.Path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return json.Unmarshal(raw, out)
}
