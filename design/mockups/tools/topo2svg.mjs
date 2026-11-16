// Usage: node topo2svg.mjs <geo-data-dir> <out-dir>
// Builds dark-map SVGs for the Aegis Terra mockup from Natural Earth (world-atlas, public domain).
import { readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const [dataDir, outDir] = process.argv.slice(2)
const load = (f) => JSON.parse(readFileSync(join(dataDir, f), 'utf8'))

function decodeArcs(topo) {
  const t = topo.transform
  return topo.arcs.map((arc) => {
    let x = 0, y = 0
    return arc.map(([dx, dy]) => {
      x += dx; y += dy
      return t ? [x * t.scale[0] + t.translate[0], y * t.scale[1] + t.translate[1]] : [dx, dy]
    })
  })
}

function ringCoords(arcs, ring) {
  const pts = []
  for (const i of ring) {
    const a = i < 0 ? arcs[~i].slice().reverse() : arcs[i]
    pts.push(...(pts.length ? a.slice(1) : a))
  }
  return pts
}

function polygons(geom) {
  if (geom.type === 'GeometryCollection') return geom.geometries.flatMap(polygons)
  if (geom.type === 'Polygon') return [geom.arcs]
  if (geom.type === 'MultiPolygon') return geom.arcs
  return []
}

function pathFor(topo, objName, project, clip) {
  const arcs = decodeArcs(topo)
  let d = ''
  for (const poly of polygons(topo.objects[objName])) {
    for (const ring of poly) {
      const pts = ringCoords(arcs, ring)
      if (clip && !pts.some(([lon, lat]) => lon >= clip[0] && lon <= clip[2] && lat >= clip[1] && lat <= clip[3])) continue
      let prev = null
      pts.forEach(([lon, lat], k) => {
        const [x, y] = project(lon, lat)
        const jump = prev && Math.abs(lon - prev) > 180
        d += (k === 0 || jump ? 'M' : 'L') + x.toFixed(1) + ' ' + y.toFixed(1)
        prev = lon
      })
      d += 'Z'
    }
  }
  return d
}

const C = { ocean: '#060B15', land: '#101929', border: '#1D2A40', grat: '#0D1626', coast: '#1A2A42' }

// World: equirectangular, lon -180..180, lat 84..-60
{
  const W = 1440, H = 576
  const proj = (lon, lat) => [((lon + 180) / 360) * W, ((84 - lat) / 144) * H]
  const land = pathFor(load('land-110m.json'), 'land', proj)
  const borders = pathFor(load('countries-110m.json'), 'countries', proj)
  let grat = ''
  for (let lon = -150; lon <= 150; lon += 30) { const [x] = proj(lon, 0); grat += `M${x} 0V${H}` }
  for (let lat = -30; lat <= 60; lat += 30) { const [, y] = proj(0, lat); grat += `M0 ${y.toFixed(1)}H${W}` }
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}"><rect width="${W}" height="${H}" fill="${C.ocean}"/><path d="${grat}" stroke="${C.grat}" stroke-width="0.6" fill="none"/><path d="${land}" fill="${C.land}" stroke="${C.coast}" stroke-width="0.7" stroke-linejoin="round"/><path d="${borders}" fill="none" stroke="${C.border}" stroke-width="0.45" stroke-linejoin="round"/></svg>`
  writeFileSync(join(outDir, 'world-dark.svg'), svg)
  console.log('world-dark.svg', svg.length)
}

// Iberia: lon -12..5, lat 45..34.5, cos(40deg) aspect
{
  const [lon0, lon1, lat0, lat1] = [-12, 5, 45, 34.5]
  const W = 780
  const k = Math.cos((40 * Math.PI) / 180)
  const H = Math.round((W * (lat0 - lat1)) / ((lon1 - lon0) * k))
  const proj = (lon, lat) => [((lon - lon0) / (lon1 - lon0)) * W, ((lat0 - lat) / (lat0 - lat1)) * H]
  const topo = load('countries-50m.json')
  const clip = [-20, 28, 15, 52]
  const all = pathFor(topo, 'countries', proj, clip)
  // Portugal highlighted (ISO numeric 620)
  const pt = { ...topo, objects: { pt: { type: 'GeometryCollection', geometries: topo.objects.countries.geometries.filter((g) => g.id === '620') } } }
  const ptPath = pathFor(pt, 'pt', proj, clip)
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${W} ${H}" width="${W}" height="${H}"><rect width="${W}" height="${H}" fill="${C.ocean}"/><path d="${all}" fill="${C.land}" stroke="${C.border}" stroke-width="0.8" stroke-linejoin="round"/><path d="${ptPath}" fill="#14223A" stroke="#52B6FF" stroke-opacity="0.55" stroke-width="1.2" stroke-linejoin="round"/></svg>`
  writeFileSync(join(outDir, 'iberia-dark.svg'), svg)
  console.log('iberia-dark.svg', svg.length, 'H', H)
}
