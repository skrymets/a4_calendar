package timeline

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"pinplan/internal/dsl"
	"pinplan/internal/period"
)

// Days are handled as day numbers (days since 1970-01-01) in the engine.
func dayNum(t time.Time) int       { return int(t.Unix() / 86400) }
func dayDate(n int) time.Time      { return time.Unix(int64(n)*86400, 0).UTC() }
func weekday(n int) int            { return period.Weekday(dayDate(n)) }
func spanOverlap(a, b [2]int) bool { return a[0] <= b[1] && b[0] <= a[1] }

// msOcc is a visible milestone occurrence.
type msOcc struct {
	*Milestone
	t, idx, n int
	series    bool
}

// ivOcc is a visible interval occurrence; e is the last drawn day.
type ivOcc struct {
	*Interval
	id     int
	s, e   int
	idx, n int
	series bool
}

// unit is what occupies a lane: one front occurrence or one merged group.
type unit struct {
	id    int
	occ   *ivOcc // nil for a group
	grp   *Group
	s, e  int
	prio  Priority
	color string
	style Style
}

func (u *unit) members() []*ivOcc {
	if u.grp != nil {
		return u.grp.members
	}
	return []*ivOcc{u.occ}
}

func (u *unit) covers(t int) bool {
	if u.grp == nil {
		return u.s <= t && t <= u.e
	}
	for _, m := range u.grp.members {
		if m.s <= t && t <= m.e {
			return true
		}
	}
	return false
}

func (u *unit) activeMembers(t int) []*ivOcc {
	var out []*ivOcc
	for _, m := range u.members() {
		if m.s <= t && t <= m.e {
			out = append(out, m)
		}
	}
	return out
}

type engine struct {
	o           Options
	l           *Layout
	start, end  int
	ms          []*msOcc // visible, declaration then occurrence order
	iv          []*ivOcc // visible
	allMs       []*Milestone
	allIv       []*Interval
	listed      []*Interval       // numbered intervals, in key order (REQ-LB-4)
	firstDay    map[*Interval]int // their first in-period day
	back, hatch []*ivOcc
	units       []*unit
	groups      []*Group
	msByDay     map[int][]*msOcc
	countdown   map[int]*Countdown
	seen        map[string]bool     // interval labelled in an earlier column
	flowLane    map[int]map[int]int // day -> unit id -> lane
	flowUsed    map[int]int         // day -> slots in use (incl. empty ones)
	flowOver    map[int]int         // day -> units on the overflow rail
}

func newEngine(d *Decls, r *dsl.Resolved, o Options, l *Layout) *engine {
	e := &engine{
		o: o, l: l, start: dayNum(o.Start), end: dayNum(o.End),
		allMs: d.Milestones, allIv: d.Intervals,
		msByDay: map[int][]*msOcc{}, countdown: map[int]*Countdown{}, seen: map[string]bool{},
	}
	for _, x := range r.Milestones {
		m := d.Milestones[x.Index]
		if m.Hidden {
			continue
		}
		o := &msOcc{Milestone: m, t: dayNum(x.Date), idx: x.Idx, n: x.N, series: m.Rec != nil}
		e.ms = append(e.ms, o)
		e.msByDay[o.t] = append(e.msByDay[o.t], o)
	}
	for _, m := range e.ms { // REQ-MS-5; the first milestone claiming a day wins
		if m.Countdown == 0 || m.series {
			continue
		}
		for k := 1; k <= m.Countdown; k++ {
			if e.countdown[m.t-k] == nil {
				e.countdown[m.t-k] = &Countdown{N: k, Color: m.Color}
			}
		}
	}
	for i, x := range r.Intervals {
		iv := d.Intervals[x.Index]
		if iv.Hidden {
			continue
		}
		o := &ivOcc{Interval: iv, id: i, s: dayNum(x.Start), e: dayNum(x.End) - 1, idx: x.Idx, n: x.N, series: iv.Rec != nil}
		e.iv = append(e.iv, o)
		switch {
		case iv.Style == Hatch:
			e.hatch = append(e.hatch, o)
		case iv.Layer == Back:
			e.back = append(e.back, o)
		default:
			e.addUnit(o)
		}
	}
	e.numberIntervals()
	e.assignGroupKeys()
	return e
}

