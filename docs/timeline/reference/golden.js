// Writes docs/timeline/examples/<fixture>.expected.json — the reference renderer's resolved dates and layout decisions.
// Usage: node golden.js
const fs = require('fs'), path = require('path');
require('./engine.js');
const TL = globalThis.TL;
const EX = path.join(__dirname, '..', 'examples');
const H1 = { start: '2027-01-01', end: '2027-05-31' }, YEAR = { start: '2027-01-01', end: '2027-12-31' };
const CASES = [
  ['exams', { start: '2026-11-11', end: '2027-03-10' }],
  ['release', H1], ['release', Object.assign({ layout: 'flow' }, H1), 'release.flow'],
  ['release', Object.assign({ layout: 'flow', compact: 'weekly' }, H1), 'release.flow-weekly'],
  ['year-plan', { start: '2026-11-11', end: '2027-11-10' }],
  ['leave-2027', YEAR],
  ['x01-marks', H1], ['x02-colors', H1], ['x03-countdown', H1], ['x04-anchor', H1], ['x05-interval', H1],
  ['x07-continuation', H1], ['x08-dashed', H1], ['x09-hatch', H1], ['x10-back', H1], ['x11-overlap', H1],
  ['x12-keys', H1], ['x13-adaptive', Object.assign({ cap: 8 }, H1)], ['x14-overflow', H1],
  ['x15-recurring-milestone', H1], ['x16-recurring-interval', H1], ['x17-gap', H1], ['x18-monthly', H1],
  ['x19-overlapping-series', H1], ['x20-groups', YEAR], ['x20-groups', Object.assign({ mergeGroups: 'off' }, YEAR), 'x20-groups.unmerged'],
  ['x23-clipping', { start: '2027-01-15', end: '2027-06-14' }],
];
const iso = TL.iso, DAY = 86400000;
for (const [file, opts, outName] of CASES) {
  const spec = Object.assign({ today: '2026-11-11', layout: 'packed', dates: fs.readFileSync(path.join(EX, file + '.dates'), 'utf8'), file: file + '.dates' }, opts);
  const B = TL.build(spec);
  const out = {
    fixture: file + '.dates', options: Object.assign({}, opts, { today: '2026-11-11' }),
    resolved: {
      milestones: B.R.all.ms.map(m => ({ id: m.decl.id, date: iso(m.t), occurrence: m.series ? m.idx + '/' + m.n : undefined, hidden: m.hidden || undefined })),
      intervals: B.R.all.iv.map(x => ({ id: x.decl.id || null, key: x.key, start: iso(x.s), endExclusive: iso(x.e + DAY), lastDrawnDay: iso(x.e), occurrence: x.series ? x.idx + '/' + x.n : undefined }))
    },
    layout: {
      geometry: B.L.g.narrow ? 'narrow' : 'wide', laneCap: B.L.cap, groupsMerged: B.L.merge, layout: B.L.flow ? 'flow' : 'packed',
      columns: B.L.cols.map(c => {
        const labels = [], overflowDays = [];
        c.cells.forEach(cell => {
          if (!cell.inPeriod) return;
          const d = cell.num;
          if (cell.top && cell.top.length) labels.push({ day: d, items: cell.top.map(p => p.txt) });
          if (cell.ms) labels.push({ day: d, milestone: cell.ms.glyph + (cell.ms.title ? ' ' + cell.ms.title : '') });
        });
        return { month: c.name, railLanes: c.lanes, labelMode: c.labelMode, labels };
      })
    }
  };
  fs.writeFileSync(path.join(EX, (outName || file) + '.expected.json'), JSON.stringify(out, null, 1));
  console.log('ok', outName || file);
}
