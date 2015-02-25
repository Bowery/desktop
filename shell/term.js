lib.rtdep('lib.f', 'lib.Storage', 'hterm')
var ipc = require('ipc')

ipc.on('canceled', function (){
  if (window.instance.exited) {
    location.reload()
  }
})

function qmark (name) {
  var queryString = {}
  window.location.href.replace(
    new RegExp("([^?=&]+)(=([^&]*))?", "g"),
    function($0, $1, $2, $3) { queryString[$1] = $3 }
  )
  return name ? queryString[name] : queryString
}

/**
 * TODO (thebyrd) id to hterm Iframe Dom stored as a string... lol
 */
window.terminals = {}

function newID() {
  return Math.floor(10000000 * Math.random()).toString(16)
}

window.newtab = function () {
  console.log('$$$$ new tab called')
  var id = newID()
  terminals[id] = new hterm.Terminal('default')
  var terminalContainer = document.querySelector('#terminal')
  var terminalEl = document.createElement('div')
  terminalContainer.appendChild(terminalEl)
  terminalEl.id = 'terminal-' + id
  terminalEl.className = 'terminal'

  console.log('$$$ add terminal to dom')
  terminals[id].decorate(terminalEl)
  terminals[id].onTerminalReady = function () {
    this.setCursorPosition(0, 0)
    this.setCursorVisible(true)
    this.runCommandClass(Instance, document.location.hash.substr(1))
  }
  window.term = terminals[id]

  // Add Tab
  console.log('$$$$ add tab')
  var tabsEl = document.querySelector('.tabs')
  var children = tabsEl.childNodes
  for (var existingTabs in children)
    children[existingTabs].className = 'tab'

  var tab = document.createElement('li')
  tab.className = 'tab selected'
  tab.id = 'tab-' + id
  var title = 'website'
  tab.innerHTML = title

  console.log('$$$$ define close tab')

  // close tab
  var x = document.createElement('button')
  x.className = 'x'
  x.innerHTML = 'x'
  x.onclick = function (e) {
    e.preventDefault()
    var id = this.parentElement.id.slice(4)
    
    console.log('$$$ remove tab, remove terminal, & remove from map of terminals', id)
    delete terminals[id]
    this.parentElement.remove()
    document.querySelector('#terminal-'+id).remove()

    var firstTerm = Object.keys(terminals)[0]
    if (firstTerm) {
      document.querySelector('#tab-'+firstTerm).className = 'tab selected'
      document.querySelector('#terminal-'+firstTerm).style.display = 'block'
    } else {
      console.log('$$$$$ TODO home screen')
    }
  }
  tab.appendChild(x)


  console.log('$$$ define tab.onclick')
  // switch tabs
  tab.onclick = function (e) {
    // if it's not a tab, don't do anything
    if (!~e.target.className.indexOf('tab'))
      return

    var terminalEls = document.querySelectorAll('.terminal')
    console.log('$$$$ hide all terminals', terminalEls)
    for (var i = 0, term; term = terminalEls[i]; i++) {
      console.log('$$$ term:', term)
      term.style.display = 'none'
    }

    var children = this.parentElement.childNodes
    for (var c in children) {
      children[c].className = 'tab'
    }

    this.className = 'tab selected'
    console.log('$$$$ show terminal', this.id)
    document.querySelector('#terminal-' + this.id.slice(4)).style.display = 'block'
  }

  console.log('$$$$ add tab to dom')
  tabsEl.appendChild(tab)
}

// Create the terminal and start an instance.
window.onload = window.newtab

window.onbeforeunload = function () {
  var conn = window.instance.conn

  if (conn) {
    conn.onclose = null
    conn.send('data: exit\r')
  }
}