// numberIntervals gives the intervals the legend lists their keys
// (REQ-LB-4): 1…N by first in-period day, ties in declaration order. Any
// other interval (hidden, or with no day in the period) gets "". Keys
// belong to the page, so Build sets them on every call.
func (e *engine) numberIntervals() {
	e.firstDay = map[*Interval]int{}
	for _, o := range e.iv { // visible occurrences only
		if o.e < e.start || o.s > e.end {
			continue
		}
		t := max(o.s, e.start)
		if f, ok := e.firstDay[o.Interval]; !ok || t < f {
			e.firstDay[o.Interval] = t
		}
	}
	e.listed = nil
	for _, x := range e.allIv {
		x.Key = ""
		if _, ok := e.firstDay[x]; ok {
			e.listed = append(e.listed, x)
		}
	}
	sort.SliceStable(e.listed, func(i, j int) bool { return e.firstDay[e.listed[i]] < e.firstDay[e.listed[j]] })
	for i, x := range e.listed {
		x.Key = strconv.Itoa(i + 1)
	}
}

// addUnit makes o a unit, or adds it to its merged group (03 §2).
func (e *engine) addUnit(o *ivOcc) {
	if e.l.GroupsMerged && o.Group != "" {
		for _, u := range e.units {
			if u.grp != nil && u.grp.Name == o.Group {
				u.grp.members = append(u.grp.members, o)
				u.s, u.e = min(u.s, o.s), max(u.e, o.e)
				if o.Priority == High {
					u.prio = High
				}
				return
			}
		}
		g := &Group{Name: o.Group, Color: o.Color, members: []*ivOcc{o}}
		e.groups = append(e.groups, g)
		prio := Normal
		if o.Priority == High {
			prio = High
		}
		e.units = append(e.units, &unit{id: len(e.units), grp: g, s: o.s, e: o.e, prio: prio, color: o.Color, style: Solid})
		return
	}
	e.units = append(e.units, &unit{id: len(e.units), occ: o, s: o.s, e: o.e, prio: o.Priority, color: o.Color, style: o.Style})
}

// assignGroupKeys: the first letter, upper case; on collision the shortest
// unique prefix (REQ-GR-4), then a number if the name runs out.
func (e *engine) assignGroupKeys() {
	used := map[string]bool{}
	for _, g := range e.groups {
		name := strings.ToUpper(g.Name)
		k, n := name[:1], 1
		for used[k] && n < len(name) {
			n++
			k = name[:n]
		}
		for i := 2; used[k]; i++ {
			k = name + strconv.Itoa(i)
		}
		used[k] = true
		g.Key = k
	}
}

// ---------------------------------------------------------------- flow

// flow assigns lanes by one pass over the period (03 §4, REQ-FL-*).
func (e *engine) flow() {
	e.flowLane, e.flowUsed, e.flowOver = map[int]map[int]int{}, map[int]int{}, map[int]int{}
	const empty = -1
	var slots []int
	over := map[int]bool{}
	for t := e.start; t <= e.end; t++ {
		active := map[int]bool{}
		var act []*unit
		for _, u := range e.units {
			if u.covers(t) {
				active[u.id] = true
				act = append(act, u)
			}
		}
		for i, id := range slots {
			if id != empty && !active[id] {
				slots[i] = empty
			}
		}
		for id := range over {
			if !active[id] {
				delete(over, id)
			}
		}
		if !e.l.Weekly || weekday(t) == e.o.WeekStart {
			kept := slots[:0]
			for _, id := range slots {
				if id != empty {
					kept = append(kept, id)
				}
			}
			slots = kept
		}
		inSlots := map[int]bool{}
		busy := 0
		for _, id := range slots {
			if id != empty {
				inSlots[id] = true
				busy++
			}
		}
		var newcomers []*unit
		for _, u := range act {
			if !inSlots[u.id] && !over[u.id] {
				newcomers = append(newcomers, u)
			}
		}
		sort.SliceStable(newcomers, func(i, j int) bool {
			a, b := newcomers[i], newcomers[j]
			if a.prio != b.prio {
				return a.prio < b.prio
			}
			if a.s != b.s {
				return a.s < b.s
			}
			return a.e > b.e
		})
		for _, u := range newcomers {
			if busy >= e.l.LaneCap {
				over[u.id] = true
				continue
			}
			placed := false
			for i, id := range slots {
				if id == empty {
					slots[i], placed = u.id, true
					break
				}
			}
			if !placed {
				slots = append(slots, u.id)
			}
			busy++
		}
		for len(slots) > 0 && slots[len(slots)-1] == empty {
			slots = slots[:len(slots)-1]
		}
		lanes := map[int]int{}
		for i, id := range slots {
			if id != empty {
				lanes[id] = i
			}
		}
		e.flowLane[t], e.flowUsed[t], e.flowOver[t] = lanes, len(slots), len(over)
	}
}

