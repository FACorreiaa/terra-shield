package domain

// ChangeOp is what happened to an event, as the live stream reports it.
type ChangeOp string

const (
	OpCreated  ChangeOp = "created"
	OpUpdated  ChangeOp = "updated"
	OpResolved ChangeOp = "resolved"
	OpRemoved  ChangeOp = "removed" // retracted by its source; Event is nil
)

// StreamChange is one entry of the event change log: the event's state
// after its latest change, at cursor Seq. Changes are coalesced: an event
// changed twice since a cursor appears once, at its newest Seq.
type StreamChange struct {
	Seq     int64
	Op      ChangeOp
	EventID string
	Event   *EventRecord // nil when Op is OpRemoved
	Box     *BBox        // the event geometry's bounding box; nil when removed
}

// Intersects reports whether two boxes overlap. Either may cross the
// antimeridian (MinLon > MaxLon).
func (b *BBox) Intersects(o *BBox) bool {
	if b.MinLat > o.MaxLat || o.MinLat > b.MaxLat {
		return false
	}
	for _, x := range b.lonRanges() {
		for _, y := range o.lonRanges() {
			if x[0] <= y[1] && y[0] <= x[1] {
				return true
			}
		}
	}
	return false
}

func (b *BBox) lonRanges() [][2]float64 {
	if b.MinLon <= b.MaxLon {
		return [][2]float64{{b.MinLon, b.MaxLon}}
	}
	return [][2]float64{{b.MinLon, 180}, {-180, b.MaxLon}}
}
