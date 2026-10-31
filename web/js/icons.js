// Minimal 16px stroke icon set (inspired by Lucide, hand-tuned for NavMini).
const P = {
  plus: '<path d="M8 3.5v9M3.5 8h9"/>',
  minus: '<path d="M3.5 8h9"/>',
  x: '<path d="M4.2 4.2l7.6 7.6M11.8 4.2l-7.6 7.6"/>',
  check: '<path d="M3.3 8.4l3 3 6.4-7"/>',
  'chevron-right': '<path d="M6 3.4L10.6 8 6 12.6"/>',
  'chevron-left': '<path d="M10 3.4L5.4 8 10 12.6"/>',
  'chevron-down': '<path d="M3.4 6L8 10.6 12.6 6"/>',
  'chevron-up': '<path d="M3.4 10L8 5.4 12.6 10"/>',
  search: '<circle cx="7" cy="7" r="4.3"/><path d="M10.2 10.2l3 3"/>',
  refresh: '<path d="M13.2 7.2A5.2 5.2 0 1 0 12 11.6"/><path d="M13.2 3.4v3.8H9.4"/>',
  settings: '<path d="M2.5 5h1.8M7.2 5h6.3M2.5 11h6.3M10.3 11h3.2"/><circle cx="5.6" cy="5" r="1.5"/><circle cx="8.8" cy="11" r="1.5"/>',
  sun: '<circle cx="8" cy="8" r="3"/><path d="M8 1.6v1.4M8 13v1.4M1.6 8H3M13 8h1.4M3.5 3.5l1 1M11.5 11.5l1 1M12.5 3.5l-1 1M4.5 11.5l-1 1"/>',
  moon: '<path d="M13 9.4A5.4 5.4 0 0 1 6.6 3 5.4 5.4 0 1 0 13 9.4z"/>',
  database: '<ellipse cx="8" cy="3.9" rx="5.1" ry="2.1"/><path d="M2.9 3.9v8.1c0 1.2 2.3 2.1 5.1 2.1s5.1-.9 5.1-2.1V3.9"/><path d="M2.9 8c0 1.2 2.3 2.1 5.1 2.1s5.1-.9 5.1-2.1"/>',
  table: '<rect x="2.2" y="3" width="11.6" height="10" rx="1.2"/><path d="M2.2 6.4h11.6M6.4 6.4V13M10.2 6.4V13"/>',
  view: '<path d="M1.6 8S3.8 3.9 8 3.9 14.4 8 14.4 8 12.2 12.1 8 12.1 1.6 8 1.6 8z"/><circle cx="8" cy="8" r="1.9"/>',
  columns: '<rect x="2.2" y="3" width="11.6" height="10" rx="1.2"/><path d="M6 3v10M10 3v10"/>',
  play: '<path d="M5 3.4l7.2 4.6L5 12.6z" fill="currentColor" stroke="none"/>',
  stop: '<rect x="4.2" y="4.2" width="7.6" height="7.6" rx="1.2" fill="currentColor" stroke="none"/>',
  save: '<path d="M3 2.8h8.2L13.2 4.8V13H3z"/><path d="M5.2 2.8v3.4h5.2V2.8M5.2 13v-3.6h5.6V13"/>',
  history: '<path d="M2.6 8a5.4 5.4 0 1 0 1.6-3.8"/><path d="M2.4 2.9v3.2h3.2"/><path d="M8 5.4V8l1.9 1.2"/>',
  trash: '<path d="M2.8 4.4h10.4M5.6 4.4V2.9h4.8v1.5M4.1 4.4l.7 8.2h6.4l.7-8.2"/><path d="M6.5 6.6v4M9.5 6.6v4"/>',
  pencil: '<path d="M3 13l.5-2.8 7-7a1.5 1.5 0 0 1 2.1 2.1l-7 7z"/><path d="M9.4 4.4l2.2 2.2"/>',
  copy: '<rect x="5.4" y="5.4" width="8" height="8" rx="1.2"/><path d="M10.6 5.4V3.6a1.2 1.2 0 0 0-1.2-1.2H3.6a1.2 1.2 0 0 0-1.2 1.2v5.8a1.2 1.2 0 0 0 1.2 1.2h1.8"/>',
  download: '<path d="M8 2.6v7.2M5.2 7.2L8 10l2.8-2.8"/><path d="M2.8 11.4v1.2a1 1 0 0 0 1 1h8.4a1 1 0 0 0 1-1v-1.2"/>',
  upload: '<path d="M8 10.2V3M5.2 5.8L8 3l2.8 2.8"/><path d="M2.8 11.4v1.2a1 1 0 0 0 1 1h8.4a1 1 0 0 0 1-1v-1.2"/>',
  terminal: '<rect x="2.2" y="3" width="11.6" height="10" rx="1.4"/><path d="M4.8 6.4L6.8 8l-2 1.6M8.4 10h2.8"/>',
  activity: '<path d="M1.8 8.4h2.6l1.6-4 2.4 8 1.6-4h3.8"/>',
  server: '<rect x="2.4" y="2.6" width="11.2" height="4.4" rx="1"/><rect x="2.4" y="9" width="11.2" height="4.4" rx="1"/><path d="M4.6 4.8h.01M4.6 11.2h.01"/>',
  filter: '<path d="M2.4 3.4h11.2L9.6 8.6v4l-3.2 1.2V8.6z"/>',
  key: '<circle cx="5.4" cy="10.6" r="2.4"/><path d="M7.3 8.7l5.4-5.4M10.6 5.4l1.3 1.3M9.2 6.8l1.3 1.3"/>',
  link: '<path d="M6.6 9.4l2.8-2.8M7.2 4.6l1-1a2.6 2.6 0 0 1 3.7 3.7l-1 1M8.8 11.4l-1 1a2.6 2.6 0 0 1-3.7-3.7l1-1"/>',
  shield: '<path d="M8 2.2l4.6 1.8v3.6c0 2.7-1.8 4.7-4.6 5.8-2.8-1.1-4.6-3.1-4.6-5.8V4z"/>',
  folder: '<path d="M2.2 4.2h4l1.2 1.4h6.4v7.2H2.2z"/>',
  file: '<path d="M4 2.6h5l3 3v7.8H4z"/><path d="M9 2.6v3h3"/>',
  more: '<circle cx="3.6" cy="8" r=".9" fill="currentColor" stroke="none"/><circle cx="8" cy="8" r=".9" fill="currentColor" stroke="none"/><circle cx="12.4" cy="8" r=".9" fill="currentColor" stroke="none"/>',
  external: '<path d="M9.2 3.2h3.6v3.6M12.6 3.4L7.4 8.6"/><path d="M11.4 9.6v2.8a1 1 0 0 1-1 1H3.6a1 1 0 0 1-1-1V5.6a1 1 0 0 1 1-1H6.4"/>',
  bolt: '<path d="M9.2 2.2L4 9h3.4l-.8 4.8L12 7H8.6z"/>',
  clock: '<circle cx="8" cy="8" r="5.6"/><path d="M8 4.8V8l2.2 1.4"/>',
  arrowup: '<path d="M8 12.4V4M4.4 7.6L8 4l3.6 3.6"/>',
  arrowdown: '<path d="M8 3.6V12M4.4 8.4L8 12l3.6-3.6"/>',
  plug: '<path d="M5.4 2.4v3.2M10.6 2.4v3.2"/><path d="M3.6 5.6h8.8v2.2a4.4 4.4 0 0 1-8.8 0z"/><path d="M8 12.2v1.6"/>',
  code: '<path d="M5.6 4.2L2.4 8l3.2 3.8M10.4 4.2L13.6 8l-3.2 3.8"/>',
  layers: '<path d="M8 2.4l5.6 2.8L8 8 2.4 5.2z"/><path d="M2.4 8.2L8 11l5.6-2.8M2.4 11.2L8 14l5.6-2.8"/>',
  book: '<path d="M2.6 3.2h4a1.6 1.6 0 0 1 1.4 1.6v8a1.4 1.4 0 0 0-1.2-1.2H2.6z"/><path d="M13.4 3.2h-4a1.6 1.6 0 0 0-1.4 1.6v8a1.4 1.4 0 0 1 1.2-1.2h4.2z"/>',
  lock: '<rect x="3.4" y="7" width="9.2" height="6.2" rx="1.2"/><path d="M5.4 7V5.2a2.6 2.6 0 0 1 5.2 0V7"/>',
  paperclip: '<path d="M11.2 7.4l-4.6 4.6a2.1 2.1 0 0 1-3-3l5.2-5.2a3.2 3.2 0 0 1 4.5 4.5l-5.2 5.2a4.2 4.2 0 0 1-5.9-5.9l4.8-4.8"/>',
  sparkles: '<path d="M6.2 2.6l.7 1.9 1.9.7-1.9.7-.7 1.9-.7-1.9-1.9-.7 1.9-.7z"/><path d="M11.4 8.2l.6 1.6 1.6.6-1.6.6-.6 1.6-.6-1.6-1.6-.6 1.6-.6z"/>',
  alert: '<path d="M8 2.6l6 10.6H2z"/><path d="M8 6.2v3.2M8 11.4h.01"/>',
  info: '<circle cx="8" cy="8" r="5.6"/><path d="M8 7.2v3.4M8 5.2h.01"/>',
  panel: '<rect x="2.2" y="3" width="11.6" height="10" rx="1.2"/><path d="M6.2 3v10"/>',
  rows: '<rect x="2.2" y="3" width="11.6" height="10" rx="1.2"/><path d="M2.2 6.4h11.6M2.2 9.6h11.6"/>',
  fx: '<path d="M4.2 13V6.2A2.2 2.2 0 0 1 6.4 4h.6"/><path d="M3 7.4h4.4"/><path d="M9 9.2l3.4 3.6M12.4 9.2L9 12.8"/>',
  diagram: '<rect x="2" y="2.4" width="4.4" height="3.4" rx=".8"/><rect x="9.6" y="2.4" width="4.4" height="3.4" rx=".8"/><rect x="5.8" y="10" width="4.4" height="3.4" rx=".8"/><path d="M4.2 5.8v2.4h7.6V5.8M8 8.2v1.8"/>',
  gridplus: '<rect x="2.2" y="3" width="11.6" height="10" rx="1.2"/><path d="M8 6v4M6 8h4"/>',
};

export function icon(name, cls = '') {
  const body = P[name] || P.file;
  return `<svg class="${cls}" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${body}</svg>`;
}

export function iconNode(name, cls = '') {
  const span = document.createElement('span');
  span.className = 'node-icon ' + cls;
  span.innerHTML = icon(name);
  return span;
}