// ---------------------------------------------------------------- packed

type packUnit struct {
	spans [][2]int
	s, e  int
	prio  Priority
	list  []*unit
	cs    int
}

func overlaps(a, b [][2]int) bool {
	for _, x := range a {
		for _, y := range b {
			if spanOverlap(x, y) {
				return true
			}
		}
	}
	return false
}

// pack assigns lanes in one column (03 §3): lane per unit id, the overflowed
// unit ids, and the number of lanes shown.
func (e *engine) pack(colS, colE int) (lane map[int]int, overflowed map[int]bool, shown int) {
	clip := func(s, t int) [2]int { return [2]int{max(s, colS), min(t, colE)} }
	var order []*packUnit
	byKey := map[any]*packUnit{}
	for _, u := range e.units {
		var spans [][2]int
		for _, m := range u.members() {
			if sp := clip(m.s, m.e); sp[0] <= sp[1] {
				spans = append(spans, sp)
			}
		}
		if len(spans) == 0 {
			continue
		}
		var key any = u.id
		if u.occ != nil && u.occ.series {
			key = u.occ.Interval // one pack unit per series declaration
		}
		p := byKey[key]
		if p == nil {
			p = &packUnit{s: u.s, e: u.e, prio: u.prio}
			byKey[key] = p
			order = append(order, p)
		}
		p.spans = append(p.spans, spans...)
		p.s, p.e, p.prio = min(p.s, u.s), max(p.e, u.e), min(p.prio, u.prio)
		p.list = append(p.list, u)
	}
	// a series whose occurrences overlap each other is split into sub-units
	var units []*packUnit
	for _, p := range order {
		if len(p.list) == 1 || p.list[0].occ == nil || !p.list[0].occ.series {
			units = append(units, p)
			continue
		}
		list := append([]*unit(nil), p.list...)
		sort.SliceStable(list, func(i, j int) bool { return list[i].s < list[j].s })
		var subs []*packUnit
		for _, u := range list {
			sp := [][2]int{clip(u.s, u.e)}
			var sub *packUnit
			for _, q := range subs {
				if !overlaps(q.spans, sp) {
					sub = q
					break
				}
			}
			if sub == nil {
				sub = &packUnit{s: u.s, e: u.e, prio: p.prio}
				subs = append(subs, sub)
			}
			sub.spans = append(sub.spans, sp...)
			sub.list = append(sub.list, u)
		}
		units = append(units, subs...)
	}
	for _, p := range units {
		p.cs = p.spans[0][0]
		for _, sp := range p.spans {
			p.cs = min(p.cs, sp[0])
		}
	}
	sort.SliceStable(units, func(i, j int) bool {
		a, b := units[i], units[j]
		switch {
		case a.prio != b.prio:
			return a.prio < b.prio
		case a.cs != b.cs:
			return a.cs < b.cs
		case a.s != b.s:
			return a.s < b.s
		}
		return a.e > b.e
	})
	lane, overflowed = map[int]int{}, map[int]bool{}
	var lanes [][][2]int
	for _, p := range units {
		i := 0
		for i < len(lanes) && overlaps(lanes[i], p.spans) {
			i++
		}
		if i >= e.l.LaneCap {
			for _, u := range p.list {
				overflowed[u.id] = true
			}
			continue
		}
		if i == len(lanes) {
			lanes = append(lanes, nil)
		}
		lanes[i] = append(lanes[i], p.spans...)
		for _, u := range p.list {
			lane[u.id] = i
		}
	}
	return lane, overflowed, len(lanes)
}

// ---------------------------------------------------------------- one column

// candidate is something that can get a label or key on a day.
type candidate struct {
	id   string // labelled-once identity: occurrence or group
	lane int    // back and hatch: -1; overflowed: 99
	occ  *ivOcc // nil for a group
	unit *unit
}

func (c candidate) start() int {
	if c.occ != nil {
		return c.occ.s
	}
	return c.unit.s
}

