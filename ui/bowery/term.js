lib.rtdep('lib.f', 'lib.Storage', 'hterm')

function qmark (name) {
    var queryString = {}
    window.location.href.replace(
      new RegExp("([^?=&]+)(=([^&]*))?", "g"),
      function($0, $1, $2, $3) { queryString[$1] = $3 }
    )
    return name ? queryString[name] : queryString
  }

// Create the terminal and start an instance.
window.onload = function () {
  var terminal = new hterm.Terminal('default')
  terminal.decorate(document.querySelector('#terminal'))
  terminal.onTerminalReady = function () {
    terminal.setCursorPosition(0, 0)
    terminal.setCursorVisible(true)
    terminal.runCommandClass(Instance, document.location.hash.substr(1))
  }

  window.term = terminal
}

window.onbeforeunload = function () {
  window.instance.conn && window.instance.conn.send('exit\r')
}

// Preferences for hterm.
hterm.PreferenceManager = function (id) {
  hterm.defaultStorage = new lib.Storage.Local
  lib.PreferenceManager.call(this, hterm.defaultStorage, '/hterm/profiles/'+id)

  this.definePreferences([
    ['alt-is-meta', false],
    ['alt-sends-what', 'escape'],
    ['audible-bell-sound', ''],
    ['background-color', 'rgb(16,16,16)'],
    ['background-image', ''],
    ['background-size', ''],
    ['background-position', ''],
    ['backspace-sends-backspace', false], // Send 0x7f rather than ^H.
    ['close-on-exit', true],
    ['cursor-blink', true],
    ['cursor-color', 'rgba(255,0,0,0.5)'],
    ['color-palette-overrides', null],
    ['copy-on-select', true],
    ['enable-8-bit-control', false],
    ['enable-bold', null],
    ['enable-clipboard-notice', false],
    ['enable-clipboard-write', true],
    ['font-family', ('"DejaVu Sans Mono", "Everson Mono", FreeMono, "Menlo", ' +
                    '"Terminal", monospace')],
    ['font-size', 15],
    ['font-smoothing', 'antialiased'],
    ['foreground-color', 'rgb(240,240,250)'],
    ['home-keys-scroll', false],
    ['max-string-sequence', 100000],
    ['meta-sends-escape', true],
    ['mouse-cell-motion-trick', false],
    ['mouse-paste-button', null],
    ['pass-alt-number', null],
    ['pass-ctrl-number', null],
    ['pass-meta-number', null],
    ['scroll-on-keystroke', true],
    ['scroll-on-output', false],
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
  this.user = ''
  this.addr = ''
  this.pass = ''
  this.io = null
  this.conn = null
  this.exited = false
  this.gotErr = false
  this.restarting = false

  window.instance = this
}

// Start the instance, loading the websocket connection.
Instance.prototype.run = function () {
  var self = this
  this.io = this.argv.io.push()
  var term = this.io.terminal_
  this.exited = false

  // Create websocket connection.
  var query = 'cols='+term.screenSize.width+'&rows='+term.screenSize.height
  this.conn = new WebSocket('ws://localhost:32055/ssh/'+qmark('appId')+'?'+query)
  this.conn.binaryType = 'arraybuffer'

  this.conn.onopen = function () {
    self.conn.send('cd ' + qmark('remotePath') + '\n')
    if (qmark('logs') == 'true') {
      var path = '/home/'+qmark('user')+'/.bowery/log/'+qmark('appId')+'-std'
      self.conn.send('tail -f '+path+'out.log '+path+'err.log\n')
    }
  }

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

    self.io.writeUTF8(decoder.decode(dataView))
  }

  // Handle io events.
  this.io.setTerminalProfile('default')
  this.io.onVTKeystroke = function (data) {
    self.conn && self.conn.send(data)
  }
  this.io.sendString = function (data) {
    self.conn && self.conn.send(data)
  }
  this.termsel.focus()
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