// Preferences for hterm.
hterm.PreferenceManager = function (id) {
  hterm.defaultStorage = new lib.Storage.Local
  lib.PreferenceManager.call(this, hterm.defaultStorage, '/hterm/profiles/'+id)

  this.definePreferences([
    ['alt-is-meta', false],
    ['alt-sends-what', 'escape'],
    ['audible-bell-sound', ''],
    ['background-color', 'rgb(7,21,38)'],
    ['background-image', ''],
    ['background-size', ''],
    ['background-position', ''],
    ['backspace-sends-backspace', false], // Send 0x7f rather than ^H.
    ['close-on-exit', true],
    ['cursor-blink', true],
    ['cursor-color', 'rgba(255,105,81,0.5)'],
    ['color-palette-overrides', null],
    ['copy-on-select', true],
    ['enable-8-bit-control', false],
    ['enable-bold', null],
    ['enable-clipboard-notice', false],
    ['enable-clipboard-write', true],
    ['font-family', '"DejaVu Sans Mono", "Everson Mono", FreeMono, "Menlo", monospace'],
    ['font-size', 12],
    ['font-smoothing', 'antialiased'],
    ['foreground-color', 'rgb(255,255,255)'],
    ['home-keys-scroll', false],
    ['max-string-sequence', 100000],
    ['meta-sends-escape', true],
    ['mouse-cell-motion-trick', false],
    ['mouse-paste-button', null],
    ['pass-alt-number', null],
    ['pass-ctrl-number', null],
    ['pass-meta-number', null],
    ['scroll-on-keystroke', true],
    ['scroll-on-output', true],
    ['scrollbar-visible', true],
    ['shift-insert-paste', true],
    ['page-keys-scroll', false],
    ['environment', {TERM: 'xterm-256color'}]
    ])
  }

  hterm.PreferenceManager.prototype = {
    __proto__: lib.PreferenceManager.prototype
  }

  // Instance is command line instance.
  function Instance (argv) {
    this.argv = argv;
    this.termsel = document.querySelector('#terminal')
    this.environment = argv.environment || {}
    this.io = null
    this.conn = null
    this.exited = false
    this.gotErr = false
    this.restarting = false
    this.cols = 0
    this.rows = 0

    window.instance = this
  }

  // Start the instance, loading the websocket connection.
  Instance.prototype.run = function () {
    var self = this
    this.io = this.argv.io.push()
    var term = this.io.terminal_
    this.exited = false
    this.cols = term.screenSize.width
    this.rows = term.screenSize.height

    // Create websocket connection.
    var query = 'cols=' + this.cols + '&rows=' + this.rows
      + '&ip=' + qmark('ip') + '&user=' + qmark('user') + '&password=' + qmark('password')
    this.conn = new WebSocket('ws://localhost:32055/_/ssh'+'?'+query)
    this.conn.binaryType = 'arraybuffer'

    this.conn.onerror = function (err) {
      self.gotErr = true
    }

    this.conn.onclose = function (ev) {
      if (ev.reason) {
        self.gotErr = true
      }

      self.exit(0)
    }

    this.conn.onmessage = function (ev) {
      var dataView = new DataView(ev.data)
      var decoder = new TextDecoder('utf-8')

      self.io.writeUTF8(decoder.decode(dataView).slice('data: '.length))
    }

    // Handle io events.
    this.io.setTerminalProfile('default')
    this.io.onVTKeystroke = function (data) {
      self.conn && self.conn.send('data: ' + data)
    }
    this.io.sendString = function (data) {
      self.conn && self.conn.send('data: ' + data)
    }
    this.io.onTerminalResize = function (cols, rows) {
      if (!self.conn || (self.cols == cols && self.rows == rows)) {
        return
      }
      self.cols = cols
      self.rows = rows

      self.conn.send('event: resize ' + cols + ' ' + rows)
    }
  }

  // Exit the connection and cleanup.
  Instance.prototype.exit = function (code) {
    var self = this

    if (this.gotErr) {
      if (this.restarting) {
        return
      }
      this.restarting = true
      this.io.pop()

      setTimeout(function () {
        self.gotErr = false
        self.restarting = false
        self.run()
      }, 5000)
      return
    }

    if (this.exited) {
      return
    }
    this.exited = true

    this.io.pop()
    if (this.argv.onExit) {
      this.argv.onExit(code)
    }
  }