func (e *engine) column(col *Column) {
	l := e.l
	dim := period.DaysIn(col.Year, col.Month)
	first := dayNum(period.Date(col.Year, col.Month, 1))
	colS, colE := max(e.start, first), min(e.end, first+dim-1)

	var lane map[int]int
	var overflowed map[int]bool
	shown := 0
	if l.Flow {
		for t := colS; t <= colE; t++ {
			n := e.flowUsed[t]
			if e.flowOver[t] > 0 {
				n++
			}
			col.K = max(col.K, n)
		}
	} else {
		lane, overflowed, shown = e.pack(colS, colE)
		col.K = shown
		if len(overflowed) > 0 {
			col.K++
		}
	}
	col.RailW, col.Step = railSize(l.Narrow, max(col.K, 1))
	col.Labels = KeyLabels
	if !l.Narrow && col.K <= 3 {
		col.Labels = TextLabels
	}
	laneOf := func(u *unit, t int) (int, bool) {
		if l.Flow {
			n, ok := e.flowLane[t][u.id]
			return n, ok
		}
		n, ok := lane[u.id]
		return n, ok
	}

	placed := map[string]bool{}  // labelled in this column
	seriesSeen := map[any]bool{} // series titled in this column (by declaration)
	prevChip := -1
	for d := 1; d <= dim; d++ {
		t := first + d - 1
		if t < e.start || t > e.end {
			continue
		}
		day := &Day{Date: dayDate(t), OverflowLane: -1}
		col.Days[d-1] = day

		n := col.K
		if l.Flow {
			n = e.flowUsed[t]
			if e.flowOver[t] > 0 {
				n++
			}
		}
		day.Zone = zoneWidth(l.Pad, col.Step, n)

		var cands []candidate
		for _, x := range e.back {
			if x.s <= t && t <= x.e {
				if slot := len(cands); slot < 3 {
					day.Back = append(day.Back, BackLine{Slot: slot, Color: x.Color, TopCap: t == x.s, BotCap: t == x.e})
				}
				cands = append(cands, candidate{id: "o" + strconv.Itoa(x.id), lane: -1, occ: x})
			}
		}
		for _, x := range e.hatch {
			if x.s <= t && t <= x.e {
				day.Hatch = true
				cands = append(cands, candidate{id: "o" + strconv.Itoa(x.id), lane: -1, occ: x})
			}
		}

		day.Milestones = e.milestones(t, seriesSeen)
		if c := e.countdown[t]; c != nil && !l.Narrow {
			day.Countdown = c
		}

		// rails
		nOver := 0
		for _, u := range e.units {
			if !u.covers(t) {
				continue
			}
			ln, ok := laneOf(u, t)
			if !ok || (!l.Flow && overflowed[u.id]) {
				nOver++
				cands = append(cands, candidate{id: unitID(u), lane: 99, occ: u.occ, unit: u})
				continue
			}
			cands = append(cands, candidate{id: unitID(u), lane: ln, occ: u.occ, unit: u})
			day.Rails = append(day.Rails, e.rail(u, t, d, ln))
		}
		if nOver > 0 {
			day.OverflowLane = shown
			if l.Flow {
				day.OverflowLane = e.flowUsed[t]
			}
		}

		// labels and keys (03 §5)
		if col.Labels == TextLabels {
			sorted := append([]candidate(nil), cands...)
			sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].lane < sorted[j].lane })
			for _, c := range sorted {
				if placed[c.id] || day.Milestones != nil || len(day.Top) > 0 {
					continue
				}
				placed[c.id] = true
				if txt := e.labelText(c, seriesSeen); txt != "" {
					day.Top = append(day.Top, TopItem{Text: strings.ToUpper(txt), Color: candColor(c), Kind: TopLabel})
				}
			}
		} else {
			type keyItem struct {
				TopItem
				part int // 0: back and hatch, 1: the rest (REQ-LB-4)
			}
			var keys []keyItem
			add := func(c candidate, text, color string) {
				part := 1
				if c.lane < 0 {
					part = 0
				}
				keys = append(keys, keyItem{TopItem{Text: text, Color: color, Kind: TopKey}, part})
			}
			for _, c := range cands {
				colTop := !l.Narrow && t == colS && c.start() < colS && (c.occ == nil || !c.occ.series)
				if t == c.start() || colTop {
					add(c, candKey(c), candColor(c))
				}
				if c.occ == nil {
					for _, m := range c.unit.grp.members {
						if t == m.s && t != c.unit.s {
							add(c, m.Key, m.Color)
						}
					}
				}
			}
			sort.SliceStable(keys, func(i, j int) bool {
				if keys[i].part != keys[j].part {
					return keys[i].part < keys[j].part
				}
				return keyRank(keys[i].Text) < keyRank(keys[j].Text)
			})
			for _, k := range keys {
				day.Top = append(day.Top, k.TopItem)
			}
		}
		if nOver > 0 && nOver != prevChip {
			day.Top = append(day.Top, TopItem{Text: "+" + strconv.Itoa(nOver), Color: Palette["grey"], Kind: TopMore})
		}
		prevChip = nOver
	}
}

