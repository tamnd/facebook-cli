package fb

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// graphql.go replays an operation the page already shipped.
//
// The rule, from spec 3004 doc 01 section 3, is narrow on purpose: fb only ever
// sends a doc id it harvested from a page it just fetched, with the variables
// that page sent, changing only the count and the cursor. It never hardcodes a
// doc id, because they rotate, and it never hand-builds variables, because the
// __relay_internal__pv__* provider flags are required and unguessable, and
// leaving one out gets "A server error occurred" with nothing to go on.
//
// This is also the reason a paged read costs one page fetch first. That is not
// waste: the page is where the doc id, the variables and the lsd token come
// from, and it is the same request that answers the first page of results.

// refusalCode is Facebook's allowlist refusal.
//
// It arrives at HTTP 200 with the message "Rate limit exceeded", and it is not
// a rate limit: it comes back instantly, for exactly two operations, on the
// first request of a cold run. It maps to exit 4, not 5, and it is never
// retried. Anybody reading only the message string will want to "fix" this, so:
// doc 01 section 3.2 has the measurement.
const refusalCode = 1675004

// gqlError is one entry in the errors array.
type gqlError struct {
	Message     string `json:"message"`
	Code        int    `json:"code"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Summary     string `json:"summary"`
}

// Replay sends one operation and returns its stitched document.
//
// pre is a descriptor harvested from page, and vars overrides individual
// variables on it, which is how a cursor and a count get in.
func (c *Client) Replay(ctx context.Context, page *Page, pre Preload, vars map[string]any) (*Document, error) {
	tokens := parseTokens(page.HTML)
	if !tokens.usable() {
		return nil, needAuth("the page carried no request token, so %s cannot be replayed: try again, or use --no-cache", pre.Op)
	}
	merged := map[string]any{}
	for k, v := range pre.Variables {
		merged[k] = v
	}
	for k, v := range vars {
		merged[k] = v
	}
	varsJSON, err := json.Marshal(merged)
	if err != nil {
		return nil, usage("%s: variables do not encode: %v", pre.Op, err)
	}

	form := url.Values{}
	form.Set("av", tokens.UserID)
	form.Set("__user", tokens.UserID)
	form.Set("__a", "1")
	form.Set("__req", "1")
	form.Set("__hs", tokens.Haste)
	form.Set("dpr", "1")
	form.Set("__ccg", "EXCELLENT")
	form.Set("__rev", tokens.Rev)
	form.Set("__hsi", tokens.HSI)
	form.Set("__comet_req", "15")
	form.Set("lsd", tokens.LSD)
	form.Set("jazoest", tokens.Jazoest)
	form.Set("__spin_r", tokens.Rev)
	form.Set("__spin_b", tokens.SpinB)
	form.Set("__spin_t", tokens.SpinT)
	form.Set("fb_api_caller_class", "RelayModern")
	form.Set("fb_api_req_friendly_name", pre.Op)
	form.Set("variables", string(varsJSON))
	form.Set("server_timestamps", "true")
	form.Set("doc_id", pre.DocID)
	if tokens.DTSG != "" {
		form.Set("fb_dtsg", tokens.DTSG)
	}

	endpoint := "https://" + canonHost + "/api/graphql/"
	body, err := c.postForm(ctx, endpoint, form, pre.Op, tokens, page.FinalURL)
	if err != nil {
		return nil, err
	}
	return readStream(pre.Op, body)
}

func (c *Client) postForm(ctx context.Context, endpoint string, form url.Values, op string, tokens Tokens, referer string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		if err := c.wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		// This is a fetch, not a navigation, so the headers are the cors set
		// and not the navigate set. Sending the navigate headers here gets a
		// 400 just as surely as omitting them on a page request does.
		req.Header.Set("User-Agent", userAgent())
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Origin", "https://"+canonHost)
		if referer == "" {
			referer = "https://" + canonHost + "/"
		}
		req.Header.Set("Referer", referer)
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("X-FB-LSD", tokens.LSD)
		req.Header.Set("X-FB-Friendly-Name", op)
		req.Header.Set("X-ASBD-ID", "359341")
		if c.Cookies != "" {
			req.Header.Set("Cookie", c.Cookies)
		}
		c.log("POST %s %s", endpoint, op)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			c.logRead(endpoint+"#"+op, surfaceGraphQL, 0, 0, err)
			if attempt < c.Retries && AsNetwork(err) != nil {
				if err := c.backoff(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			if ne := AsNetwork(err); ne != nil {
				return nil, ne
			}
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		// The operation goes in the audit log as a fragment on the endpoint.
		// Twenty replays are twenty POSTs to the same URL, and a log that
		// cannot tell them apart is a log nobody can check a record against.
		// A fragment is never sent to a server, so writing one down here
		// invents nothing about the request.
		c.logRead(endpoint+"#"+op, surfaceGraphQL, resp.StatusCode, len(body), readErr)
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode >= 400 {
			if attempt < c.Retries && retryable(resp.StatusCode, nil) {
				if err := c.backoff(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			if e := asHTTPStatus(resp.StatusCode, op, string(truncate(body))); e != nil {
				return nil, e
			}
			return nil, &HTTPError{Status: resp.StatusCode, URL: endpoint, Body: string(truncate(body))}
		}
		return body, nil
	}
}

// readStream parses a replayed response into one document.
//
// An operation with deferred fragments answers as several JSON objects, one per
// line, exactly like the script blocks on a page. So the same stitcher does
// both, which is the reason stitch() takes payloads rather than a page.
func readStream(op string, body []byte) (*Document, error) {
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	var ps []payload
	seq := 0
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		// Facebook prefixes some responses with the anti-JSON-hijacking guard.
		line = bytes.TrimPrefix(line, []byte("for (;;);"))
		var probe struct {
			Errors []gqlError      `json:"errors"`
			Data   json.RawMessage `json:"data"`
			Label  string          `json:"label"`
			Path   []any           `json:"path"`
		}
		if err := json.Unmarshal(line, &probe); err != nil {
			continue
		}
		if err := refusal(op, probe.Errors); err != nil {
			return nil, err
		}
		if len(probe.Data) == 0 {
			continue
		}
		ps = append(ps, payload{
			Op:       op,
			Label:    probe.Label,
			Path:     probe.Path,
			Data:     probe.Data,
			Sequence: seq,
		})
		seq++
	}
	if err := sc.Err(); err != nil {
		return nil, &NetworkError{Msg: "the response for " + op + " did not read as JSON lines", Err: err}
	}
	if len(ps) == 0 {
		return nil, noResults("%s returned nothing", op)
	}
	return stitch(op, ps)
}

// refusal turns the errors array into the right kind of error.
func refusal(op string, errs []gqlError) error {
	for _, e := range errs {
		if e.Code == refusalCode {
			// Not a rate limit. See the comment on refusalCode.
			return needAuth("facebook does not serve %s to a signed-out caller: import a session with `fb auth import` and run it again", op)
		}
	}
	if len(errs) > 0 {
		msg := errs[0].Message
		if errs[0].Description != "" {
			msg = errs[0].Description
		}
		return fmtErr("%s: %s", op, msg)
	}
	return nil
}
