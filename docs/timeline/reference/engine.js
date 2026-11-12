/* PinPlan timeline — reference renderer used to draw the design-guideline examples.
 * Plain JS, no dependencies. Exposes window.TL = { parse, resolve, layout, renderPage, renderCrop }.
 * Units: CSS px at 96 dpi (1 px = 0.75 pt = 0.2646 mm). The A4 page is 794 x 1123 px.
 */
(function (G) {
  'use strict';
  var DAY = 86400000;
  var MONTHS = ['JAN','FEB','MAR','APR','MAY','JUN','JUL','AUG','SEP','OCT','NOV','DEC'];
  var MSHORT = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
  var LET = 'MTWTFSS';

  // ---------------------------------------------------------------- dates
  function D(s) { var p = s.split('-'); return Date.UTC(+p[0], +p[1] - 1, +p[2]); }
  function pad(n, w) { n = String(n); while (n.length < w) n = '0' + n; return n; }
  function iso(t) { var d = new Date(t); return d.getUTCFullYear() + '-' + pad(d.getUTCMonth() + 1, 2) + '-' + pad(d.getUTCDate(), 2); }
  function dw(t) { return (new Date(t).getUTCDay() + 6) % 7; }           // 0 = Monday
  function fmt(t) { var d = new Date(t); return d.getUTCDate() + ' ' + MSHORT[d.getUTCMonth()]; }
  function addMonths(t, n) {
    var d = new Date(t), y = d.getUTCFullYear(), m = d.getUTCMonth() + n, day = d.getUTCDate();
    y += Math.floor(m / 12); m = ((m % 12) + 12) % 12;
    var md = new Date(Date.UTC(y, m + 1, 0)).getUTCDate();
    return Date.UTC(y, m, Math.min(day, md));                              // clamp to month end
  }
  function addP(t, n, u) {
    u = u.toLowerCase();
    if (u === 'd') return t + n * DAY;
    if (u === 'w') return t + 7 * n * DAY;
    if (u === 'm') return addMonths(t, n);
    return addMonths(t, 12 * n);
  }
  function periodText(p) { return p.n + ({ d: 'd', w: 'w', m: 'm', y: 'y' })[p.u]; }

  // ---------------------------------------------------------------- Date DSL (Phase 1, with recurrence)
  var ID = /^[A-Za-z_][A-Za-z0-9_]*$/;
  var PERIOD = /^([1-9][0-9]*)([dwmy])$/i;
  var RESERVED = { title: 1, every: 1, gap: 1, count: 1, until: 1 };

  function splitTop(s, ch) {
    var out = [], cur = '', q = false;
    for (var i = 0; i < s.length; i++) {
      var c = s[i];
      if (q) { cur += c; if (c === '\\') { cur += s[++i] || ''; } else if (c === '"') q = false; }
      else if (c === '"') { q = true; cur += c; }
      else if (c === ch) { out.push(cur); cur = ''; }
      else cur += c;
    }
    if (q) throw new Error('unterminated quoted value');
    out.push(cur); return out;
  }
  function unq(v) {
    v = v.trim();
    if (v[0] === '"') {
      if (v.length < 2 || v[v.length - 1] !== '"') throw new Error('bad quoted value ' + v);
      return v.slice(1, -1).replace(/\\(["\\])/g, '$1');
    }
    if (!v.length) throw new Error('empty property value');
    if (/[\s;]/.test(v)) throw new Error('unquoted value contains whitespace: ' + v);
    return v;
  }
  function parsePeriod(s) { var m = PERIOD.exec(s.trim()); return m ? { n: +m[1], u: m[2].toLowerCase() } : null; }
  function parseDate(s, ctx) {
    s = s.trim();
    var m = /^(?:(\d{4}-\d{2}-\d{2})|(today)|@([A-Za-z_][A-Za-z0-9_]*))?(?:([+-])([1-9][0-9]*)([dwmy]))?$/i.exec(s);
    if (!m || (!m[1] && !m[2] && !m[3] && !m[4])) throw new Error('invalid date expression "' + s + '"');
    var t, refs = [];
    if (m[1]) { t = D(m[1]); if (iso(t) !== m[1]) throw new Error('invalid date ' + m[1]); }
    else if (m[3]) {
      var ref = ctx.ms[m[3].toLowerCase()];
      if (!ref) throw new Error('unknown milestone @' + m[3]);
      if (ref.series) throw new Error('@' + m[3] + ' is recurring and cannot be used as a date');
      t = ref.t; refs.push(m[3].toLowerCase());
    } else t = ctx.today;
    if (m[4]) t = addP(t, m[4] === '-' ? -(+m[5]) : +m[5], m[6]);
    return { t: t, refs: refs, today: !m[1] && !m[3] };
  }
  function parseRec(props, isInterval, ctx) {
    var has = function (k) { return Object.prototype.hasOwnProperty.call(props, k); };
    if (!has('every') && !has('gap')) { if (has('count') || has('until')) throw new Error('count and until need every or gap'); return null; }
    if (has('every') && has('gap')) throw new Error('every and gap are mutually exclusive');
    if (has('gap') && !isInterval) throw new Error('gap is only valid for intervals');
    if (has('count') && has('until')) throw new Error('count and until are mutually exclusive');
    var p = parsePeriod(props.every || props.gap);
    if (!p) throw new Error('every/gap needs a positive period such as 2w');
    var r = { mode: has('every') ? 'every' : 'gap', p: p, count: null, until: null };
    if (has('count')) { if (!/^[1-9][0-9]*$/.test(props.count)) throw new Error('count must be a positive integer'); r.count = +props.count; }
    if (has('until')) r.until = parseDate(props.until, ctx).t;
    return r;
  }

  /** sources: [{name, text}] in loading order (files first, then inline milestones, then inline intervals). */
  function parse(sources, opts) {
    var ctx = { today: D(opts.today), ms: {}, ids: {} };
    var out = { milestones: [], intervals: [] };
    sources.forEach(function (src) {
      var seenInterval = false;
      src.text.split(/\r?\n/).forEach(function (raw, i) {
        var line = raw.trim();
        if (!line || line[0] === '#') return;
        try {
          var parts = splitTop(line, ';'), head = parts[0].trim(), props = {};
          parts.slice(1).forEach(function (p) {
            if (!p.trim()) return;
            var eq = p.indexOf('=');
            if (eq < 0) throw new Error('property without value: "' + p.trim() + '"');
            var k = p.slice(0, eq).trim().toLowerCase();
            if (!ID.test(k)) throw new Error('invalid property key "' + k + '"');
            if (Object.prototype.hasOwnProperty.call(props, k)) throw new Error('duplicate property "' + k + '"');
            props[k] = unq(p.slice(eq + 1));
          });
          var isInterval = splitTop(head, '/').length > 1;
          var name = null, expr = head, nm = /^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/.exec(head);
          if (nm) { name = nm[1]; expr = nm[2]; }
          if (!isInterval) {
            if (!name) throw new Error('a milestone needs a name');
            if (seenInterval && src.kind !== 'inline') throw new Error('milestones must be declared before the first interval');
          } else seenInterval = true;
          if (name) {
            var lk = name.toLowerCase();
            if (lk === 'today') throw new Error('"today" is reserved');
            if (ctx.ids[lk]) throw new Error('duplicate identifier "' + name + '"');
            ctx.ids[lk] = true;
          }
          var rec = parseRec(props, isInterval, ctx);
          var where = src.name + ':' + (i + 1);
          if (!isInterval) {
            var dt = parseDate(expr, ctx);
            out.milestones.push({ id: name, t: dt.t, refs: dt.refs, rec: rec, props: props, where: where, expr: expr });
            ctx.ms[name.toLowerCase()] = rec ? { series: true } : { t: dt.t };
          } else {
            var lr = splitTop(expr, '/');
            if (lr.length !== 2) throw new Error('an interval needs exactly one "/"');
            // the END is the last day (inclusive); "<END" is the exclusive end (DSL §5)
            var rhs = lr[1].trim(), excl = rhs.charAt(0) === '<';
            if (lr[0].trim().charAt(0) === '<') throw new Error('the start is always included; < marks an exclusive end');
            if (excl) rhs = rhs.slice(1);
            var endOf = function (t) { return excl ? t : t + DAY; };
            var pl = parsePeriod(lr[0]), pr = parsePeriod(rhs), s, e, dur, refs = [];
            if (pl && pr) throw new Error('only one side of a range may be a duration');
            if (pr && excl) throw new Error('a duration has no end to exclude');
            if (pr) { var a = parseDate(lr[0], ctx); s = a.t; refs = a.refs; e = addP(s, pr.n, pr.u); dur = { kind: 'start', p: pr }; }
            else if (pl) { var b = parseDate(rhs, ctx); e = endOf(b.t); refs = b.refs; s = addP(e, -pl.n, pl.u); dur = { kind: 'end', p: pl }; }
            else { var c1 = parseDate(lr[0], ctx), c2 = parseDate(rhs, ctx); s = c1.t; e = endOf(c2.t); refs = c1.refs.concat(c2.refs); dur = { kind: 'dates', days: Math.round((e - s) / DAY) }; }
            if (!(s < e)) throw new Error('interval start must be before its end');
            if (rec && rec.until !== null && rec.until < s) throw new Error('until is before the first occurrence');
            out.intervals.push({ id: name, s: s, e: e, dur: dur, refs: refs, rec: rec, props: props, where: where, expr: expr });
          }
          if (rec && rec.until !== null && !isInterval && rec.until < out.milestones[out.milestones.length - 1].t) throw new Error('until is before the first occurrence');
        } catch (err) { throw new Error(src.name + ':' + (i + 1) + ': ' + err.message); }
      });
    });
    return out;
  }

  // ---------------------------------------------------------------- recurrence expansion
  function expand(model, w0, w1) {
    var ms = [], iv = [];
    model.milestones.forEach(function (m, mi) {
      if (!m.rec) { ms.push({ decl: m, di: mi, t: m.t, idx: 1, n: 1 }); return; }
      var occ = [], r = m.rec;
      for (var i = 0; i < 5000; i++) {
        var t = addP(m.t, i * r.p.n, r.p.u);                              // always from the original date
        if (r.count !== null && i >= r.count) break;
        if (r.until !== null && t > r.until) break;
        if (t > w1) break;
        occ.push({ decl: m, di: mi, t: t, idx: i + 1 });
      }
      occ.forEach(function (o) { o.n = occ.length; ms.push(o); });
    });
    model.intervals.forEach(function (x, xi) {
      if (!x.rec) { iv.push({ decl: x, di: xi, s: x.s, e: x.e, idx: 1, n: 1 }); return; }
      var occ = [], r = x.rec, s = x.s, e = x.e;
      for (var i = 0; i < 5000; i++) {
        if (i > 0) {
          s = r.mode === 'every' ? addP(x.s, i * r.p.n, r.p.u) : addP(e, r.p.n, r.p.u);
          e = x.dur.kind === 'dates' ? s + x.dur.days * DAY : addP(s, x.dur.p.n, x.dur.p.u);
        }
        if (r.count !== null && i >= r.count) break;
        if (r.until !== null && s > r.until) break;
        if (s > w1) break;
        occ.push({ decl: x, di: xi, s: s, e: e, idx: i + 1 });
      }
      occ.forEach(function (o) { o.n = occ.length; iv.push(o); });
    });
    return { ms: ms, iv: iv };
  }

  // ---------------------------------------------------------------- renderer vocabulary
  var COLORS = { ink: '#1F1F1F', red: '#8E1B1B', teal: '#2E6E62', violet: '#5E4A8F', orange: '#A04A14', steel: '#3D6A99', ochre: '#8A6D1F', magenta: '#9C3D6B', grey: '#7A7A7A', gray: '#7A7A7A' };
  function colorOf(v, dflt) {
    if (!v) return dflt;
    var k = v.toLowerCase();
    if (COLORS[k]) return COLORS[k];
    if (/^#[0-9a-f]{6}$/i.test(v)) return v.toUpperCase();
    throw new Error('unknown color "' + v + '"');
  }
  function oneOf(v, list, dflt, key) {
    if (v === undefined) return dflt;
    var k = v.toLowerCase();
    if (list.indexOf(k) < 0) throw new Error(key + '=' + v + ': expected one of ' + list.join(', '));
    return k;
  }
  var GLYPH = { diamond: '\u25C6', star: '\u2605', dot: '\u25CF', ring: '' };
  var HATCH = 'repeating-linear-gradient(135deg, rgba(31,31,31,0.16) 0 1px, transparent 1px 5px)';
  function stripes(c, on, off) { return 'repeating-linear-gradient(to bottom, ' + c + ' 0 ' + on + 'px, transparent ' + on + 'px ' + (on + off) + 'px)'; }
  function workdays(s, eIncl) { var n = 0; for (var t = s; t <= eIncl; t += DAY) if (dw(t) < 5) n++; return n; }

  /** Validates renderer keys and builds drawable items. */
  function resolve(model, opts) {
    var start = D(opts.start), end = D(opts.end);
    var X = expand(model, start, end);
    var keyN = 0;
    model.intervals.forEach(function (x) { x.key = String(++keyN); });
    var ms = X.ms.map(function (o) {
      var p = o.decl.props;
      return {
        decl: o.decl, t: o.t, idx: o.idx, n: o.n, series: !!o.decl.rec,
        title: p.title || o.decl.id, color: colorOf(p.color, '#1F1F1F'),
        mark: oneOf(p.mark, ['diamond', 'star', 'dot', 'ring'], 'diamond', 'mark'),
        countdown: p.countdown ? +p.countdown : 0,
        hidden: oneOf(p.hide, ['yes', 'no'], 'no', 'hide') === 'yes',
        label: oneOf(p.label, ['first', 'each', 'none'], 'first', 'label')
      };
    });
    var iv = X.iv.map(function (o, i) {
      var p = o.decl.props;
      var status = oneOf(p.status, ['tentative', 'confirmed'], 'confirmed', 'status');
      var style = oneOf(p.style, ['solid', 'dashed', 'hatch'], 'solid', 'style');
      if (status === 'tentative' && style === 'solid') style = 'dashed';
      return {
        decl: o.decl, uid: 'o' + i, key: o.decl.key, s: o.s, e: o.e - DAY, idx: o.idx, n: o.n, series: !!o.decl.rec,
        title: p.title || o.decl.id || '', color: colorOf(p.color, '#1F1F1F'), style: style, tentative: status === 'tentative',
        length: oneOf(p.length, ['days', 'workdays'], null, 'length'),
        layer: oneOf(p.layer, ['front', 'back'], 'front', 'layer'),
        priority: oneOf(p.priority, ['high', 'normal', 'low'], 'normal', 'priority'),
        group: p.group ? p.group.toLowerCase() : null,
        hidden: oneOf(p.hide, ['yes', 'no'], 'no', 'hide') === 'yes',
        label: oneOf(p.label, ['first', 'each', 'none'], 'first', 'label')
      };
    });
    return { ms: ms.filter(function (m) { return !m.hidden; }), iv: iv.filter(function (x) { return !x.hidden; }), all: { ms: ms, iv: iv }, start: start, end: end };
  }

  // ---------------------------------------------------------------- geometry
  var A4 = { w: 794, h: 1123, margin: 38 };
  function geometry(ncols) {
    var colW = (A4.w - 2 * A4.margin) / ncols;
    var narrow = colW < 100;
    return {
      colW: colW, narrow: narrow,
      padL: narrow ? 3 : 4, labelW: narrow ? 20 : 30, x0: narrow ? 24 : 36,
      pad: narrow ? 2 : 3,
      cap: narrow ? 3 : (colW >= 120 ? 6 : 4),
      textLabelMaxLanes: narrow ? 0 : 3
    };
  }
  function railSize(g, k) {
    if (!g.narrow) return k <= 4 ? { w: 3, step: 5 } : k <= 7 ? { w: 2, step: 3.5 } : { w: 1.5, step: 3 };
    return k <= 3 ? { w: 2, step: 3.5 } : k <= 5 ? { w: 1.5, step: 2.5 } : { w: 1, step: 2 };
  }
  var JOG = 6;

  // ---------------------------------------------------------------- layout
  function covers(u, t) { return u.members ? u.members.some(function (m) { return t >= m.s && t <= m.e; }) : t >= u.s && t <= u.e; }
  function nActive(u, t) { return u.members ? u.members.filter(function (m) { return t >= m.s && t <= m.e; }).length : (covers(u, t) ? 1 : 0); }
  function prio(u) { return u.priority === 'high' ? 0 : u.priority === 'low' ? 2 : 1; }

  function layout(R, opts) {
    var start = R.start, end = R.end, sdw = dw(start);
    var months = [];
    var y = new Date(start).getUTCFullYear(), mo = new Date(start).getUTCMonth();
    var ey = new Date(end).getUTCFullYear(), em = new Date(end).getUTCMonth();
    while (y < ey || (y === ey && mo <= em)) { months.push({ y: y, mo: mo }); mo++; if (mo > 11) { mo = 0; y++; } }
    var g = geometry(months.length);
    var cap = opts.cap || g.cap;
    var merge = opts.mergeGroups === 'on' || (opts.mergeGroups !== 'off' && g.narrow);
    var flow = opts.layout === 'flow';

    var back = R.iv.filter(function (x) { return x.layer === 'back' && x.style !== 'hatch'; });
    var hatch = R.iv.filter(function (x) { return x.style === 'hatch'; });
    var fore = R.iv.filter(function (x) { return x.layer !== 'back' && x.style !== 'hatch'; });
    // group merge: members share one unit
    var units = [], groups = {};
    fore.forEach(function (x) {
      if (merge && x.group) {
        var G0 = groups[x.group];
        if (!G0) {
          G0 = groups[x.group] = { uid: 'g:' + x.group, isGroup: true, key: null, title: x.group, color: x.color, s: x.s, e: x.e, members: [], priority: 'normal', style: 'solid', series: false, decl: null };
          units.push(G0);
        }
        G0.s = Math.min(G0.s, x.s); G0.e = Math.max(G0.e, x.e); G0.members.push(x);
        if (x.priority === 'high') G0.priority = 'high';
      } else units.push(x);
    });
    // group keys: first letter, extended on collision
    var used = {};
    Object.keys(groups).forEach(function (name) {
      var k = name[0].toUpperCase(), n = 1;
      while (used[k] && n < name.length) { n++; k = name.slice(0, n).toUpperCase(); }
      used[k] = true; groups[name].key = k;
    });

    var ms = {};
    R.ms.forEach(function (m) { (ms[m.t] = ms[m.t] || []).push(m); });
    var cd = {};
    R.ms.forEach(function (m) { if (m.countdown && !m.series) for (var k = 1; k <= m.countdown; k++) cd[m.t - k * DAY] = cd[m.t - k * DAY] || { txt: 'T\u2212' + k, color: m.color }; });

    // ---- flow: one pass over all days; slots keep order; ended slots are removed (daily) or kept until Monday (weekly)
    var fl = {}, fn = {}, fo = {};
    if (flow) {
      var slots = [], over = {};
      for (var t = start; t <= end; t += DAY) {
        var act = units.filter(function (u) { return covers(u, t); });
        var on = {}; act.forEach(function (u) { on[u.uid] = true; });
        slots = slots.map(function (k) { return k && on[k] ? k : null; });
        Object.keys(over).forEach(function (k) { if (!on[k]) delete over[k]; });
        if (opts.compact !== 'weekly' || dw(t) === 0) slots = slots.filter(Boolean);
        var have = {}; slots.forEach(function (k) { if (k) have[k] = true; });
        act.filter(function (u) { return !have[u.uid] && !over[u.uid]; })
          .sort(function (a, b) { return (prio(a) - prio(b)) || (a.s - b.s) || (b.e - a.e); })
          .forEach(function (u) {
            if (slots.filter(Boolean).length >= cap) { over[u.uid] = true; return; }
            var h = slots.indexOf(null); if (h >= 0) slots[h] = u.uid; else slots.push(u.uid);
          });
        while (slots.length && !slots[slots.length - 1]) slots.pop();
        var mp = {}; slots.forEach(function (k, i) { if (k) mp[k] = i; });
        fl[t] = mp; fn[t] = slots.length; fo[t] = Object.keys(over).length;
      }
    }

    var seen = {}, cols = [];
    months.forEach(function (M, ci) {
      var dim = new Date(Date.UTC(M.y, M.mo + 1, 0)).getUTCDate();
      var colS = Math.max(start, Date.UTC(M.y, M.mo, 1)), colE = Math.min(end, Date.UTC(M.y, M.mo, dim));
      // ---- packed: per-column greedy; a series packs as one unit within the column
      // A pack unit is one interval occurrence, one whole series, or one merged group. It occupies a lane only on
      // the days it is drawn (its spans), so a series or group can share its lane with others during its gaps.
      var clip = function (x) { return [Math.max(x.s, colS), Math.min(x.e, colE)]; };
      var packUnits = [], byKey = {};
      units.forEach(function (u) {
        var spans = (u.members || [u]).map(clip).filter(function (sp) { return sp[0] <= sp[1]; });
        if (!spans.length) return;
        var k = u.series ? 'series:' + u.key : u.uid;
        var P = byKey[k];
        if (!P) { P = byKey[k] = { k: k, spans: [], s: u.s, e: u.e, pr: prio(u), list: [] }; packUnits.push(P); }
        P.spans = P.spans.concat(spans); P.s = Math.min(P.s, u.s); P.e = Math.max(P.e, u.e); P.pr = Math.min(P.pr, prio(u));
        P.list.push(u);
      });
      var overlaps = function (A, B) { return A.some(function (a) { return B.some(function (b) { return !(a[1] < b[0] || a[0] > b[1]); }); }); };
      // occurrences of one series that overlap each other are split into sub-units on adjacent lanes
      var finalUnits = [];
      packUnits.forEach(function (P) {
        if (P.list.length === 1 || !P.list[0].series) { finalUnits.push(P); return; }
        var subs = [];
        P.list.slice().sort(function (a, b) { return a.s - b.s; }).forEach(function (u) {
          var sp = [clip(u)];
          var sub = subs.filter(function (q) { return !overlaps(q.spans, sp); })[0];
          if (!sub) { sub = { k: P.k + ':' + subs.length, spans: [], s: u.s, e: u.e, pr: P.pr, list: [] }; subs.push(sub); }
          sub.spans = sub.spans.concat(sp); sub.list.push(u);
        });
        subs.forEach(function (q) { finalUnits.push(q); });
      });
      finalUnits.forEach(function (P) { P.cs = Math.min.apply(null, P.spans.map(function (sp) { return sp[0]; })); });
      finalUnits.sort(function (a, b) { return (a.pr - b.pr) || (a.cs - b.cs) || (a.s - b.s) || (b.e - a.e); });
      // lowest lane whose occupied spans do not overlap the unit's spans; units beyond the cap go to the overflow rail
      var ends = [], laneOf = {}, overflowed = {};
      finalUnits.forEach(function (P) {
        var l = 0;
        while (ends[l] && overlaps(ends[l], P.spans)) l++;
        if (l >= cap) { P.list.forEach(function (u) { overflowed[u.uid] = true; }); return; }
        ends[l] = (ends[l] || []).concat(P.spans); P.list.forEach(function (u) { laneOf[u.uid] = l; });
      });
      var lanesShown = ends.length, hasOverflow = Object.keys(overflowed).length > 0;
      var k = lanesShown + (hasOverflow ? 1 : 0);
      if (flow) { k = 0; for (var q = colS; q <= colE; q += DAY) k = Math.max(k, fn[q] + (fo[q] ? 1 : 0)); }
      var rs = railSize(g, Math.max(k, 1));
      var zoneCol = k ? g.pad + k * rs.step + g.pad : g.pad + 1;
      var labelMode = g.narrow ? 'keys' : (k <= g.textLabelMaxLanes ? 'text' : 'keys');
      var placed = {}, serCol = {}, prevChip = -1, cells = [];

      for (var d = 1; d <= 31; d++) {
        var c = { show: d <= dim, d: d };
        if (d > dim) { cells.push(c); continue; }
        var t = Date.UTC(M.y, M.mo, d), wd = dw(t), we = wd >= 5;
        c.num = d; c.letter = LET[wd]; c.inPeriod = t >= start && t <= end; c.weekend = we;
        c.rects = []; c.top = []; c.ms = null; c.cd = null; c.ring = null;
        if (!c.inPeriod) { cells.push(c); continue; }
        var idx = Math.round((t - start) / DAY), wk = Math.floor((idx + sdw) / 7) + 1;
        c.tiny = pad(idx + 1, 3) + ((wd === 0 || idx === 0 || d === 1) ? '/' + pad(wk, 2) : '');
        c.bg = we ? '#E6E6E6' : '#FFFFFF';
        if (hatch.some(function (x) { return t >= x.s && t <= x.e; })) c.bg = HATCH + ', ' + c.bg;

        var nToday = flow ? fn[t] + (fo[t] ? 1 : 0) : k;
        var zone = flow ? (nToday ? g.pad + nToday * rs.step + g.pad : g.pad + 1) : zoneCol;
        c.zone = zone;
        c.rects.push({ l: g.x0, r: zone, t: null, b: 5, h: 0, bb: '1px dotted #B4B4B4' });       // note line

        // back layer: margin lines between date and note space, no fill
        var bi = 0, backAct = [];
        back.forEach(function (x) {
          if (t < x.s || t > x.e) return;
          backAct.push({ u: x, lane: -1 });
          if (bi < 3) c.rects.push({ l: g.x0 - 2 + bi * 3, w: 1, t: t === x.s ? 4 : 0, b: t === x.e ? 4 : 0, bg: bi === 0 ? x.color : stripes(x.color, 2, 2) });
          bi++;
        });

        // milestones
        var list = ms[t];
        if (list) {
          // several milestones on one day: all marks are drawn; the title is the first one that is due a title, plus "+N"
          var titled = list.filter(function (m) {
            var show = m.label === 'each' || (m.label === 'first' && !(m.series && serCol['m' + m.decl.where]));
            if (m.series) serCol['m' + m.decl.where] = true;
            return show;
          });
          var m0 = titled[0] || list[0];
          c.ms = { glyph: list.map(function (m) { return GLYPH[m.mark]; }).join(''), color: m0.color, title: titled.length ? titled[0].title + (list.length > 1 ? ' +' + (list.length - 1) : '') : '' };
          var ringM = list.filter(function (m) { return m.mark === 'ring'; })[0];
          if (ringM) c.ring = ringM.color;
        }
        if (cd[t] && !g.narrow) c.cd = cd[t];

        // rails
        var act = units.filter(function (u) { return covers(u, t); });
        var ov = act.filter(function (u) { return flow ? fl[t][u.uid] === undefined : overflowed[u.uid]; });
        act.forEach(function (u) {
          var lane = flow ? fl[t][u.uid] : laneOf[u.uid];
          if (lane === undefined) return;
          var ro = g.pad + lane * rs.step;
          var top = t === u.s || !covers(u, t - DAY), bot = t === u.e || !covers(u, t + DAY);
          var ww = (u.members && nActive(u, t) > 1) ? rs.w + 1 : rs.w;
          var bg = u.style === 'dashed' ? stripes(u.color, 3, 2) : u.color;
          if (u.members) {                                   // one member active: draw it in its own style
            var am = u.members.filter(function (m) { return t >= m.s && t <= m.e; });
            if (am.length === 1) bg = am[0].style === 'dashed' ? stripes(am[0].color, 3, 2) : am[0].color;
          }
          var jog = 0;
          if (flow && d !== 1 && !top && fl[t - DAY] && fl[t - DAY][u.uid] !== undefined && fl[t - DAY][u.uid] !== lane) {
            var ro0 = g.pad + fl[t - DAY][u.uid] * rs.step;
            c.rects.push({ r: ro, w: rs.w, t: 0, b: (opts.rowH || 28) - JOG, bg: u.color, tf: 'skewX(' + (Math.atan((ro0 - ro) / JOG) * 180 / Math.PI).toFixed(2) + 'deg)' });
            jog = JOG;
          }
          c.rects.push({ r: ro - (ww - rs.w), w: ww, t: top ? 4 : jog, b: bot ? 4 : 0, bg: bg, rad: 1 });
          if (u.members) u.members.forEach(function (m) {            // ticks where a member starts/ends inside a running group rail
            if (t === m.s && !top) c.rects.push({ r: ro - 1.5, w: rs.w + 3, t: 4, b: null, h: 1.5, bg: u.color });
            if (t === m.e && !bot) c.rects.push({ r: ro - 1.5, w: rs.w + 3, t: null, b: 4, h: 1.5, bg: u.color });
          });
        });
        var chip = null;
        if (ov.length) {
          var lanesToday = flow ? fn[t] : lanesShown;
          c.rects.push({ r: g.pad + lanesToday * rs.step, w: 1.5, t: 0, b: 0, bg: stripes('#7A7A7A', 1.5, 1.5) });
          if (ov.length !== prevChip) chip = { txt: '+' + ov.length, color: '#7A7A7A' };
        }
        prevChip = ov.length;

        // labels
        var lab = backAct.concat(hatch.filter(function (x) { return t >= x.s && t <= x.e; }).map(function (x) { return { u: x, lane: -1 }; }))
          .concat(act.map(function (u) { return { u: u, lane: flow ? (fl[t][u.uid] === undefined ? 99 : fl[t][u.uid]) : (laneOf[u.uid] === undefined ? 99 : laneOf[u.uid]) }; }));
        if (labelMode === 'text') {
          lab.slice().sort(function (a, b) { return a.lane - b.lane; }).forEach(function (o) {
            var u = o.u;
            if (placed[u.uid] || c.ms || c.top.length) return;
            if (u.label === 'none') { placed[u.uid] = true; return; }
            placed[u.uid] = true;
            var txt;
            if (u.series) {
              var sk = 'i' + u.decl.where;
              txt = (u.label === 'each' || !serCol[sk]) ? u.title + ' #' + u.idx : '#' + u.idx;
              serCol[sk] = true;
            } else {
              var firstCol = !seen[u.uid]; seen[u.uid] = true;
              txt = u.title + (u.tentative ? ' ?' : '');
              if (!firstCol) txt += ' (cont.)';
              else if (u.length === 'days') txt += ' \u00B7 ' + (Math.round((u.e - u.s) / DAY) + 1) + ' d';
              else if (u.length === 'workdays') txt += ' \u00B7 ' + workdays(u.s, u.e) + ' wd';
            }
            if (txt) c.top.push({ txt: txt.toUpperCase(), color: u.color, text: true });
          });
        } else {
          lab.forEach(function (o) {
            var u = o.u;
            var colTop = !g.narrow && t === colS && u.s < colS && !u.series;
            if (t === u.s || colTop) c.top.push({ txt: u.key, color: u.color });
            if (u.members) u.members.forEach(function (m) { if (t === m.s && t !== u.s) c.top.push({ txt: m.key, color: m.color }); });
          });
        }
        if (chip) c.top.push(chip);
        cells.push(c);
      }
      var name = MONTHS[M.mo] + ((months.length > 1 && new Date(start).getUTCFullYear() !== new Date(end).getUTCFullYear() && (ci === 0 || ci === months.length - 1 || M.mo === 0)) ? " '" + String(M.y).slice(2) : '');
      cols.push({ name: name, cells: cells, lanes: k, labelMode: labelMode });
    });

    var days = Math.round((end - start) / DAY) + 1;
    return { cols: cols, g: g, groups: groups, merge: merge, cap: cap, days: days, weeks: Math.floor((days - 1 + sdw) / 7) + 1, flow: flow, compact: opts.compact || 'daily' };
  }

  // ---------------------------------------------------------------- legend
  function legend(R, L) {
    var items = [], done = {};
    R.all.ms.forEach(function (m) {
      if (m.hidden) return;
      var k = m.decl.where; if (done[k]) return; done[k] = true;
      var occ = R.ms.filter(function (z) { return z.decl === m.decl && z.t >= R.start && z.t <= R.end; });
      if (!occ.length) return;
      var when = fmt(occ[0].t);
      if (m.series) when += '\u2013' + fmt(occ[occ.length - 1].t) + ' \u00B7 every ' + periodText(m.decl.rec.p) + ' \u00B7 \u00D7' + occ.length;
      items.push({ sort: occ[0].t, key: GLYPH[m.mark] || '\u25EF', title: m.title, when: when, color: m.color, sw: 'transparent' });
    });
    R.all.iv.forEach(function (x) {
      if (x.hidden) return;
      var k = x.decl.where; if (done[k]) return; done[k] = true;
      var occ = R.iv.filter(function (z) { return z.decl === x.decl && z.e >= R.start && z.s <= R.end; });
      if (!occ.length) return;
      var when = fmt(occ[0].s) + '\u2013' + fmt(occ[x.series ? occ.length - 1 : 0].e);
      if (x.series) when += ' \u00B7 ' + (x.decl.rec.mode === 'every' ? 'every ' : 'gap ') + periodText(x.decl.rec.p) + ' \u00B7 \u00D7' + occ.length;
      else if (x.length === 'workdays') when += ' \u00B7 ' + workdays(x.s, x.e) + ' wd';
      else when += ' \u00B7 ' + (Math.round((x.e - x.s) / DAY) + 1) + ' d';
      var sw = x.style === 'hatch' ? HATCH + ', #FFFFFF' : x.style === 'dashed' ? stripes(x.color, 2, 1.5) : x.color;
      items.push({ sort: occ[0].s, key: x.key, title: (x.title || '(untitled)') + (x.tentative ? ' ?' : '') + (x.layer === 'back' ? ' \u00B7 back' : '') + (L.merge && x.group ? ' \u00B7 ' + L.groups[x.group].key : ''), when: when, color: x.color, sw: sw, hatch: x.style === 'hatch', thin: x.layer === 'back' });
    });
    items.sort(function (a, b) { return a.sort - b.sort; });
    if (L.merge) Object.keys(L.groups).forEach(function (name) {
      var G0 = L.groups[name];
      items.push({ sort: 9e15, key: G0.key, title: name, when: '= ' + G0.members.map(function (m) { return m.key; }).filter(function (v, i, a) { return a.indexOf(v) === i; }).join(' + '), color: G0.color, sw: G0.color });
    });
    return items;
  }

  // ---------------------------------------------------------------- HTML
  function esc(s) { return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'); }
  function rectHTML(r, rowH) {
    var s = 'position:absolute;';
    if (r.l !== undefined) s += 'left:' + r.l + 'px;';
    if (r.r !== undefined) s += 'right:' + r.r + 'px;';
    if (r.w !== undefined) s += 'width:' + r.w + 'px;';
    if (r.t !== null && r.t !== undefined) s += 'top:' + r.t + 'px;';
    if (r.b !== null && r.b !== undefined) s += 'bottom:' + r.b + 'px;';
    if (r.h !== undefined) s += 'height:' + r.h + 'px;';
    if (r.bg) s += 'background:' + r.bg + ';';
    if (r.bb) s += 'border-bottom:' + r.bb + ';';
    if (r.rad) s += 'border-radius:' + r.rad + 'px;';
    if (r.tf) s += 'transform:' + r.tf + ';transform-origin:bottom left;';
    return '<div style="' + s + '"></div>';
  }
  function cellHTML(c, g, rowH) {
    var h = '<div class="cell" style="height:' + rowH + 'px;background:' + (c.show && c.inPeriod ? c.bg : '#FFFFFF') + '">';
    if (!c.show) return h + '</div>';
    if (c.inPeriod) c.rects.forEach(function (r) { h += rectHTML(r, rowH); });
    var faint = !c.inPeriod;
    h += '<div class="lab" style="left:' + g.padL + 'px;width:' + g.labelW + 'px">';
    h += '<div class="tiny">' + (c.tiny || '&nbsp;') + '</div>';
    h += '<div class="main"><span class="num" style="color:' + (faint ? '#C8C8C8' : '#1F1F1F') + ';border-color:' + (c.ring || 'transparent') + '">' + c.num + '</span>' +
      '<span style="color:' + (faint ? '#C8C8C8' : (c.weekend ? '#8E1B1B' : '#1A3A7A')) + '">' + c.letter + '</span>' +
      (c.ms && !g.narrow ? '<span class="glyph" style="color:' + c.ms.color + '">' + c.ms.glyph + '</span>' : '') + '</div></div>';
    if (c.inPeriod) {
      if (c.cd) h += '<div class="cd" style="left:' + (g.x0 + 2) + 'px;color:' + c.cd.color + '">' + c.cd.txt + '</div>';
      if (c.top.length) {
        h += '<div class="top' + (g.narrow ? ' narrow' : '') + '" style="left:' + (g.x0 + 2) + 'px;right:' + (c.zone + 1) + 'px">';
        c.top.forEach(function (p) { h += '<span style="color:' + p.color + '"' + (p.text ? ' class="t"' : '') + '>' + esc(p.txt) + '</span>'; });
        h += '</div>';
      }
      if (c.ms) {
        if (g.narrow) h += '<div class="msg" style="left:' + (g.x0 + 1) + 'px;color:' + c.ms.color + '">' + c.ms.glyph + '</div>';
        else if (c.ms.title) h += '<div class="mst" style="left:' + (g.x0 + 2) + 'px;right:' + (c.zone + 1) + 'px;color:' + c.ms.color + '">' + esc(c.ms.title) + '</div>';
      }
    }
    return h + '</div>';
  }
  function legendHTML(items, narrow) {
    var h = '<div class="legend" style="grid-template-columns:repeat(4,minmax(0,1fr))">';
    items.forEach(function (it) {
      h += '<div class="li"><span class="sw" style="background:' + it.sw + (it.hatch ? ';outline:0.5px solid #8C8C8C;width:8px' : '') + (it.thin ? ';width:1px' : '') + '"></span>' +
        '<span class="lk" style="color:' + it.color + '">' + esc(it.key) + '</span>' +
        '<span class="lt" style="color:' + it.color + '">' + esc(it.title) + '</span><span class="lw">' + esc(it.when) + '</span></div>';
    });
    return h + '</div>';
  }
  function periodLabel(start, end) {
    var s = new Date(start), e = new Date(end);
    if (s.getUTCMonth() === 0 && s.getUTCDate() === 1 && e.getUTCMonth() === 11 && e.getUTCDate() === 31 && s.getUTCFullYear() === e.getUTCFullYear()) return String(s.getUTCFullYear());
    var f = function (t) { var d = new Date(t); return pad(d.getUTCDate(), 2) + ' ' + MONTHS[d.getUTCMonth()] + ' ' + d.getUTCFullYear(); };
    return f(start) + ' \u2013 ' + f(end);
  }

  function build(spec) {
    var sources = spec.sources || [{ name: spec.file || 'example.dates', text: spec.dates }];
    var model = parse(sources, { today: spec.today || spec.start });
    var R = resolve(model, spec);
    var L = layout(R, spec);
    return { model: model, R: R, L: L };
  }

  function renderPage(spec) {
    var B = build(spec), L = B.L, g = L.g;
    var items = spec.legend === 'off' ? [] : legend(B.R, L);
    var legendRows = Math.ceil(items.length / 4);
    var legendH = items.length ? 8 + legendRows * 10 : 0;
    var avail = A4.h - 2 * A4.margin - 22 - 8 - 22 - legendH - 16;
    var rowH = Math.floor(avail / 31 * 10) / 10;
    spec.rowH = rowH; L = layout(B.R, spec);
    var h = '<div class="page" style="width:' + A4.w + 'px;height:' + A4.h + 'px;padding:' + A4.margin + 'px">';
    h += '<div class="title"><span>' + esc(spec.title || 'Calendar') + '</span><span class="per">' + periodLabel(B.R.start, B.R.end) + '</span></div>';
    h += '<div class="band" style="grid-template-columns:repeat(' + L.cols.length + ',minmax(0,1fr))">' + L.cols.map(function (c) { return '<div>' + c.name + '</div>'; }).join('') + '</div>';
    h += '<div class="grid" style="grid-template-columns:repeat(' + L.cols.length + ',minmax(0,1fr))">';
    L.cols.forEach(function (col, i) { h += '<div class="col"' + (i ? '' : ' style="border-left:none"') + '>' + col.cells.map(function (c) { return cellHTML(c, g, rowH); }).join('') + '</div>'; });
    h += '</div>';
    if (items.length) h += legendHTML(items, g.narrow);
    var nm = B.model.milestones.length, ni = B.model.intervals.length;
    h += '<div class="foot">' + L.days + ' days \u00B7 week numbering: relative (' + L.weeks + ' weeks) \u00B7 timeline: ' + esc(spec.file || 'example.dates') + ' \u2014 ' + nm + ' milestone' + (nm === 1 ? '' : 's') + ', ' + ni + ' interval' + (ni === 1 ? '' : 's') + ' \u00B7 layout: ' + (L.flow ? 'flow' + (L.compact === 'weekly' ? ' (weekly)' : '') : 'packed') + '</div>';
    return h + '</div>';
  }

  /** A crop: chosen columns (0-based indexes) and day rows, optionally with numbered callouts. */
  function renderCrop(spec) {
    var rowH = spec.rowH = spec.rowH || 28;
    var B = build(spec), L = B.L, g = L.g;
    var colIdx = spec.cols || [0], from = spec.from || 1, to = spec.to || 31;
    var h = '<div class="crop">';
    if (spec.header !== false) h += '<div class="band" style="grid-template-columns:repeat(' + colIdx.length + ',' + g.colW + 'px)">' + colIdx.map(function (i) { return '<div>' + L.cols[i].name + '</div>'; }).join('') + '</div>';
    h += '<div class="grid" style="grid-template-columns:repeat(' + colIdx.length + ',' + g.colW + 'px)">';
    colIdx.forEach(function (i, n) {
      h += '<div class="col"' + (n ? '' : ' style="border-left:1px solid #D0D0D0"') + '>' + L.cols[i].cells.slice(from - 1, to).map(function (c) { return cellHTML(c, g, rowH); }).join('') + '</div>';
    });
    h += '</div>';
    if (spec.showLegend) h += legendHTML(legend(B.R, L), g.narrow);
    (spec.callouts || []).forEach(function (co) {
      var top = 4 + (spec.header !== false ? 22 : 0) + (co.row - from) * rowH + (co.dy === undefined ? rowH / 2 : co.dy);
      var left = 4 + (co.col || 0) * g.colW + co.x;
      h += '<div class="co" style="left:' + (left - 6) + 'px;top:' + (top - 6) + 'px">' + co.n + '</div>';
    });
    return h + '</div>';
  }

  G.TL = { parse: parse, resolve: resolve, layout: layout, legend: legend, renderPage: renderPage, renderCrop: renderCrop, build: build, D: D, iso: iso, geometry: geometry, railSize: railSize };
})(typeof window !== 'undefined' ? window : globalThis);
