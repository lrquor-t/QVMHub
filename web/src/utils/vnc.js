// X11 Keysyms — @novnc/novnc 1.7.0 包 exports 限制子路径导入，因此内联所需常量
const Keysyms = {
  XK_BackSpace:     0xff08,
  XK_Tab:           0xff09,
  XK_Return:        0xff0d,
  XK_Escape:        0xff1b,
  XK_Delete:        0xffff,
  XK_F1:            0xffbe,
  XK_F2:            0xffbf,
  XK_Shift_L:       0xffe1,
  XK_Control_L:     0xffe3,
  XK_Meta_L:        0xffe7,
  XK_Alt_L:         0xffe9,
  XK_space:         0x0020,
  XK_exclam:        0x0021,
  XK_quotedbl:      0x0022,
  XK_numbersign:    0x0023,
  XK_dollar:        0x0024,
  XK_percent:       0x0025,
  XK_ampersand:     0x0026,
  XK_apostrophe:    0x0027,
  XK_parenleft:     0x0028,
  XK_parenright:    0x0029,
  XK_asterisk:      0x002a,
  XK_plus:          0x002b,
  XK_comma:         0x002c,
  XK_minus:         0x002d,
  XK_period:        0x002e,
  XK_slash:         0x002f,
  XK_colon:         0x003a,
  XK_semicolon:     0x003b,
  XK_less:          0x003c,
  XK_equal:         0x003d,
  XK_greater:       0x003e,
  XK_question:      0x003f,
  XK_at:            0x0040,
  XK_bracketleft:   0x005b,
  XK_backslash:     0x005c,
  XK_bracketright:  0x005d,
  XK_asciicircum:   0x005e,
  XK_underscore:    0x005f,
  XK_grave:         0x0060,
  XK_braceleft:     0x007b,
  XK_bar:           0x007c,
  XK_braceright:    0x007d,
  XK_asciitilde:    0x007e,
}

const createKey = (keysym, code) => ({ keysym, code })

const CONTROL_LEFT = createKey(Keysyms.XK_Control_L, 'ControlLeft')
const SHIFT_LEFT = createKey(Keysyms.XK_Shift_L, 'ShiftLeft')
const ALT_LEFT = createKey(Keysyms.XK_Alt_L, 'AltLeft')
const META_LEFT = createKey(Keysyms.XK_Meta_L, 'MetaLeft')
const DELETE_KEY = createKey(Keysyms.XK_Delete, 'Delete')
const ESCAPE_KEY = createKey(Keysyms.XK_Escape, 'Escape')
const TAB_KEY = createKey(Keysyms.XK_Tab, 'Tab')
const BACKSPACE_KEY = createKey(Keysyms.XK_BackSpace, 'Backspace')
const F1_KEY = createKey(Keysyms.XK_F1, 'F1')
const F2_KEY = createKey(Keysyms.XK_F2, 'F2')
const ENTER_KEY = createKey(Keysyms.XK_Return, 'Enter')
const SPACE_KEY = createKey(Keysyms.XK_space, 'Space')
const MINUS_KEY = createKey(Keysyms.XK_minus, 'Minus')
const EQUAL_KEY = createKey(Keysyms.XK_equal, 'Equal')
const LEFT_BRACKET_KEY = createKey(Keysyms.XK_bracketleft, 'BracketLeft')
const RIGHT_BRACKET_KEY = createKey(Keysyms.XK_bracketright, 'BracketRight')
const BACKSLASH_KEY = createKey(Keysyms.XK_backslash, 'Backslash')
const SEMICOLON_KEY = createKey(Keysyms.XK_semicolon, 'Semicolon')
const APOSTROPHE_KEY = createKey(Keysyms.XK_apostrophe, 'Quote')
const COMMA_KEY = createKey(Keysyms.XK_comma, 'Comma')
const PERIOD_KEY = createKey(Keysyms.XK_period, 'Period')
const SLASH_KEY = createKey(Keysyms.XK_slash, 'Slash')
const BACKQUOTE_KEY = createKey(Keysyms.XK_grave, 'Backquote')
const R_KEY = createKey('r'.codePointAt(0), 'KeyR')

