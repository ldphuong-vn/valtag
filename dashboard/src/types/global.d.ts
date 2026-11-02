export {}

declare global {
  interface WindowEventMap {
    'valtag:project-changed': Event
  }
}