func unitID(u *unit) string {
	if u.grp != nil {
		return "g:" + u.grp.Name
	}
	return "o" + strconv.Itoa(u.occ.id)
}

func candColor(c candidate) string {
	if c.occ != nil {
		return c.occ.Color
	}
	return c.unit.color
}

func candKey(c candidate) string {
	if c.occ != nil {
		return c.occ.Key
	}
	return c.unit.grp.Key
}

// rail is unit u's rail on day t (day of month d) in lane ln (03 §3).
func (e *engine) rail(u *unit, t, d, ln int) Rail {
	r := Rail{
		Lane: ln, Color: u.color, Dashed: u.style == Dashed, JogFrom: -1,
		TopCap: t == u.s || !u.covers(t-1),
		BotCap: t == u.e || !u.covers(t+1),
	}
	if u.grp != nil {
		active := u.activeMembers(t)
		if len(active) == 1 { // one member: its own colour and style
			r.Color, r.Dashed = active[0].Color, active[0].Style == Dashed
		}
		r.Thick = len(active) > 1
		for _, m := range u.grp.members {
			if t == m.s && !r.TopCap {
				r.Ticks = append(r.Ticks, Tick{AtTop: true})
			}
			if t == m.e && !r.BotCap {
				r.Ticks = append(r.Ticks, Tick{AtTop: false})
			}
		}
	}
	if e.l.Flow && d != 1 && !r.TopCap {
		if prev, ok := e.flowLane[t-1][u.id]; ok && prev != ln {
			r.JogFrom = prev
		}
	}
	return r
}

// labelText is a candidate's text label (REQ-LB-3), "" for none.
func (e *engine) labelText(c candidate, seriesSeen map[any]bool) string {
	if c.occ == nil { // merged group: its name
		if e.seen[c.id] {
			return c.unit.grp.Name + " (cont.)"
		}
		e.seen[c.id] = true
		return c.unit.grp.Name
	}
	o := c.occ
	if o.Label == LabelNone || o.Title == "" {
		return ""
	}
	if o.series {
		decl := o.Interval.Interval
		first := !seriesSeen[decl]
		seriesSeen[decl] = true
		if o.Label == LabelEach || first {
			return o.Title + " #" + strconv.Itoa(o.idx)
		}
		return "#" + strconv.Itoa(o.idx)
	}
	firstCol := !e.seen[c.id]
	e.seen[c.id] = true
	txt := o.Title
	if o.Tentative {
		txt += " ?"
	}
	switch {
	case !firstCol:
		txt += " (cont.)"
	case o.Length == Days:
		txt += " · " + strconv.Itoa(o.e-o.s+1) + " d"
	case o.Length == Workdays:
		txt += " · " + strconv.Itoa(e.workdays(o.s, o.e)) + " wd"
	}
	return txt
}

// workdays counts the days s..e (inclusive) not in --weekend (REQ-KEY-7).
func (e *engine) workdays(s, t int) int {
	n := 0
	for d := s; d <= t; d++ {
		if !e.o.Weekend[weekday(d)] {
			n++
		}
	}
	return n
}

// milestones of day t (03 §6); seriesSeen records titled series per column.
func (e *engine) milestones(t int, seriesSeen map[any]bool) *DayMilestones {
	list := e.msByDay[t]
	if len(list) == 0 {
		return nil
	}
	dm := &DayMilestones{}
	var titled *msOcc
	for _, m := range list {
		decl := m.Milestone.Milestone
		due := m.Label == LabelEach || (m.Label == LabelFirst && !(m.series && seriesSeen[decl]))
		if m.series {
			seriesSeen[decl] = true
		}
		if due && titled == nil {
			titled = m
		}
		dm.Marks = append(dm.Marks, MarkItem{Mark: m.Mark, Color: m.Color})
		if m.Mark == Ring && dm.Ring == "" {
			dm.Ring = m.Color
		}
	}
	if titled != nil {
		dm.Title, dm.TitleColor = titled.Title, titled.Color
		if titled.Tentative {
			dm.Title += " ?"
		}
		if len(list) > 1 {
			dm.Title += " +" + strconv.Itoa(len(list)-1)
		}
	}
	return dm
}