const shortcutMap = {
  ctrlAltDel: {
    id: 'ctrlAltDel',
    label: 'Ctrl+Alt+Del',
    sequence: [CONTROL_LEFT, ALT_LEFT, DELETE_KEY]
  },
  ctrlShiftEsc: {
    id: 'ctrlShiftEsc',
    label: 'Ctrl+Shift+Esc',
    sequence: [CONTROL_LEFT, SHIFT_LEFT, ESCAPE_KEY]
  },
  altTab: {
    id: 'altTab',
    label: 'Alt+Tab',
    sequence: [ALT_LEFT, TAB_KEY]
  },
  winR: {
    id: 'winR',
    label: 'Win+R',
    sequence: [META_LEFT, R_KEY]
  },
  ctrlAltF1: {
    id: 'ctrlAltF1',
    label: 'Ctrl+Alt+F1',
    sequence: [CONTROL_LEFT, ALT_LEFT, F1_KEY]
  },
  ctrlAltF2: {
    id: 'ctrlAltF2',
    label: 'Ctrl+Alt+F2',
    sequence: [CONTROL_LEFT, ALT_LEFT, F2_KEY]
  },
  ctrlAltBackspace: {
    id: 'ctrlAltBackspace',
    label: 'Ctrl+Alt+Backspace',
    sequence: [CONTROL_LEFT, ALT_LEFT, BACKSPACE_KEY]
  }
}

const wait = (ms) => new Promise((resolve) => {
  window.setTimeout(resolve, ms)
})

export const COMMON_VNC_SHORTCUTS = [
  shortcutMap.ctrlShiftEsc,
  shortcutMap.altTab,
  shortcutMap.winR,
  shortcutMap.ctrlAltF1,
  shortcutMap.ctrlAltF2,
  shortcutMap.ctrlAltBackspace
]

export const PRIMARY_VNC_SHORTCUT = shortcutMap.ctrlAltDel

export function buildVncWsUrl(vmName, token) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  return `${protocol}//${host}/api/vm/${vmName}/vnc/ws?token=${encodeURIComponent(token)}`
}

export function refreshVncViewport(rfb) {
  if (!rfb) {
    return
  }

  rfb.scaleViewport = false
  rfb.scaleViewport = true
}

export function sendVncShortcut(rfb, shortcut) {
  if (!rfb || !shortcut?.sequence?.length) {
    return false
  }

  if (typeof rfb.focus === 'function') {
    rfb.focus()
  }

  const modifiers = shortcut.sequence.slice(0, -1)
  const primaryKey = shortcut.sequence[shortcut.sequence.length - 1]

  modifiers.forEach((key) => {
    rfb.sendKey(key.keysym, key.code, true)
  })

  rfb.sendKey(primaryKey.keysym, primaryKey.code, true)
  rfb.sendKey(primaryKey.keysym, primaryKey.code, false)

  modifiers.slice().reverse().forEach((key) => {
    rfb.sendKey(key.keysym, key.code, false)
  })

  return true
}

function sendKeyStroke(rfb, key, modifiers = []) {
  modifiers.forEach((modifier) => {
    rfb.sendKey(modifier.keysym, modifier.code, true)
  })

  rfb.sendKey(key.keysym, key.code, true)
  rfb.sendKey(key.keysym, key.code, false)

  modifiers.slice().reverse().forEach((modifier) => {
    rfb.sendKey(modifier.keysym, modifier.code, false)
  })
}

function dispatchTextKeyEvent(rfb, keysym, code, down, capslock = false) {
  if (typeof rfb?._handleKeyEvent === 'function' && code) {
    rfb._handleKeyEvent(keysym, code, down, null, capslock)
    return
  }

  rfb.sendKey(keysym, code || null, down)
}

function sendTextKeyStroke(rfb, key, modifiers = [], capslock = false) {
  modifiers.forEach((modifier) => {
    dispatchTextKeyEvent(rfb, modifier.keysym, modifier.code, true, capslock)
  })

  dispatchTextKeyEvent(rfb, key.keysym, key.code, true, capslock)
  dispatchTextKeyEvent(rfb, key.keysym, key.code, false, capslock)

  modifiers.slice().reverse().forEach((modifier) => {
    dispatchTextKeyEvent(rfb, modifier.keysym, modifier.code, false, capslock)
  })
}

