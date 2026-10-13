// The public face of Stroppy Cloud (landing, public rating) carries the
// stroppy.io identity rather than the Grafana app theme: graphite and the blue
// of its bars; actions and progress use the app's primary colour. The app itself stays on
// GrafanaTheme2; only public pages read these.
export const brand = {
  ink: '#15171c',
  panel: '#1d2027',
  raised: '#252932',
  line: '#30353f',
  text: '#e8eaee',
  muted: '#8d95a3',
  signal: '#5b8ef0',
  signalSoft: 'rgba(91, 142, 240, 0.18)',
  good: '#4fbf86',
} as const
