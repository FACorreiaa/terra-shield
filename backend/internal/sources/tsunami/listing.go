package tsunami

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// Entry is one message in tsunami.gov's list of recent messages
// (previous.js): a product (WMOID) of one center (TWCID) about one
// incident, at a bulletin number. A center sends several products per
// round, one per region it serves, so the Atom feeds, which hold only the
// latest message, miss most of them.
type Entry struct {
	Incident string    // the center's event id: "25210002", "togrrv"
	Center   string    // TWCID: PHEB (PTWC, Honolulu) or PAAQ (NTWC, Palmer)
	Product  string    // WMOID: WEHW40 (Hawaii), WEPA40 (Pacific threat), ...
	Bulletin int       // message number within the incident and product
	Issued   time.Time // issueTime, UTC
	Path     string    // "/events/PHEB/2025/07/29/25210002/3"
}

// Key identifies one product of one incident: it is the stored event's
// source id, kept as its level changes from bulletin to bulletin.
func (e Entry) Key() string { return e.Center + "-" + e.Incident + "-" + e.Product }

var (
	objectRe = regexp.MustCompile(`(?s)\{(.*?)\}`)
	fieldRe  = regexp.MustCompile(`'(\w+)'\s*:\s*(?:'([^']*)'|(-?[0-9.]+))`)
	// Only paths of this shape are ever fetched: the list is data, and a
	// URL taken from it must stay on tsunami.gov's events tree.
	pathRe     = regexp.MustCompile(`^/events/(PAAQ|PHEB)/\d{4}/\d{2}/\d{2}/[A-Za-z0-9]{1,16}/\d{1,3}$`)
	productRe  = regexp.MustCompile(`^[A-Z]{4}\d{2}$`)
	incidentRe = regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`)
)

// ParseListing reads previous.js. Entries that do not match the expected
// shapes are dropped; a file with objects but none valid is an error,
// since it means the format changed.
func ParseListing(body []byte) ([]Entry, error) {
	objects := objectRe.FindAllSubmatch(body, -1)
	var out []Entry
	for _, o := range objects {
		f := map[string]string{}
		for _, m := range fieldRe.FindAllSubmatch(o[1], -1) {
			v := string(m[2])
			if len(m[3]) > 0 {
				v = string(m[3])
			}
			f[string(m[1])] = v
		}
		e, ok := entry(f)
		if ok {
			out = append(out, e)
		}
	}
	if len(objects) > 0 && len(out) == 0 {
		return nil, fmt.Errorf("tsunami: none of %d listed messages is readable", len(objects))
	}
	return out, nil
}

func entry(f map[string]string) (Entry, bool) {
	n, err := strconv.Atoi(f["bulletinNumber"])
	if err != nil || n < 1 {
		return Entry{}, false
	}
	issued, err := time.Parse("2006-01-02 15:04:05.0", f["issueTime"])
	if err != nil {
		return Entry{}, false
	}
	e := Entry{
		Incident: f["id"], Center: f["TWCID"], Product: f["WMOID"],
		Bulletin: n, Issued: issued.UTC(), Path: f["URL"],
	}
	m := pathRe.FindStringSubmatch(e.Path)
	if len(m) < 2 || m[1] != e.Center || !productRe.MatchString(e.Product) || !incidentRe.MatchString(e.Incident) {
		return Entry{}, false
	}
	return e, true
}

// Latest keeps the newest bulletin of each product of each incident issued
// at or after since, oldest first.
func Latest(entries []Entry, since time.Time) []Entry {
	best := map[string]Entry{}
	for _, e := range entries {
		if e.Issued.Before(since) {
			continue
		}
		if b, ok := best[e.Key()]; !ok || e.Bulletin > b.Bulletin {
			best[e.Key()] = e
		}
	}
	out := make([]Entry, 0, len(best))
	for _, e := range best {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Issued.Equal(out[j].Issued) {
			return out[i].Issued.Before(out[j].Issued)
		}
		return out[i].Key() < out[j].Key()
	})
	return out
}