function resolveTextCharStroke(char) {
  if (char === '\n') {
    return { key: ENTER_KEY }
  }

  if (char === '\t') {
    return { key: TAB_KEY }
  }

  if (char === ' ') {
    return { key: SPACE_KEY }
  }

  if (char >= 'a' && char <= 'z') {
    return {
      key: createKey(char.codePointAt(0), `Key${char.toUpperCase()}`)
    }
  }

  if (char >= 'A' && char <= 'Z') {
    return {
      key: createKey(char.codePointAt(0), `Key${char}`),
      modifiers: [SHIFT_LEFT]
    }
  }

  if (char >= '0' && char <= '9') {
    return {
      key: createKey(char.codePointAt(0), `Digit${char}`)
    }
  }

  const symbolMap = {
    '!': { key: createKey(Keysyms.XK_exclam, 'Digit1'), modifiers: [SHIFT_LEFT] },
    '@': { key: createKey(Keysyms.XK_at, 'Digit2'), modifiers: [SHIFT_LEFT] },
    '#': { key: createKey(Keysyms.XK_numbersign, 'Digit3'), modifiers: [SHIFT_LEFT] },
    '$': { key: createKey(Keysyms.XK_dollar, 'Digit4'), modifiers: [SHIFT_LEFT] },
    '%': { key: createKey(Keysyms.XK_percent, 'Digit5'), modifiers: [SHIFT_LEFT] },
    '^': { key: createKey(Keysyms.XK_asciicircum, 'Digit6'), modifiers: [SHIFT_LEFT] },
    '&': { key: createKey(Keysyms.XK_ampersand, 'Digit7'), modifiers: [SHIFT_LEFT] },
    '(': { key: createKey(Keysyms.XK_parenleft, 'Digit9'), modifiers: [SHIFT_LEFT] },
    ')': { key: createKey(Keysyms.XK_parenright, 'Digit0'), modifiers: [SHIFT_LEFT] },
    '*': { key: createKey(Keysyms.XK_asterisk, 'Digit8'), modifiers: [SHIFT_LEFT] },
    '_': { key: createKey(Keysyms.XK_underscore, 'Minus'), modifiers: [SHIFT_LEFT] },
    '-': { key: MINUS_KEY },
    '+': { key: createKey(Keysyms.XK_plus, 'Equal'), modifiers: [SHIFT_LEFT] },
    '=': { key: EQUAL_KEY },
    '[': { key: LEFT_BRACKET_KEY },
    ']': { key: RIGHT_BRACKET_KEY },
    '{': { key: createKey(Keysyms.XK_braceleft, 'BracketLeft'), modifiers: [SHIFT_LEFT] },
    '}': { key: createKey(Keysyms.XK_braceright, 'BracketRight'), modifiers: [SHIFT_LEFT] },
    '\\': { key: BACKSLASH_KEY },
    '|': { key: createKey(Keysyms.XK_bar, 'Backslash'), modifiers: [SHIFT_LEFT] },
    ';': { key: SEMICOLON_KEY },
    ':': { key: createKey(Keysyms.XK_colon, 'Semicolon'), modifiers: [SHIFT_LEFT] },
    '\'': { key: APOSTROPHE_KEY },
    '"': { key: createKey(Keysyms.XK_quotedbl, 'Quote'), modifiers: [SHIFT_LEFT] },
    ',': { key: COMMA_KEY },
    '<': { key: createKey(Keysyms.XK_less, 'Comma'), modifiers: [SHIFT_LEFT] },
    '.': { key: PERIOD_KEY },
    '>': { key: createKey(Keysyms.XK_greater, 'Period'), modifiers: [SHIFT_LEFT] },
    '?': { key: createKey(Keysyms.XK_question, 'Slash'), modifiers: [SHIFT_LEFT] },
    '/': { key: SLASH_KEY },
    '`': { key: BACKQUOTE_KEY },
    '~': { key: createKey(Keysyms.XK_asciitilde, 'Backquote'), modifiers: [SHIFT_LEFT] }
  }

  if (symbolMap[char]) {
    return symbolMap[char]
  }

  if (char.length === 1) {
    return {
      keysym: char.codePointAt(0)
    }
  }

  return null
}

export async function sendTextToVnc(rfb, text, options = {}) {
  if (!rfb || !text) {
    return false
  }

  const charDelay = typeof options.charDelay === 'number' ? options.charDelay : 18

  if (typeof rfb.focus === 'function') {
    rfb.focus()
  }

  const normalizedText = String(text).replace(/\r\n/g, '\n').replace(/\r/g, '\n')

  for (const char of normalizedText) {
    const stroke = resolveTextCharStroke(char)

    if (stroke?.key) {
      sendTextKeyStroke(rfb, stroke.key, stroke.modifiers || [])
    } else if (stroke?.keysym) {
      rfb.sendKey(stroke.keysym, null)
    } else {
      rfb.sendKey(char.codePointAt(0), null)
    }

    if (charDelay > 0) {
      await wait(charDelay)
    }
  }

  return true
}
