package httpx

import "testing"

func TestNegotiate(t *testing.T) {
	const js, geo = "application/json", "application/geo+json"
	cases := map[string]string{
		"":                                       js,
		"*/*":                                    js,
		"application/json":                       js,
		"application/geo+json":                   geo,
		"application/json, application/geo+json": js, // generated clients: a tie keeps the default
		"application/geo+json, application/json": js,
		"application/geo+json;q=1, application/json;q=0.5": geo,
		"application/json;q=0.2, application/*;q=0.9":      geo, // json's own range beats application/*
		"application/*;q=0.9, */*;q=0.1":                   js,
		"text/html":                                        js, // nothing acceptable: the default, no 406
		"application/json;q=0, application/geo+json;q=0.1": geo,
		"APPLICATION/GEO+JSON":                             geo,
		"application/geo+json;q=abc":                       geo, // a bad q is ignored (q=1)
		"garbage, application/geo+json":                    geo,
	}
	for accept, want := range cases {
		if got := Negotiate(accept, js, geo); got != want {
			t.Errorf("%q: %s, want %s", accept, got, want)
		}
	}
	if Negotiate("x/y") != "" {
		t.Error("no offers should give empty")
	}
}
